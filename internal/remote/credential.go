package remote

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"strings"
)

// credentials 는 git credential helper 가 채운 사용자/비밀번호.
type credentials struct {
	username string
	password string
}

// credentialQuery 는 git credential 프로토콜 입력(protocol/host/path) 한 묶음.
type credentialQuery struct {
	protocol string
	host     string
	path     string
}

// parseCredentialQuery 는 https URL 에서 protocol/host/path 를 추출한다. https 가
// 아니면 (zero,false).
func parseCredentialQuery(rawURL string) (credentialQuery, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return credentialQuery{}, false
	}
	return credentialQuery{
		protocol: u.Scheme,
		host:     u.Host,
		path:     strings.TrimPrefix(u.Path, "/"),
	}, true
}

func (q credentialQuery) input(userHint string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "protocol=%s\n", q.protocol)
	fmt.Fprintf(&b, "host=%s\n", q.host)
	if q.path != "" {
		fmt.Fprintf(&b, "path=%s\n", q.path)
	}
	if userHint != "" {
		fmt.Fprintf(&b, "username=%s\n", userHint)
	}
	b.WriteString("\n")
	return b.String()
}

// runGitCredential 은 `git credential <action>` 를 입력과 함께 실행하고 stdout 을 반환한다.
func runGitCredential(action, dir, input string) (string, error) {
	cmd := exec.Command("git", "credential", action)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// fillCredentials 는 `git credential fill` 로 설정된 helper(osxkeychain/GCM/libsecret 등)
// 에서 자격증명을 가져온다. 성공하면 (creds, true).
func fillCredentials(rawURL, userHint, dir string) (credentials, bool) {
	q, ok := parseCredentialQuery(rawURL)
	if !ok {
		return credentials{}, false
	}
	out, err := runGitCredential("fill", dir, q.input(userHint))
	if err != nil {
		slog.Debug("git credential fill 실패: " + err.Error())
		return credentials{}, false
	}
	var c credentials
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "username":
			c.username = v
		case "password":
			c.password = v
		}
	}
	if c.password == "" {
		return credentials{}, false
	}
	return c, true
}

// approveCredentials 는 인증 성공 시 helper 에 자격증명 저장/갱신을 알린다(get/store).
func approveCredentials(rawURL string, c credentials, dir string) {
	q, ok := parseCredentialQuery(rawURL)
	if !ok {
		return
	}
	input := q.protocolHostPath() + fmt.Sprintf("username=%s\npassword=%s\n\n", c.username, c.password)
	_, _ = runGitCredential("approve", dir, input)
}

// rejectCredentials 는 인증 실패 시 helper 에서 잘못된 자격증명을 지운다(erase).
func rejectCredentials(rawURL string, c credentials, dir string) {
	q, ok := parseCredentialQuery(rawURL)
	if !ok {
		return
	}
	input := q.protocolHostPath() + fmt.Sprintf("username=%s\npassword=%s\n\n", c.username, c.password)
	_, _ = runGitCredential("reject", dir, input)
}

func (q credentialQuery) protocolHostPath() string {
	var b strings.Builder
	fmt.Fprintf(&b, "protocol=%s\nhost=%s\n", q.protocol, q.host)
	if q.path != "" {
		fmt.Fprintf(&b, "path=%s\n", q.path)
	}
	return b.String()
}
