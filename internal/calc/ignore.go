// Package calc 는 버전 계산 엔진이다.
//
// 원본 GitVersion.Core/VersionCalculation 의 전략 → 증분 → 선택 → deployment mode
// 흐름을 옮긴다.
package calc

import (
	"strings"
	"time"

	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
)

// ignoreSet 은 버전 계산에서 제외할 커밋 집합. 원본 `ignore` 설정.
type ignoreSet struct {
	shas   map[string]bool
	before *time.Time
	paths  []string
}

func ignoreFromConfig(c *config.GitVersionConfiguration) ignoreSet {
	shas := map[string]bool{}
	for _, s := range c.Ignore.Sha {
		shas[strings.ToLower(s)] = true
	}
	var before *time.Time
	if c.Ignore.CommitsBefore != nil {
		before = parseIgnoreDate(*c.Ignore.CommitsBefore)
	}
	return ignoreSet{shas: shas, before: before, paths: append([]string(nil), c.Ignore.Paths...)}
}

func (s ignoreSet) isIgnored(sha string, when time.Time) bool {
	low := strings.ToLower(sha)
	if s.shas[low] {
		return true
	}
	for prefix := range s.shas {
		if len(prefix) >= 7 && strings.HasPrefix(low, prefix) {
			return true
		}
	}
	if s.before != nil && when.Before(*s.before) {
		return true
	}
	return false
}

// isPathIgnored: 커밋의 변경 파일이 전부 무시 경로 안에 있으면 true.
func (s ignoreSet) isPathIgnored(repo *git.GitRepo, sha string) bool {
	if len(s.paths) == 0 {
		return false
	}
	changed := repo.ChangedPathsForCommit(sha)
	// 변경 파일이 없는 커밋(예: --allow-empty)은 vacuous truth 로 무시(원본 동작).
	if len(changed) == 0 {
		return true
	}
	for _, file := range changed {
		matched := false
		for _, prefix := range s.paths {
			p := strings.TrimRight(prefix, "/")
			if file == p || strings.HasPrefix(file, p+"/") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (s ignoreSet) filter(repo *git.GitRepo, commits []git.CommitInfo) []git.CommitInfo {
	if len(s.shas) == 0 && s.before == nil && len(s.paths) == 0 {
		return commits
	}
	out := make([]git.CommitInfo, 0, len(commits))
	for _, c := range commits {
		if !s.isIgnored(c.Sha, c.When) && !s.isPathIgnored(repo, c.Sha) {
			out = append(out, c)
		}
	}
	return out
}

// parseIgnoreDate 는 yyyy-MM-ddTHH:mm:ss(혹은 날짜만) 형태를 UTC 로 파싱한다.
func parseIgnoreDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	layouts := []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return &t
		}
	}
	return nil
}
