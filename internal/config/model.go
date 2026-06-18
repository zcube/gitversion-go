// Package config 는 GitVersion 설정: 데이터 모델, 워크플로 기본값, effective 설정, 로더.
//
// 원본 GitVersion.Configuration/GitVersionConfiguration.cs 와 1:1 대응한다.
// YAML 키는 모두 kebab-case.
package config

import "sort"

// PreventIncrement: increment 방지 설정. `prevent-increment` 키.
type PreventIncrement struct {
	OfMergedBranch          *bool `yaml:"of-merged-branch,omitempty"`
	WhenBranchMerged        *bool `yaml:"when-branch-merged,omitempty"`
	WhenCurrentCommitTagged *bool `yaml:"when-current-commit-tagged,omitempty"`
}

// IgnoreConfig: 무시할 커밋 설정. `ignore` 키.
type IgnoreConfig struct {
	CommitsBefore *string  `yaml:"commits-before,omitempty"`
	Sha           []string `yaml:"sha,omitempty"`
	// 이 경로 아래 파일만 변경한 커밋을 버전 계산에서 제외.
	Paths []string `yaml:"paths,omitempty"`
}

// BranchConfiguration: 개별 브랜치 설정. 전역 설정에서 상속받아 병합된다.
type BranchConfiguration struct {
	Regex                     *string                     `yaml:"regex,omitempty"`
	Label                     *string                     `yaml:"label,omitempty"`
	Increment                 *IncrementStrategy          `yaml:"increment,omitempty"`
	Mode                      *DeploymentMode             `yaml:"mode,omitempty"`
	CommitMessageIncrementing *CommitMessageIncrementMode `yaml:"commit-message-incrementing,omitempty"`
	PreventIncrement          *PreventIncrement           `yaml:"prevent-increment,omitempty"`
	TrackMergeTarget          *bool                       `yaml:"track-merge-target,omitempty"`
	TrackMergeMessage         *bool                       `yaml:"track-merge-message,omitempty"`
	TracksReleaseBranches     *bool                       `yaml:"tracks-release-branches,omitempty"`
	IsReleaseBranch           *bool                       `yaml:"is-release-branch,omitempty"`
	IsMainBranch              *bool                       `yaml:"is-main-branch,omitempty"`
	PreReleaseWeight          *int64                      `yaml:"pre-release-weight,omitempty"`
	SourceBranches            []string                    `yaml:"source-branches,omitempty"`
	IsSourceBranchFor         []string                    `yaml:"is-source-branch-for,omitempty"`
	// pre-release label 에서 번호를 추출하는 정규식.
	LabelNumberPattern *string `yaml:"label-number-pattern,omitempty"`
}

// GitVersionConfiguration: 루트 GitVersion 설정.
type GitVersionConfiguration struct {
	Workflow                     *string                `yaml:"workflow,omitempty"`
	AssemblyVersioningScheme     *VersioningScheme      `yaml:"assembly-versioning-scheme,omitempty"`
	AssemblyFileVersioningScheme *VersioningScheme      `yaml:"assembly-file-versioning-scheme,omitempty"`
	AssemblyInformationalFormat  *string                `yaml:"assembly-informational-format,omitempty"`
	AssemblyVersioningFormat     *string                `yaml:"assembly-versioning-format,omitempty"`
	AssemblyFileVersioningFormat *string                `yaml:"assembly-file-versioning-format,omitempty"`
	TagPrefix                    *string                `yaml:"tag-prefix,omitempty"`
	VersionInBranchPattern       *string                `yaml:"version-in-branch-pattern,omitempty"`
	NextVersion                  *string                `yaml:"next-version,omitempty"`
	MajorVersionBumpMessage      *string                `yaml:"major-version-bump-message,omitempty"`
	MinorVersionBumpMessage      *string                `yaml:"minor-version-bump-message,omitempty"`
	PatchVersionBumpMessage      *string                `yaml:"patch-version-bump-message,omitempty"`
	NoBumpMessage                *string                `yaml:"no-bump-message,omitempty"`
	TagPreReleaseWeight          *int64                 `yaml:"tag-pre-release-weight,omitempty"`
	CommitDateFormat             *string                `yaml:"commit-date-format,omitempty"`
	SemanticVersionFormat        *SemanticVersionFormat `yaml:"semantic-version-format,omitempty"`
	UpdateBuildNumber            *bool                  `yaml:"update-build-number,omitempty"`
	Strategies                   []VersionStrategy      `yaml:"strategies,omitempty"`

	// 브랜치 단위로도 지정 가능한 전역 기본값
	Increment                 *IncrementStrategy          `yaml:"increment,omitempty"`
	Mode                      *DeploymentMode             `yaml:"mode,omitempty"`
	Label                     *string                     `yaml:"label,omitempty"`
	Regex                     *string                     `yaml:"regex,omitempty"`
	CommitMessageIncrementing *CommitMessageIncrementMode `yaml:"commit-message-incrementing,omitempty"`
	PreventIncrement          *PreventIncrement           `yaml:"prevent-increment,omitempty"`
	TrackMergeTarget          *bool                       `yaml:"track-merge-target,omitempty"`
	TrackMergeMessage         *bool                       `yaml:"track-merge-message,omitempty"`
	TracksReleaseBranches     *bool                       `yaml:"tracks-release-branches,omitempty"`
	IsReleaseBranch           *bool                       `yaml:"is-release-branch,omitempty"`
	IsMainBranch              *bool                       `yaml:"is-main-branch,omitempty"`
	PreReleaseWeight          *int64                      `yaml:"pre-release-weight,omitempty"`
	SourceBranches            []string                    `yaml:"source-branches,omitempty"`
	IsSourceBranchFor         []string                    `yaml:"is-source-branch-for,omitempty"`
	LabelNumberPattern        *string                     `yaml:"label-number-pattern,omitempty"`

	Ignore              IgnoreConfig                    `yaml:"ignore,omitempty"`
	MergeMessageFormats map[string]string               `yaml:"merge-message-formats,omitempty"`
	Branches            map[string]*BranchConfiguration `yaml:"branches,omitempty"`

	// 외부 명령 훅(우리 확장 기능). 훅 이름 -> 쉘 명령.
	Exec map[string]string `yaml:"exec,omitempty"`
}

// SortedBranchKeys 는 branches 키를 알파벳 순으로 반환한다(원본 BTreeMap 순회 일치).
func (c *GitVersionConfiguration) SortedBranchKeys() []string {
	keys := make([]string, 0, len(c.Branches))
	for k := range c.Branches {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortedMergeFormatValues 는 merge-message-formats 값을 키 순서대로 반환한다.
func (c *GitVersionConfiguration) SortedMergeFormatValues() []string {
	keys := make([]string, 0, len(c.MergeMessageFormats))
	for k := range c.MergeMessageFormats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, c.MergeMessageFormats[k])
	}
	return out
}

func boolPtr(b bool) *bool                                            { return &b }
func strPtr(s string) *string                                         { return &s }
func i64Ptr(v int64) *int64                                           { return &v }
func incPtr(s IncrementStrategy) *IncrementStrategy                   { return &s }
func modePtr(m DeploymentMode) *DeploymentMode                        { return &m }
func schemePtr(s VersioningScheme) *VersioningScheme                  { return &s }
func fmtPtr(f SemanticVersionFormat) *SemanticVersionFormat           { return &f }
func cmiPtr(m CommitMessageIncrementMode) *CommitMessageIncrementMode { return &m }
