package calc

import (
	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/output"
	v "github.com/zcube/gitversion-go/internal/version"
)

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// mainlineCalculate: Mainline 전략. base(최고 태그 또는 0.0.0)부터 각 커밋의 증분을
// 누적한다. 원본 MainlineVersionStrategy.
func mainlineCalculate(repo *git.GitRepo, cfg *config.GitVersionConfiguration, eff *config.EffectiveConfiguration, branchName string, head *git.CommitInfo, ig ignoreSet) (*output.VersionVariables, error) {
	// 도달 가능한 모든 태그를 sha -> 코어 버전 맵으로(같은 커밋에 여러 태그면 최고).
	tagsBySha := map[string]v.SemanticVersion{}
	tags, err := repo.Tags()
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if ig.isIgnored(tag.TargetSha, tag.When) {
			continue
		}
		if sv, ok := parseVersion(tag.Name, eff); ok {
			core := v.NewSemanticVersion(sv.Major, sv.Minor, sv.Patch)
			if e, exists := tagsBySha[tag.TargetSha]; !exists || core.CmpCore(e) > 0 {
				tagsBySha[tag.TargetSha] = core
			}
		}
	}
	coreGt := func(a, b v.SemanticVersion) bool { return a.CmpCore(b) > 0 }

	defaultField := strategyToField(eff.Increment)

	// 비-트렁크 브랜치는 source 브랜치 merge-base 까지만 트렁크 증분 적용.
	var mergeBaseSha *string
	if !eff.IsMainBranch && len(eff.SourceBranches) > 0 {
		src := eff.SourceBranches[0]
		if srcInfo, ok := repo.CommitInfoOf(src); ok {
			mb, err := repo.MergeBase(head.Sha, srcInfo.Sha)
			if err != nil {
				return nil, err
			}
			mergeBaseSha = mb
		}
	}

	trunkTarget := head.Sha
	if mergeBaseSha != nil {
		trunkTarget = *mergeBaseSha
	}
	trunkEff := eff
	if mergeBaseSha != nil {
		e := config.ResolveEffective(cfg, eff.SourceBranches[0])
		trunkEff = &e
	}
	trunkDefault := strategyToField(trunkEff.Increment)

	trunk, err := repo.FirstParentBetween(nil, trunkTarget)
	if err != nil {
		return nil, err
	}
	trunk = ig.filter(repo, trunk)
	// 오래된 순으로 뒤집기.
	for i, j := 0, len(trunk)-1; i < j; i, j = i+1, j-1 {
		trunk[i], trunk[j] = trunk[j], trunk[i]
	}

	version := v.NewSemanticVersion(0, 0, 0)
	highestTag := v.NewSemanticVersion(0, 0, 0)
	prevTrunkVersion := v.NewSemanticVersion(0, 0, 0)
	for ci := range trunk {
		c := &trunk[ci]
		prevTrunkVersion = version

		var introduced []git.CommitInfo
		if c.ParentCount >= 2 {
			p0 := c.Parents[0]
			cb, err := repo.CommitsBetween(&p0, c.Parents[1])
			if err != nil {
				return nil, err
			}
			introduced = ig.filter(repo, cb)
		} else {
			introduced = []git.CommitInfo{*c}
		}

		var stepTag *v.SemanticVersion
		shas := make([]string, 0, len(introduced)+1)
		for _, x := range introduced {
			shas = append(shas, x.Sha)
		}
		shas = append(shas, c.Sha)
		for _, sha := range shas {
			if tv, ok := tagsBySha[sha]; ok {
				if stepTag == nil || coreGt(tv, *stepTag) {
					t := tv
					stepTag = &t
				}
			}
		}

		if stepTag != nil {
			if coreGt(*stepTag, highestTag) {
				highestTag = *stepTag
			}
			if !coreGt(version, *stepTag) {
				version = *stepTag
				continue
			}
		}

		field := trunkDefault
		for _, ic := range introduced {
			if f, ok := incrementFromMessage(ic.Message, trunkEff); ok {
				if f > field {
					field = f
				}
			}
		}
		if c.ParentCount >= 2 {
			if mf, has := mergeBranchIncrement(cfg, c.Message); has {
				if mf == v.FieldNone {
					field = v.FieldNone
				} else if mf > field {
					field = mf
				}
			}
		}
		version = version.Increment(field, nil, true)
	}
	_ = highestTag
	trunkVersionEnd := version

	var sourceSha *string
	var distance int64
	if mergeBaseSha != nil {
		fc, err := repo.CommitsBetween(mergeBaseSha, head.Sha)
		if err != nil {
			return nil, err
		}
		featureCommits := ig.filter(repo, fc)
		_, headIsTagged := tagsBySha[head.Sha]

		var ftSha string
		var ft v.SemanticVersion
		haveFt := false
		for _, c := range featureCommits {
			if tv, ok := tagsBySha[c.Sha]; ok {
				if !haveFt || coreGt(tv, ft) {
					ftSha, ft, haveFt = c.Sha, tv, true
				}
			}
		}

		if haveFt {
			if headIsTagged && !eff.PreventIncrementWhenCurrentCommitTagged {
				version = ft.Increment(defaultField, nil, true)
				s := head.Sha
				sourceSha, distance = &s, 0
			} else {
				cb, err := repo.CommitsBetween(&ftSha, head.Sha)
				if err != nil {
					return nil, err
				}
				version = ft
				s := ftSha
				sourceSha, distance = &s, int64(len(cb))
			}
		} else {
			version = version.Increment(defaultField, nil, true)
			s := *mergeBaseSha
			sourceSha, distance = &s, int64(len(featureCommits))
		}
	} else {
		_, headIsTagged := tagsBySha[head.Sha]
		if headIsTagged && !eff.PreventIncrementWhenCurrentCommitTagged {
			version = version.Increment(defaultField, nil, true)
			s := head.Sha
			sourceSha, distance = &s, 0
		} else {
			var s *string
			if len(head.Parents) > 0 {
				p := head.Parents[0]
				s = &p
			}
			cb, err := repo.CommitsBetween(s, head.Sha)
			if err != nil {
				return nil, err
			}
			sourceSha, distance = s, int64(len(cb))
		}
	}

	// deployment mode 별 pre-release / build metadata.
	label := eff.Label
	var commitsSinceTag *int64
	switch eff.DeploymentMode {
	case config.ContinuousDeployment:
		version.PreReleaseTag = v.PreReleaseTag{}
	case config.ContinuousDelivery:
		version.PreReleaseTag = v.NewPreReleaseTag(label, i64p(distance), label == "")
	case config.ManualDeployment:
		commitsSinceTag = i64p(distance)
		version.PreReleaseTag = v.NewPreReleaseTag(label, i64p(1), label == "")
	}
	version.BuildMetaData = v.BuildMetaData{
		CommitsSinceTag:        commitsSinceTag,
		Branch:                 branchName,
		Sha:                    head.Sha,
		ShortSha:               head.ShortSha,
		CommitDate:             tptr(head.When),
		VersionSourceSha:       deref(sourceSha),
		VersionSourceDistance:  distance,
		UncommittedChanges:     repo.UncommittedChanges(),
		VersionSourceIncrement: v.FieldNone,
	}

	// VersionSourceSemVer 계산.
	versionAtSource := prevTrunkVersion
	if mergeBaseSha != nil {
		versionAtSource = trunkVersionEnd
	}
	var sourceSemver v.SemanticVersion
	switch {
	case sourceSha == nil:
		sourceSemver = v.NewSemanticVersion(0, 0, 0)
	default:
		if tv, ok := tagsBySha[*sourceSha]; ok {
			sourceSemver = tv
		} else {
			sv := versionAtSource
			sv.PreReleaseTag = v.NewPreReleaseTag("", i64p(1), true)
			sourceSemver = sv
		}
	}

	return buildVariables(eff, branchName, head, &version, &sourceSemver)
}
