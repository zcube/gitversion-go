// options 예제: 브랜치/오버라이드/캐시 비활성 등 옵션을 지정하고, 출력 헬퍼를 쓴다.
//
// 실행: go run ./examples/options [path]
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

	v, err := gitversion.Calculate(gitversion.Options{
		Path:      path,
		Overrides: []string{"tag-prefix=[vV]?"},
		NoCache:   true, // 캐시 우회(항상 재계산)
	})
	if err != nil {
		log.Fatalf("버전 계산 실패: %v", err)
	}

	// 단일 변수
	semver, _ := v.ShowVariable("SemVer")
	fmt.Println("SemVer:", semver)

	// 포맷 문자열({Variable}/{env:VAR})
	s, _ := v.FormatTemplate("v{Major}.{Minor}.{Patch} ({EscapedBranchName})", os.Getenv)
	fmt.Println("Formatted:", s)

	// 전체 변수 맵
	fmt.Println("--- 모든 변수 ---")
	m := v.ToMap()
	for _, k := range []string{"FullSemVer", "BranchName", "CommitDate", "AssemblySemVer"} {
		fmt.Printf("  %s = %s\n", k, m[k])
	}

	// GitVersion 호환 JSON
	j, _ := v.ToJSON()
	fmt.Println("--- JSON ---")
	fmt.Println(j)
}
