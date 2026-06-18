package gogitversion_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/zcube/go-gitversion/internal/calc"
	"github.com/zcube/go-gitversion/internal/config"
	"github.com/zcube/go-gitversion/internal/git"
)

// comparedKeys: 비교할 출력 변수 키(버전 핵심 필드).
var comparedKeys = []string{
	"FullSemVer", "SemVer", "MajorMinorPatch", "Major", "Minor", "Patch",
	"PreReleaseLabel", "PreReleaseLabelWithDash", "PreReleaseNumber",
	"PreReleaseTag", "PreReleaseTagWithDash", "BranchName", "EscapedBranchName",
	"CommitDate", "AssemblySemVer", "AssemblySemFileVer", "InformationalVersion",
	"WeightedPreReleaseNumber", "VersionSourceDistance", "VersionSourceIncrement",
	"VersionSourceSemVer", "Sha", "ShortSha",
}

func extractFixtures(t *testing.T) string {
	t.Helper()
	archive := filepath.Join("testdata", "fixtures.tar.gz")
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("fixture 압축이 없습니다: %s", archive)
	}
	dest := t.TempDir()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dest, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				t.Fatal(err)
			}
			out.Close()
		case tar.TypeSymlink:
			_ = os.Symlink(hdr.Linkname, target)
		}
	}
	return dest
}

func normalize(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		// JSON 숫자는 float64. 정수면 정수 표기.
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}

func TestFixturesMatchRealGitVersion(t *testing.T) {
	root := extractFixtures(t)

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioDirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "expected.json")); err == nil {
			scenarioDirs = append(scenarioDirs, dir)
		}
	}
	sort.Strings(scenarioDirs)
	if len(scenarioDirs) == 0 {
		t.Fatalf("시나리오를 찾지 못했습니다: %s", root)
	}

	checked := 0
	for _, dir := range scenarioDirs {
		name := filepath.Base(dir)
		// 시나리오별 서브테스트로 하나씩 검증한다(go test -v 에서 개별 PASS 표시).
		t.Run(name, func(t *testing.T) {
			expectedText, err := os.ReadFile(filepath.Join(dir, "expected.json"))
			if err != nil {
				t.Fatalf("expected.json 읽기 실패: %v", err)
			}
			var expected map[string]interface{}
			if err := json.Unmarshal(expectedText, &expected); err != nil {
				t.Fatalf("expected.json 파싱 실패: %v", err)
			}
			repo, err := git.Discover(dir)
			if err != nil {
				t.Fatalf("저장소 오픈 실패: %v", err)
			}
			workdir := repo.Workdir()
			if workdir == "" {
				workdir = dir
			}
			cfg, err := config.Load("", workdir, workdir)
			if err != nil {
				t.Fatalf("설정 로드 실패: %v", err)
			}
			vars, err := calc.Calculate(repo, cfg, nil)
			if err != nil {
				t.Fatalf("계산 실패: %v", err)
			}
			actual := vars.ToMap()
			for _, key := range comparedKeys {
				exp := normalize(expected[key])
				got := actual[key]
				if exp != got {
					t.Errorf("%s: 기대(real)=%q 실제(mine)=%q", key, exp, got)
				}
			}
		})
		checked++
	}
	t.Logf("%d개 시나리오 검증", checked)
}

func joinLines(s []string) string {
	out := ""
	for i, l := range s {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
