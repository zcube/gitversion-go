// basic 예제: 경로만으로 현재 저장소의 버전을 계산한다.
//
// 실행: go run ./examples/basic [path]
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
	fmt.Println("SemVer:    ", v.SemVer)
	fmt.Printf("Major.Minor.Patch: %d.%d.%d\n", v.Major, v.Minor, v.Patch)
	fmt.Println("Sha:       ", v.Sha)
}
