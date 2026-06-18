package semrel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zcube/gitversion-go/internal/config"
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
// ac 는 커밋 분석 설정(tag-prefix + bump 정규식 등).
func Compute(repo *git.GitRepo, branch string, ac AnalyzerConfig) (Result, error) {
	head, err := repo.HeadCommit()
	if err != nil {
		return Result{}, err
	}
	channel, isPre := branchChannel(branch)

	tags, err := repo.Tags()
	if err != nil {
		return Result{}, err
	}
	var reachable []taggedVersion
	for _, t := range tags {
		v, ok := parseSV(stripTagPrefix(t.Name, ac.TagPrefix))
		if !ok {
			continue
		}
		if ok, _ := repo.IsAncestorOf(t.TargetSha, head.Sha); !ok {
			continue
		}
		reachable = append(reachable, taggedVersion{v: v, sha: t.TargetSha})
	}

	// lastRelease: 릴리스 브랜치는 non-prerelease 만, 프리릴리스 브랜치는
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
	relType := levelName(analyze(commits, ac))

	res := Result{LastVersion: lastVersion, Channel: channel}
	if relType == "" {
		return res, nil
	}
	res.Released = true
	res.Type = relType

	var ver sv
	switch {
	case isPre && last != nil && last.v.hasPre() && last.v.channel() == channel:
		term1 := last.v.incPrerelease()
		term2 := last.v.incRelease(relType).withChannel(channel)
		ver = maxSV(term1, term2)
	case isPre && last != nil:
		core := sv{major: last.v.major, minor: last.v.minor, patch: last.v.patch}
		ver = core.incRelease(relType).withChannel(channel)
	case isPre:
		ver = sv{major: 1}.withChannel(channel) // 1.0.0-channel.1
	case last != nil:
		ver = last.v.incRelease(relType)
	default:
		ver = sv{major: 1} // 1.0.0
	}
	res.Version = ver.String()
	return res, nil
}

// CalculateEff 는 EffectiveConfiguration 을 사용해 출력 변수를 만든다(CLI/공개 API용).
// tag-prefix, bump 정규식, commit-date-format, assembly scheme, pre-release-weight 등
// 기존 GitVersion 설정과 의미론적으로 동일한 값을 반영한다.
func CalculateEff(repo *git.GitRepo, eff *config.EffectiveConfiguration, branch string) (*output.VersionVariables, error) {
	ac := FromEffective(eff)
	r, err := Compute(repo, branch, ac)
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

	preTag, preLabel := "", ""
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

	// pre-release-weight(가중 번호): GitVersion 의미론과 동일.
	var weighted *int64
	if preNumber != nil {
		w := *preNumber + eff.PreReleaseWeight
		weighted = &w
	} else {
		w := eff.TagPreReleaseWeight
		weighted = &w
	}

	layout := dotnetDateToGoLayout(eff.CommitDateFormat)
	escaped := rx.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAll(branch, "-")
	preNum := 0
	if preNumber != nil {
		preNum = int(*preNumber)
	}

	return &output.VersionVariables{
		Major:                    uint32(parsed.major),
		Minor:                    uint32(parsed.minor),
		Patch:                    uint32(parsed.patch),
		PreReleaseTag:            preTag,
		PreReleaseTagWithDash:    withDash(preTag),
		PreReleaseLabel:          preLabel,
		PreReleaseLabelWithDash:  withDash(preLabel),
		PreReleaseNumber:         preNumber,
		WeightedPreReleaseNumber: weighted,
		MajorMinorPatch:          fmt.Sprintf("%d.%d.%d", parsed.major, parsed.minor, parsed.patch),
		SemVer:                   verStr,
		FullSemVer:               verStr,
		AssemblySemVer:           assemblyVersion(parsed.major, parsed.minor, parsed.patch, preNum, eff.AssemblyVersioningScheme),
		AssemblySemFileVer:       assemblyVersion(parsed.major, parsed.minor, parsed.patch, preNum, eff.AssemblyFileVersioningScheme),
		InformationalVersion:     verStr,
		BranchName:               branch,
		EscapedBranchName:        escaped,
		Sha:                      head.Sha,
		ShortSha:                 head.ShortSha,
		CommitDate:               head.When.UTC().Format(layout),
		VersionSourceSemVer:      r.LastVersion,
	}, nil
}

func assemblyVersion(major, minor, patch, pre int, scheme config.VersioningScheme) string {
	switch scheme {
	case config.SchemeMajor:
		return fmt.Sprintf("%d.0.0.0", major)
	case config.SchemeMajorMinor:
		return fmt.Sprintf("%d.%d.0.0", major, minor)
	case config.SchemeMajorMinorPatch:
		return fmt.Sprintf("%d.%d.%d.0", major, minor, patch)
	case config.SchemeMajorMinorPatchTag:
		return fmt.Sprintf("%d.%d.%d.%d", major, minor, patch, pre)
	default:
		return ""
	}
}

func dotnetDateToGoLayout(format string) string {
	out := format
	for _, p := range [][2]string{
		{"yyyy", "2006"}, {"yy", "06"}, {"MMMM", "January"}, {"MMM", "Jan"}, {"MM", "01"},
		{"dddd", "Monday"}, {"ddd", "Mon"}, {"dd", "02"}, {"HH", "15"}, {"mm", "04"}, {"ss", "05"},
	} {
		out = strings.ReplaceAll(out, p[0], p[1])
	}
	if out == "" {
		return "2006-01-02"
	}
	return out
}
