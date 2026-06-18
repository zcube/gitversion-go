# go-gitversion

GitVersion(.NET)의 **Go 포트**. Git 히스토리로부터 시맨틱 버전을 계산한다.
[`../gitversion`](../gitversion) 의 Rust 포트를 동일한 골든 픽스쳐로 다시 Go 로 옮긴 것으로,
실제 GitVersion 6.x 가 생성한 기대값(`expected.json`)과 **차등(differential) 테스트**로 검증한다.

## 검증 현황

- `testdata/fixtures.tar.gz` 는 Rust 포트와 **완전히 동일한 픽스쳐**(시나리오별 git 저장소 +
  .NET GitVersion 6.7.0 golden).
- `go test` 가 압축을 풀어 우리 엔진 출력을 golden 과 비교한다. **150개 차등 시나리오 전부 일치.**
- 비교 키: `FullSemVer`, `SemVer`, `MajorMinorPatch`, `PreReleaseTag`, `AssemblySemVer`,
  `VersionSourceSemVer`, `CommitDate`, `Sha` 등 22개 핵심 변수.

```bash
go build ./...                 # 빌드
go test ./...                  # 단위 + 픽스쳐 차등 테스트
go build -o gitversion ./cmd/gitversion
./gitversion                   # 현재 저장소 버전 계산(JSON)
./gitversion -v SemVer         # 단일 변수
./gitversion --format "{Major}.{Minor}"
./gitversion --output dot-env  # GitVersion_Major='1' ...
./gitversion --showconfig      # effective 설정(YAML)
./gitversion --lang ko         # 메시지 로케일(en/ko/ja/zh)
```

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
| `src/cli/mod.rs` + `src/app.rs` | `internal/cli` + `cmd/gitversion` | cobra/fang 명령과 진입 로직 |
| `src/i18n.rs` + `locales/` | `internal/i18n` | go-i18n 로케일 처리 |
| (신규) | `internal/rx` | regexp2 래퍼(.NET named capture 호환) |

## 정규식 호환성

원본 GitVersion 설정/머지 포맷은 .NET 정규식(`(?<name>...)`)을 사용한다. Go 표준 `regexp`(RE2)는
이 문법을 지원하지 않으므로, `internal/rx` 가 `regexp2` 를 감싸 .NET 과 동일한 매칭(named capture,
backtracking, 미참여 그룹 구분)을 보장한다.

## 미포팅 범위

핵심 버전 계산 엔진과 출력은 100% 포팅했다. 다음은 차등 골든과 무관한 부수 기능으로 범위에서 제외했다:
빌드 에이전트 어댑터(`buildagent`), TUI(`ratatui`), 외부 명령 훅(`exec`), 원격 clone(`remote`),
디스크 캐시(`cache`), AssemblyInfo/프로젝트 파일 갱신(`output/files`).
