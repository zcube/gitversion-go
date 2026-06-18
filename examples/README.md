# examples — 모듈로 사용하기

`gitversion-go` 를 외부 Go 프로그램에서 라이브러리로 쓰는 예제 모음.

## 외부 프로젝트에서 설치

```bash
go get github.com/zcube/gitversion-go@latest
```

```go
import "github.com/zcube/gitversion-go/gitversion"
```

> `internal/*` 는 외부에서 임포트할 수 없으며, 공개 API 는 `gitversion` 패키지 하나로 모았다.

## 예제 실행 (이 저장소 안에서)

```bash
go run ./examples/basic       # 경로만으로 버전 계산
go run ./examples/options     # 옵션 지정 + 출력 헬퍼(ShowVariable/FormatTemplate/ToJSON)
go run ./examples/config      # 설정 직접 입력(ConfigYAML / Config 객체 / ParseConfig)
go run ./examples/buildagent  # CI(빌드에이전트) 형식 출력(감지 + 명시 선택)
go run ./examples/semrel      # semantic-release 동일 버전 체계(Conventional Commits)

# 다른 저장소를 대상으로
go run ./examples/basic /path/to/repo
```

## 핵심 API

| 함수/타입 | 설명 |
|---|---|
| `gitversion.Compute(path)` | 경로만으로 계산하는 간편 함수 |
| `gitversion.Calculate(Options)` | 브랜치/오버라이드/설정/캐시 옵션 지정 |
| `gitversion.Options` | `Path`, `Config`, `ConfigYAML`, `ConfigPath`, `Branch`, `Overrides`, `NoCache` |
| `gitversion.DefaultConfig(workflow)` | GitFlow/GitHubFlow/TrunkBased 프리셋 설정 |
| `gitversion.ParseConfig(yaml)` | YAML → 설정 객체(기본값 병합) |
| `Variables.ToMap()` / `ToJSON()` / `ShowVariable(name)` / `FormatTemplate(t, getenv)` | 출력 헬퍼 |
| `gitversion.DetectBuildAgent()` / `BuildAgentByName(name)` | CI 어댑터 감지/선택 |
| `gitversion.BuildServerOutput(v, ubn)` | 감지된 CI 형식의 통합 출력 라인 |
