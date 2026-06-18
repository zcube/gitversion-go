// Command gitversion 은 Git 히스토리로부터 시맨틱 버전을 계산하는 CLI 다.
// GitVersion(.NET)의 Go 포트로, 출력은 .NET GitVersion 6.x 와 차등 테스트로 검증된다.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"github.com/zcube/go-gitversion/internal/cli"
)

// version 은 빌드 시 -ldflags 로 주입할 수 있다(기본은 dev).
var version = "dev"

func main() {
	root := cli.NewRootCommand(version)
	if err := fang.Execute(context.Background(), root); err != nil {
		os.Exit(1)
	}
}
