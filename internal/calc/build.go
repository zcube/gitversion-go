package calc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zcube/go-gitversion/internal/config"
	"github.com/zcube/go-gitversion/internal/git"
	"github.com/zcube/go-gitversion/internal/output"
	"github.com/zcube/go-gitversion/internal/rx"
	v "github.com/zcube/go-gitversion/internal/version"
)

func compileCI(pat string) (*rx.Regexp, error) {
	return rx.CompileCached("(?i)" + pat)
}

func i64p(n int64) *int64         { return &n }
func tptr(t time.Time) *time.Time { return &t }
func getenv(name string) string   { return os.Getenv(name) }

// applyDeploymentMode: deployment mode 별 최종 버전(+빌드 메타데이터) 산출.
func applyDeploymentMode(repo *git.GitRepo, eff *config.EffectiveConfiguration, branchName string, head *git.CommitInfo, chosen *nextVersion, baseSource *string, ig ignoreSet) (v.SemanticVersion, error) {
	baseSrc := baseSource
	if chosen.base.exact {
		baseSrc = chosen.base.baseVersionSource
	}
	cs, err := repo.CommitsBetween(baseSrc, head.Sha)
	if err != nil {
		return v.SemanticVersion{}, err
	}
	commits := int64(len(ig.filter(repo, cs)))
	uncommitted := repo.UncommittedChanges()

	sv := chosen.incremented
	var vsSha string
	if baseSrc != nil {
		vsSha = *baseSrc
	}
	meta := v.BuildMetaData{
		CommitsSinceTag:        i64p(commits),
		Branch:                 branchName,
		Sha:                    head.Sha,
		ShortSha:               head.ShortSha,
		CommitDate:             tptr(head.When),
		VersionSourceSha:       vsSha,
		VersionSourceDistance:  commits,
		UncommittedChanges:     uncommitted,
		VersionSourceIncrement: v.FieldNone,
	}

	if chosen.base.exact {
		meta.CommitsSinceTag = nil
		sv.BuildMetaData = meta
		return sv, nil
	}

	switch eff.DeploymentMode {
	case config.ManualDeployment:
		// 코어/태그 유지, 빌드 메타데이터(짧은 형태)를 FullSemVer 에 노출.
	case config.ContinuousDelivery:
		if sv.PreReleaseTag.HasTag() {
			n := int64(1)
			if sv.PreReleaseTag.Number != nil {
				n = *sv.PreReleaseTag.Number
			}
			sv.PreReleaseTag.Number = i64p(n + commits - 1)
		}
		meta.CommitsSinceTag = nil
	case config.ContinuousDeployment:
		sv.PreReleaseTag = v.PreReleaseTag{}
		meta.CommitsSinceTag = nil
	}

	sv.BuildMetaData = meta
	return sv, nil
}

// buildVariables: 최종 출력 변수 구성.
func buildVariables(eff *config.EffectiveConfiguration, branchName string, head *git.CommitInfo, sv *v.SemanticVersion, sourceSemver *v.SemanticVersion) (*output.VersionVariables, error) {
	pre := sv.PreReleaseTag
	preLabel := pre.Name
	preNumber := pre.Number
	preTagStr := ""
	if pre.HasTag() {
		preTagStr = pre.Format()
	}

	withDash := func(s string) string {
		if s == "" {
			return ""
		}
		return "-" + s
	}

	majorMinorPatch := sv.MajorMinorPatch()
	semVer := sv.String()
	commits := sv.BuildMetaData.VersionSourceDistance
	fullBuildMeta := sv.BuildMetaData.FormatFull()

	var fullSemVer string
	if sv.BuildMetaData.CommitsSinceTag != nil {
		fullSemVer = fmt.Sprintf("%s+%d", semVer, *sv.BuildMetaData.CommitsSinceTag)
	} else {
		fullSemVer = semVer
	}

	var weighted *int64
	if preNumber != nil {
		weighted = i64p(*preNumber + eff.PreReleaseWeight)
	} else {
		weighted = i64p(eff.TagPreReleaseWeight)
	}

	assemblySemVer := assemblyVersion(sv, eff.AssemblyVersioningScheme)
	assemblySemFileVer := assemblyVersion(sv, eff.AssemblyFileVersioningScheme)

	informational := semVer
	if fullBuildMeta != "" {
		informational = semVer + "+" + fullBuildMeta
	}

	escapedBranch := escapeBranchRe.ReplaceAll(branchName, "-")

	layout := dotnetDateFormatToGoLayout(eff.CommitDateFormat)
	commitDate := head.When.UTC().Format(layout)

	vars := &output.VersionVariables{
		Major:                     uint32(sv.Major),
		Minor:                     uint32(sv.Minor),
		Patch:                     uint32(sv.Patch),
		PreReleaseTag:             preTagStr,
		PreReleaseTagWithDash:     withDash(preTagStr),
		PreReleaseLabel:           preLabel,
		PreReleaseLabelWithDash:   withDash(preLabel),
		PreReleaseNumber:          preNumber,
		WeightedPreReleaseNumber:  weighted,
		BuildMetaData:             sv.BuildMetaData.CommitsSinceTag,
		FullBuildMetaData:         fullBuildMeta,
		MajorMinorPatch:           majorMinorPatch,
		SemVer:                    semVer,
		FullSemVer:                fullSemVer,
		AssemblySemVer:            assemblySemVer,
		AssemblySemFileVer:        assemblySemFileVer,
		InformationalVersion:      informational,
		BranchName:                branchName,
		EscapedBranchName:         escapedBranch,
		Sha:                       head.Sha,
		ShortSha:                  head.ShortSha,
		VersionSourceDistance:     i64p(commits),
		VersionSourceIncrement:    sv.BuildMetaData.VersionSourceIncrement.String(),
		VersionSourceSemVer:       sourceSemver.String(),
		VersionSourceSha:          sv.BuildMetaData.VersionSourceSha,
		CommitsSinceVersionSource: i64p(commits),
		CommitDate:                commitDate,
		UncommittedChanges:        sv.BuildMetaData.UncommittedChanges,
	}

	// 커스텀 포맷 후처리.
	ctx := vars.ToMap()
	if eff.AssemblyVersioningFormat != nil {
		out, err := renderTemplate(*eff.AssemblyVersioningFormat, ctx)
		if err != nil {
			return nil, err
		}
		vars.AssemblySemVer = out
	}
	if eff.AssemblyFileVersioningFormat != nil {
		out, err := renderTemplate(*eff.AssemblyFileVersioningFormat, ctx)
		if err != nil {
			return nil, err
		}
		vars.AssemblySemFileVer = out
	}
	out, err := renderTemplate(eff.AssemblyInformationalFormat, ctx)
	if err != nil {
		return nil, err
	}
	vars.InformationalVersion = out

	return vars, nil
}

var (
	escapeBranchRe  = rx.MustCompile(`[^a-zA-Z0-9-]`)
	templateTokenRe = rx.MustCompile(`\{(?<t>[A-Za-z0-9_:]+)\}`)
)

// renderTemplate: `{Variable}` 및 `{env:VAR}` 토큰을 변수 맵으로 치환. 알 수 없는
// 토큰이 있으면 원본처럼 에러를 반환한다.
func renderTemplate(format string, ctx map[string]string) (string, error) {
	var unknown string
	out := templateTokenRe.ReplaceAllFunc(format, func(m *rx.Match) string {
		t, _ := m.Named("t")
		if envVar, ok := strings.CutPrefix(t, "env:"); ok {
			return getenv(envVar)
		}
		if val, ok := ctx[t]; ok {
			return val
		}
		if unknown == "" {
			unknown = t
		}
		return ""
	})
	if unknown != "" {
		return "", fmt.Errorf("Unknown template token '{%s}' in format string", unknown)
	}
	return out, nil
}

// assemblyVersion: AssemblyVersion 스킴 적용.
func assemblyVersion(sv *v.SemanticVersion, scheme config.VersioningScheme) string {
	var pre int64
	if sv.PreReleaseTag.Number != nil {
		pre = *sv.PreReleaseTag.Number
	}
	switch scheme {
	case config.SchemeMajor:
		return strconv.FormatInt(sv.Major, 10) + ".0.0.0"
	case config.SchemeMajorMinor:
		return fmt.Sprintf("%d.%d.0.0", sv.Major, sv.Minor)
	case config.SchemeMajorMinorPatch:
		return fmt.Sprintf("%d.%d.%d.0", sv.Major, sv.Minor, sv.Patch)
	case config.SchemeMajorMinorPatchTag:
		return fmt.Sprintf("%d.%d.%d.%d", sv.Major, sv.Minor, sv.Patch, pre)
	default:
		return ""
	}
}
