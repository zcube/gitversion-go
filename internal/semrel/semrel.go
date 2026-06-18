package semrel

import (
	"strconv"
	"strings"

	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/output"
	"github.com/zcube/gitversion-go/internal/rx"
)

// DefaultPrereleaseBranches: semantic-release 기본 프리릴리스 브랜치 → 채널.
var DefaultPrereleaseBranches = map[string]string{"beta": "beta", "alpha": "alpha"}

// Result 는 semantic-release 가 계산할 "다음 릴리스" 결과.
type Result struct {
	Released    bool   // 릴리스할 변경이 있는지
	Version     string // 다음 릴리스 버전(예: "1.1.0-beta.2"). 미릴리스면 "".
	Type        string // "major"/"minor"/"patch"/""
	LastVersion string // 직전 릴리스 버전(없으면 "")
	Channel     string // 프리릴리스 채널(릴리스 브랜치면 "")
}

type taggedVersion struct {
	v   sv
	sha string
}

func branchChannel(branch string) (string, bool) {
	short := branch
	if i := strings.LastIndex(branch, "/"); i >= 0 {
		short = branch[i+1:]
	}
	for _, name := range []string{branch, short} {
		if ch, ok := DefaultPrereleaseBranches[name]; ok {
			return ch, true
		}
	}
	return "", false
}

// stripTagPrefix 는 tag-prefix 정규식을 제거한다(기본 "[vV]?").
func stripTagPrefix(name, tagPrefix string) string {
	if tagPrefix == "" {
		return name
	}
	re, err := rx.CompileCached("^(" + tagPrefix + ")")
	if err != nil {
		return name
	}
	return re.ReplaceFirst(name, "")
}

// Compute 는 semantic-release 와 동일하게 다음 릴리스 버전을 계산한다.
func Compute(repo *git.GitRepo, tagPrefix, branch string) (Result, error) {
	head, err := repo.HeadCommit()
	if err != nil {
		return Result{}, err
	}
	channel, isPre := branchChannel(branch)

	// HEAD 에서 도달 가능한 semver 태그 수집.
	tags, err := repo.Tags()
	if err != nil {
		return Result{}, err
	}
	var reachable []taggedVersion
	for _, t := range tags {
		v, ok := parseSV(stripTagPrefix(t.Name, tagPrefix))
		if !ok {
			continue
		}
		if ok, _ := repo.IsAncestorOf(t.TargetSha, head.Sha); !ok {
			continue
		}
		reachable = append(reachable, taggedVersion{v: v, sha: t.TargetSha})
	}

	// lastRelease 선택: 릴리스 브랜치는 non-prerelease 만, 프리릴리스 브랜치는
	// (non-prerelease ∪ 같은 채널 prerelease) 중 최고.
	var last *taggedVersion
	for i := range reachable {
		t := &reachable[i]
		eligible := !t.v.hasPre()
		if isPre && t.v.hasPre() && t.v.channel() == channel {
			eligible = true
		}
		if !eligible {
			continue
		}
		if last == nil || cmpSV(t.v, last.v) > 0 {
			last = t
		}
	}

	// lastRelease(제외)~HEAD 커밋 메시지 분석.
	var from *string
	lastVersion := ""
	if last != nil {
		from = &last.sha
		lastVersion = last.v.String()
	}
	commits, err := repo.CommitsBetween(from, head.Sha)
	if err != nil {
		return Result{}, err
	}
	msgs := make([]string, 0, len(commits))
	for _, c := range commits {
		msgs = append(msgs, c.Message)
	}
	lvl := analyzeLevel(msgs)
	relType := levelName(lvl)

	res := Result{LastVersion: lastVersion, Channel: channel}
	if relType == "" {
		res.Released = false
		return res, nil
	}
	res.Released = true
	res.Type = relType

	var ver sv
	if isPre {
		if last != nil {
			if last.v.hasPre() && last.v.channel() == channel {
				term1 := last.v.incPrerelease()
				term2 := last.v.incRelease(relType).withChannel(channel)
				ver = maxSV(term1, term2)
			} else {
				core := sv{major: last.v.major, minor: last.v.minor, patch: last.v.patch}
				ver = core.incRelease(relType).withChannel(channel)
			}
		} else {
			ver = sv{major: 1}.withChannel(channel) // 1.0.0-channel.1
		}
	} else {
		if last != nil {
			ver = last.v.incRelease(relType)
		} else {
			ver = sv{major: 1} // 1.0.0
		}
	}
	res.Version = ver.String()
	return res, nil
}

// Calculate 는 semantic-release 워크플로의 출력 변수를 만든다(CLI/공개 API용).
// 릴리스가 없으면 직전 버전(없으면 0.0.0)을 출력한다.
func Calculate(repo *git.GitRepo, tagPrefix, branch string) (*output.VersionVariables, error) {
	r, err := Compute(repo, tagPrefix, branch)
	if err != nil {
		return nil, err
	}
	head, err := repo.HeadCommit()
	if err != nil {
		return nil, err
	}

	verStr := r.Version
	if !r.Released {
		verStr = r.LastVersion
		if verStr == "" {
			verStr = "0.0.0"
		}
	}
	parsed, _ := parseSV(verStr)

	preTag := ""
	preLabel := ""
	var preNumber *int64
	if parsed.hasPre() {
		parts := make([]string, len(parsed.pre))
		for i, p := range parsed.pre {
			if p.isNum {
				parts[i] = strconv.Itoa(p.num)
			} else {
				parts[i] = p.str
			}
		}
		preTag = strings.Join(parts, ".")
		preLabel = parsed.channel()
		for i := len(parsed.pre) - 1; i >= 0; i-- {
			if parsed.pre[i].isNum {
				n := int64(parsed.pre[i].num)
				preNumber = &n
				break
			}
		}
	}
	withDash := func(s string) string {
		if s == "" {
			return ""
		}
		return "-" + s
	}
	mmp := strconv.Itoa(parsed.major) + "." + strconv.Itoa(parsed.minor) + "." + strconv.Itoa(parsed.patch)
	commitDate := head.When.UTC().Format("2006-01-02")
	escaped := rx.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAll(branch, "-")

	return &output.VersionVariables{
		Major:                   uint32(parsed.major),
		Minor:                   uint32(parsed.minor),
		Patch:                   uint32(parsed.patch),
		PreReleaseTag:           preTag,
		PreReleaseTagWithDash:   withDash(preTag),
		PreReleaseLabel:         preLabel,
		PreReleaseLabelWithDash: withDash(preLabel),
		PreReleaseNumber:        preNumber,
		MajorMinorPatch:         mmp,
		SemVer:                  verStr,
		FullSemVer:              verStr,
		InformationalVersion:    verStr,
		BranchName:              branch,
		EscapedBranchName:       escaped,
		Sha:                     head.Sha,
		ShortSha:                head.ShortSha,
		CommitDate:              commitDate,
		VersionSourceSemVer:     r.LastVersion,
	}, nil
}
