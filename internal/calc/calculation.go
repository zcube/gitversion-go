package calc

import (
	"fmt"
	"strings"
	"time"

	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/output"
	"github.com/zcube/gitversion-go/internal/rx"
	"github.com/zcube/gitversion-go/internal/semrel"
	v "github.com/zcube/gitversion-go/internal/version"
	"github.com/zcube/gitversion-go/internal/workflow"
)

// baseVersion 은 한 전략이 만들어 낸 base version 후보.
type baseVersion struct {
	source            string
	semanticVersion   v.SemanticVersion
	baseVersionSource *string
	sourceWhen        *time.Time
	increment         v.VersionField
	label             *string
	forceIncrement    bool
	exact             bool
}

func newBaseVersion(source string, sv v.SemanticVersion, baseSrc *string, inc v.VersionField, label *string) baseVersion {
	return baseVersion{source: source, semanticVersion: sv, baseVersionSource: baseSrc, increment: inc, label: label}
}

// nextVersion 은 후보에 증분을 적용한 결과.
type nextVersion struct {
	incremented v.SemanticVersion
	base        baseVersion
}

func strategyToField(s config.IncrementStrategy) v.VersionField {
	switch s {
	case config.IncrementMajor:
		return v.FieldMajor
	case config.IncrementMinor:
		return v.FieldMinor
	case config.IncrementPatch:
		return v.FieldPatch
	default:
		return v.FieldNone
	}
}

// incrementFromMessage 는 단일 커밋 메시지에서 bump 필드 추출. 매칭 없으면 (None,false).
func incrementFromMessage(msg string, eff *config.EffectiveConfiguration) (v.VersionField, bool) {
	test := func(pat string) bool {
		re, err := rx.CompileCached("(?im)" + pat)
		if err != nil {
			return false
		}
		return re.MatchString(msg)
	}
	switch {
	case test(eff.MajorBumpMessage):
		return v.FieldMajor, true
	case test(eff.MinorBumpMessage):
		return v.FieldMinor, true
	case test(eff.PatchBumpMessage):
		return v.FieldPatch, true
	case test(eff.NoBumpMessage):
		return v.FieldNone, true
	default:
		return v.FieldNone, false
	}
}

// determineIncrement: base_source(제외)~head 사이 커밋들을 보고 증분 필드 결정.
func determineIncrement(repo *git.GitRepo, baseSource *string, headSha string, shouldIncrement bool, eff *config.EffectiveConfiguration, ig ignoreSet) (v.VersionField, error) {
	defaultIncrement := strategyToField(eff.Increment)

	var messageIncrement *v.VersionField
	if eff.CommitMessageIncrementing != config.CommitMsgDisabled {
		commits, err := repo.CommitsBetween(baseSource, headSha)
		if err != nil {
			return v.FieldNone, err
		}
		commits = ig.filter(repo, commits)
		mergeOnly := eff.CommitMessageIncrementing == config.CommitMsgMergeMessageOnly
		var best *v.VersionField
		for i := range commits {
			c := &commits[i]
			if mergeOnly && c.ParentCount < 2 {
				continue
			}
			if f, ok := incrementFromMessage(c.Message, eff); ok {
				if best == nil {
					ff := f
					best = &ff
				} else {
					m := best.Max(f)
					best = &m
				}
			}
		}
		messageIncrement = best
	}

	if messageIncrement == nil {
		if shouldIncrement {
			return defaultIncrement, nil
		}
		return v.FieldNone, nil
	}
	mi := *messageIncrement
	if shouldIncrement && mi < defaultIncrement {
		return defaultIncrement, nil
	}
	return mi, nil
}

func parseVersion(input string, eff *config.EffectiveConfiguration) (v.SemanticVersion, bool) {
	strict := eff.SemanticVersionFormat == config.FormatStrict
	return v.ParseWith(input, eff.TagPrefix, strict)
}

// validateConfigRegexes: 잘못된 정규식 설정을 원본처럼 계산 에러로 처리한다.
func validateConfigRegexes(eff *config.EffectiveConfiguration) error {
	check := func(label, pat string) error {
		if !rx.IsValid(pat) {
			return fmt.Errorf("Invalid %s regex '%s'", label, pat)
		}
		return nil
	}
	if err := check("tag-prefix", eff.TagPrefix); err != nil {
		return err
	}
	if eff.CommitMessageIncrementing != config.CommitMsgDisabled {
		for _, p := range [][2]string{
			{"major-version-bump-message", eff.MajorBumpMessage},
			{"minor-version-bump-message", eff.MinorBumpMessage},
			{"patch-version-bump-message", eff.PatchBumpMessage},
			{"no-bump-message", eff.NoBumpMessage},
		} {
			if err := check(p[0], p[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractVersion: 메시지/브랜치명에서 버전 토큰 추출(원본 ReferenceNameExtensions).
func extractVersion(text string, eff *config.EffectiveConfiguration) (v.SemanticVersion, bool) {
	pattern := "(?i)^" + strings.TrimPrefix(eff.VersionInBranchPattern, "^")
	re, err := rx.CompileCached(pattern)
	if err != nil {
		return v.SemanticVersion{}, false
	}
	var sep byte = '/'
	if !strings.Contains(text, "/") && strings.Contains(text, "-") {
		sep = '-'
	}
	for _, part := range strings.Split(text, string(sep)) {
		if part == "" {
			continue
		}
		m, ok := re.Find(part)
		if !ok {
			continue
		}
		raw, ok := m.Named("version")
		if !ok {
			raw = m.Whole()
		}
		if sv, ok := parseVersion(raw, eff); ok {
			return sv, true
		}
	}
	return v.SemanticVersion{}, false
}

// resolveInheritViaGit: Inherit 증분을 git 조상 기반으로 해석.
func resolveInheritViaGit(repo *git.GitRepo, cfg *config.GitVersionConfiguration, branchName string) (*config.IncrementStrategy, error) {
	_, bc, ok := config.FindBranchConfig(cfg, branchName)
	if !ok {
		return nil, nil
	}
	own := config.IncrementInherit
	if bc.Increment != nil {
		own = *bc.Increment
	} else if cfg.Increment != nil {
		own = *cfg.Increment
	}
	if own != config.IncrementInherit {
		return nil, nil
	}

	repoBranches, _ := repo.BranchNames()
	var bestDepth int64 = -1
	var bestInc config.IncrementStrategy
	found := false

	for _, srcKey := range bc.SourceBranches {
		srcBc, ok := cfg.Branches[srcKey]
		if !ok || srcBc.Regex == nil {
			continue
		}
		re, err := rx.CompileCached("(?i)" + *srcBc.Regex)
		if err != nil {
			continue
		}
		for _, rb := range repoBranches {
			if rb == branchName {
				continue
			}
			short := shortName(rb)
			if !(re.MatchString(rb) || re.MatchString(short)) {
				continue
			}
			mb, err := repo.MergeBase(branchName, rb)
			if err != nil || mb == nil {
				continue
			}
			commits, err := repo.CommitsBetween(nil, *mb)
			if err != nil {
				continue
			}
			depth := int64(len(commits))
			inc := config.ResolveIncrement(cfg, srcBc, 0)
			if !found || depth > bestDepth {
				bestDepth = depth
				bestInc = inc
				found = true
			}
		}
	}
	if !found {
		return nil, nil
	}
	return &bestInc, nil
}

func shortName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// builtinMergeFormats: 내장 merge 메시지 포맷(원본 MergeMessage.cs).
var builtinMergeFormats = []string{
	`^Merge (branch|tag) '(?<SourceBranch>[^']*)'(?: into (?<TargetBranch>[^\s]*))*`,
	`^Finish (?<SourceBranch>[^\s]*)(?: into (?<TargetBranch>[^\s]*))*`,
	`^Merge pull request #(?<PullRequestNumber>\d+) (from|in) (?<Source>.*) from (?<SourceBranch>[^\s]*) to (?<TargetBranch>[^\s]*)`,
	`^Pull request #(?<PullRequestNumber>\d+)[^\r\n]*\r?\n\r?\nMerge in (?<Source>[^\r\n]*) from (?<SourceBranch>[^\s]*) to (?<TargetBranch>[^\s]*)`,
	`^Merged in (?<SourceBranch>[^\s]*) \(pull request #(?<PullRequestNumber>\d+)\)`,
	`^Merge pull request #(?<PullRequestNumber>\d+) (from|in) (?:[^\s/]+/)?(?<SourceBranch>[^\s]*)(?: into (?<TargetBranch>[^\s]*))*`,
	`^Merge remote-tracking branch '(?<SourceBranch>[^\s]*)'(?: into (?<TargetBranch>[^\s]*))*`,
	`^Merge pull request (?<PullRequestNumber>\d+) from (?<SourceBranch>[^\s]*) into (?<TargetBranch>[^\s]*)`,
}

// parseMergeMessage: merge 메시지를 파싱해 (병합된 브랜치명, 추출된 버전)을 반환.
func parseMergeMessage(message string, eff *config.EffectiveConfiguration) (string, v.SemanticVersion, bool) {
	fromBranch := func(sb string) (v.SemanticVersion, bool) {
		if sv, ok := parseVersion(sb, eff); ok {
			return sv, true
		}
		return extractVersion(sb, eff)
	}

	patterns := append([]string(nil), sortedMergeFormatValues(eff)...)
	patterns = append(patterns, builtinMergeFormats...)
	for _, pattern := range patterns {
		re, err := rx.CompileCached("(?s)" + pattern)
		if err != nil {
			continue
		}
		m, ok := re.Find(message)
		if !ok {
			continue
		}
		branch, ok := m.Named("SourceBranch")
		if !ok {
			continue
		}
		if vs, ok := m.Named("Version"); ok {
			if sv, ok := parseVersion(vs, eff); ok {
				return branch, sv, true
			}
		}
		if sv, ok := fromBranch(branch); ok {
			return branch, sv, true
		}
		return "", v.SemanticVersion{}, false // 포맷은 맞지만 버전 없음.
	}
	return "", v.SemanticVersion{}, false
}

func sortedMergeFormatValues(eff *config.EffectiveConfiguration) []string {
	keys := make([]string, 0, len(eff.MergeMessageFormats))
	for k := range eff.MergeMessageFormats {
		keys = append(keys, k)
	}
	// 키 정렬(원본 BTreeMap 순회).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, eff.MergeMessageFormats[k])
	}
	return out
}

// mergeBranchIncrement: 병합 커밋 메시지에서 병합된 브랜치명을 추출해 그 설정 증분을 반환.
// 반환: (field, hasValue). field==FieldNone & hasValue=true 는 when_branch_merged 강제 no-op.
func mergeBranchIncrement(cfg *config.GitVersionConfiguration, message string) (v.VersionField, bool) {
	for _, pattern := range builtinMergeFormats {
		re, err := rx.CompileCached("(?s)" + pattern)
		if err != nil {
			continue
		}
		m, ok := re.Find(message)
		if !ok {
			continue
		}
		branch, ok := m.Named("SourceBranch")
		if !ok {
			continue
		}
		_, bc, ok := config.FindBranchConfig(cfg, branch)
		if !ok {
			return v.FieldNone, false
		}
		if bc.PreventIncrement != nil && bc.PreventIncrement.WhenBranchMerged != nil && *bc.PreventIncrement.WhenBranchMerged {
			return v.FieldNone, true
		}
		increment := config.IncrementInherit
		if bc.Increment != nil {
			increment = *bc.Increment
		}
		if increment == config.IncrementInherit || increment == config.IncrementNone {
			return v.FieldNone, false
		}
		return strategyToField(increment), true
	}
	return v.FieldNone, false
}

// isSemanticWorkflow: workflow 가 semantic-release 계열인지.
func isSemanticWorkflow(cfg *config.GitVersionConfiguration) bool {
	return cfg.Workflow != nil && strings.Contains(strings.ToLower(*cfg.Workflow), "semantic")
}

// isReleaseBranch: 브랜치명이 release 브랜치 설정(is-release-branch)에 매칭되는지.
func isReleaseBranch(cfg *config.GitVersionConfiguration, branchName string) bool {
	short := shortName(branchName)
	for _, key := range cfg.SortedBranchKeys() {
		bc := cfg.Branches[key]
		if bc.IsReleaseBranch == nil || !*bc.IsReleaseBranch || bc.Regex == nil {
			continue
		}
		re, err := rx.CompileCached("(?i)" + *bc.Regex)
		if err != nil {
			continue
		}
		if re.MatchString(branchName) || re.MatchString(short) {
			return true
		}
	}
	return false
}

// Calculate 는 전체 계산 진입점. 공통 컨텍스트(HEAD/브랜치/effective 설정)를 해석한 뒤
// 워크플로에 맞는 Calculator(GitVersion 계열 또는 SemanticRelease)를 선택해 위임한다.
func Calculate(repo *git.GitRepo, cfg *config.GitVersionConfiguration, branchOverride *string) (*output.VersionVariables, error) {
	var head git.CommitInfo
	var branchName string
	if branchOverride != nil {
		if ci, ok := repo.CommitInfoOf(*branchOverride); ok {
			head = ci
		} else {
			h, err := repo.HeadCommit()
			if err != nil {
				return nil, err
			}
			head = h
		}
		branchName = *branchOverride
	} else {
		h, err := repo.HeadCommit()
		if err != nil {
			return nil, err
		}
		head = h
		bn, err := repo.CurrentBranchName()
		if err != nil {
			return nil, err
		}
		branchName = bn
	}

	eff := config.ResolveEffective(cfg, branchName)
	if err := validateConfigRegexes(&eff); err != nil {
		return nil, err
	}

	ctx := &workflow.Context{Repo: repo, Config: cfg, Eff: &eff, Branch: branchName, Head: head}
	return selectCalculator(cfg).Calculate(ctx)
}

// selectCalculator 는 워크플로에 맞는 계산기를 고른다.
func selectCalculator(cfg *config.GitVersionConfiguration) workflow.Calculator {
	if isSemanticWorkflow(cfg) {
		return semrel.Calculator{}
	}
	return gitVersionCalculator{}
}

// gitVersionCalculator 는 GitVersion 계열(GitFlow/GitHubFlow/TrunkBased/Mainline) 계산기.
type gitVersionCalculator struct{}

func (gitVersionCalculator) Calculate(ctx *workflow.Context) (*output.VersionVariables, error) {
	repo, cfg, branchName, head := ctx.Repo, ctx.Config, ctx.Branch, ctx.Head
	eff := *ctx.Eff

	ig := ignoreFromConfig(cfg)

	if containsStrategy(cfg.Strategies, config.StrategyMainline) {
		return mainlineCalculate(repo, cfg, &eff, branchName, &head, ig)
	}

	if inc, err := resolveInheritViaGit(repo, cfg, branchName); err != nil {
		return nil, err
	} else if inc != nil {
		eff.Increment = *inc
	}

	var candidates []baseVersion
	var tagAlternatives []v.SemanticVersion
	strategies := cfg.Strategies
	if len(strategies) == 0 {
		strategies = []config.VersionStrategy{
			config.StrategyFallback, config.StrategyConfiguredNextVersion,
			config.StrategyMergeMessage, config.StrategyTaggedCommit,
			config.StrategyVersionInBranchName,
		}
	}

	for _, strat := range strategies {
		switch strat {
		case config.StrategyConfiguredNextVersion:
			if eff.NextVersion != nil && *eff.NextVersion != "" {
				nv := *eff.NextVersion
				sv, ok := parseVersion(nv, &eff)
				if !ok {
					return nil, fmt.Errorf("Failed to parse %s into a Semantic Version", nv)
				}
				labelOK := !sv.PreReleaseTag.HasTag() || sv.PreReleaseTag.Name == eff.Label
				if labelOK {
					lbl := eff.Label
					candidates = append(candidates, newBaseVersion("ConfiguredNextVersion", sv, nil, v.FieldNone, &lbl))
				}
			}
		case config.StrategyTaggedCommit, config.StrategyMainline:
			c, alts, err := gatherTagged(repo, &eff, &head, ig)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, c...)
			tagAlternatives = append(tagAlternatives, alts...)
		case config.StrategyVersionInBranchName:
			if eff.IsReleaseBranch {
				if sv, ok := extractVersion(branchName, &eff); ok {
					lbl := eff.Label
					candidates = append(candidates, newBaseVersion("VersionInBranchName", sv, nil, v.FieldNone, &lbl))
				}
			}
		case config.StrategyMergeMessage:
			if eff.TrackMergeMessage {
				c, err := gatherMergeMessages(repo, cfg, &eff, &head, ig)
				if err != nil {
					return nil, err
				}
				candidates = append(candidates, c...)
			}
		case config.StrategyTrackReleaseBranches:
			c, err := gatherTrackRelease(repo, cfg, &eff, &head, branchName)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, c...)
		case config.StrategyFallback:
			field, err := determineIncrement(repo, nil, head.Sha, true, &eff, ig)
			if err != nil {
				return nil, err
			}
			lbl := eff.Label
			candidates = append(candidates, newBaseVersion("Fallback (0.0.0)", v.NewSemanticVersion(0, 0, 0), nil, field, &lbl))
		case config.StrategyNone:
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("No base versions determined on the current branch.")
	}

	next := make([]nextVersion, 0, len(candidates))
	for _, b := range candidates {
		var incremented v.SemanticVersion
		if b.exact {
			incremented = b.semanticVersion
		} else {
			incremented = b.semanticVersion.Increment(b.increment, b.label, b.forceIncrement)
		}
		next = append(next, nextVersion{incremented: incremented, base: b})
	}

	maxIdx := 0
	for i := 1; i < len(next); i++ {
		if next[i].incremented.Compare(next[maxIdx].incremented) > 0 {
			maxIdx = i
		}
	}

	// base version source: source 를 가진 후보 중 가장 최신.
	latestIdx := -1
	for i := range next {
		if next[i].base.baseVersionSource == nil {
			continue
		}
		if latestIdx < 0 || sourceWhenLess(next[latestIdx].base.sourceWhen, next[i].base.sourceWhen) {
			latestIdx = i
		}
	}
	var baseSource *string
	if latestIdx >= 0 {
		baseSource = next[latestIdx].base.baseVersionSource
	} else {
		baseSource = next[maxIdx].base.baseVersionSource
	}

	chosen := next[maxIdx]
	sourceSemver := chosen.base.semanticVersion

	finalSemver, err := applyDeploymentMode(repo, &eff, branchName, &head, &chosen, baseSource, ig)
	if err != nil {
		return nil, err
	}

	// AlternativeSemanticVersion 조정.
	if len(tagAlternatives) > 0 {
		alt := tagAlternatives[0]
		for _, a := range tagAlternatives[1:] {
			if a.CmpCore(alt) > 0 {
				alt = a
			}
		}
		if alt.CmpCore(finalSemver) > 0 {
			finalSemver.Major = alt.Major
			finalSemver.Minor = alt.Minor
			finalSemver.Patch = alt.Patch
		}
	}

	return buildVariables(&eff, branchName, &head, &finalSemver, &sourceSemver)
}

func containsStrategy(s []config.VersionStrategy, target config.VersionStrategy) bool {
	for _, x := range s {
		if x == target {
			return true
		}
	}
	return false
}

// sourceWhenLess 는 a < b 인지(nil 은 가장 작음). max_by(source_when) 용.
func sourceWhenLess(a, b *time.Time) bool {
	if a == nil {
		return b != nil
	}
	if b == nil {
		return false
	}
	return a.Before(*b)
}
