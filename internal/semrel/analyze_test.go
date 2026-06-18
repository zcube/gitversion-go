package semrel

import (
	"testing"

	"github.com/zcube/gitversion-go/internal/git"
)

func TestConventionalLevel(t *testing.T) {
	cases := map[string]int{
		"feat: x":                        lvlMinor,
		"feat(api): x":                   lvlMinor,
		"fix: x":                         lvlPatch,
		"perf: x":                        lvlPatch,
		"docs: x":                        lvlNone,
		"chore: x":                       lvlNone,
		"feat!: x":                       lvlNone, // angular 프리셋: ! 미지원
		"revert: x":                      lvlNone, // conventional revert 는 무시
		"feat: x\n\nBREAKING CHANGE: y":  lvlMajor,
		"chore: c\n\nBREAKING CHANGE: y": lvlMajor,
		// angular: 복수형/하이픈은 note 로 인식 안 됨(feat 만 minor), 소문자는 인식(major).
		"feat: x\n\nBREAKING CHANGES: y":  lvlMinor,
		"feat: x\n\nBREAKING-CHANGE: y":   lvlMinor,
		"fix: x\n\nbreaking change: y":    lvlMajor,
		"fix: BREAKING CHANGE in subject": lvlPatch, // 헤더(subject)의 문구는 note 아님
	}
	for msg, want := range cases {
		if got := conventionalLevel(msg); got != want {
			t.Errorf("conventionalLevel(%q) = %d, want %d", msg, got, want)
		}
	}
	gitRevert := "Revert \"feat: x\"\n\nThis reverts commit 0123456789abcdef."
	if got := conventionalLevel(gitRevert); got != lvlPatch {
		t.Errorf("git revert level = %d, want patch", got)
	}
}

// 기본 +semver bump 정규식(GitVersion 의미론)이 SemanticRelease 에서도 동작.
func TestRegexBumpDefault(t *testing.T) {
	ac := AnalyzerConfig{
		BumpEnabled: true,
		MajorMsg:    `\+semver:\s?(breaking|major)`,
		MinorMsg:    `\+semver:\s?(feature|minor)`,
		PatchMsg:    `\+semver:\s?(fix|patch)`,
	}
	// chore 는 conventional none 이지만 +semver footer 로 bump 가 올라간다(합집합).
	commits := []git.CommitInfo{{Message: "chore: c\n\n+semver: minor"}}
	if got := analyze(commits, ac); got != lvlMinor {
		t.Errorf("+semver:minor union = %d, want minor", got)
	}
	// +semver 없는 일반 커밋은 정규식 영향 없음.
	if got := analyze([]git.CommitInfo{{Message: "chore: c"}}, ac); got != lvlNone {
		t.Errorf("no +semver = %d, want none", got)
	}
}

// 사용자 정의 bump 정규식으로 SemanticRelease 커밋→bump 규칙 커스터마이즈.
func TestRegexBumpCustom(t *testing.T) {
	ac := AnalyzerConfig{BumpEnabled: true, MinorMsg: `^Add `}
	// 비-conventional 커밋이지만 커스텀 정규식으로 minor.
	if got := analyze([]git.CommitInfo{{Message: "Add new widget"}}, ac); got != lvlMinor {
		t.Errorf("custom minor regex = %d, want minor", got)
	}
	// BumpEnabled=false 면 정규식 미적용(commit-message-incrementing=Disabled).
	off := AnalyzerConfig{BumpEnabled: false, MinorMsg: `^Add `}
	if got := analyze([]git.CommitInfo{{Message: "Add new widget"}}, off); got != lvlNone {
		t.Errorf("disabled regex = %d, want none", got)
	}
}

// filterReverted: revert 와 그 대상 커밋(header+full SHA 일치)을 쌍으로 제거.
func TestFilterReverted(t *testing.T) {
	feat := git.CommitInfo{Sha: "abc1234def5678", Message: "feat: x"}
	revert := git.CommitInfo{Sha: "rrrr", Message: "Revert \"feat: x\"\n\nThis reverts commit abc1234def5678."}
	fix := git.CommitInfo{Sha: "f1", Message: "fix: y"}

	// 최신순: revert, feat, fix → revert+feat 제거, fix 만 남음.
	got := filterReverted([]git.CommitInfo{revert, feat, fix})
	if len(got) != 1 || got[0].Sha != "f1" {
		t.Fatalf("revert+target 미제거: %+v", got)
	}

	// SHA 불일치(범위 밖 대상) → revert 유지.
	badRevert := git.CommitInfo{Sha: "rrrr", Message: "Revert \"feat: x\"\n\nThis reverts commit 9999999999."}
	got = filterReverted([]git.CommitInfo{badRevert, fix})
	if len(got) != 2 {
		t.Fatalf("불일치 revert 는 유지되어야 함: %+v", got)
	}
}

// MergeMessageOnly: 정규식은 머지 커밋(parent>=2)에만 적용.
func TestRegexBumpMergeOnly(t *testing.T) {
	ac := AnalyzerConfig{BumpEnabled: true, MergeOnly: true, MinorMsg: `^Add `}
	if got := analyze([]git.CommitInfo{{Message: "Add x", ParentCount: 1}}, ac); got != lvlNone {
		t.Errorf("non-merge = %d, want none", got)
	}
	if got := analyze([]git.CommitInfo{{Message: "Add x", ParentCount: 2}}, ac); got != lvlMinor {
		t.Errorf("merge = %d, want minor", got)
	}
}
