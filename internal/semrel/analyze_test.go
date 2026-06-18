package semrel

import (
	"testing"

	"github.com/zcube/gitversion-go/internal/git"
)

var ang = angularPreset()
var cc = conventionalCommitsPreset()

func TestConventionalLevelAngular(t *testing.T) {
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
		if got := conventionalLevel(msg, ang); got != want {
			t.Errorf("angular conventionalLevel(%q) = %d, want %d", msg, got, want)
		}
	}
	gitRevert := "Revert \"feat: x\"\n\nThis reverts commit 0123456789abcdef."
	if got := conventionalLevel(gitRevert, ang); got != lvlPatch {
		t.Errorf("git revert level = %d, want patch", got)
	}
}

// conventionalcommits 프리셋: ! 단축과 BREAKING-CHANGE 를 인식(angular 와 차이).
func TestConventionalLevelCC(t *testing.T) {
	cases := map[string]int{
		"feat!: x":                       lvlMajor, // ! 단축 → major
		"fix!: x":                        lvlMajor, // 타입 무관 ! → major
		"feat(api)!: x":                  lvlMajor,
		"feat: x":                        lvlMinor,
		"fix: x":                         lvlPatch,
		"feat: x\n\nBREAKING-CHANGE: y":  lvlMajor, // 하이픈 note 인식
		"feat: x\n\nBREAKING CHANGE: y":  lvlMajor,
		"feat: x\n\nBREAKING CHANGES: y": lvlMinor, // 복수형은 여전히 미인식
	}
	for msg, want := range cases {
		if got := conventionalLevel(msg, cc); got != want {
			t.Errorf("cc conventionalLevel(%q) = %d, want %d", msg, got, want)
		}
	}
}

// 기본 +semver bump 정규식(GitVersion 의미론)이 SemanticRelease 에서도 동작.
func TestRegexBumpDefault(t *testing.T) {
	acfg := AnalyzerConfig{
		BumpEnabled: true,
		MajorMsg:    `\+semver:\s?(breaking|major)`,
		MinorMsg:    `\+semver:\s?(feature|minor)`,
		PatchMsg:    `\+semver:\s?(fix|patch)`,
	}
	commits := []git.CommitInfo{{Message: "chore: c\n\n+semver: minor"}}
	if got := analyze(commits, acfg, ang); got != lvlMinor {
		t.Errorf("+semver:minor union = %d, want minor", got)
	}
	if got := analyze([]git.CommitInfo{{Message: "chore: c"}}, acfg, ang); got != lvlNone {
		t.Errorf("no +semver = %d, want none", got)
	}
}

// 사용자 정의 bump 정규식으로 SemanticRelease 커밋→bump 규칙 커스터마이즈.
func TestRegexBumpCustom(t *testing.T) {
	acfg := AnalyzerConfig{BumpEnabled: true, MinorMsg: `^Add `}
	if got := analyze([]git.CommitInfo{{Message: "Add new widget"}}, acfg, ang); got != lvlMinor {
		t.Errorf("custom minor regex = %d, want minor", got)
	}
	off := AnalyzerConfig{BumpEnabled: false, MinorMsg: `^Add `}
	if got := analyze([]git.CommitInfo{{Message: "Add new widget"}}, off, ang); got != lvlNone {
		t.Errorf("disabled regex = %d, want none", got)
	}
}

// filterReverted: revert 와 그 대상 커밋(header+full SHA 일치)을 쌍으로 제거.
func TestFilterReverted(t *testing.T) {
	feat := git.CommitInfo{Sha: "abc1234def5678", Message: "feat: x"}
	revert := git.CommitInfo{Sha: "rrrr", Message: "Revert \"feat: x\"\n\nThis reverts commit abc1234def5678."}
	fix := git.CommitInfo{Sha: "f1", Message: "fix: y"}

	got := filterReverted([]git.CommitInfo{revert, feat, fix}, ang)
	if len(got) != 1 || got[0].Sha != "f1" {
		t.Fatalf("revert+target 미제거: %+v", got)
	}

	badRevert := git.CommitInfo{Sha: "rrrr", Message: "Revert \"feat: x\"\n\nThis reverts commit 9999999999."}
	got = filterReverted([]git.CommitInfo{badRevert, fix}, ang)
	if len(got) != 2 {
		t.Fatalf("불일치 revert 는 유지되어야 함: %+v", got)
	}
}

// MergeMessageOnly: 정규식은 머지 커밋(parent>=2)에만 적용.
func TestRegexBumpMergeOnly(t *testing.T) {
	acfg := AnalyzerConfig{BumpEnabled: true, MergeOnly: true, MinorMsg: `^Add `}
	if got := analyze([]git.CommitInfo{{Message: "Add x", ParentCount: 1}}, acfg, ang); got != lvlNone {
		t.Errorf("non-merge = %d, want none", got)
	}
	if got := analyze([]git.CommitInfo{{Message: "Add x", ParentCount: 2}}, acfg, ang); got != lvlMinor {
		t.Errorf("merge = %d, want minor", got)
	}
}

func TestPresetFromWorkflow(t *testing.T) {
	if PresetFromWorkflow("SemanticRelease").Name != "angular" {
		t.Error("기본은 angular 여야 함")
	}
	if PresetFromWorkflow("SemanticRelease/conventionalcommits").Name != "conventionalcommits" {
		t.Error("conventionalcommits 선택 실패")
	}
	if PresetFromWorkflow("SemanticRelease/cc").Name != "conventionalcommits" {
		t.Error("cc 별칭 실패")
	}
	if PresetFromWorkflow("SemanticRelease/angular").Name != "angular" {
		t.Error("angular 선택 실패")
	}
}
