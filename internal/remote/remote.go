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
	auth, creds, usedHelper, err := authMethod(url, opts.Username, opts.Password, base)
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
		// credential helper 로 받은 자격증명이 인증에 실패했으면 helper 에서 지운다(erase).
		if usedHelper && isAuthError(err) {
			rejectCredentials(opts.URL, creds, base)
		}
		return "", fmt.Errorf("clone 실패 %s: %w", opts.URL, err)
	}
	// 성공 시 helper 에 자격증명 저장/갱신(store).
	if usedHelper {
		approveCredentials(opts.URL, creds, base)
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

// authMethod 는 URL 유형에 맞는 인증 방법을 반환한다.
//
//   - https: -u/-p 가 모두 있으면 그대로 BasicAuth. 아니면 git credential helper
//     (osxkeychain/GCM/libsecret 등)로 자격증명을 조회해 사용한다.
//   - ssh: 시스템 ssh-agent.
//
// 반환: (auth, helper 로 받은 자격증명, helper 사용 여부, error).
func authMethod(url, user, pass, dir string) (transport.AuthMethod, credentials, bool, error) {
	switch {
	case strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://"):
		if user != "" && pass != "" {
			return &http.BasicAuth{Username: user, Password: pass}, credentials{}, false, nil
		}
		// -u/-p 가 불완전하면 credential helper 로 보충(user 는 힌트로 전달).
		if c, ok := fillCredentials(url, user, dir); ok {
			slog.Debug("git credential helper 로 자격증명 조회 성공: " + c.username)
			return &http.BasicAuth{Username: c.username, Password: c.password}, c, true, nil
		}
		if user != "" || pass != "" {
			return &http.BasicAuth{Username: user, Password: pass}, credentials{}, false, nil
		}
		return nil, credentials{}, false, nil
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
			return nil, credentials{}, false, nil // 에이전트가 없으면 기본 동작에 맡긴다.
		}
		return auth, credentials{}, false, nil
	default:
		return nil, credentials{}, false, nil
	}
}

// isAuthError 는 clone 에러가 인증 실패인지 추정한다.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	if err == transport.ErrAuthenticationRequired || err == transport.ErrAuthorizationFailed {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication") || strings.Contains(msg, "authorization") ||
		strings.Contains(msg, "401") || strings.Contains(msg, "403")
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
