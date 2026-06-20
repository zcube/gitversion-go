package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseCredentialQuery(t *testing.T) {
	q, ok := parseCredentialQuery("https://github.com/owner/repo.git")
	if !ok || q.protocol != "https" || q.host != "github.com" || q.path != "owner/repo.git" {
		t.Fatalf("parse https = %+v ok=%v", q, ok)
	}
	if _, ok := parseCredentialQuery("git@host:repo.git"); ok {
		t.Error("scp-like 는 https 가 아니므로 false 여야 함")
	}
	if _, ok := parseCredentialQuery("ssh://host/repo.git"); ok {
		t.Error("ssh 는 false 여야 함")
	}
}

// 실제 `git credential fill` 프로토콜 왕복: 임시 store helper 를 구성해 자격증명을
// 조회한다.
func TestCredentialHelperFill(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 명령이 없어 건너뜀")
	}
	dir := t.TempDir()
	store := filepath.Join(dir, "creds")
	if err := os.WriteFile(store, []byte("https://alice:s3cr3t@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gcfg := filepath.Join(dir, "gitconfig")
	cfg := "[credential]\n\thelper = store --file=" + filepath.ToSlash(store) + "\n"
	if err := os.WriteFile(gcfg, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// 격리: 전역/시스템 git config 를 임시 파일로 대체.
	t.Setenv("GIT_CONFIG_GLOBAL", gcfg)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	c, ok := fillCredentials("https://example.com/owner/repo.git", "", dir)
	if !ok {
		t.Fatal("credential fill 실패(자격증명 미조회)")
	}
	if c.username != "alice" || c.password != "s3cr3t" {
		t.Fatalf("자격증명 불일치: %+v", c)
	}
}
