// config 예제: 설정을 직접 입력하는 세 가지 방법(인라인 YAML / 설정 객체 / 프리셋).
//
// 실행: go run ./examples/config [path]
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

	// 1) 인라인 YAML (GitVersion.yml 과 동일 형식, 워크플로 기본값과 병합).
	v1, err := gitversion.Calculate(gitversion.Options{
		Path:       path,
		ConfigYAML: []byte("workflow: GitHubFlow/v1\nnext-version: 2.0.0\n"),
		NoCache:    true,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("[ConfigYAML]       FullSemVer:", v1.FullSemVer)

	// 2) 프리셋에서 설정 객체를 만들어 조정.
	cfg := gitversion.DefaultConfig("GitFlow")
	nv := "3.0.0"
	cfg.NextVersion = &nv
	v2, err := gitversion.Calculate(gitversion.Options{Path: path, Config: cfg, NoCache: true})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("[Config 객체]      FullSemVer:", v2.FullSemVer)

	// 3) YAML 을 파싱해 설정 객체로(검사/추가 조정 후 사용).
	parsed, err := gitversion.ParseConfig([]byte("next-version: 4.0.0\ntag-prefix: \"ver\"\n"))
	if err != nil {
		log.Fatal(err)
	}
	v3, err := gitversion.Calculate(gitversion.Options{Path: path, Config: parsed, NoCache: true})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("[ParseConfig]      FullSemVer:", v3.FullSemVer)
}
