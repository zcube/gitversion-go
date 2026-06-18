package version

// VersionField 는 증분 대상 필드. 원본 GitVersion.Core/SemVer/VersionField.cs.
//
// 정수 순서가 곧 우선순위: None < Patch < Minor < Major.
type VersionField int

const (
	FieldNone VersionField = iota
	FieldPatch
	FieldMinor
	FieldMajor
)

// String 은 출력용 이름(PascalCase).
func (f VersionField) String() string {
	switch f {
	case FieldPatch:
		return "Patch"
	case FieldMinor:
		return "Minor"
	case FieldMajor:
		return "Major"
	default:
		return "None"
	}
}

// Max 는 두 필드 중 우선순위가 높은 쪽을 반환한다.
func (f VersionField) Max(other VersionField) VersionField {
	if other > f {
		return other
	}
	return f
}
