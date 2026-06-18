package config

import "strings"

// 원본 GitVersion.Configuration/Builders/{GitFlow,GitHubFlow,TrunkBased}ConfigurationBuilder.cs.
const (
	mainRegex    = "^master$|^main$"
	developRegex = "^dev(elop)?(ment)?$"
	releaseRegex = `^releases?[\/-](?<BranchName>.+)`
	featureRegex = `^features?[\/-](?<BranchName>.+)`
	prRegex      = `^(pull-requests|pull|pr)[\/-](?<Number>\d*)`
	hotfixRegex  = `^hotfix(es)?[\/-](?<BranchName>.+)`
	supportRegex = `^support[\/-](?<BranchName>.+)`
	unknownRegex = `(?<BranchName>.+)`

	majorBump = `\+semver:\s?(breaking|major)`
	minorBump = `\+semver:\s?(feature|minor)`
	patchBump = `\+semver:\s?(fix|patch)`
	noBump    = `\+semver:\s?(none|skip)`
)

func prevent(ofMerged, whenMerged, whenTagged *bool) *PreventIncrement {
	return &PreventIncrement{OfMergedBranch: ofMerged, WhenBranchMerged: whenMerged, WhenCurrentCommitTagged: whenTagged}
}

// globalBase: 전역 기본 필드를 채운 루트 설정(브랜치 미포함).
func globalBase(mode DeploymentMode, strategies []VersionStrategy) *GitVersionConfiguration {
	return &GitVersionConfiguration{
		AssemblyVersioningScheme:     schemePtr(SchemeMajorMinorPatch),
		AssemblyFileVersioningScheme: schemePtr(SchemeMajorMinorPatch),
		AssemblyInformationalFormat:  strPtr("{InformationalVersion}"),
		TagPrefix:                    strPtr("[vV]?"),
		VersionInBranchPattern:       strPtr(`(?<version>[vV]?\d+(\.\d+)?(\.\d+)?).*`),
		MajorVersionBumpMessage:      strPtr(majorBump),
		MinorVersionBumpMessage:      strPtr(minorBump),
		PatchVersionBumpMessage:      strPtr(patchBump),
		NoBumpMessage:                strPtr(noBump),
		TagPreReleaseWeight:          i64Ptr(60000),
		CommitDateFormat:             strPtr("yyyy-MM-dd"),
		SemanticVersionFormat:        fmtPtr(FormatStrict),
		UpdateBuildNumber:            boolPtr(true),
		Strategies:                   strategies,
		Increment:                    incPtr(IncrementInherit),
		Mode:                         modePtr(mode),
		Label:                        strPtr("{BranchName}"),
		Regex:                        strPtr(""),
		CommitMessageIncrementing:    cmiPtr(CommitMsgEnabled),
		PreventIncrement:             prevent(boolPtr(false), boolPtr(false), boolPtr(true)),
		TrackMergeTarget:             boolPtr(false),
		TrackMergeMessage:            boolPtr(true),
		TracksReleaseBranches:        boolPtr(false),
		IsReleaseBranch:              boolPtr(false),
		IsMainBranch:                 boolPtr(false),
		MergeMessageFormats:          map[string]string{},
		Branches:                     map[string]*BranchConfiguration{},
		Exec:                         map[string]string{},
	}
}

func defaultStrategies() []VersionStrategy {
	return []VersionStrategy{
		StrategyFallback, StrategyConfiguredNextVersion, StrategyMergeMessage,
		StrategyTaggedCommit, StrategyTrackReleaseBranches, StrategyVersionInBranchName,
	}
}

// GitFlow 워크플로(기본값).
func GitFlow() *GitVersionConfiguration {
	c := globalBase(ContinuousDelivery, defaultStrategies())
	c.Branches = map[string]*BranchConfiguration{
		"develop": {
			Regex:                 strPtr(developRegex),
			Increment:             incPtr(IncrementMinor),
			Mode:                  modePtr(ContinuousDelivery),
			Label:                 strPtr("alpha"),
			SourceBranches:        []string{"main"},
			PreventIncrement:      prevent(nil, nil, boolPtr(false)),
			TrackMergeTarget:      boolPtr(true),
			TrackMergeMessage:     boolPtr(true),
			TracksReleaseBranches: boolPtr(true),
			IsMainBranch:          boolPtr(false),
			IsReleaseBranch:       boolPtr(false),
			PreReleaseWeight:      i64Ptr(0),
		},
		"main": {
			Regex:             strPtr(mainRegex),
			Increment:         incPtr(IncrementPatch),
			Label:             strPtr(""),
			SourceBranches:    []string{},
			PreventIncrement:  prevent(boolPtr(true), nil, nil),
			TrackMergeTarget:  boolPtr(false),
			TrackMergeMessage: boolPtr(true),
			IsMainBranch:      boolPtr(true),
			PreReleaseWeight:  i64Ptr(55000),
		},
		"release": {
			Regex:            strPtr(releaseRegex),
			Increment:        incPtr(IncrementMinor),
			Mode:             modePtr(ManualDeployment),
			Label:            strPtr("beta"),
			SourceBranches:   []string{"main", "support"},
			PreventIncrement: prevent(boolPtr(true), nil, boolPtr(false)),
			TrackMergeTarget: boolPtr(false),
			IsReleaseBranch:  boolPtr(true),
			PreReleaseWeight: i64Ptr(30000),
		},
		"feature": {
			Regex:             strPtr(featureRegex),
			Increment:         incPtr(IncrementInherit),
			Mode:              modePtr(ManualDeployment),
			Label:             strPtr("{BranchName}"),
			SourceBranches:    []string{"develop", "main", "release", "support", "hotfix"},
			PreventIncrement:  prevent(nil, nil, boolPtr(false)),
			TrackMergeMessage: boolPtr(true),
			PreReleaseWeight:  i64Ptr(30000),
		},
		"pull-request": {
			Regex:             strPtr(prRegex),
			Increment:         incPtr(IncrementInherit),
			Mode:              modePtr(ContinuousDelivery),
			Label:             strPtr("PullRequest{Number}"),
			SourceBranches:    []string{"develop", "main", "release", "feature", "support", "hotfix"},
			PreventIncrement:  prevent(boolPtr(true), nil, boolPtr(false)),
			TrackMergeMessage: boolPtr(true),
			PreReleaseWeight:  i64Ptr(30000),
		},
		"hotfix": {
			Regex:            strPtr(hotfixRegex),
			Increment:        incPtr(IncrementInherit),
			Mode:             modePtr(ManualDeployment),
			Label:            strPtr("beta"),
			SourceBranches:   []string{"main", "support"},
			PreventIncrement: prevent(nil, nil, boolPtr(false)),
			IsReleaseBranch:  boolPtr(true),
			PreReleaseWeight: i64Ptr(30000),
		},
		"support": {
			Regex:            strPtr(supportRegex),
			Increment:        incPtr(IncrementPatch),
			Label:            strPtr(""),
			SourceBranches:   []string{"main"},
			PreventIncrement: prevent(boolPtr(true), nil, nil),
			TrackMergeTarget: boolPtr(false),
			IsMainBranch:     boolPtr(true),
			PreReleaseWeight: i64Ptr(55000),
		},
		"unknown": {
			Regex:          strPtr(unknownRegex),
			Increment:      incPtr(IncrementInherit),
			Mode:           modePtr(ManualDeployment),
			Label:          strPtr("{BranchName}"),
			SourceBranches: []string{"main", "develop", "release", "feature", "pull-request", "support", "hotfix"},
		},
	}
	return c
}

// GitHubFlow 워크플로.
func GitHubFlow() *GitVersionConfiguration {
	c := globalBase(ContinuousDelivery, defaultStrategies())
	c.Branches = map[string]*BranchConfiguration{
		"main": {
			Regex:            strPtr(mainRegex),
			Increment:        incPtr(IncrementPatch),
			Label:            strPtr(""),
			SourceBranches:   []string{},
			PreventIncrement: prevent(boolPtr(true), nil, nil),
			IsMainBranch:     boolPtr(true),
			PreReleaseWeight: i64Ptr(55000),
		},
		"release": {
			Regex:             strPtr(releaseRegex),
			Increment:         incPtr(IncrementPatch),
			Mode:              modePtr(ManualDeployment),
			Label:             strPtr("beta"),
			SourceBranches:    []string{"main"},
			PreventIncrement:  prevent(boolPtr(true), boolPtr(false), boolPtr(false)),
			TrackMergeTarget:  boolPtr(false),
			TrackMergeMessage: boolPtr(true),
			IsReleaseBranch:   boolPtr(true),
			PreReleaseWeight:  i64Ptr(30000),
		},
		"feature": {
			Regex:            strPtr(featureRegex),
			Increment:        incPtr(IncrementInherit),
			Mode:             modePtr(ManualDeployment),
			Label:            strPtr("{BranchName}"),
			SourceBranches:   []string{"main", "release"},
			PreventIncrement: prevent(nil, nil, boolPtr(false)),
			PreReleaseWeight: i64Ptr(30000),
		},
		"pull-request": {
			Regex:            strPtr(prRegex),
			Increment:        incPtr(IncrementInherit),
			Mode:             modePtr(ContinuousDelivery),
			Label:            strPtr("PullRequest{Number}"),
			SourceBranches:   []string{"main", "release", "feature"},
			PreventIncrement: prevent(boolPtr(true), nil, boolPtr(false)),
			PreReleaseWeight: i64Ptr(30000),
		},
		"unknown": {
			Regex:             strPtr(unknownRegex),
			Increment:         incPtr(IncrementInherit),
			Mode:              modePtr(ManualDeployment),
			Label:             strPtr("{BranchName}"),
			SourceBranches:    []string{"main", "release", "feature", "pull-request"},
			PreventIncrement:  prevent(nil, nil, boolPtr(false)),
			TrackMergeMessage: boolPtr(false),
		},
	}
	return c
}

// TrunkBased(Mainline) 워크플로.
func TrunkBased() *GitVersionConfiguration {
	c := globalBase(ContinuousDelivery, []VersionStrategy{StrategyConfiguredNextVersion, StrategyMainline})
	c.Branches = map[string]*BranchConfiguration{
		"main": {
			Regex:            strPtr(mainRegex),
			Increment:        incPtr(IncrementPatch),
			Mode:             modePtr(ContinuousDeployment),
			Label:            strPtr(""),
			SourceBranches:   []string{},
			PreventIncrement: prevent(boolPtr(true), nil, nil),
			IsMainBranch:     boolPtr(true),
			PreReleaseWeight: i64Ptr(55000),
		},
		"feature": {
			Regex:            strPtr(featureRegex),
			Increment:        incPtr(IncrementMinor),
			Mode:             modePtr(ContinuousDelivery),
			Label:            strPtr("{BranchName}"),
			SourceBranches:   []string{"main"},
			PreventIncrement: prevent(nil, nil, boolPtr(false)),
			PreReleaseWeight: i64Ptr(30000),
		},
		"hotfix": {
			Regex:            strPtr(hotfixRegex),
			Increment:        incPtr(IncrementPatch),
			Mode:             modePtr(ContinuousDelivery),
			Label:            strPtr("{BranchName}"),
			SourceBranches:   []string{"main"},
			PreventIncrement: prevent(nil, nil, boolPtr(false)),
			IsReleaseBranch:  boolPtr(true),
			PreReleaseWeight: i64Ptr(30000),
		},
		"pull-request": {
			Regex:            strPtr(prRegex),
			Increment:        incPtr(IncrementInherit),
			Mode:             modePtr(ContinuousDelivery),
			Label:            strPtr("PullRequest{Number}"),
			SourceBranches:   []string{"main", "feature", "hotfix"},
			PreventIncrement: prevent(boolPtr(true), nil, boolPtr(false)),
			PreReleaseWeight: i64Ptr(30000),
		},
		"unknown": {
			Regex:            strPtr(unknownRegex),
			Increment:        incPtr(IncrementPatch),
			Mode:             modePtr(ContinuousDelivery),
			Label:            strPtr("{BranchName}"),
			SourceBranches:   []string{"main"},
			PreventIncrement: prevent(nil, nil, boolPtr(false)),
			PreReleaseWeight: i64Ptr(30000),
		},
	}
	return c
}

// ForWorkflow 는 워크플로 이름으로 기본 설정 선택. nil 이면 GitFlow.
func ForWorkflow(workflow *string) *GitVersionConfiguration {
	if workflow == nil {
		return GitFlow()
	}
	w := strings.ToLower(*workflow)
	switch {
	case strings.HasPrefix(w, "githubflow"):
		return GitHubFlow()
	case strings.HasPrefix(w, "trunkbased"), strings.HasPrefix(w, "mainline"):
		return TrunkBased()
	default:
		return GitFlow()
	}
}
