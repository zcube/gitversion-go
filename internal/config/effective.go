package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/zcube/go-gitversion/internal/rx"
)

// 원본 EffectiveConfiguration.cs, EffectiveBranchConfigurationFinder.cs.

func shortBranch(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// FindBranchConfig 는 브랜치명에 매칭되는 브랜치 설정 키와 그 설정을 반환한다.
// 구체적 브랜치 우선, 매칭 없으면 unknown.
func FindBranchConfig(config *GitVersionConfiguration, branchName string) (string, *BranchConfiguration, bool) {
	short := shortBranch(branchName)
	var unknownKey string
	var unknownBc *BranchConfiguration
	haveUnknown := false
	for _, key := range config.SortedBranchKeys() {
		bc := config.Branches[key]
		if bc.Regex == nil || *bc.Regex == "" {
			continue
		}
		re, err := rx.CompileCached("(?i)" + *bc.Regex)
		if err != nil {
			continue
		}
		if re.MatchString(branchName) || re.MatchString(short) {
			if key == "unknown" {
				unknownKey, unknownBc, haveUnknown = key, bc, true
			} else {
				return key, bc, true
			}
		}
	}
	if haveUnknown {
		return unknownKey, unknownBc, true
	}
	return "", nil, false
}

// normalizeNextVersion: 원본 NextVersion setter. 정수면 "{major}.0" 으로 보정.
func normalizeNextVersion(value string) string {
	if major, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
		return strconv.FormatInt(major, 10) + ".0"
	}
	return value
}

var sanitizeLabelRe = rx.MustCompile(`[^a-zA-Z0-9-]`)
var labelTokenRe = rx.MustCompile(`\{([^}]+)\}`)

func sanitizeLabel(s string) string { return sanitizeLabelRe.ReplaceAll(s, "-") }

// resolveLabel: label 의 `{token}` 을 치환(원본 GetBranchSpecificLabel + BuildLabelPlaceholders).
func resolveLabel(label string, regexSrc *string, branchName string) string {
	captures := map[string]string{}
	if regexSrc != nil && strings.TrimSpace(*regexSrc) != "" && branchName != "" {
		if re, err := rx.CompileCached("(?i)" + *regexSrc); err == nil {
			if m, ok := re.Find(branchName); ok {
				for name, val := range m.NamedGroups() {
					captures[name] = sanitizeLabel(val)
				}
			}
		}
	}

	return labelTokenRe.ReplaceAllFunc(label, func(m *rx.Match) string {
		whole := m.Whole()
		inner := strings.TrimSpace(whole[1 : len(whole)-1])
		var fallback *string
		if idx := strings.Index(inner, "??"); idx >= 0 {
			expr := strings.TrimSpace(inner[:idx])
			fb := strings.Trim(strings.TrimSpace(inner[idx+2:]), `"`)
			fallback = &fb
			inner = expr
		}
		expr := inner
		name := expr
		if i := strings.Index(expr, ":"); i >= 0 {
			name = strings.TrimSpace(expr[:i])
		} else {
			name = strings.TrimSpace(expr)
		}
		if strings.HasPrefix(expr, "env:") {
			varName := strings.TrimPrefix(expr, "env:")
			if i := strings.Index(varName, "??"); i >= 0 {
				varName = varName[:i]
			}
			varName = strings.TrimSpace(varName)
			if v, ok := os.LookupEnv(varName); ok && v != "" {
				return v
			}
		} else if v, ok := captures[name]; ok {
			return v
		}
		if fallback != nil {
			return *fallback
		}
		return whole
	})
}

// inheritLabel: label 미지정 시 source-branches 부모에서 상속.
func inheritLabel(config *GitVersionConfiguration, bc *BranchConfiguration, depth int) (string, bool) {
	if bc.Label != nil {
		return *bc.Label, true
	}
	if depth > 8 {
		return "", false
	}
	for _, src := range bc.SourceBranches {
		if srcBc, ok := config.Branches[src]; ok {
			if l, ok := inheritLabel(config, srcBc, depth+1); ok {
				return l, true
			}
		}
	}
	return "", false
}

// ResolveIncrement: Increment == Inherit 를 source-branch 를 따라 해석.
func ResolveIncrement(config *GitVersionConfiguration, bc *BranchConfiguration, depth int) IncrementStrategy {
	own := IncrementInherit
	if bc.Increment != nil {
		own = *bc.Increment
	} else if config.Increment != nil {
		own = *config.Increment
	}
	if own != IncrementInherit {
		return own
	}
	if depth > 8 {
		return IncrementNone
	}
	for _, src := range bc.SourceBranches {
		if srcBc, ok := config.Branches[src]; ok {
			resolved := ResolveIncrement(config, srcBc, depth+1)
			if resolved != IncrementInherit {
				return resolved
			}
		}
	}
	return IncrementNone
}

// EffectiveConfiguration 은 브랜치에 적용되는 모든 설정값을 평탄화한 구조.
type EffectiveConfiguration struct {
	BranchKey                               string
	DeploymentMode                          DeploymentMode
	Label                                   string
	Increment                               IncrementStrategy
	Regex                                   *string
	PreventIncrementOfMergedBranch          bool
	PreventIncrementWhenBranchMerged        bool
	PreventIncrementWhenCurrentCommitTagged bool
	TrackMergeTarget                        bool
	TrackMergeMessage                       bool
	TracksReleaseBranches                   bool
	IsReleaseBranch                         bool
	IsMainBranch                            bool
	PreReleaseWeight                        int64
	TagPreReleaseWeight                     int64
	CommitMessageIncrementing               CommitMessageIncrementMode
	MajorBumpMessage                        string
	MinorBumpMessage                        string
	PatchBumpMessage                        string
	NoBumpMessage                           string
	TagPrefix                               string
	VersionInBranchPattern                  string
	NextVersion                             *string
	SemanticVersionFormat                   SemanticVersionFormat
	CommitDateFormat                        string
	AssemblyVersioningScheme                VersioningScheme
	AssemblyFileVersioningScheme            VersioningScheme
	AssemblyInformationalFormat             string
	AssemblyVersioningFormat                *string
	AssemblyFileVersioningFormat            *string
	MergeMessageFormats                     map[string]string
	SourceBranches                          []string
	LabelNumberPattern                      string
}

func coalesceBool(b, g *bool) bool {
	if b != nil {
		return *b
	}
	if g != nil {
		return *g
	}
	return false
}

func orBoolDefault(b, g *bool, def bool) bool {
	if b != nil {
		return *b
	}
	if g != nil {
		return *g
	}
	return def
}

// ResolveEffective 는 전역 설정 + 매칭된 브랜치 설정을 병합해 effective 설정 생성.
func ResolveEffective(config *GitVersionConfiguration, branchName string) EffectiveConfiguration {
	key, matched, ok := FindBranchConfig(config, branchName)
	var bc *BranchConfiguration
	if ok {
		bc = matched
	} else {
		key = "unknown"
		bc = &BranchConfiguration{}
	}

	var piBranch, piGlobal PreventIncrement
	if bc.PreventIncrement != nil {
		piBranch = *bc.PreventIncrement
	}
	if config.PreventIncrement != nil {
		piGlobal = *config.PreventIncrement
	}

	rawLabel, hasLabel := inheritLabel(config, bc, 0)
	if !hasLabel && config.Label != nil {
		rawLabel, hasLabel = *config.Label, true
	}
	if !hasLabel {
		rawLabel = ""
	}
	label := resolveLabel(rawLabel, bc.Regex, branchName)

	deploymentMode := ContinuousDelivery
	if bc.Mode != nil {
		deploymentMode = *bc.Mode
	} else if config.Mode != nil {
		deploymentMode = *config.Mode
	}

	eff := EffectiveConfiguration{
		BranchKey:                               key,
		DeploymentMode:                          deploymentMode,
		Label:                                   label,
		Increment:                               ResolveIncrement(config, bc, 0),
		Regex:                                   bc.Regex,
		PreventIncrementOfMergedBranch:          coalesceBool(piBranch.OfMergedBranch, piGlobal.OfMergedBranch),
		PreventIncrementWhenBranchMerged:        coalesceBool(piBranch.WhenBranchMerged, piGlobal.WhenBranchMerged),
		PreventIncrementWhenCurrentCommitTagged: orBoolDefault(piBranch.WhenCurrentCommitTagged, piGlobal.WhenCurrentCommitTagged, true),
		TrackMergeTarget:                        coalesceBool(bc.TrackMergeTarget, config.TrackMergeTarget),
		TrackMergeMessage:                       orBoolDefault(bc.TrackMergeMessage, config.TrackMergeMessage, true),
		TracksReleaseBranches:                   coalesceBool(bc.TracksReleaseBranches, config.TracksReleaseBranches),
		IsReleaseBranch:                         coalesceBool(bc.IsReleaseBranch, config.IsReleaseBranch),
		IsMainBranch:                            coalesceBool(bc.IsMainBranch, config.IsMainBranch),
		SourceBranches:                          bc.SourceBranches,
		MergeMessageFormats:                     config.MergeMessageFormats,
	}

	eff.PreReleaseWeight = firstInt(bc.PreReleaseWeight, config.PreReleaseWeight, 0)
	eff.TagPreReleaseWeight = ptrIntOr(config.TagPreReleaseWeight, 60000)

	if bc.CommitMessageIncrementing != nil {
		eff.CommitMessageIncrementing = *bc.CommitMessageIncrementing
	} else if config.CommitMessageIncrementing != nil {
		eff.CommitMessageIncrementing = *config.CommitMessageIncrementing
	} else {
		eff.CommitMessageIncrementing = CommitMsgEnabled
	}

	eff.MajorBumpMessage = ptrStrOr(config.MajorVersionBumpMessage, `\+semver:\s?(breaking|major)`)
	eff.MinorBumpMessage = ptrStrOr(config.MinorVersionBumpMessage, `\+semver:\s?(feature|minor)`)
	eff.PatchBumpMessage = ptrStrOr(config.PatchVersionBumpMessage, `\+semver:\s?(fix|patch)`)
	eff.NoBumpMessage = ptrStrOr(config.NoBumpMessage, `\+semver:\s?(none|skip)`)
	eff.TagPrefix = ptrStrOr(config.TagPrefix, "[vV]?")
	eff.VersionInBranchPattern = ptrStrOr(config.VersionInBranchPattern, `(?<version>[vV]?\d+(\.\d+)?(\.\d+)?).*`)

	if config.NextVersion != nil {
		nv := normalizeNextVersion(*config.NextVersion)
		eff.NextVersion = &nv
	}

	if config.SemanticVersionFormat != nil {
		eff.SemanticVersionFormat = *config.SemanticVersionFormat
	} else {
		eff.SemanticVersionFormat = FormatStrict
	}
	eff.CommitDateFormat = ptrStrOr(config.CommitDateFormat, "yyyy-MM-dd")

	eff.AssemblyVersioningScheme = ptrSchemeOr(config.AssemblyVersioningScheme, SchemeMajorMinorPatch)
	eff.AssemblyFileVersioningScheme = ptrSchemeOr(config.AssemblyFileVersioningScheme, SchemeMajorMinorPatch)
	eff.AssemblyInformationalFormat = ptrStrOr(config.AssemblyInformationalFormat, "{InformationalVersion}")
	eff.AssemblyVersioningFormat = config.AssemblyVersioningFormat
	eff.AssemblyFileVersioningFormat = config.AssemblyFileVersioningFormat

	if bc.LabelNumberPattern != nil {
		eff.LabelNumberPattern = *bc.LabelNumberPattern
	} else if config.LabelNumberPattern != nil {
		eff.LabelNumberPattern = *config.LabelNumberPattern
	} else {
		eff.LabelNumberPattern = `(?<name>.*?)\.?(?<number>\d+)?$`
	}

	return eff
}

func firstInt(a, b *int64, def int64) int64 {
	if a != nil {
		return *a
	}
	if b != nil {
		return *b
	}
	return def
}

func ptrIntOr(a *int64, def int64) int64 {
	if a != nil {
		return *a
	}
	return def
}

func ptrStrOr(a *string, def string) string {
	if a != nil {
		return *a
	}
	return def
}

func ptrSchemeOr(a *VersioningScheme, def VersioningScheme) VersioningScheme {
	if a != nil {
		return *a
	}
	return def
}
