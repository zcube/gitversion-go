package semrel

import (
	"regexp"
	"strings"

	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/rx"
)

// 원본 @semantic-release/commit-analyzer + conventional-changelog-angular 동작:
//   - 헤더 `type(scope): subject` 로 type 추출(`!` 단축 breaking 미지원 — 설치된
//     angular 프리셋과 동일).
//   - BREAKING CHANGE footer note → major.
//   - git-style revert("Revert \"...\"\n\nThis reverts commit X.") → patch.
//   - feat/FEAT → minor, fix/perf/FIX → patch, breaking → major, revert → patch.
//
// 추가로, 기존 GitVersion 과 의미론적으로 동일한 커밋-메시지 bump 정규식
// (major/minor/patch-version-bump-message)을 합집합(최고 우선)으로 적용한다. 기본값은
// `+semver:` 패턴이라 Conventional Commits 결과를 바꾸지 않으며, 사용자가 정규식을
// 바꾸면 SemanticRelease 에서도 커밋→bump 규칙을 커스터마이즈할 수 있다.

const (
	lvlNone  = 0
	lvlPatch = 1
	lvlMinor = 2
	lvlMajor = 3
)

var (
	headerRe = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?: `)
	// angular noteKeywords=['BREAKING CHANGE'] + parser note regex
	// `^[\s|*]*(BREAKING CHANGE)[:\s]+` (case-insensitive). 복수형/하이픈은 미지원.
	// 헤더가 아닌 body/footer 에서만 인식하므로 첫 줄을 제외한 본문에서 찾는다.
	breakingRe = regexp.MustCompile(`(?im)^[\s|*]*BREAKING CHANGE[:\s]`)
	// angular revertPattern: `^(?:Revert|revert:)\s"?(header)"?\s*This reverts commit (hash 7-40)\b` (i).
	revertRe = regexp.MustCompile(`(?i)^(?:Revert|revert:)\s"?([\s\S]+?)"?\s*This reverts commit (\w{7,40})\b`)
)

func firstLine(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}
	return strings.TrimSpace(msg)
}

func body(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return msg[i+1:]
	}
	return ""
}

// revertInfo 는 git-style revert 의 (reverted header, reverted hash) 를 추출한다.
func revertInfo(msg string) (header, hash string, ok bool) {
	m := revertRe.FindStringSubmatch(msg)
	if m == nil {
		return "", "", false
	}
	return strings.TrimSpace(m[1]), m[2], true
}

// filterReverted 는 commit-analyzer 의 filterRevertedCommits 처럼, 범위 내에서 revert 와
// 그 대상 커밋(header+full SHA 일치)을 쌍으로 제거한다. 대상이 범위 밖이면 revert 는 남아
// patch 로 분석된다. commits 는 최신순(newest-first)이라고 가정한다.
func filterReverted(commits []git.CommitInfo) []git.CommitInfo {
	n := len(commits)
	isRev := make([]bool, n)
	rHeader := make([]string, n)
	rHash := make([]string, n)
	hdr := make([]string, n)
	for i := range commits {
		hdr[i] = firstLine(commits[i].Message)
		if h, hs, ok := revertInfo(commits[i].Message); ok {
			isRev[i], rHeader[i], rHash[i] = true, h, hs
		}
	}
	consumed := make([]bool, n)
	for i := 0; i < n; i++ {
		if !isRev[i] || consumed[i] {
			continue
		}
		for j := 0; j < n; j++ {
			if j == i || consumed[j] {
				continue
			}
			if hdr[j] == rHeader[i] && commits[j].Sha == rHash[i] {
				consumed[i], consumed[j] = true, true
				break
			}
		}
	}
	out := make([]git.CommitInfo, 0, n)
	for i := range commits {
		if !consumed[i] {
			out = append(out, commits[i])
		}
	}
	return out
}

// AnalyzerConfig 는 커밋 분석 설정(EffectiveConfiguration 에서 파생).
type AnalyzerConfig struct {
	TagPrefix   string
	BumpEnabled bool // commit-message-incrementing != Disabled
	MergeOnly   bool // commit-message-incrementing == MergeMessageOnly
	MajorMsg    string
	MinorMsg    string
	PatchMsg    string
}

// FromEffective 는 EffectiveConfiguration 으로부터 분석 설정을 만든다.
func FromEffective(eff *config.EffectiveConfiguration) AnalyzerConfig {
	return AnalyzerConfig{
		TagPrefix:   eff.TagPrefix,
		BumpEnabled: eff.CommitMessageIncrementing != config.CommitMsgDisabled,
		MergeOnly:   eff.CommitMessageIncrementing == config.CommitMsgMergeMessageOnly,
		MajorMsg:    eff.MajorBumpMessage,
		MinorMsg:    eff.MinorBumpMessage,
		PatchMsg:    eff.PatchBumpMessage,
	}
}

// conventionalLevel 은 Conventional Commits(angular) 분석 레벨.
func conventionalLevel(msg string) int {
	lvl := lvlNone
	// BREAKING CHANGE note 는 헤더가 아닌 body/footer 에서만 인식.
	if breakingRe.MatchString(body(msg)) {
		lvl = lvlMajor
	}
	if lvl < lvlPatch && revertRe.MatchString(msg) {
		lvl = lvlPatch
	}
	if lvl < lvlMinor {
		if m := headerRe.FindStringSubmatch(msg); m != nil {
			switch m[1] {
			case "feat", "FEAT":
				lvl = lvlMinor
			case "fix", "perf", "FIX":
				if lvl < lvlPatch {
					lvl = lvlPatch
				}
			}
		}
	}
	return lvl
}

// regexLevel 은 GitVersion 의미론의 bump 정규식 레벨(매칭 없으면 none, no-bump 는 무시).
func regexLevel(msg string, ac AnalyzerConfig) int {
	test := func(pat string) bool {
		if pat == "" {
			return false
		}
		re, err := rx.CompileCached("(?im)" + pat)
		return err == nil && re.MatchString(msg)
	}
	switch {
	case test(ac.MajorMsg):
		return lvlMajor
	case test(ac.MinorMsg):
		return lvlMinor
	case test(ac.PatchMsg):
		return lvlPatch
	default:
		return lvlNone
	}
}

// analyze 는 커밋 묶음의 최고 릴리스 레벨을 반환한다(Conventional ∪ bump 정규식).
func analyze(commits []git.CommitInfo, ac AnalyzerConfig) int {
	best := lvlNone
	for i := range commits {
		c := &commits[i]
		lvl := conventionalLevel(c.Message)
		if ac.BumpEnabled && (!ac.MergeOnly || c.ParentCount >= 2) {
			if r := regexLevel(c.Message, ac); r > lvl {
				lvl = r
			}
		}
		if lvl > best {
			best = lvl
		}
	}
	return best
}

func levelName(lvl int) string {
	switch lvl {
	case lvlMajor:
		return "major"
	case lvlMinor:
		return "minor"
	case lvlPatch:
		return "patch"
	default:
		return ""
	}
}
