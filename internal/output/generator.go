package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zcube/go-gitversion/internal/rx"
)

// jsonModel 은 JSON 직렬화 순서/생략 규칙을 원본 VersionVariablesJsonModel 과 맞춘다.
type jsonModel struct {
	Major                     uint32 `json:"Major"`
	Minor                     uint32 `json:"Minor"`
	Patch                     uint32 `json:"Patch"`
	PreReleaseTag             string `json:"PreReleaseTag"`
	PreReleaseTagWithDash     string `json:"PreReleaseTagWithDash"`
	PreReleaseLabel           string `json:"PreReleaseLabel"`
	PreReleaseLabelWithDash   string `json:"PreReleaseLabelWithDash"`
	PreReleaseNumber          *int64 `json:"PreReleaseNumber,omitempty"`
	WeightedPreReleaseNumber  *int64 `json:"WeightedPreReleaseNumber,omitempty"`
	BuildMetaData             *int64 `json:"BuildMetaData,omitempty"`
	FullBuildMetaData         string `json:"FullBuildMetaData"`
	MajorMinorPatch           string `json:"MajorMinorPatch"`
	SemVer                    string `json:"SemVer"`
	FullSemVer                string `json:"FullSemVer"`
	AssemblySemVer            string `json:"AssemblySemVer"`
	AssemblySemFileVer        string `json:"AssemblySemFileVer"`
	InformationalVersion      string `json:"InformationalVersion"`
	BranchName                string `json:"BranchName"`
	EscapedBranchName         string `json:"EscapedBranchName"`
	Sha                       string `json:"Sha"`
	ShortSha                  string `json:"ShortSha"`
	VersionSourceDistance     *int64 `json:"VersionSourceDistance,omitempty"`
	VersionSourceIncrement    string `json:"VersionSourceIncrement"`
	VersionSourceSemVer       string `json:"VersionSourceSemVer"`
	VersionSourceSha          string `json:"VersionSourceSha"`
	CommitsSinceVersionSource *int64 `json:"CommitsSinceVersionSource,omitempty"`
	CommitDate                string `json:"CommitDate"`
	UncommittedChanges        int64  `json:"UncommittedChanges"`
}

func (v *VersionVariables) toJSONModel() jsonModel {
	return jsonModel{
		Major: v.Major, Minor: v.Minor, Patch: v.Patch,
		PreReleaseTag: v.PreReleaseTag, PreReleaseTagWithDash: v.PreReleaseTagWithDash,
		PreReleaseLabel: v.PreReleaseLabel, PreReleaseLabelWithDash: v.PreReleaseLabelWithDash,
		PreReleaseNumber: v.PreReleaseNumber, WeightedPreReleaseNumber: v.WeightedPreReleaseNumber,
		BuildMetaData: v.BuildMetaData, FullBuildMetaData: v.FullBuildMetaData,
		MajorMinorPatch: v.MajorMinorPatch, SemVer: v.SemVer, FullSemVer: v.FullSemVer,
		AssemblySemVer: v.AssemblySemVer, AssemblySemFileVer: v.AssemblySemFileVer,
		InformationalVersion: v.InformationalVersion, BranchName: v.BranchName,
		EscapedBranchName: v.EscapedBranchName, Sha: v.Sha, ShortSha: v.ShortSha,
		VersionSourceDistance: v.VersionSourceDistance, VersionSourceIncrement: v.VersionSourceIncrement,
		VersionSourceSemVer: v.VersionSourceSemVer, VersionSourceSha: v.VersionSourceSha,
		CommitsSinceVersionSource: v.CommitsSinceVersionSource, CommitDate: v.CommitDate,
		UncommittedChanges: v.UncommittedChanges,
	}
}

// ToJSON 은 JSON 출력(원본과 동일한 PascalCase 키, pretty).
func (v *VersionVariables) ToJSON() (string, error) {
	b, err := json.MarshalIndent(v.toJSONModel(), "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FromJSON 은 JSON 을 VersionVariables 로 역직렬화한다(디스크 캐시 로드용).
func FromJSON(data []byte) (*VersionVariables, error) {
	var m jsonModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	v := &VersionVariables{
		Major: m.Major, Minor: m.Minor, Patch: m.Patch,
		PreReleaseTag: m.PreReleaseTag, PreReleaseTagWithDash: m.PreReleaseTagWithDash,
		PreReleaseLabel: m.PreReleaseLabel, PreReleaseLabelWithDash: m.PreReleaseLabelWithDash,
		PreReleaseNumber: m.PreReleaseNumber, WeightedPreReleaseNumber: m.WeightedPreReleaseNumber,
		BuildMetaData: m.BuildMetaData, FullBuildMetaData: m.FullBuildMetaData,
		MajorMinorPatch: m.MajorMinorPatch, SemVer: m.SemVer, FullSemVer: m.FullSemVer,
		AssemblySemVer: m.AssemblySemVer, AssemblySemFileVer: m.AssemblySemFileVer,
		InformationalVersion: m.InformationalVersion, BranchName: m.BranchName,
		EscapedBranchName: m.EscapedBranchName, Sha: m.Sha, ShortSha: m.ShortSha,
		VersionSourceDistance: m.VersionSourceDistance, VersionSourceIncrement: m.VersionSourceIncrement,
		VersionSourceSemVer: m.VersionSourceSemVer, VersionSourceSha: m.VersionSourceSha,
		CommitsSinceVersionSource: m.CommitsSinceVersionSource, CommitDate: m.CommitDate,
		UncommittedChanges: m.UncommittedChanges,
	}
	return v, nil
}

// ToDotEnv 는 dotenv 출력: `GitVersion_Major='1'` 형식.
func (v *VersionVariables) ToDotEnv() string {
	m := v.ToMap()
	var b strings.Builder
	for _, k := range SortedKeys(m) {
		fmt.Fprintf(&b, "GitVersion_%s='%s'\n", k, m[k])
	}
	return b.String()
}

// ToBuildServerEnv 는 빌드서버용 환경변수 export 라인.
func (v *VersionVariables) ToBuildServerEnv() string {
	m := v.ToMap()
	var b strings.Builder
	for _, k := range SortedKeys(m) {
		fmt.Fprintf(&b, "GitVersion_%s=%s\n", k, m[k])
	}
	return b.String()
}

// ShowVariable 은 단일 변수 값 출력. 존재하지 않으면 에러.
func (v *VersionVariables) ShowVariable(name string) (string, error) {
	m := v.ToMap()
	if val, ok := m[name]; ok {
		return val, nil
	}
	return "", fmt.Errorf("알 수 없는 변수 '%s'. 사용 가능: %s", name, strings.Join(SortedKeys(m), ", "))
}

var formatTokenRe = rx.MustCompile(`\{(?<token>[A-Za-z0-9_:]+)\}`)

// FormatTemplate 은 포맷 문자열의 `{Variable}` 치환. `{env:VAR}` 도 지원.
func (v *VersionVariables) FormatTemplate(template string, getenv func(string) string) (string, error) {
	m := v.ToMap()
	var missing string
	out := formatTokenRe.ReplaceAllFunc(template, func(mt *rx.Match) string {
		token, _ := mt.Named("token")
		if envVar, ok := strings.CutPrefix(token, "env:"); ok {
			return getenv(envVar)
		}
		if val, ok := m[token]; ok {
			return val
		}
		if missing == "" {
			missing = token
		}
		return ""
	})
	if missing != "" {
		return "", fmt.Errorf("알 수 없는 토큰 '{%s}'", missing)
	}
	return out, nil
}
