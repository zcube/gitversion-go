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
| (신규) | `internal/workflow` | 워크플로 공통 인터페이스(`Calculator`/`Context`) |
| (신규) | `internal/semrel` | semantic-release 호환 계산기(`workflow.Calculator` 구현) |
| (신규) | `internal/varutil` | 공통 출력 유틸(날짜 포맷/AssemblyVersion/브랜치 escape) |

> 계산 진입점 `calc.Calculate` 는 공통 컨텍스트(HEAD/브랜치/effective 설정)를 해석한 뒤
> 워크플로에 맞는 `workflow.Calculator`(GitVersion 계열 또는 SemanticRelease)를 선택해 위임한다.

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

## 버전 baseline 관리 (`next-version`)

`next-version` 은 "다음 릴리스의 시작점(baseline)"이며, **영구 고정값이 아니라 한 사이클용 floor**다.
우리 엔진은 후보 중 **가장 높은 버전**을 고르므로(원본 .NET 과 동일), `next-version` 과 도달 가능한
태그를 비교해 동작한다:

- `next-version` > 최신 태그  → `next-version` 이 채택됨 (예: 태그 v1.0.0, next-version 3.0.0 → 3.0.0)
- 최신 태그 ≥ `next-version`  → **태그가 이김** (예: 태그 v1.0.0, next-version 0.5.0 → 1.0.x)
- HEAD 가 정확히 `vX.Y.Z` 로 태그됨 → 그 태그 버전 그대로(기본 `when-current-commit-tagged`)

**릴리스 후 갱신이 필요하다.** 예를 들어 `next-version: 1.0.0` 으로 개발하다 `v1.0.0` 태그를 만들면,
그 다음부터는 태그가 `next-version` 을 가리므로(같거나 높음) `next-version` 은 사실상 무효가 된다.
다음 사이클을 진행하려면 둘 중 하나:

1. **`GitVersion.yml` 의 `next-version` 을 상향**(예: `1.1.0` 또는 `2.0.0`) — 다음 baseline 지정, 또는
2. **더 높은 `v*` 태그 생성** — 태그가 항상 최종 소스이므로 별도 설정 없이도 버전이 올라간다.

> 정리: `next-version` 은 "아직 태그가 없을 때의 임시 baseline"이고, 한 번 그 이상으로 태그하면
> 다음 사이클을 위해 `next-version` 을 다시 올려 주거나 태그로만 관리하면 된다.

## semantic-release 호환 워크플로

GitVersion 의 GitFlow/GitHubFlow/TrunkBased 외에, **`workflow: SemanticRelease`** 를 지정하면
[semantic-release](https://github.com/semantic-release/semantic-release) 와 동일한 버전 체계로
계산한다(Conventional Commits 분석):

- `feat:` → minor, `fix:`/`perf:` → patch, `BREAKING CHANGE:` footer → major (angular 프리셋)
- 최초 릴리스는 `1.0.0`, 이후 직전 태그에서 `semver.inc`
- `beta`/`alpha` 브랜치는 프리릴리스 채널(`1.1.0-beta.1` 등)
- 릴리스할 변경이 없으면 직전 버전 유지

**angular 프리셋 충실도**(설치본 동작과 일치, 차등 검증):
- `BREAKING CHANGE` note 는 **body/footer 에서만**, **대소문자 무시**로 major. `BREAKING CHANGES`(복수)·
  `BREAKING-CHANGE`(하이픈)·subject 의 문구는 트리거하지 않음. `feat!:` 의 `!` 단축도 미지원(angular).
- **revert 필터링**: 범위 내에서 `Revert "..."`(+`This reverts commit <full-sha>`)와 그 대상 커밋을
  쌍으로 분석에서 제거(commit-analyzer 의 filterRevertedCommits). 대상이 범위 밖이면 revert 는 patch.
- **merge 커밋**: `git log lastTag..HEAD` 범위(merged-in 커밋 포함)를 분석. 머지 커밋 메시지 자체는
  타입이 없어 릴리스를 트리거하지 않지만, 병합으로 들어온 `feat:`/`fix:` 는 반영됨.

> conventionalcommits 프리셋(`!` breaking 단축 등)은 기본(angular)이 아니므로 현재 미지원이다.

```bash
gitversion-go --config <(echo "workflow: SemanticRelease")   # 또는 GitVersion.yml 에 명시
```

이 동작은 **설치된 semantic-release(25.x)로 생성한 골든**(`testdata/semrel_fixtures.tar.gz`)과
`go test` 차등 검증으로 일치를 보장한다(재생성: `./tests/build_semrel_fixtures.sh`).

### 설정(기존 GitVersion 키 재사용)

SemanticRelease 워크플로도 의미론적으로 동일한 기존 GitVersion 설정 키를 그대로 따른다.
`--showconfig` 로 effective config 를 확인할 수 있다.

| 설정 키 | SemanticRelease 에서의 의미 |
|---|---|
| `tag-prefix` | 버전 태그 접두어(기본 `[vV]?`, semantic-release 의 `vX.Y.Z` 호환) |
| `major/minor/patch-version-bump-message` | 커밋→bump **정규식**. Conventional Commits 분석에 **합집합(최고 우선)**으로 추가되어 규칙을 커스터마이즈(기본값은 `+semver:` 패턴이라 기본 동작 불변) |
| `commit-message-incrementing` | `Disabled` 면 위 정규식 미적용(Conventional 분석만), `MergeMessageOnly` 면 머지 커밋에만 |
| `commit-date-format` | `CommitDate` 출력 포맷 |
| `assembly-versioning-scheme` / `assembly-file-versioning-scheme` | `AssemblySemVer` / `AssemblySemFileVer` 산출 |
| `pre-release-weight` / `tag-pre-release-weight` | `WeightedPreReleaseNumber` 산출 |

예) 커스텀 bump 정규식:

```yaml
workflow: SemanticRelease
minor-version-bump-message: "^Add "   # 비-conventional "Add ..." 커밋도 minor 로
```

## 미포팅 범위

핵심 엔진·출력·CI 통합·훅·원격·캐시·파일 갱신을 모두 포팅했다. 대화형 TUI(`ratatui`)만
범위에서 제외했다(터미널 UI 의존, 차등 골든과 무관).
