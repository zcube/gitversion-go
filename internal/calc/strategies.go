package calc

import (
	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	v "github.com/zcube/gitversion-go/internal/version"
)

// isMatchForBranchLabel: 브랜치 label 매칭 여부.
// .NET SemanticVersion.IsMatchForBranchSpecificLabel.
func isMatchForBranchLabel(version v.SemanticVersion, label string) bool {
	pre := version.PreReleaseTag
	if pre.Name == "" && pre.Number == nil {
		return true
	}
	return pre.HasTag() && pre.Name == label
}

// gatherTagged: HEAD 에서 도달 가능한 버전 태그를 후보로 수집.
func gatherTagged(repo *git.GitRepo, eff *config.EffectiveConfiguration, head *git.CommitInfo, ig ignoreSet) ([]baseVersion, []v.SemanticVersion, error) {
	var out []baseVersion
	var alternatives []v.SemanticVersion
	tags, err := repo.Tags()
	if err != nil {
		return nil, nil, err
	}
	for _, tag := range tags {
		if ig.isIgnored(tag.TargetSha, tag.When) {
			continue
		}
		if ig.isPathIgnored(repo, tag.TargetSha) {
			continue
		}
		if ok, _ := repo.IsAncestorOf(tag.TargetSha, head.Sha); !ok {
			continue
		}
		version, ok := parseVersion(tag.Name, eff)
		if !ok {
			continue
		}
		alternatives = append(alternatives, version)
		if !isMatchForBranchLabel(version, eff.Label) {
			continue
		}
		isCurrent := tag.TargetSha == head.Sha
		exact := isCurrent && eff.PreventIncrementWhenCurrentCommitTagged
		hasPre := version.PreReleaseTag.HasTag()
		isNumericOnlyPre := hasPre && version.PreReleaseTag.Name == ""
		useAsSource := exact || !hasPre || isNumericOnlyPre

		var baseSrc *string
		if useAsSource {
			s := tag.TargetSha
			baseSrc = &s
		}

		var field v.VersionField
		if exact {
			field = v.FieldNone
		} else {
			var from *string
			if useAsSource {
				s := tag.TargetSha
				from = &s
			}
			field, err = determineIncrement(repo, from, head.Sha, true, eff, ig)
			if err != nil {
				return nil, nil, err
			}
		}
		lbl := eff.Label
		bv := newBaseVersion("Tag "+tag.Name, version, baseSrc, field, &lbl)
		bv.exact = exact
		if useAsSource {
			w := tag.When
			bv.sourceWhen = &w
		}
		out = append(out, bv)
	}
	return out, alternatives, nil
}

// gatherMergeMessages: merge 커밋 메시지에서 버전을 추출.
func gatherMergeMessages(repo *git.GitRepo, cfg *config.GitVersionConfiguration, eff *config.EffectiveConfiguration, head *git.CommitInfo, ig ignoreSet) ([]baseVersion, error) {
	var out []baseVersion
	commits, err := repo.CommitsBetween(nil, head.Sha)
	if err != nil {
		return nil, err
	}
	commits = ig.filter(repo, commits)
	count := 0
	for i := range commits {
		c := &commits[i]
		if count >= 5 {
			break
		}
		mergedBranch, sv, ok := parseMergeMessage(c.Message, eff)
		if !ok {
			continue
		}
		if !isReleaseBranch(cfg, mergedBranch) {
			continue
		}
		var baseSrc string
		if c.ParentCount >= 2 {
			mb, err := repo.MergeBase(c.Parents[0], c.Parents[1])
			if err == nil && mb != nil {
				baseSrc = *mb
			} else {
				baseSrc = c.Sha
			}
		} else {
			baseSrc = c.Sha
		}
		var field v.VersionField
		if eff.PreventIncrementOfMergedBranch {
			field = v.FieldNone
		} else {
			field, err = determineIncrement(repo, &baseSrc, head.Sha, true, eff, ig)
			if err != nil {
				return nil, err
			}
		}
		lbl := eff.Label
		src := baseSrc
		bv := newBaseVersion("MergeMessage", sv, &src, field, &lbl)
		w := c.When
		bv.sourceWhen = &w
		out = append(out, bv)
		count++
	}
	return out, nil
}

// gatherTrackRelease: release 브랜치를 추적(develop 등에서).
func gatherTrackRelease(repo *git.GitRepo, cfg *config.GitVersionConfiguration, eff *config.EffectiveConfiguration, head *git.CommitInfo, branchName string) ([]baseVersion, error) {
	var out []baseVersion
	if !eff.TracksReleaseBranches {
		return out, nil
	}
	releaseBc, ok := cfg.Branches["release"]
	if !ok || releaseBc.Regex == nil {
		return out, nil
	}
	re, err := compileCI(*releaseBc.Regex)
	if err != nil {
		return out, nil
	}
	branches, err := repo.BranchNames()
	if err != nil {
		return nil, err
	}
	for _, rb := range branches {
		short := shortName(rb)
		if !(re.MatchString(rb) || re.MatchString(short)) {
			continue
		}
		if sv, ok := extractVersion(rb, eff); ok {
			mb, _ := repo.MergeBase(branchName, rb)
			var baseSrc *string
			if mb != nil {
				baseSrc = mb
			} else {
				s := head.Sha
				baseSrc = &s
			}
			lbl := eff.Label
			out = append(out, newBaseVersion("TrackReleaseBranches: "+rb, sv, baseSrc, strategyToField(eff.Increment), &lbl))
		}
	}
	return out, nil
}
