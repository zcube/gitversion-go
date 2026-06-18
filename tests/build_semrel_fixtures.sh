#!/usr/bin/env bash
#
# semantic-release 차등 테스트 fixture 생성기.
#
# 시나리오별 git 저장소를 만들고, 설치된 semantic-release(dry-run)를 돌려 "다음
# 릴리스 버전" golden(expected.json)을 각 저장소에 기록한 뒤, 전부
# testdata/semrel_fixtures.tar.gz 로 압축한다. 테스트(semrel_fixtures_test.go)는
# 이 압축만 풀어 우리 semrel 엔진 출력을 golden 과 비교하므로, 테스트 시점에는
# git/semantic-release 가 필요 없다.
#
# 사용법:  ./tests/build_semrel_fixtures.sh   (요구: semantic-release(전역) + git + node)
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/testdata/semrel_fixtures.tar.gz"
STAGE="$(mktemp -d)"
BAREROOT="$(mktemp -d)"
trap 'rm -rf "$STAGE" "$BAREROOT"' EXIT

command -v semantic-release >/dev/null || { echo "오류: semantic-release(전역) 필요"; exit 1; }
echo "semantic-release: $(semantic-release --version 2>/dev/null | tail -1)"
# 비-angular 프리셋(conventionalcommits 등) 패키지 resolve 용. 전역 설치 필요:
#   npm i -g conventional-changelog-conventionalcommits
export NODE_PATH="$(npm root -g)"
mkdir -p "$ROOT/testdata"

# 결정론적 커밋(날짜/작성자 고정).
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com
TICK=1609459200 # 2021-01-01T00:00:00Z

newrepo() { # $1 = name
  REPO="$STAGE/$1"; NAME="$1"
  mkdir -p "$REPO"
  git -C "$REPO" init -q -b main
  git -C "$REPO" config commit.gpgsign false
  git -C "$REPO" config tag.gpgsign false
  git -C "$REPO" config core.hooksPath /dev/null
  CUR="$REPO"; FILE=0; PRESET=angular
}

usepreset() { PRESET="$1"; }  # 시나리오의 commit-analyzer preset 지정

commit() { # $1 = message (multiline 허용)
  TICK=$((TICK + 60)); FILE=$((FILE + 1))
  echo "$FILE" > "$CUR/f$FILE"; git -C "$CUR" add -A
  GIT_AUTHOR_DATE="$TICK +0000" GIT_COMMITTER_DATE="$TICK +0000" \
    git -C "$CUR" commit -q --no-verify -m "$1"
}

tag() { git -C "$CUR" tag "$1"; }                  # 현재 커밋에 태그
tagcommit() { commit "chore: release $1"; tag "$1"; } # 커밋 후 태그
branch() { git -C "$CUR" checkout -q -b "$1"; }     # 새 브랜치 생성+체크아웃
co() { git -C "$CUR" checkout -q "$1"; }            # 기존 브랜치 체크아웃
mergeNoFF() { # $1 = 머지할 브랜치 (머지 커밋 생성)
  TICK=$((TICK + 60))
  GIT_AUTHOR_DATE="$TICK +0000" GIT_COMMITTER_DATE="$TICK +0000" \
    git -C "$CUR" merge -q --no-ff --no-verify -m "Merge branch '$1'" "$1"
}
headsha() { git -C "$CUR" rev-parse HEAD; }         # 현재 HEAD 전체 SHA
# ptag: 프리릴리스 태그 + semantic-release 채널 note(실제 릴리스가 남기는 것과 동일).
ptag() { git -C "$CUR" tag "$1"; git -C "$CUR" notes --ref semantic-release add -m "{\"channels\":[\"$2\"]}" "$1"; }

# record: 설치된 semantic-release(dry-run)로 golden 기록.
record() {
  local bare="$BAREROOT/$NAME.git"
  git init -q --bare "$bare"
  git -C "$bare" symbolic-ref HEAD refs/heads/main
  git -C "$CUR" remote add origin "$bare"
  git -C "$CUR" push -q origin --all
  git -C "$CUR" push -q origin --tags || true
  git -C "$CUR" push -q origin "refs/notes/semantic-release:refs/notes/semantic-release" 2>/dev/null || true
  local plugins
  if [ "$PRESET" = "angular" ]; then
    plugins='["@semantic-release/commit-analyzer"]'
  else
    plugins="[[\"@semantic-release/commit-analyzer\",{\"preset\":\"$PRESET\"}]]"
  fi
  cat > "$CUR/.releaserc.json" <<EOF
{ "repositoryUrl": "file://$bare",
  "branches": ["main","master",{"name":"beta","prerelease":true},{"name":"alpha","prerelease":true},"next","next-major"],
  "plugins": $plugins }
EOF
  local out ver released
  out="$(cd "$CUR" && CI=true semantic-release --dry-run --no-ci 2>&1 || true)"
  ver="$(printf '%s' "$out" | sed -nE 's/.*next release version is ([0-9][0-9A-Za-z.+-]*).*/\1/p' | head -1)"
  if [ -n "$ver" ]; then released=true; else released=false; ver=""; fi
  printf '{"NextVersion":"%s","Released":%s,"Preset":"%s"}\n' "$ver" "$released" "$PRESET" > "$CUR/expected.json"
  printf '%-22s [%s] -> %s\n' "$NAME" "$PRESET" "${ver:-none}"
  # 정리: SR 전용 산출물 제거(엔진은 원격/설정 불필요).
  rm -f "$CUR/.releaserc.json"
  git -C "$CUR" remote remove origin
}

# ───────────────────────── 시나리오: 릴리스 브랜치(main) ─────────────────────────
newrepo first_feat;     commit "feat: a"; record
newrepo first_fix;      commit "fix: a"; record
newrepo first_breaking; commit $'feat: a\n\nBREAKING CHANGE: boom'; record
newrepo first_chore;    commit "chore: a"; record
newrepo first_perf;     commit "perf: a"; record

newrepo tag_fix;        tagcommit v1.0.0; commit "fix: a"; record
newrepo tag_feat;       tagcommit v1.0.0; commit "feat: a"; record
newrepo tag_perf;       tagcommit v1.0.0; commit "perf: a"; record
newrepo tag_breaking;   tagcommit v1.0.0; commit $'feat: x\n\nBREAKING CHANGE: removed api'; record
newrepo tag_chore_only; tagcommit v1.0.0; commit "chore: a"; record
newrepo tag_feat_fix;   tagcommit v1.2.3; commit "fix: a"; commit "feat: b"; record
newrepo scope_feat;     tagcommit v1.0.0; commit "feat(api): x"; record
newrepo revert_conv;    tagcommit v1.0.0; commit "revert: x"; record
newrepo revert_git;     tagcommit v1.0.0; commit $'Revert "feat: x"\n\nThis reverts commit 0123456789abcdef.'; record
newrepo docs_only;      tagcommit v1.0.0; commit "docs: x"; record
newrepo multi_patch;    tagcommit v1.0.0; commit "fix: a"; commit "fix: b"; record
newrepo mixed_break;    tagcommit v1.0.0; commit "fix: a"; commit "feat: b"; commit $'chore: c\n\nBREAKING CHANGE: boom'; record
newrepo higher_tag_feat; tagcommit v2.5.9; commit "feat: a"; record
newrepo two_tags_fix;   tagcommit v1.0.0; commit "feat: a"; tag v1.1.0; commit "fix: b"; record

# ───────────────────────── 시나리오: 프리릴리스 브랜치(beta) ─────────────────────
newrepo beta_first;     commit "chore: init"; branch beta; commit "feat: a"; record
newrepo beta_from_main; tagcommit v1.0.0; branch beta; commit "feat: y"; record
newrepo beta_second;    commit "chore: init"; branch beta; commit "feat: y"; ptag v1.1.0-beta.1 beta; commit "fix: z"; record

# ───────────────────────── BREAKING note 변형(angular 프리셋) ─────────────────────
# 복수형/하이픈은 major 미트리거(feat 만 → minor), 소문자는 i 플래그로 트리거.
newrepo break_plural; tagcommit v1.0.0; commit $'feat: x\n\nBREAKING CHANGES: removed'; record
newrepo break_hyphen; tagcommit v1.0.0; commit $'feat: x\n\nBREAKING-CHANGE: removed'; record
newrepo break_lower;  tagcommit v1.0.0; commit $'fix: x\n\nbreaking change: removed'; record
newrepo break_subject_only; tagcommit v1.0.0; commit "fix: BREAKING CHANGE in subject"; record

# ───────────────────────── merge 커밋(merged-in 커밋 분석) ────────────────────────
newrepo merge_feature; tagcommit v1.0.0; branch feature; commit "feat: x"; co main; mergeNoFF feature; record
newrepo merge_chore;   tagcommit v1.0.0; branch chorebr; commit "chore: x"; co main; mergeNoFF chorebr; record

# ───────────────────────── revert(범위 내 대상 커밋 제거) ─────────────────────────
newrepo revert_in_range
tagcommit v1.0.0; commit "feat: x"; SHA=$(headsha)
commit "Revert \"feat: x\"

This reverts commit $SHA."
record

newrepo revert_then_fix
tagcommit v1.0.0; commit "feat: a"; SHA=$(headsha)
commit "Revert \"feat: a\"

This reverts commit $SHA."
commit "fix: b"
record

# ───────────────────────── conventionalcommits 프리셋 ────────────────────────────
# angular 와 달리 `!` 단축과 BREAKING-CHANGE(하이픈)를 인식.
newrepo cc_bang_feat;  usepreset conventionalcommits; tagcommit v1.0.0; commit "feat!: x"; record
newrepo cc_bang_fix;   usepreset conventionalcommits; tagcommit v1.0.0; commit "fix!: x"; record
newrepo cc_bang_scope; usepreset conventionalcommits; tagcommit v1.0.0; commit "feat(api)!: x"; record
newrepo cc_hyphen;     usepreset conventionalcommits; tagcommit v1.0.0; commit $'feat: x\n\nBREAKING-CHANGE: y'; record
newrepo cc_plain_feat; usepreset conventionalcommits; tagcommit v1.0.0; commit "feat: x"; record
newrepo cc_first_bang; usepreset conventionalcommits; commit "feat!: x"; record

echo "압축: $OUT"
tar -C "$STAGE" -czf "$OUT" .
echo "완료. 시나리오 수: $(ls "$STAGE" | wc -l | tr -d ' ')"
