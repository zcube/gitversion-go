package semrel

import "regexp"

// 원본 @semantic-release/commit-analyzer + conventional-changelog-angular 동작:
//   - 헤더 `type(scope): subject` 로 type 추출(`!` 단축 breaking 미지원 — 설치된
//     angular 프리셋과 동일).
//   - BREAKING CHANGE footer note → major.
//   - git-style revert("Revert \"...\"\n\nThis reverts commit X.") → patch.
//   - 기본 releaseRules 중 angular 가 채우는 필드에 매칭되는 것:
//     feat/FEAT → minor, fix/perf/FIX → patch, breaking → major, revert → patch.

const (
	lvlNone  = 0
	lvlPatch = 1
	lvlMinor = 2
	lvlMajor = 3
)

var (
	headerRe    = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?: `)
	breakingRe  = regexp.MustCompile(`(?m)^[ \t*]*BREAKING CHANGES?[: ]`)
	revertGitRe = regexp.MustCompile(`(?s)^Revert ".*"\s*This reverts commit \w+\.`)
)

// commitLevel 은 한 커밋 메시지의 릴리스 레벨을 반환한다(0=none..3=major).
func commitLevel(msg string) int {
	lvl := lvlNone
	if breakingRe.MatchString(msg) {
		lvl = lvlMajor
	}
	if lvl < lvlPatch && revertGitRe.MatchString(msg) {
		lvl = lvlPatch
	}
	if lvl < lvlMinor {
		if m := headerRe.FindStringSubmatch(msg); m != nil {
			switch m[1] {
			case "feat", "FEAT":
				if lvl < lvlMinor {
					lvl = lvlMinor
				}
			case "fix", "perf", "FIX":
				if lvl < lvlPatch {
					lvl = lvlPatch
				}
			}
		}
	}
	return lvl
}

// analyzeLevel 은 커밋 묶음의 최고 릴리스 레벨을 반환한다.
func analyzeLevel(messages []string) int {
	best := lvlNone
	for _, m := range messages {
		if l := commitLevel(m); l > best {
			best = l
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
