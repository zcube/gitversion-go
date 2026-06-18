// semrel 예제: semantic-release 와 동일한 버전 체계(workflow: SemanticRelease).
//
// Conventional Commits 를 분석해 다음 릴리스 버전을 계산한다(feat→minor, fix/perf→patch,
// BREAKING CHANGE→major, 최초 릴리스 1.0.0, beta/alpha 프리릴리스 채널).
//
// 실행: go run ./examples/semrel [path]
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/zcube/gitversion-go/gitversion"
)

func main() {
	path := "."
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	// workflow: SemanticRelease 를 인라인 설정으로 지정.
	v, err := gitversion.Calculate(gitversion.Options{
		Path:       path,
		ConfigYAML: []byte("workflow: SemanticRelease\n"),
		NoCache:    true,
	})
	if err != nil {
		log.Fatalf("계산 실패: %v", err)
	}

	fmt.Println("다음 릴리스 버전(SemVer):", v.SemVer)
	fmt.Printf("Major.Minor.Patch: %d.%d.%d\n", v.Major, v.Minor, v.Patch)
	if v.PreReleaseTag != "" {
		fmt.Println("PreRelease:", v.PreReleaseTag, "(label:", v.PreReleaseLabel+")")
	}
	fmt.Println("직전 릴리스(VersionSourceSemVer):", v.VersionSourceSemVer)
}
