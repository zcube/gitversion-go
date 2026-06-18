// Package workflow 는 버전 계산 워크플로의 공통 인터페이스를 정의한다.
//
// GitVersion 계열(GitFlow/GitHubFlow/TrunkBased/Mainline)과 SemanticRelease 는 모두
// 동일한 Context 를 입력받아 출력 변수를 만드는 Calculator 로 추상화된다. 진입점은
// 공통 컨텍스트(HEAD/브랜치/effective 설정)를 해석한 뒤 적절한 Calculator 를 선택한다.
package workflow

import (
	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/output"
)

// Context 는 워크플로 계산기 공통 입력. 진입점에서 해석해 채운다.
type Context struct {
	Repo   *git.GitRepo
	Config *config.GitVersionConfiguration
	Eff    *config.EffectiveConfiguration
	Branch string
	Head   git.CommitInfo
}

// Calculator 는 워크플로별 버전 계산기의 공통 인터페이스.
type Calculator interface {
	Calculate(ctx *Context) (*output.VersionVariables, error)
}
