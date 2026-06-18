// Package git 은 go-git 기반 저장소 접근 계층이다.
//
// 원본 GitVersion.LibGit2Sharp(및 Rust 포트의 gix 계층)에 대응하며, 버전 계산에
// 필요한 최소 그래프 연산(태그 수집, 커밋 워킹, merge-base, 미커밋 변경)을 제공한다.
package git

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitInfo 는 단일 커밋 요약.
type CommitInfo struct {
	Sha         string
	ShortSha    string
	Message     string
	When        time.Time
	ParentCount int
	Parents     []string
}

// TagInfo 는 버전 태그 후보.
type TagInfo struct {
	Name      string
	TargetSha string // 태그가 가리키는 커밋 SHA(annotated 태그는 peel 후).
	When      time.Time
}

// GitRepo 는 저장소 래퍼.
type GitRepo struct {
	repo    *git.Repository
	workdir string
}

// Discover 는 path 또는 상위에서 .git 을 탐색해 연다.
func Discover(path string) (*GitRepo, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("git 저장소를 찾지 못했습니다 %s: %w", path, err)
	}
	wd := ""
	if wt, err := repo.Worktree(); err == nil {
		wd = wt.Filesystem.Root()
	}
	return &GitRepo{repo: repo, workdir: wd}, nil
}

// Workdir 는 저장소 작업 트리 루트.
func (g *GitRepo) Workdir() string { return g.workdir }

func shortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func commitInfo(c *object.Commit) CommitInfo {
	sha := c.Hash.String()
	parents := make([]string, 0, c.NumParents())
	for _, p := range c.ParentHashes {
		parents = append(parents, p.String())
	}
	return CommitInfo{
		Sha:         sha,
		ShortSha:    shortSha(sha),
		Message:     c.Message,
		When:        c.Committer.When,
		ParentCount: len(parents),
		Parents:     parents,
	}
}

// resolveCommit 은 spec(브랜치/태그/sha)을 커밋 객체로 해석한다(annotated 태그 peel).
func (g *GitRepo) resolveCommit(spec string) (*object.Commit, bool) {
	hash, err := g.repo.ResolveRevision(plumbing.Revision(spec))
	if err != nil || hash == nil {
		return nil, false
	}
	return g.peelToCommit(*hash)
}

func (g *GitRepo) peelToCommit(hash plumbing.Hash) (*object.Commit, bool) {
	if c, err := g.repo.CommitObject(hash); err == nil {
		return c, true
	}
	if tag, err := g.repo.TagObject(hash); err == nil {
		if c, err := tag.Commit(); err == nil {
			return c, true
		}
	}
	return nil, false
}

// HeadCommit 은 HEAD 가 가리키는 커밋.
func (g *GitRepo) HeadCommit() (CommitInfo, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return CommitInfo{}, fmt.Errorf("HEAD 읽기 실패: %w", err)
	}
	c, err := g.repo.CommitObject(ref.Hash())
	if err != nil {
		return CommitInfo{}, fmt.Errorf("HEAD 커밋 읽기 실패: %w", err)
	}
	return commitInfo(c), nil
}

// CurrentBranchName 은 현재 체크아웃된 브랜치 이름(friendly).
// detached HEAD 면 원본처럼 GetBranchesContainingCommit 로 결정한다.
func (g *GitRepo) CurrentBranchName() (string, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return "", err
	}
	if ref.Name().IsBranch() {
		return ref.Name().Short(), nil
	}
	// detached HEAD.
	headSha := ref.Hash().String()
	containing := g.branchesContaining(headSha)
	if len(containing) == 1 {
		return containing[0], nil
	}
	return "(no branch)", nil
}

func (g *GitRepo) localBranchRefs() []*plumbing.Reference {
	var out []*plumbing.Reference
	iter, err := g.repo.Branches()
	if err != nil {
		return out
	}
	_ = iter.ForEach(func(r *plumbing.Reference) error {
		out = append(out, r)
		return nil
	})
	return out
}

func (g *GitRepo) localBranchesAt(sha string) []string {
	var out []string
	for _, r := range g.localBranchRefs() {
		if c, ok := g.peelToCommit(r.Hash()); ok && c.Hash.String() == sha {
			out = append(out, r.Name().Short())
		}
	}
	return out
}

// branchesContaining: HEAD 커밋을 tip 으로 갖는 브랜치(direct)가 있으면 그것,
// 없으면 HEAD 커밋을 포함(reachable)하는 로컬 브랜치들.
func (g *GitRepo) branchesContaining(headSha string) []string {
	direct := g.localBranchesAt(headSha)
	if len(direct) > 0 {
		return direct
	}
	var out []string
	for _, r := range g.localBranchRefs() {
		tip := r.Hash().String()
		if ok, _ := g.IsAncestorOf(headSha, tip); ok {
			out = append(out, r.Name().Short())
		}
	}
	return out
}

// Tags 는 모든 태그 수집(가리키는 커밋과 함께). 이름 순 정렬로 결정성을 보장한다.
func (g *GitRepo) Tags() ([]TagInfo, error) {
	var out []TagInfo
	iter, err := g.repo.Tags()
	if err != nil {
		return nil, err
	}
	err = iter.ForEach(func(r *plumbing.Reference) error {
		name := r.Name().Short()
		if c, ok := g.peelToCommit(r.Hash()); ok {
			out = append(out, TagInfo{Name: name, TargetSha: c.Hash.String(), When: c.Committer.When})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// BranchNames 는 로컬 + 원격 브랜치 이름 목록(shorthand).
func (g *GitRepo) BranchNames() ([]string, error) {
	var out []string
	for _, r := range g.localBranchRefs() {
		out = append(out, r.Name().Short())
	}
	refIter, err := g.repo.References()
	if err == nil {
		_ = refIter.ForEach(func(r *plumbing.Reference) error {
			if r.Name().IsRemote() {
				out = append(out, r.Name().Short())
			}
			return nil
		})
	}
	return out, nil
}

// LocalBranchNames 는 로컬 브랜치 이름 목록(정렬).
func (g *GitRepo) LocalBranchNames() ([]string, error) {
	var out []string
	for _, r := range g.localBranchRefs() {
		out = append(out, r.Name().Short())
	}
	sort.Strings(out)
	return out, nil
}

// ancestorsSet 은 start 커밋(포함)의 모든 조상 sha 집합을 반환한다.
func (g *GitRepo) ancestorsSet(start plumbing.Hash) map[string]bool {
	seen := map[string]bool{}
	stack := []plumbing.Hash{start}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[h.String()] {
			continue
		}
		seen[h.String()] = true
		c, err := g.repo.CommitObject(h)
		if err != nil {
			continue
		}
		stack = append(stack, c.ParentHashes...)
	}
	return seen
}

// CommitsBetween 은 from(제외)부터 to(포함)까지 도달 가능한 커밋들을 최신순으로
// 반환한다. from 이 nil 이면 to 의 모든 조상. (git rev-list to ^from 과 동일)
func (g *GitRepo) CommitsBetween(from *string, to string) ([]CommitInfo, error) {
	toCommit, ok := g.resolveCommit(to)
	if !ok {
		return nil, fmt.Errorf("커밋을 찾지 못했습니다: %s", to)
	}
	hidden := map[string]bool{}
	if from != nil {
		if fc, ok := g.resolveCommit(*from); ok {
			hidden = g.ancestorsSet(fc.Hash)
		}
	}

	var collected []*object.Commit
	seen := map[string]bool{}
	stack := []plumbing.Hash{toCommit.Hash}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		key := h.String()
		if seen[key] || hidden[key] {
			continue
		}
		seen[key] = true
		c, err := g.repo.CommitObject(h)
		if err != nil {
			continue
		}
		collected = append(collected, c)
		stack = append(stack, c.ParentHashes...)
	}
	return sortNewestFirst(collected), nil
}

func sortNewestFirst(commits []*object.Commit) []CommitInfo {
	sort.SliceStable(commits, func(i, j int) bool {
		a, b := commits[i], commits[j]
		if !a.Committer.When.Equal(b.Committer.When) {
			return a.Committer.When.After(b.Committer.When)
		}
		return a.Hash.String() > b.Hash.String()
	})
	out := make([]CommitInfo, 0, len(commits))
	for _, c := range commits {
		out = append(out, commitInfo(c))
	}
	return out
}

// FirstParentBetween 은 from(제외)부터 to(포함)까지 첫 번째 부모만 따라가며 커밋을
// 최신순으로 반환한다(Mainline 트렁크 순회용).
func (g *GitRepo) FirstParentBetween(from *string, to string) ([]CommitInfo, error) {
	toCommit, ok := g.resolveCommit(to)
	if !ok {
		return nil, fmt.Errorf("커밋을 찾지 못했습니다: %s", to)
	}
	hidden := map[string]bool{}
	if from != nil {
		if fc, ok := g.resolveCommit(*from); ok {
			hidden = g.ancestorsSet(fc.Hash)
		}
	}
	var out []CommitInfo
	cur := toCommit
	for cur != nil {
		if hidden[cur.Hash.String()] {
			break
		}
		out = append(out, commitInfo(cur))
		if len(cur.ParentHashes) == 0 {
			break
		}
		next, err := g.repo.CommitObject(cur.ParentHashes[0])
		if err != nil {
			break
		}
		cur = next
	}
	return out, nil
}

// MergeBase 는 두 커밋의 merge-base(가장 가까운 공통 조상).
func (g *GitRepo) MergeBase(a, b string) (*string, error) {
	ca, oka := g.resolveCommit(a)
	cb, okb := g.resolveCommit(b)
	if !oka || !okb {
		return nil, nil
	}
	bases, err := ca.MergeBase(cb)
	if err != nil || len(bases) == 0 {
		return nil, nil
	}
	s := bases[0].Hash.String()
	return &s, nil
}

// IsAncestorOf 는 ancestor 가 descendant 의 조상(또는 동일)인지.
func (g *GitRepo) IsAncestorOf(ancestor, descendant string) (bool, error) {
	ca, oka := g.resolveCommit(ancestor)
	cd, okd := g.resolveCommit(descendant)
	if !oka || !okd {
		return false, nil
	}
	if ca.Hash == cd.Hash {
		return true, nil
	}
	bases, err := ca.MergeBase(cd)
	if err != nil {
		return false, nil
	}
	for _, base := range bases {
		if base.Hash == ca.Hash {
			return true, nil
		}
	}
	return false, nil
}

// ChangedPathsForCommit 은 커밋이 첫 번째 부모 대비 변경한 파일 경로 목록.
// 루트 커밋이거나 diff 를 얻을 수 없으면 빈 슬라이스.
func (g *GitRepo) ChangedPathsForCommit(sha string) []string {
	c, ok := g.resolveCommit(sha)
	if !ok || len(c.ParentHashes) == 0 {
		return nil
	}
	newTree, err := c.Tree()
	if err != nil {
		return nil
	}
	parent, err := g.repo.CommitObject(c.ParentHashes[0])
	if err != nil {
		return nil
	}
	oldTree, err := parent.Tree()
	if err != nil {
		return nil
	}
	changes, err := oldTree.Diff(newTree)
	if err != nil {
		return nil
	}
	var paths []string
	for _, ch := range changes {
		if ch.To.Name != "" {
			paths = append(paths, ch.To.Name)
		} else if ch.From.Name != "" {
			paths = append(paths, ch.From.Name)
		}
	}
	return paths
}

// CommitInfoOf 는 spec(브랜치/태그/sha)을 CommitInfo 로 해석. 실패 시 (zero, false).
func (g *GitRepo) CommitInfoOf(spec string) (CommitInfo, bool) {
	c, ok := g.resolveCommit(spec)
	if !ok {
		return CommitInfo{}, false
	}
	return commitInfo(c), true
}

// UncommittedChanges 는 작업 트리의 미커밋 변경 수.
func (g *GitRepo) UncommittedChanges() int64 {
	wt, err := g.repo.Worktree()
	if err != nil {
		return 0
	}
	status, err := wt.Status()
	if err != nil {
		return 0
	}
	var n int64
	for _, s := range status {
		if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
			n++
		}
	}
	return n
}

// CreateTag 는 지정 커밋(기본 HEAD)에 lightweight 태그 생성.
func (g *GitRepo) CreateTag(name string, targetSpec string) error {
	hash, err := g.targetHash(targetSpec)
	if err != nil {
		return err
	}
	_, err = g.repo.CreateTag(name, hash, nil)
	return err
}

// CreateBranch 는 지정 커밋(기본 HEAD)에 브랜치 ref 생성(작업 트리 미변경).
func (g *GitRepo) CreateBranch(name string, targetSpec string) error {
	hash, err := g.targetHash(targetSpec)
	if err != nil {
		return err
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), hash)
	return g.repo.Storer.SetReference(ref)
}

func (g *GitRepo) targetHash(targetSpec string) (plumbing.Hash, error) {
	if targetSpec != "" {
		c, ok := g.resolveCommit(targetSpec)
		if !ok {
			return plumbing.ZeroHash, fmt.Errorf("대상 커밋을 찾지 못했습니다: %s", targetSpec)
		}
		return c.Hash, nil
	}
	ref, err := g.repo.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}

// TagsOnCommit 은 특정 커밋에 직접 붙은 태그 이름 집합.
func (g *GitRepo) TagsOnCommit(sha string) (map[string]bool, error) {
	tags, err := g.Tags()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, t := range tags {
		if t.TargetSha == sha {
			out[t.Name] = true
		}
	}
	return out, nil
}
