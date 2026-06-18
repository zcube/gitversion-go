// Package semrel 은 semantic-release 와 동일한 버전 체계를 구현한다.
//
// 원본 semantic-release(refs/semantic-release) 의 lib/get-next-version.js,
// get-last-release.js + @semantic-release/commit-analyzer(angular 프리셋 + 기본
// releaseRules)의 동작을 옮긴다. 버전 산술은 node-semver(semver.inc/compare)와
// 일치시킨다.
package semrel

import (
	"regexp"
	"strconv"
	"strings"
)

// ident 는 prerelease 식별자 한 조각(숫자 또는 문자열).
type ident struct {
	str   string
	num   int
	isNum bool
}

// sv 는 최소 SemVer(빌드 메타데이터는 비교에 영향 없으므로 보관만).
type sv struct {
	major, minor, patch int
	pre                 []ident
}

var svRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.\-]+))?(?:\+([0-9A-Za-z.\-]+))?$`)

// parseSV 는 (tag-prefix 제거된) semver 문자열을 파싱한다.
func parseSV(s string) (sv, bool) {
	m := svRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return sv{}, false
	}
	var v sv
	v.major, _ = strconv.Atoi(m[1])
	v.minor, _ = strconv.Atoi(m[2])
	v.patch, _ = strconv.Atoi(m[3])
	if m[4] != "" {
		for _, p := range strings.Split(m[4], ".") {
			if n, err := strconv.Atoi(p); err == nil && !(len(p) > 1 && p[0] == '0') {
				v.pre = append(v.pre, ident{num: n, isNum: true})
			} else {
				v.pre = append(v.pre, ident{str: p})
			}
		}
	}
	return v, true
}

func (v sv) hasPre() bool { return len(v.pre) > 0 }

// channel 은 prerelease 의 채널명(첫 비숫자 식별자). 예: beta.1 -> "beta".
func (v sv) channel() string {
	if len(v.pre) > 0 && !v.pre[0].isNum {
		return v.pre[0].str
	}
	return ""
}

func (v sv) String() string {
	s := strconv.Itoa(v.major) + "." + strconv.Itoa(v.minor) + "." + strconv.Itoa(v.patch)
	if len(v.pre) > 0 {
		parts := make([]string, len(v.pre))
		for i, p := range v.pre {
			if p.isNum {
				parts[i] = strconv.Itoa(p.num)
			} else {
				parts[i] = p.str
			}
		}
		s += "-" + strings.Join(parts, ".")
	}
	return s
}

// cmpSV 는 node-semver 우선순위 비교. -1/0/1.
func cmpSV(a, b sv) int {
	if c := cmpInt(a.major, b.major); c != 0 {
		return c
	}
	if c := cmpInt(a.minor, b.minor); c != 0 {
		return c
	}
	if c := cmpInt(a.patch, b.patch); c != 0 {
		return c
	}
	// prerelease: 없음 > 있음.
	if len(a.pre) == 0 && len(b.pre) == 0 {
		return 0
	}
	if len(a.pre) == 0 {
		return 1
	}
	if len(b.pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if c := cmpIdent(a.pre[i], b.pre[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(a.pre), len(b.pre))
}

func cmpIdent(a, b ident) int {
	if a.isNum && b.isNum {
		return cmpInt(a.num, b.num)
	}
	if a.isNum {
		return -1 // 숫자 < 문자열
	}
	if b.isNum {
		return 1
	}
	return strings.Compare(a.str, b.str)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func maxSV(a, b sv) sv {
	if cmpSV(a, b) >= 0 {
		return a
	}
	return b
}

// incRelease 는 node-semver 의 semver.inc(v, "major"|"minor"|"patch") 를 구현한다.
func (v sv) incRelease(t string) sv {
	out := sv{major: v.major, minor: v.minor, patch: v.patch}
	switch t {
	case "major":
		if v.minor != 0 || v.patch != 0 || len(v.pre) == 0 {
			out.major++
		}
		out.minor, out.patch = 0, 0
	case "minor":
		if v.patch != 0 || len(v.pre) == 0 {
			out.minor++
		}
		out.patch = 0
	case "patch":
		if len(v.pre) == 0 {
			out.patch++
		}
	}
	return out
}

// incPrerelease 는 node-semver 의 semver.inc(v, "prerelease") 를 구현한다.
func (v sv) incPrerelease() sv {
	out := sv{major: v.major, minor: v.minor, patch: v.patch}
	out.pre = append([]ident(nil), v.pre...)
	if len(out.pre) == 0 {
		out.pre = []ident{{num: 0, isNum: true}}
		return out
	}
	found := false
	for i := len(out.pre) - 1; i >= 0; i-- {
		if out.pre[i].isNum {
			out.pre[i].num++
			found = true
			break
		}
	}
	if !found {
		out.pre = append(out.pre, ident{num: 0, isNum: true})
	}
	return out
}

// withPrerelease 는 코어 버전에 "<channel>.1" prerelease 를 붙인다.
func (v sv) withChannel(channel string) sv {
	out := sv{major: v.major, minor: v.minor, patch: v.patch}
	out.pre = []ident{{str: channel}, {num: 1, isNum: true}}
	return out
}
