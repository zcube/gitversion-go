package version

import (
	"strconv"
	"strings"
	"time"

	"github.com/zcube/go-gitversion/internal/rx"
)

// PreReleaseTag 는 pre-release 태그. 예: `beta.1` => Name="beta", Number=1.
// 원본 GitVersion.Core/SemVer/SemanticVersionPreReleaseTag.cs.
type PreReleaseTag struct {
	Name string
	// Number 가 nil 이면 번호 없음(Option<i64>::None).
	Number *int64
	// 이름이 비어 있어도 Number 가 있으면 태그로 취급(promote).
	PromoteTagEvenIfNameIsEmpty bool
}

func i64ptr(v int64) *int64 { return &v }

// NewPreReleaseTag 생성자.
func NewPreReleaseTag(name string, number *int64, promote bool) PreReleaseTag {
	return PreReleaseTag{Name: name, Number: number, PromoteTagEvenIfNameIsEmpty: promote}
}

// HasTag 는 의미 있는 태그가 존재하는지.
func (t PreReleaseTag) HasTag() bool {
	return t.Name != "" || (t.Number != nil && t.PromoteTagEvenIfNameIsEmpty)
}

var preReleaseParseRe = rx.MustCompile(`(?<name>.*?)\.?(?<number>\d+)?$`)

// ParsePreReleaseTag 는 원본 SemanticVersionPreReleaseTag.Parse. `beta.1`, `beta`, `1`.
// 입력이 비어있지 않으면 promote=true 로 설정한다.
func ParsePreReleaseTag(input string) PreReleaseTag {
	if strings.TrimSpace(input) == "" {
		return PreReleaseTag{}
	}
	if m, ok := preReleaseParseRe.Find(input); ok {
		name, _ := m.Named("name")
		var number *int64
		if numStr, ok := m.Named("number"); ok && numStr != "" {
			if n, err := strconv.ParseInt(numStr, 10, 64); err == nil {
				number = &n
			}
		}
		return PreReleaseTag{Name: name, Number: number, PromoteTagEvenIfNameIsEmpty: true}
	}
	return PreReleaseTag{Name: input, PromoteTagEvenIfNameIsEmpty: true}
}

// Format: 기본 포맷 `name.number`.
func (t PreReleaseTag) Format() string {
	switch {
	case t.Number != nil && t.Name != "":
		return t.Name + "." + strconv.FormatInt(*t.Number, 10)
	case t.Number != nil:
		return strconv.FormatInt(*t.Number, 10)
	default:
		return t.Name
	}
}

// Compare: 태그 없음(안정 버전) > 태그 있음(pre-release). -1/0/1.
func (t PreReleaseTag) Compare(other PreReleaseTag) int {
	a, b := t.HasTag(), other.HasTag()
	switch {
	case !a && !b:
		return 0
	case !a && b:
		return 1
	case a && !b:
		return -1
	}
	if c := strings.Compare(strings.ToLower(t.Name), strings.ToLower(other.Name)); c != 0 {
		return c
	}
	an, bn := int64(-1), int64(-1)
	if t.Number != nil {
		an = *t.Number
	}
	if other.Number != nil {
		bn = *other.Number
	}
	switch {
	case an < bn:
		return -1
	case an > bn:
		return 1
	default:
		return 0
	}
}

// BuildMetaData. commits-since-tag, branch, sha 등.
type BuildMetaData struct {
	CommitsSinceTag        *int64
	Branch                 string
	Sha                    string
	ShortSha               string
	CommitDate             *time.Time
	OtherMetadata          string
	VersionSourceSha       string
	VersionSourceDistance  int64
	UncommittedChanges     int64
	VersionSourceIncrement VersionField
}

var sanitizeBuildRe = rx.MustCompile(`[^0-9A-Za-z\-.]`)

func sanitizeBuild(s string) string {
	return sanitizeBuildRe.ReplaceAll(s, "-")
}

// FormatShort `b`: commits-since-tag 만.
func (m BuildMetaData) FormatShort() string {
	if m.CommitsSinceTag == nil {
		return ""
	}
	return strconv.FormatInt(*m.CommitsSinceTag, 10)
}

// FormatFull `f`: commits.Branch.<branch>.Sha.<sha>[.other].
func (m BuildMetaData) FormatFull() string {
	var parts []string
	if m.CommitsSinceTag != nil {
		parts = append(parts, strconv.FormatInt(*m.CommitsSinceTag, 10))
	}
	if m.Branch != "" {
		parts = append(parts, "Branch."+sanitizeBuild(m.Branch))
	}
	if m.Sha != "" {
		parts = append(parts, "Sha."+m.Sha)
	}
	if m.OtherMetadata != "" {
		parts = append(parts, sanitizeBuild(m.OtherMetadata))
	}
	return strings.Join(parts, ".")
}

// SemanticVersion 완전한 의미론적 버전.
type SemanticVersion struct {
	Major         int64
	Minor         int64
	Patch         int64
	PreReleaseTag PreReleaseTag
	BuildMetaData BuildMetaData
}

// NewSemanticVersion(major, minor, patch).
func NewSemanticVersion(major, minor, patch int64) SemanticVersion {
	return SemanticVersion{Major: major, Minor: minor, Patch: patch}
}

// MajorMinorPatch: `Major.Minor.Patch`.
func (v SemanticVersion) MajorMinorPatch() string {
	return strconv.FormatInt(v.Major, 10) + "." + strconv.FormatInt(v.Minor, 10) + "." + strconv.FormatInt(v.Patch, 10)
}

// Parse(Loose). tag_prefix 정규식으로 접두어 제거 후 파싱.
func Parse(input, tagPrefix string) (SemanticVersion, bool) {
	return ParseWith(input, tagPrefix, false)
}

var (
	strictRe = rx.MustCompile(`^(?<major>0|[1-9]\d*)\.(?<minor>0|[1-9]\d*)\.(?<patch>0|[1-9]\d*)(?:-(?<tag>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?<meta>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
	looseRe  = rx.MustCompile(`^(?<major>\d+)(\.(?<minor>\d+))?(\.(?<patch>\d+))?(\.(?<fourth>\d+))?(-(?<tag>[^+]*))?(\+(?<meta>.*))?$`)
)

// ParseWith: strict 면 Major.Minor.Patch 3요소 모두 요구. Loose 면 부분 버전 허용.
func ParseWith(input, tagPrefix string, strict bool) (SemanticVersion, bool) {
	trimmed := strings.TrimSpace(input)
	body := trimmed
	if tagPrefix != "" {
		re, err := rx.CompileCached("^(" + tagPrefix + ")")
		if err != nil {
			return SemanticVersion{}, false
		}
		body = re.ReplaceFirst(trimmed, "")
	}
	body = strings.TrimSpace(body)
	if strict {
		return parseStrict(body)
	}
	return parseLoose(body)
}

func parseStrict(body string) (SemanticVersion, bool) {
	m, ok := strictRe.Find(body)
	if !ok {
		return SemanticVersion{}, false
	}
	maj, _ := m.Named("major")
	min, _ := m.Named("minor")
	pat, _ := m.Named("patch")
	v := SemanticVersion{}
	var err error
	if v.Major, err = strconv.ParseInt(maj, 10, 64); err != nil {
		return SemanticVersion{}, false
	}
	if v.Minor, err = strconv.ParseInt(min, 10, 64); err != nil {
		return SemanticVersion{}, false
	}
	if v.Patch, err = strconv.ParseInt(pat, 10, 64); err != nil {
		return SemanticVersion{}, false
	}
	if tag, ok := m.Named("tag"); ok {
		v.PreReleaseTag = ParsePreReleaseTag(tag)
	}
	return v, true
}

func parseLoose(body string) (SemanticVersion, bool) {
	m, ok := looseRe.Find(body)
	if !ok {
		return SemanticVersion{}, false
	}
	maj, ok := m.Named("major")
	if !ok {
		return SemanticVersion{}, false
	}
	v := SemanticVersion{}
	var err error
	if v.Major, err = strconv.ParseInt(maj, 10, 64); err != nil {
		return SemanticVersion{}, false
	}
	if s, ok := m.Named("minor"); ok && s != "" {
		v.Minor, _ = strconv.ParseInt(s, 10, 64)
	}
	if s, ok := m.Named("patch"); ok && s != "" {
		v.Patch, _ = strconv.ParseInt(s, 10, 64)
	}
	if tag, ok := m.Named("tag"); ok {
		v.PreReleaseTag = ParsePreReleaseTag(tag)
	}
	if s, ok := m.Named("fourth"); ok && s != "" {
		if n, e := strconv.ParseInt(s, 10, 64); e == nil {
			v.BuildMetaData.CommitsSinceTag = &n
		}
	}
	return v, true
}

// CmpCore: 코어 버전만 비교(pre-release 무시). -1/0/1.
func (v SemanticVersion) CmpCore(other SemanticVersion) int {
	switch {
	case v.Major != other.Major:
		return cmpInt(v.Major, other.Major)
	case v.Minor != other.Minor:
		return cmpInt(v.Minor, other.Minor)
	case v.Patch != other.Patch:
		return cmpInt(v.Patch, other.Patch)
	default:
		return 0
	}
}

func cmpInt(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Compare: 코어 비교 후 pre-release 비교.
func (v SemanticVersion) Compare(other SemanticVersion) int {
	if c := v.CmpCore(other); c != 0 {
		return c
	}
	return v.PreReleaseTag.Compare(other.PreReleaseTag)
}

// Increment 는 지정 필드를 증분하고 label 을 적용. 원본 SemanticVersion.Increment.
//
// label 이 nil 이면 label 미적용. label 이 빈 문자열 포인터면 promoted pre-release.
func (v SemanticVersion) Increment(field VersionField, label *string, force bool) SemanticVersion {
	out := v
	hasPre := v.PreReleaseTag.HasTag()
	bumpCore := !hasPre || force

	if bumpCore {
		switch field {
		case FieldPatch:
			out.Patch++
		case FieldMinor:
			out.Minor++
			out.Patch = 0
		case FieldMajor:
			out.Major++
			out.Minor = 0
			out.Patch = 0
		}
	}

	if bumpCore && field != FieldNone {
		out.PreReleaseTag = PreReleaseTag{}
	}

	if label != nil {
		l := *label
		if out.PreReleaseTag.HasTag() && out.PreReleaseTag.Name == l {
			n := int64(0)
			if out.PreReleaseTag.Number != nil {
				n = *out.PreReleaseTag.Number
			}
			n++
			out.PreReleaseTag.Number = &n
		} else {
			out.PreReleaseTag = NewPreReleaseTag(l, i64ptr(1), l == "")
		}
	}
	return out
}

// String `s`: Major.Minor.Patch[-pre].
func (v SemanticVersion) String() string {
	s := v.MajorMinorPatch()
	if v.PreReleaseTag.HasTag() {
		s += "-" + v.PreReleaseTag.Format()
	}
	return s
}
