package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// 원본 ConfigurationFileLocator.cs, ConfigurationProvider.cs 대응.

var candidates = []string{"GitVersion.yml", "GitVersion.yaml", ".GitVersion.yml", ".GitVersion.yaml"}

// Locate 는 dir 와 repoRoot 에서 설정 파일을 탐색한다.
func Locate(dir string, repoRoot string) string {
	searchDirs := []string{dir}
	if repoRoot != "" && repoRoot != dir {
		searchDirs = append(searchDirs, repoRoot)
	}
	for _, d := range searchDirs {
		for _, name := range candidates {
			p := filepath.Join(d, name)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
	}
	return ""
}

func isWorkflowFilePath(s string) bool {
	return strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "/") || strings.HasSuffix(s, ".yml") || strings.HasSuffix(s, ".yaml")
}

func loadWorkflowFile(wfPath, configDir string) (*GitVersionConfiguration, error) {
	abs := wfPath
	if !filepath.IsAbs(wfPath) {
		abs = filepath.Join(configDir, wfPath)
	}
	text, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("설정 파일을 읽지 못했습니다 %s: %w", abs, err)
	}
	var cfg GitVersionConfiguration
	if err := yaml.Unmarshal(text, &cfg); err != nil {
		return nil, fmt.Errorf("YAML 파싱 실패 %s: %w", abs, err)
	}
	return &cfg, nil
}

// Load 는 명시 경로 또는 탐색으로 설정을 로드하고 워크플로 기본값과 병합한다.
func Load(explicitPath, workDir, repoRoot string) (*GitVersionConfiguration, error) {
	path := explicitPath
	if path == "" {
		path = Locate(workDir, repoRoot)
	}
	if path == "" {
		return GitFlow(), nil
	}

	text, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("설정 파일을 읽지 못했습니다 %s: %w", path, err)
	}
	var overrides GitVersionConfiguration
	if err := yaml.Unmarshal(text, &overrides); err != nil {
		return nil, fmt.Errorf("YAML 파싱 실패 %s: %w", path, err)
	}

	configDir := filepath.Dir(path)
	var base *GitVersionConfiguration
	if overrides.Workflow != nil && isWorkflowFilePath(*overrides.Workflow) {
		base, err = loadWorkflowFile(*overrides.Workflow, configDir)
		if err != nil {
			return nil, err
		}
		ensureMaps(base)
	} else {
		base = ForWorkflow(overrides.Workflow)
	}
	Merge(base, &overrides)
	ApplySourceBranchMappings(base)
	if err := Validate(base); err != nil {
		return nil, fmt.Errorf("설정 검증 실패 %s: %w", path, err)
	}
	return base, nil
}

func ensureMaps(c *GitVersionConfiguration) {
	if c.Branches == nil {
		c.Branches = map[string]*BranchConfiguration{}
	}
	if c.MergeMessageFormats == nil {
		c.MergeMessageFormats = map[string]string{}
	}
	if c.Exec == nil {
		c.Exec = map[string]string{}
	}
}

const validateHelp = "\nSee https://gitversion.net/docs/reference/configuration for more info"

// Validate 는 설정 검증(원본 ConfigurationBuilderBase.ValidateConfiguration).
func Validate(config *GitVersionConfiguration) error {
	for _, name := range config.SortedBranchKeys() {
		bc := config.Branches[name]
		if bc.Regex == nil {
			return fmt.Errorf("Branch configuration '%s' is missing required configuration 'regex'%s", name, validateHelp)
		}
		var missing []string
		for _, sb := range bc.SourceBranches {
			if _, ok := config.Branches[sb]; !ok {
				missing = append(missing, sb)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("Branch configuration '%s' defines these 'source-branches' that are not configured: '[%s]'%s",
				name, strings.Join(missing, ","), validateHelp)
		}
	}
	return nil
}

// ApplySourceBranchMappings: is-source-branch-for 역매핑(원본 ApplySourceBranchesSourceBranch).
func ApplySourceBranchMappings(config *GitVersionConfiguration) {
	type mapping struct {
		source  string
		targets []string
	}
	var mappings []mapping
	for _, k := range config.SortedBranchKeys() {
		b := config.Branches[k]
		if len(b.IsSourceBranchFor) > 0 {
			mappings = append(mappings, mapping{k, b.IsSourceBranchFor})
		}
	}
	for _, m := range mappings {
		for _, target := range m.targets {
			tb, ok := config.Branches[target]
			if !ok {
				continue
			}
			if !contains(tb.SourceBranches, m.source) {
				tb.SourceBranches = append(tb.SourceBranches, m.source)
			}
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Merge 는 override 설정을 base 위에 덮어쓴다(non-nil/비어있지 않은 값만).
func Merge(base, over *GitVersionConfiguration) {
	if over.Workflow != nil {
		base.Workflow = over.Workflow
	}
	if over.AssemblyVersioningScheme != nil {
		base.AssemblyVersioningScheme = over.AssemblyVersioningScheme
	}
	if over.AssemblyFileVersioningScheme != nil {
		base.AssemblyFileVersioningScheme = over.AssemblyFileVersioningScheme
	}
	if over.AssemblyInformationalFormat != nil {
		base.AssemblyInformationalFormat = over.AssemblyInformationalFormat
	}
	if over.AssemblyVersioningFormat != nil {
		base.AssemblyVersioningFormat = over.AssemblyVersioningFormat
	}
	if over.AssemblyFileVersioningFormat != nil {
		base.AssemblyFileVersioningFormat = over.AssemblyFileVersioningFormat
	}
	if over.TagPrefix != nil {
		base.TagPrefix = over.TagPrefix
	}
	if over.VersionInBranchPattern != nil {
		base.VersionInBranchPattern = over.VersionInBranchPattern
	}
	if over.NextVersion != nil {
		base.NextVersion = over.NextVersion
	}
	if over.MajorVersionBumpMessage != nil {
		base.MajorVersionBumpMessage = over.MajorVersionBumpMessage
	}
	if over.MinorVersionBumpMessage != nil {
		base.MinorVersionBumpMessage = over.MinorVersionBumpMessage
	}
	if over.PatchVersionBumpMessage != nil {
		base.PatchVersionBumpMessage = over.PatchVersionBumpMessage
	}
	if over.NoBumpMessage != nil {
		base.NoBumpMessage = over.NoBumpMessage
	}
	if over.TagPreReleaseWeight != nil {
		base.TagPreReleaseWeight = over.TagPreReleaseWeight
	}
	if over.CommitDateFormat != nil {
		base.CommitDateFormat = over.CommitDateFormat
	}
	if over.SemanticVersionFormat != nil {
		base.SemanticVersionFormat = over.SemanticVersionFormat
	}
	if over.UpdateBuildNumber != nil {
		base.UpdateBuildNumber = over.UpdateBuildNumber
	}
	if over.Increment != nil {
		base.Increment = over.Increment
	}
	if over.Mode != nil {
		base.Mode = over.Mode
	}
	if over.Label != nil {
		base.Label = over.Label
	}
	if over.Regex != nil {
		base.Regex = over.Regex
	}
	if over.CommitMessageIncrementing != nil {
		base.CommitMessageIncrementing = over.CommitMessageIncrementing
	}
	if over.PreventIncrement != nil {
		base.PreventIncrement = over.PreventIncrement
	}
	if over.TrackMergeTarget != nil {
		base.TrackMergeTarget = over.TrackMergeTarget
	}
	if over.TrackMergeMessage != nil {
		base.TrackMergeMessage = over.TrackMergeMessage
	}
	if over.TracksReleaseBranches != nil {
		base.TracksReleaseBranches = over.TracksReleaseBranches
	}
	if over.IsReleaseBranch != nil {
		base.IsReleaseBranch = over.IsReleaseBranch
	}
	if over.IsMainBranch != nil {
		base.IsMainBranch = over.IsMainBranch
	}
	if over.PreReleaseWeight != nil {
		base.PreReleaseWeight = over.PreReleaseWeight
	}
	if over.LabelNumberPattern != nil {
		base.LabelNumberPattern = over.LabelNumberPattern
	}
	if len(over.Strategies) > 0 {
		base.Strategies = over.Strategies
	}
	if len(over.SourceBranches) > 0 {
		base.SourceBranches = over.SourceBranches
	}
	if len(over.IsSourceBranchFor) > 0 {
		base.IsSourceBranchFor = over.IsSourceBranchFor
	}
	if over.Ignore.CommitsBefore != nil || len(over.Ignore.Sha) > 0 || len(over.Ignore.Paths) > 0 {
		base.Ignore = over.Ignore
	}
	for k, v := range over.MergeMessageFormats {
		base.MergeMessageFormats[k] = v
	}
	for k, v := range over.Exec {
		base.Exec[k] = v
	}
	for key, ob := range over.Branches {
		entry, ok := base.Branches[key]
		if !ok {
			entry = &BranchConfiguration{}
			base.Branches[key] = entry
		}
		mergeBranch(entry, ob)
	}
}

func mergeBranch(base, over *BranchConfiguration) {
	if over.Regex != nil {
		base.Regex = over.Regex
	}
	if over.Label != nil {
		base.Label = over.Label
	}
	if over.Increment != nil {
		base.Increment = over.Increment
	}
	if over.Mode != nil {
		base.Mode = over.Mode
	}
	if over.CommitMessageIncrementing != nil {
		base.CommitMessageIncrementing = over.CommitMessageIncrementing
	}
	if over.PreventIncrement != nil {
		base.PreventIncrement = over.PreventIncrement
	}
	if over.TrackMergeTarget != nil {
		base.TrackMergeTarget = over.TrackMergeTarget
	}
	if over.TrackMergeMessage != nil {
		base.TrackMergeMessage = over.TrackMergeMessage
	}
	if over.TracksReleaseBranches != nil {
		base.TracksReleaseBranches = over.TracksReleaseBranches
	}
	if over.IsReleaseBranch != nil {
		base.IsReleaseBranch = over.IsReleaseBranch
	}
	if over.IsMainBranch != nil {
		base.IsMainBranch = over.IsMainBranch
	}
	if over.PreReleaseWeight != nil {
		base.PreReleaseWeight = over.PreReleaseWeight
	}
	if over.LabelNumberPattern != nil {
		base.LabelNumberPattern = over.LabelNumberPattern
	}
	if len(over.SourceBranches) > 0 {
		base.SourceBranches = over.SourceBranches
	}
	if len(over.IsSourceBranchFor) > 0 {
		base.IsSourceBranchFor = over.IsSourceBranchFor
	}
}
