# 기능 검토 (../gitversion README "Features" 대조)

원본 Rust 포트(`../gitversion`)의 README "Features" 항목을 Go 포트 기준으로 하나씩 검토한다.
상태: ✅ 구현·검증 / ⚠️ 구현했으나 구현 방식 차이 / ❌ 미구현.

| # | 기능 | 상태 | Go 구현 / 비고 |
|---|---|---|---|
| 1 | Git 접근(원본 gix) | ⚠️ | `internal/git` — go-git 사용(라이브러리만 다름, 동일 동작). 150 차등 시나리오로 검증 |
| 2 | CLI(원본 clap) | ✅ | `internal/cli` — cobra + charmbracelet/fang |
| 3 | 로깅(원본 env_logger) | ⚠️ | `log/slog`(stderr), `--verbosity`/`--diag`. `RUST_LOG`/`-l` 파일 로그는 아래 10번 참고 |
| 4 | i18n(원본 rust-i18n) | ✅ | `internal/i18n` — go-i18n, 기본 영어, `--lang`/`LANG`/`LC_ALL`(en/ko/ja/zh) |
| 5 | TUI(`--tui`) | ❌ | **미구현** — 대화형 ratatui UI(터미널 의존, 차등 골든 무관) |
| 6 | 워크플로 GitFlow/GitHubFlow/TrunkBased | ✅ | `internal/config/defaults.go` |
| 7 | 버전 전략 7종 | ✅ | `internal/calc` — Configured/Tagged/MergeMessage/VersionInBranchName/TrackRelease/Fallback/Mainline |
| 8 | 배포 모드 3종 | ✅ | Manual/ContinuousDelivery/ContinuousDeployment |
| 9 | 출력 JSON/dot-env/build-server/`-v`/`--format` | ✅ | `internal/output` + CLI. 검증 완료 |
| 10 | 로그 파일 `--log`/`-l <FILE>`(타임스탬프 append) | ❌ | **미구현** — 현재 로그는 stderr 전용 |
| 11 | 빌드 에이전트 15종 통합 | ✅ | `internal/buildagent` — 28개 골든 라인 일치 |
| 12 | 파일 출력: AssemblyInfo/프로젝트/Wix | ⚠️ | `internal/output/files.go`. 프로젝트 파일은 원본의 XML 이벤트 파서 대신 **포맷 보존 타깃 정규식** 사용(결과 동일, 주석·속성·들여쓰기 보존) |
| 13 | 패키지 매니페스트(package.json/Cargo.toml/pyproject.toml) | ⚠️ | 동일. TOML 은 toml_edit 대신 줄 단위 포맷 보존 치환(주석 보존) |
| 14 | exec 훅(verify/prepare/publish/success/fail + version) | ✅ | `internal/exec`. `--exec`/`--exec-version`/`--dry-run`. 검증 완료 |
| 15 | 결과 캐시(`<.git>/gitversion_cache`) | ✅ | `internal/cache` — SHA1(refs·HEAD·config·override), `--nocache`. 검증 완료 |
| 16 | 동적 원격 저장소(`--url --branch` 등) | ⚠️ | `internal/remote` — go-git PlainClone. https BasicAuth + ssh-agent. 아래 16-a 참고 |
| 16-a | credential helper / OS 키링(https) | ✅ | `internal/remote/credential.go` — `git credential fill/approve/reject` 연동. -u/-p 없으면 osxkeychain/GCM/libsecret 등에서 자동 조회, 성공 시 store, 인증 실패 시 erase |

## Known simplifications (원본과 동일하게 유지)

| 항목 | 상태 | 비고 |
|---|---|---|
| `track-merge-target` | ✅ 동일 | HEAD 도달 가능한 태그만 고려(원본과 같은 단순화) |
| `--nofetch`/`--nonormalize`/`--nocache`/`--allowshallow` | ✅ | no-op 플래그로 인식(원본과 동일) |
| `GitVersionInformation` 소스파일 생성 | ✅ 범위 외 | 원본도 MSBuild 태스크 담당, CLI 범위 외 |

## 정리: 미구현/차이 항목

1. **TUI (`--tui`)** — 미구현. Bubble Tea 등으로 별도 포팅 필요.
2. **로그 파일 `--log`/`-l <FILE>`** — 미구현. slog 핸들러를 파일(append, 타임스탬프)로 전환하는 작업.
3. **프로젝트/패키지 파일 갱신 방식 차이(⚠️)** — 기능·결과는 동일하나 구현이 정규식 기반(원본은 XML/TOML 파서). 포맷 보존은 유지됨.

(credential helper / OS 키링은 구현 완료 — 16-a 참고.)

핵심 버전 계산·출력·CI 통합·훅·원격·캐시·파일 갱신은 모두 구현·검증되었고, 남은 것은
부수 기능 3종(TUI, 로그 파일, 자격증명 헬퍼)이다.
