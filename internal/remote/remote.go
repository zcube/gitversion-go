// Package remote 는 동적 원격 저장소 clone 을 제공한다.
//
// 원본 GitVersion.Core/Core/GitPreparer.cs 의 동적 저장소 동작 대응. --url 로 원격
// 저장소를 임시(또는 지정) 위치에 clone 하고 그 위에서 버전을 계산한다.
package remote

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// Options 는 동적 clone 옵션.
type Options struct {
	URL      string
	Branch   string
	Username string
	Password string
	Commit   string
	Location string
}

// Prepare 는 원격 저장소를 clone 하고 대상 브랜치/커밋을 체크아웃한 뒤 작업 트리
// 경로를 반환한다.
func Prepare(opts *Options) (string, error) {
	if opts.Branch == "" {
		return "", fmt.Errorf("동적 저장소에는 --branch 가 필요합니다")
	}

	base := opts.Location
	if base == "" {
		base = os.TempDir()
	}
	sum := sha1.Sum([]byte(opts.URL))
	dest := filepath.Join(base, "gitversion-dynamic-"+hex.EncodeToString(sum[:]))

	// 항상 깨끗한 상태에서 clone(정확성 우선).
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return "", fmt.Errorf("기존 디렉터리 삭제 실패 %s: %w", dest, err)
		}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}

	url := injectCredentials(opts.URL, opts.Username, opts.Password)
	auth, err := authMethod(url, opts.Username, opts.Password)
	if err != nil {
		return "", err
	}

	slog.Info(fmt.Sprintf("clone 중: %s (branch=%s) -> %s", opts.URL, opts.Branch, dest))
	repo, err := git.PlainClone(dest, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(opts.Branch),
		SingleBranch:  true,
		Auth:          auth,
	})
	if err != nil {
		return "", fmt.Errorf("clone 실패 %s: %w", opts.URL, err)
	}

	if opts.Commit != "" {
		if err := detachHeadToCommit(repo, opts.Commit); err != nil {
			return "", err
		}
	}
	return dest, nil
}

// injectCredentials 는 인증 정보를 URL 에 반영한다.
func injectCredentials(url, user, pass string) string {
	if user == "" {
		return url
	}
	if rest, ok := strings.CutPrefix(url, "https://"); ok {
		cred := user
		if pass != "" {
			cred = user + ":" + pass
		}
		return "https://" + cred + "@" + rest
	}
	if rest, ok := strings.CutPrefix(url, "ssh://"); ok {
		hostPart := rest
		if i := strings.Index(rest, "/"); i >= 0 {
			hostPart = rest[:i]
		}
		if !strings.Contains(hostPart, "@") {
			return "ssh://" + user + "@" + rest
		}
	}
	return url
}

// authMethod 는 URL 유형에 맞는 인증 방법을 반환한다. https 는 BasicAuth,
// ssh 는 시스템 ssh-agent 를 사용한다.
func authMethod(url, user, pass string) (transport.AuthMethod, error) {
	switch {
	case strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://"):
		if user != "" || pass != "" {
			return &http.BasicAuth{Username: user, Password: pass}, nil
		}
		return nil, nil
	case strings.HasPrefix(url, "ssh://") || isSCPLike(url):
		sshUser := "git"
		if i := strings.Index(url, "@"); i >= 0 {
			scheme := strings.TrimPrefix(url, "ssh://")
			if j := strings.Index(scheme, "@"); j >= 0 {
				sshUser = scheme[:j]
			}
		}
		auth, err := ssh.NewSSHAgentAuth(sshUser)
		if err != nil {
			return nil, nil // 에이전트가 없으면 기본 동작에 맡긴다.
		}
		return auth, nil
	default:
		return nil, nil
	}
}

func isSCPLike(url string) bool {
	// `git@host:path` 형태(스킴 없음, @ 와 : 포함).
	return !strings.Contains(url, "://") && strings.Contains(url, "@") && strings.Contains(url, ":")
}

// detachHeadToCommit 은 clone 된 저장소의 HEAD 를 지정 커밋으로 detach 한다.
func detachHeadToCommit(repo *git.Repository, commit string) error {
	hash, err := repo.ResolveRevision(plumbing.Revision(commit))
	if err != nil || hash == nil {
		return fmt.Errorf("커밋을 찾지 못했습니다: %s", commit)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return fmt.Errorf("커밋 체크아웃 실패 %s: %w", commit, err)
	}
	slog.Info("HEAD 를 커밋으로 설정: " + hash.String())
	return nil
}
