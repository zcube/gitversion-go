# gitversion-go

GitVersion(.NET)의 **Go 포트**. Git 히스토리로부터 시맨틱 버전을 계산한다.
[`../gitversion`](../gitversion) 의 Rust 포트를 동일한 골든 픽스쳐로 다시 Go 로 옮긴 것으로,
실제 GitVersion 6.x 가 생성한 기대값(`expected.json`)과 **차등(differential) 테스트**로 검증한다.

## 검증 현황

- `testdata/fixtures.tar.gz` 는 Rust 포트와 **완전히 동일한 픽스쳐**(시나리오별 git 저장소 +
  .NET GitVersion 6.7.0 golden).
- `go test` 가 압축을 풀어 우리 엔진 출력을 golden 과 비교한다. **150개 차등 시나리오 전부 일치.**
- 비교 키: `FullSemVer`, `SemVer`, `MajorMinorPatch`, `PreReleaseTag`, `AssemblySemVer`,
  `VersionSourceSemVer`, `CommitDate`, `Sha` 등 22개 핵심 변수.

## 설치

```bash
# CLI 설치
go install github.com/zcube/gitversion-go/cmd/gitversion-go@latest

# 또는 릴리즈 페이지에서 플랫폼별 바이너리 다운로드
#   https://github.com/zcube/gitversion-go/releases
```

## 사용

```bash
go build ./...                 # 빌드
go test ./...                  # 단위 + 픽스쳐 차등 테스트
go build -o gitversion-go ./cmd/gitversion-go
./gitversion-go                   # 현재 저장소 버전 계산(JSON)
./gitversion-go -v SemVer         # 단일 변수
./gitversion-go --format "{Major}.{Minor}"
./gitversion-go --output dot-env  # GitVersion_Major='1' ...
./gitversion-go --showconfig      # effective 설정(YAML)
./gitversion-go --lang ko         # 메시지 로케일(en/ko/ja/zh)
```

## 라이브러리로 사용 (외부 패키지)

CLI 외에 **공개 패키지 `github.com/zcube/gitversion-go/gitversion`** 로도 임포트해 쓸 수 있다.

```go
import "github.com/zcube/gitversion-go/gitversion"

// 1) 가장 간단: 경로만
v, err := gitversion.Compute(".")
if err != nil { log.Fatal(err) }
fmt.Println(v.FullSemVer)             // 예: "1.2.3-beta.1+5"
fmt.Println(v.SemVer, v.Major, v.Sha)

// 2) 옵션 지정(브랜치/오버라이드/캐시)
v, err = gitversion.Calculate(gitversion.Options{
    Path:      "/path/to/repo",
    Branch:    "release/2.0.0",        // 선택
    Overrides: []string{"next-version=2.0.0"},
    NoCache:   true,
})

// 3) 설정 직접 입력 (우선순위 Config > ConfigYAML > ConfigPath/자동탐색)
//    a. 인라인 YAML(GitVersion.yml 과 동일 형식)
v, _ = gitversion.Calculate(gitversion.Options{
    Path:       ".",
    ConfigYAML: []byte("workflow: GitHubFlow/v1\nnext-version: 2.0.0\n"),
})
//    b. 설정 객체(프리셋에서 만들어 조정)
cfg := gitversion.DefaultConfig("GitFlow")     // 또는 ParseConfig([]byte(...))
nv := "3.0.0"; cfg.NextVersion = &nv
v, _ = gitversion.Calculate(gitversion.Options{Path: ".", Config: cfg})

// 4) 출력 헬퍼
m := v.ToMap()                         // map[string]string (모든 변수)
s, _ := v.ShowVariable("MajorMinorPatch")
j, _ := v.ToJSON()                     // GitVersion 호환 JSON
```

`internal/*` 은 외부에서 임포트할 수 없고, 공개 API 는 `gitversion` 패키지 하나로 모았다.
실행 가능한 예제는 [`examples/`](examples/) 참고(`go run ./examples/basic` 등).

## 사용 라이브러리

| 목적 | 라이브러리 |
|---|---|
| Git 접근 | [`go-git/go-git`](https://github.com/go-git/go-git) |
| .NET 호환 정규식(named capture 등) | [`dlclark/regexp2`](https://github.com/dlclark/regexp2) |
| YAML 설정 파싱 | [`goccy/go-yaml`](https://github.com/goccy/go-yaml) |
| CLI 프레임워크 | [`charmbracelet/fang`](https://github.com/charmbracelet/fang) + `spf13/cobra` |
| 다국어 메시지 | [`nicksnyder/go-i18n`](https://github.com/nicksnyder/go-i18n) |
| 로깅 | 표준 `log/slog` |

## 코드 구조 (Rust → Go 매핑)

| 원본(Rust) | Go 패키지 | 역할 |
|---|---|---|
| `src/version/semver.rs` | `internal/version` | SemanticVersion / PreReleaseTag / BuildMetaData |
| `src/version/mod.rs` | `internal/version` | VersionField |
| `src/version/calculation.rs` | `internal/calc` | 전략 → 증분 → 선택 → deployment mode 엔진(Mainline 포함) |
| `src/config/{model,defaults,loader,effective}.rs` | `internal/config` | 설정 모델 / 워크플로 기본값 / 로더 / effective 해석 |
| `src/git/mod.rs` | `internal/git` | go-git 기반 저장소 접근(태그/커밋 워킹/merge-base 등) |
| `src/output/{variables,generator}.rs` | `internal/output` | 출력 변수와 JSON/dotenv/build-server 포맷터 |
| `src/cli/mod.rs` + `src/app.rs` | `internal/cli` + `cmd/gitversion-go` | cobra/fang 명령과 진입 로직 |
| `src/i18n.rs` + `locales/` | `internal/i18n` | go-i18n 로케일 처리 |
| `src/buildagent/mod.rs` | `internal/buildagent` | 15종 CI 어댑터(build-server 출력) |
| `src/exec.rs` | `internal/exec` | 외부 명령 라이프사이클/version 훅 |
| `src/remote.rs` | `internal/remote` | 동적 원격 저장소 clone(go-git) |
| `src/cache.rs` | `internal/cache` | 디스크 캐시(SHA1 키, .git/gitversion_cache) |
| `src/output/files.rs` | `internal/output/files.go` | AssemblyInfo/프로젝트/패키지/Wix 파일 갱신 |
| (신규) | `internal/rx` | regexp2 래퍼(.NET named capture 호환) |

## 정규식 호환성

원본 GitVersion 설정/머지 포맷은 .NET 정규식(`(?<name>...)`)을 사용한다. Go 표준 `regexp`(RE2)는
이 문법을 지원하지 않으므로, `internal/rx` 가 `regexp2` 를 감싸 .NET 과 동일한 매칭(named capture,
backtracking, 미참여 그룹 구분)을 보장한다.

## 추가 기능 플래그

| 기능 | 플래그 |
|---|---|
| 빌드 에이전트 출력 | `--output build-server` (CI 자동 감지) |
| version/side-effect 훅 | `--exec-version <cmd>`, `--exec <cmd>`, `--dry-run` (+ 설정 `exec:`) |
| 원격 클론 | `--url <repo> --branch <b> [-u -p -c --dynamic-repo-location]` (https 는 `-u`/`-p` 미지정 시 git credential helper/OS 키링 자동 사용) |
| 디스크 캐시 | 기본 활성, `--nocache` 로 우회 |
| 로그 파일 | `-l`/`--log <FILE>`(타임스탬프 append) 또는 `--log console`(stderr). stdout 은 결과 전용 |
| 파일 갱신 | `--updateassemblyinfo [--ensureassemblyinfo]`, `--updateprojectfiles`, `--updatepackagefiles`, `--updatewixversionfile` |

## 미포팅 범위

핵심 엔진·출력·CI 통합·훅·원격·캐시·파일 갱신을 모두 포팅했다. 대화형 TUI(`ratatui`)만
범위에서 제외했다(터미널 UI 의존, 차등 골든과 무관).
