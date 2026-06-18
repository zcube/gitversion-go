// Package output 은 최종 산출되는 GitVersion 출력 변수와 포맷터를 제공한다.
//
// 원본 GitVersion.Output/Serializer/VersionVariablesJsonModel.cs 와 1:1 대응.
package output

import (
	"sort"
	"strconv"
)

// VersionVariables 는 GitVersion 이 계산해 내는 모든 출력 변수.
type VersionVariables struct {
	Major uint32
	Minor uint32
	Patch uint32

	PreReleaseTag            string
	PreReleaseTagWithDash    string
	PreReleaseLabel          string
	PreReleaseLabelWithDash  string
	PreReleaseNumber         *int64
	WeightedPreReleaseNumber *int64

	BuildMetaData     *int64
	FullBuildMetaData string

	MajorMinorPatch string
	SemVer          string
	FullSemVer      string

	AssemblySemVer       string
	AssemblySemFileVer   string
	InformationalVersion string

	BranchName        string
	EscapedBranchName string
	Sha               string
	ShortSha          string

	VersionSourceDistance     *int64
	VersionSourceIncrement    string
	VersionSourceSemVer       string
	VersionSourceSha          string
	CommitsSinceVersionSource *int64

	CommitDate         string
	UncommittedChanges int64
}

func optI64(o *int64) string {
	if o == nil {
		return ""
	}
	return strconv.FormatInt(*o, 10)
}

// ToMap 은 변수 이름 -> 값 문자열 맵. -showvariable 및 환경변수 출력에 사용.
func (v *VersionVariables) ToMap() map[string]string {
	m := map[string]string{
		"Major":                     strconv.FormatUint(uint64(v.Major), 10),
		"Minor":                     strconv.FormatUint(uint64(v.Minor), 10),
		"Patch":                     strconv.FormatUint(uint64(v.Patch), 10),
		"PreReleaseTag":             v.PreReleaseTag,
		"PreReleaseTagWithDash":     v.PreReleaseTagWithDash,
		"PreReleaseLabel":           v.PreReleaseLabel,
		"PreReleaseLabelWithDash":   v.PreReleaseLabelWithDash,
		"PreReleaseNumber":          optI64(v.PreReleaseNumber),
		"WeightedPreReleaseNumber":  optI64(v.WeightedPreReleaseNumber),
		"BuildMetaData":             optI64(v.BuildMetaData),
		"FullBuildMetaData":         v.FullBuildMetaData,
		"MajorMinorPatch":           v.MajorMinorPatch,
		"SemVer":                    v.SemVer,
		"FullSemVer":                v.FullSemVer,
		"AssemblySemVer":            v.AssemblySemVer,
		"AssemblySemFileVer":        v.AssemblySemFileVer,
		"InformationalVersion":      v.InformationalVersion,
		"BranchName":                v.BranchName,
		"EscapedBranchName":         v.EscapedBranchName,
		"Sha":                       v.Sha,
		"ShortSha":                  v.ShortSha,
		"VersionSourceDistance":     optI64(v.VersionSourceDistance),
		"VersionSourceIncrement":    v.VersionSourceIncrement,
		"VersionSourceSemVer":       v.VersionSourceSemVer,
		"VersionSourceSha":          v.VersionSourceSha,
		"CommitsSinceVersionSource": optI64(v.CommitsSinceVersionSource),
		"CommitDate":                v.CommitDate,
		"UncommittedChanges":        strconv.FormatInt(v.UncommittedChanges, 10),
	}
	return m
}

// SortedKeys 는 ToMap 키를 정렬해 반환한다(결정적 출력용).
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
