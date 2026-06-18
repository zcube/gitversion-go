package gogitversion_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/semrel"
)

func extractSemrelFixtures(t *testing.T) string {
	t.Helper()
	archive := filepath.Join("testdata", "semrel_fixtures.tar.gz")
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("semrel fixture 압축이 없습니다: %s (먼저 ./tests/build_semrel_fixtures.sh)", archive)
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
			_ = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
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

type semrelExpected struct {
	NextVersion string `json:"NextVersion"`
	Released    bool   `json:"Released"`
	Preset      string `json:"Preset"`
}

// 설치된 semantic-release 가 생성한 골든과 우리 semrel 엔진을 차등 비교한다.
func TestSemrelFixturesMatchSemanticRelease(t *testing.T) {
	root := extractSemrelFixtures(t)

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(root, e.Name(), "expected.json")); err == nil {
				dirs = append(dirs, e.Name())
			}
		}
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		t.Fatalf("시나리오를 찾지 못했습니다: %s", root)
	}

	checked := 0
	for _, name := range dirs {
		dir := filepath.Join(root, name)
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "expected.json"))
			if err != nil {
				t.Fatalf("expected.json 읽기 실패: %v", err)
			}
			var exp semrelExpected
			if err := json.Unmarshal(raw, &exp); err != nil {
				t.Fatalf("expected.json 파싱 실패: %v", err)
			}
			repo, err := git.Discover(dir)
			if err != nil {
				t.Fatalf("저장소 오픈 실패: %v", err)
			}
			branch, err := repo.CurrentBranchName()
			if err != nil {
				t.Fatalf("브랜치 확인 실패: %v", err)
			}
			// 실제 경로와 동일하게 effective config(SemanticRelease 워크플로)에서 분석 설정 파생.
			wf := "SemanticRelease"
			eff := config.ResolveEffective(config.ForWorkflow(&wf), branch)
			preset := semrel.PresetByName(exp.Preset) // expected.json 의 preset(기본 angular)
			r, err := semrel.Compute(repo, branch, semrel.FromEffective(&eff), preset)
			if err != nil {
				t.Fatalf("계산 실패: %v", err)
			}
			if r.Released != exp.Released {
				t.Errorf("Released: 기대(real)=%v 실제(mine)=%v (branch=%s, version=%q)", exp.Released, r.Released, branch, r.Version)
			}
			if r.Version != exp.NextVersion {
				t.Errorf("NextVersion: 기대(real)=%q 실제(mine)=%q (branch=%s)", exp.NextVersion, r.Version, branch)
			}
		})
		checked++
	}
	t.Logf("%d개 semantic-release 시나리오 검증", checked)
}
