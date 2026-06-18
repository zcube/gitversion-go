// Package varutil 은 워크플로 계산기들이 공유하는 출력 변수 유틸리티를 모은다.
// (날짜 포맷 변환, AssemblyVersion 스킴, 브랜치명 escape 등 — GitVersion/SemanticRelease
// 양쪽에서 동일하게 쓰여 중복을 제거한다.)
package varutil

import (
	"fmt"
	"strings"

	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/rx"
)

// DotNetDateToGoLayout 은 .NET 날짜 포맷을 Go reference 레이아웃으로 변환한다.
func DotNetDateToGoLayout(format string) string {
	out := format
	for _, p := range [][2]string{
		{"yyyy", "2006"}, {"yy", "06"},
		{"MMMM", "January"}, {"MMM", "Jan"}, {"MM", "01"},
		{"dddd", "Monday"}, {"ddd", "Mon"}, {"dd", "02"},
		{"HH", "15"}, {"mm", "04"}, {"ss", "05"},
	} {
		out = strings.ReplaceAll(out, p[0], p[1])
	}
	if out == "" {
		return "2006-01-02"
	}
	return out
}

// AssemblyVersion 은 AssemblyVersion/AssemblyFileVersion 스킴을 적용한다.
func AssemblyVersion(major, minor, patch, pre int64, scheme config.VersioningScheme) string {
	switch scheme {
	case config.SchemeMajor:
		return fmt.Sprintf("%d.0.0.0", major)
	case config.SchemeMajorMinor:
		return fmt.Sprintf("%d.%d.0.0", major, minor)
	case config.SchemeMajorMinorPatch:
		return fmt.Sprintf("%d.%d.%d.0", major, minor, patch)
	case config.SchemeMajorMinorPatchTag:
		return fmt.Sprintf("%d.%d.%d.%d", major, minor, patch, pre)
	default:
		return ""
	}
}

var escapeBranchRe = rx.MustCompile(`[^a-zA-Z0-9-]`)

// EscapeBranchName 은 브랜치명의 비영숫자 문자를 `-` 로 치환한다.
func EscapeBranchName(name string) string {
	return escapeBranchRe.ReplaceAll(name, "-")
}
