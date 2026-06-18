package semrel

import (
	"regexp"
	"strings"

	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/rx"
)

// 원본 @semantic-release/commit-analyzer + conventional-changelog 프리셋 동작.
// 버전 산출 알고리즘은 프리셋 무관(get-next-version)이고, 커밋→bump 매핑(파싱+노트+revert)만
// 프리셋마다 다르다. 여기서는 가장 널리 쓰이는 angular(기본)와 conventionalcommits 를 지원한다.
//
// 공통 releaseRules(commit-analyzer 기본): breaking→major, revert→patch,
// feat/FEAT→minor, fix/perf/FIX→patch. 프리셋은 type/breaking/revert "추출 방식"만 바꾼다.

const (
	lvlNone  = 0
	lvlPatch = 1
	lvlMinor = 2
	lvlMajor = 3
)

// Preset 은 커밋 파싱 방식(프리셋별 차이)을 캡슐화한다.
type Preset struct {
	Name       string
	headerType func(msg string) string                         // 커밋 type(없으면 "")
	breaking   func(msg string) bool                           // breaking change 인지
	revert     func(msg string) (header, hash string, ok bool) // git-style revert
}

var (
	angularHeaderRe = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?: `)
	ccHeaderRe      = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?!?: `)
	ccBangRe        = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?!: `)
	// note 정규식(parser): `^[\s|*]*(KEYWORDS)[:\s]+`, case-insensitive. body/footer 에서만.
	breakingAngularRe = regexp.MustCompile(`(?im)^[\s|*]*BREAKING CHANGE[:\s]`)
	breakingCCRe      = regexp.MustCompile(`(?im)^[\s|*]*(?:BREAKING CHANGE|BREAKING-CHANGE)[:\s]`)
	// revertPattern: angular 는 `\w{7,40}\b`, conventionalcommits 는 `\w*\.` (둘 다 i, Revert|revert:).
	revertAngularRe = regexp.MustCompile(`(?i)^(?:Revert|revert:)\s"?([\s\S]+?)"?\s*This reverts commit (\w{7,40})\b`)
	revertCCRe      = regexp.MustCompile(`(?i)^(?:Revert|revert:)\s"?([\s\S]+?)"?\s*This reverts commit (\w*)\.`)
)

func headerTypeWith(re *regexp.Regexp) func(string) string {
	return func(msg string) string {
		if m := re.FindStringSubmatch(msg); m != nil {
			return m[1]
		}
		return ""
	}
}

func revertWith(re *regexp.Regexp) func(string) (string, string, bool) {
	return func(msg string) (string, string, bool) {
		m := re.FindStringSubmatch(msg)
		if m == nil {
			return "", "", false
		}
		return strings.TrimSpace(m[1]), m[2], true
	}
}

func angularPreset() Preset {
	return Preset{
		Name:       "angular",
		headerType: headerTypeWith(angularHeaderRe),
		breaking:   func(msg string) bool { return breakingAngularRe.MatchString(body(msg)) },
		revert:     revertWith(revertAngularRe),
	}
}

func conventionalCommitsPreset() Preset {
	return Preset{
		Name:       "conventionalcommits",
		headerType: headerTypeWith(ccHeaderRe),
		breaking: func(msg string) bool {
			// 헤더의 `!`(breakingHeaderPattern) 또는 body 의 BREAKING CHANGE/BREAKING-CHANGE note.
			return ccBangRe.MatchString(firstLine(msg)) || breakingCCRe.MatchString(body(msg))
		},
		revert: revertWith(revertCCRe),
	}
}

var presets = map[string]Preset{
	"angular":             angularPreset(),
	"conventionalcommits": conventionalCommitsPreset(),
}

// PresetByName 은 이름으로 프리셋을 반환한다(별칭 허용). 알 수 없으면 angular.
func PresetByName(name string) Preset {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "angular":
		return presets["angular"]
	case "conventionalcommits", "conventional-commits", "cc":
		return presets["conventionalcommits"]
	default:
		return presets["angular"]
	}
}

// PresetFromWorkflow 는 "SemanticRelease/{preset}" 에서 프리셋을 파싱한다.
func PresetFromWorkflow(workflow string) Preset {
	name := ""
	if i := strings.Index(workflow, "/"); i >= 0 {
		name = workflow[i+1:]
	}
	return PresetByName(name)
}

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

// conventionalLevel 은 프리셋 기반 Conventional Commits 분석 레벨.
func conventionalLevel(msg string, p Preset) int {
	lvl := lvlNone
	if p.breaking(msg) {
		lvl = lvlMajor
	}
	if lvl < lvlPatch {
		if _, _, ok := p.revert(msg); ok {
			lvl = lvlPatch
		}
	}
	if lvl < lvlMinor {
		switch p.headerType(msg) {
		case "feat", "FEAT":
			lvl = lvlMinor
		case "fix", "perf", "FIX":
			if lvl < lvlPatch {
				lvl = lvlPatch
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
func analyze(commits []git.CommitInfo, ac AnalyzerConfig, p Preset) int {
	best := lvlNone
	for i := range commits {
		c := &commits[i]
		lvl := conventionalLevel(c.Message, p)
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

// filterReverted 는 commit-analyzer 의 filterRevertedCommits 처럼, 범위 내에서 revert 와
// 그 대상 커밋(header+full SHA 일치)을 쌍으로 제거한다. 대상이 범위 밖이면 revert 는 남아
// patch 로 분석된다. commits 는 최신순(newest-first)이라고 가정한다.
func filterReverted(commits []git.CommitInfo, p Preset) []git.CommitInfo {
	n := len(commits)
	isRev := make([]bool, n)
	rHeader := make([]string, n)
	rHash := make([]string, n)
	hdr := make([]string, n)
	for i := range commits {
		hdr[i] = firstLine(commits[i].Message)
		if h, hs, ok := p.revert(commits[i].Message); ok {
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
