// buildagent 예제: 계산한 버전을 CI(빌드에이전트) 형식으로 출력한다.
//
// 실행: go run ./examples/buildagent [path]
//   - CI 환경에서 실행하면 감지된 CI 형식으로 출력(예: TEAMCITY_VERSION 설정 시 TeamCity).
//   - CI 가 아니면 데모로 TeamCity/GitHubActions 형식을 보여준다.
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

	v, err := gitversion.Compute(path)
	if err != nil {
		log.Fatalf("버전 계산 실패: %v", err)
	}
	fmt.Println("FullSemVer:", v.FullSemVer)

	// 1) 현재 환경의 CI 자동 감지.
	if lines, ok := gitversion.BuildServerOutput(v, true); ok {
		fmt.Println("\n[감지된 CI 출력]")
		for _, l := range lines {
			fmt.Println(l)
		}
	} else {
		fmt.Println("\n[CI 미감지 → 데모로 특정 에이전트 형식 출력]")
	}

	// 2) 특정 에이전트를 명시해 형식 확인(데모).
	for _, name := range []string{"TeamCity", "GitHubActions"} {
		agent := gitversion.BuildAgentByName(name)
		if agent == nil {
			continue
		}
		fmt.Printf("\n[%s]\n", agent.Name())
		for _, l := range agent.WriteIntegration(v, true) {
			fmt.Println(l)
		}
	}
}
