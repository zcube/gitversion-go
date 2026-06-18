// Package rx 는 .NET 호환 정규식 엔진(regexp2)을 감싸 GitVersion 의 정규식 동작을
// 충실히 재현한다. 원본 GitVersion 은 .NET regex 를 쓰고, 설정/머지 포맷의 named
// capture `(?<name>...)` 와 backtracking 의미를 그대로 따른다. Go 표준 regexp(RE2)
// 대신 regexp2 를 사용해 .NET 과 동일한 매칭 결과를 얻는다.
package rx

import (
	"sync"

	"github.com/dlclark/regexp2"
)

// Regexp 는 regexp2.Regexp 래퍼.
type Regexp struct {
	re *regexp2.Regexp
}

// Match 는 단일 매칭 결과 래퍼.
type Match struct {
	m *regexp2.Match
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*Regexp{}
)

// Compile 은 .NET 문법 정규식을 컴파일한다.
func Compile(pattern string) (*Regexp, error) {
	re, err := regexp2.Compile(pattern, regexp2.None)
	if err != nil {
		return nil, err
	}
	return &Regexp{re: re}, nil
}

// MustCompile 은 컴파일 실패 시 panic. 내장 상수 패턴 전용.
func MustCompile(pattern string) *Regexp {
	re := regexp2.MustCompile(pattern, regexp2.None)
	return &Regexp{re: re}
}

// CompileCached 는 컴파일 결과를 캐시한다.
func CompileCached(pattern string) (*Regexp, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if r, ok := cache[pattern]; ok {
		return r, nil
	}
	r, err := Compile(pattern)
	if err != nil {
		return nil, err
	}
	cache[pattern] = r
	return r, nil
}

// IsValid 는 컴파일 가능한지만 확인한다.
func IsValid(pattern string) bool {
	_, err := regexp2.Compile(pattern, regexp2.None)
	return err == nil
}

// MatchString 은 어디서든 매칭되면 true(에러 시 false).
func (r *Regexp) MatchString(s string) bool {
	ok, err := r.re.MatchString(s)
	return err == nil && ok
}

// Find 는 첫 매칭을 반환한다. 매칭이 없으면 (nil, false).
func (r *Regexp) Find(s string) (*Match, bool) {
	m, err := r.re.FindStringMatch(s)
	if err != nil || m == nil {
		return nil, false
	}
	return &Match{m: m}, true
}

// ReplaceAll 은 모든 매칭을 repl 로 치환한다($ 치환 구문 적용).
func (r *Regexp) ReplaceAll(s, repl string) string {
	out, err := r.re.Replace(s, repl, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// ReplaceFirst 는 첫 매칭만 repl 로 치환한다(원본 .NET Regex.Replace(count=1)).
func (r *Regexp) ReplaceFirst(s, repl string) string {
	out, err := r.re.Replace(s, repl, -1, 1)
	if err != nil {
		return s
	}
	return out
}

// ReplaceAllFunc 은 모든 매칭을 evaluator 결과로 치환한다.
func (r *Regexp) ReplaceAllFunc(s string, eval func(*Match) string) string {
	out, err := r.re.ReplaceFunc(s, func(m regexp2.Match) string {
		mm := m
		return eval(&Match{m: &mm})
	}, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// Whole 은 전체 매칭 문자열을 반환한다.
func (m *Match) Whole() string {
	return m.m.String()
}

// Named 는 named capture 그룹의 값과 "참여 여부"를 반환한다. .NET 처럼 미참여
// 그룹은 (false) 로 구분된다(빈 캡처와 미참여를 구분).
func (m *Match) Named(name string) (string, bool) {
	g := m.m.GroupByName(name)
	if g == nil || len(g.Captures) == 0 {
		return "", false
	}
	return g.String(), true
}

// Group 은 인덱스 그룹 값과 참여 여부를 반환한다.
func (m *Match) Group(idx int) (string, bool) {
	g := m.m.GroupByNumber(idx)
	if g == nil || len(g.Captures) == 0 {
		return "", false
	}
	return g.String(), true
}

// NamedGroups 는 참여한 named capture 그룹(이름->값)을 반환한다. 숫자 이름(무명
// 그룹)은 제외한다.
func (m *Match) NamedGroups() map[string]string {
	out := map[string]string{}
	for _, g := range m.m.Groups() {
		if g.Name == "" || isNumericName(g.Name) || len(g.Captures) == 0 {
			continue
		}
		out[g.Name] = g.String()
	}
	return out
}

func isNumericName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
