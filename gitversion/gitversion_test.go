package gitversion_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zcube/gitversion-go/gitversion"
)

// extractScenario 는 testdata/fixtures.tar.gz 에서 한 시나리오 저장소를 푼다.
func extractScenario(t *testing.T, scenario string) string {
	t.Helper()
	archive := filepath.Join("..", "testdata", "fixtures.tar.gz")
	f, err := os.Open(archive)
	if err != nil {
		t.Fatalf("fixture 열기 실패: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	tr := tar.NewReader(gz)
	prefix := scenario + "/"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if name != scenario && !strings.HasPrefix(name, prefix) {
			continue
		}
		target := filepath.Join(dest, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.Create(target)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(out, tr)
			out.Close()
		}
	}
	return filepath.Join(dest, scenario)
}

func TestPublicAPICompute(t *testing.T) {
	dir := extractScenario(t, "main_tag_plus2")

	v, err := gitversion.Calculate(gitversion.Options{Path: dir, NoCache: true})
	if err != nil {
		t.Fatalf("Calculate 실패: %v", err)
	}

	// golden 과 핵심 필드 대조.
	expBytes, _ := os.ReadFile(filepath.Join(dir, "expected.json"))
	var exp map[string]any
	_ = json.Unmarshal(expBytes, &exp)

	if v.FullSemVer != exp["FullSemVer"] {
		t.Errorf("FullSemVer = %q, want %q", v.FullSemVer, exp["FullSemVer"])
	}
	if got := v.ToMap()["SemVer"]; got != exp["SemVer"] {
		t.Errorf("SemVer = %q, want %q", got, exp["SemVer"])
	}
	// 단일 변수 조회.
	if _, err := v.ShowVariable("MajorMinorPatch"); err != nil {
		t.Errorf("ShowVariable: %v", err)
	}
	// JSON 직렬화.
	if _, err := v.ToJSON(); err != nil {
		t.Errorf("ToJSON: %v", err)
	}
}

func TestPublicAPIConfigInput(t *testing.T) {
	dir := extractScenario(t, "main_tag_plus2") // 기본은 1.0.1-2(main)

	// 1) 인라인 YAML 로 next-version 주입 → ConfiguredNextVersion 이 더 높은 5.0.0 채택.
	v, err := gitversion.Calculate(gitversion.Options{
		Path:       dir,
		ConfigYAML: []byte("next-version: 5.0.0\n"),
		NoCache:    true,
	})
	if err != nil {
		t.Fatalf("ConfigYAML 계산 실패: %v", err)
	}
	if v.Major != 5 {
		t.Errorf("ConfigYAML next-version 미반영: Major=%d (want 5), FullSemVer=%s", v.Major, v.FullSemVer)
	}

	// 2) 설정 객체(프리셋 조정)로 동일 효과.
	cfg := gitversion.DefaultConfig("GitFlow")
	nv := "7.0.0"
	cfg.NextVersion = &nv
	v2, err := gitversion.Calculate(gitversion.Options{Path: dir, Config: cfg, NoCache: true})
	if err != nil {
		t.Fatalf("Config 객체 계산 실패: %v", err)
	}
	if v2.Major != 7 {
		t.Errorf("Config 객체 next-version 미반영: Major=%d (want 7)", v2.Major)
	}

	// 3) ParseConfig 헬퍼.
	if _, err := gitversion.ParseConfig([]byte("workflow: GitHubFlow/v1\n")); err != nil {
		t.Errorf("ParseConfig: %v", err)
	}
}

// Example 은 공개 API 사용법을 보여준다.
func ExampleCompute() {
	v, err := gitversion.Compute(".")
	if err != nil {
		fmt.Println("not a git repo")
		return
	}
	fmt.Println(v.FullSemVer)
}
