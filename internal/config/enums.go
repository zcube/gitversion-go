package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// IncrementStrategy 는 증분 전략. `increment` 키.
type IncrementStrategy int

const (
	IncrementNone IncrementStrategy = iota
	IncrementMajor
	IncrementMinor
	IncrementPatch
	IncrementInherit
)

var incrementNames = map[IncrementStrategy]string{
	IncrementNone: "None", IncrementMajor: "Major", IncrementMinor: "Minor",
	IncrementPatch: "Patch", IncrementInherit: "Inherit",
}

func (s IncrementStrategy) String() string { return incrementNames[s] }

func parseIncrement(v string) (IncrementStrategy, error) {
	for k, n := range incrementNames {
		if n == v {
			return k, nil
		}
	}
	return 0, fmt.Errorf("invalid increment strategy %q", v)
}

func (s *IncrementStrategy) UnmarshalYAML(b []byte) error {
	v, err := scalarString(b)
	if err != nil {
		return err
	}
	r, err := parseIncrement(v)
	if err != nil {
		return err
	}
	*s = r
	return nil
}

func (s IncrementStrategy) MarshalYAML() ([]byte, error) { return []byte(s.String()), nil }

// DeploymentMode 는 배포 모드. `mode` 키.
type DeploymentMode int

const (
	ManualDeployment DeploymentMode = iota
	ContinuousDelivery
	ContinuousDeployment
)

var deploymentNames = map[DeploymentMode]string{
	ManualDeployment: "ManualDeployment", ContinuousDelivery: "ContinuousDelivery",
	ContinuousDeployment: "ContinuousDeployment",
}

func (m DeploymentMode) String() string { return deploymentNames[m] }

func (m *DeploymentMode) UnmarshalYAML(b []byte) error {
	v, err := scalarString(b)
	if err != nil {
		return err
	}
	for k, n := range deploymentNames {
		if n == v {
			*m = k
			return nil
		}
	}
	return fmt.Errorf("invalid deployment mode %q", v)
}

func (m DeploymentMode) MarshalYAML() ([]byte, error) { return []byte(m.String()), nil }

// CommitMessageIncrementMode `commit-message-incrementing` 키.
type CommitMessageIncrementMode int

const (
	CommitMsgEnabled CommitMessageIncrementMode = iota
	CommitMsgDisabled
	CommitMsgMergeMessageOnly
)

var commitMsgNames = map[CommitMessageIncrementMode]string{
	CommitMsgEnabled: "Enabled", CommitMsgDisabled: "Disabled",
	CommitMsgMergeMessageOnly: "MergeMessageOnly",
}

func (m CommitMessageIncrementMode) String() string { return commitMsgNames[m] }

func (m *CommitMessageIncrementMode) UnmarshalYAML(b []byte) error {
	v, err := scalarString(b)
	if err != nil {
		return err
	}
	for k, n := range commitMsgNames {
		if n == v {
			*m = k
			return nil
		}
	}
	return fmt.Errorf("invalid commit-message-incrementing %q", v)
}

func (m CommitMessageIncrementMode) MarshalYAML() ([]byte, error) { return []byte(m.String()), nil }

// VersionStrategy 는 버전 탐색 전략. `strategies` 키.
type VersionStrategy int

const (
	StrategyNone VersionStrategy = iota
	StrategyFallback
	StrategyConfiguredNextVersion
	StrategyMergeMessage
	StrategyTaggedCommit
	StrategyTrackReleaseBranches
	StrategyVersionInBranchName
	StrategyMainline
)

var strategyNames = map[VersionStrategy]string{
	StrategyNone: "None", StrategyFallback: "Fallback",
	StrategyConfiguredNextVersion: "ConfiguredNextVersion", StrategyMergeMessage: "MergeMessage",
	StrategyTaggedCommit: "TaggedCommit", StrategyTrackReleaseBranches: "TrackReleaseBranches",
	StrategyVersionInBranchName: "VersionInBranchName", StrategyMainline: "Mainline",
}

func (s VersionStrategy) String() string { return strategyNames[s] }

func (s *VersionStrategy) UnmarshalYAML(b []byte) error {
	v, err := scalarString(b)
	if err != nil {
		return err
	}
	for k, n := range strategyNames {
		if n == v {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("invalid version strategy %q", v)
}

func (s VersionStrategy) MarshalYAML() ([]byte, error) { return []byte(s.String()), nil }

// VersioningScheme: AssemblyVersion / AssemblyFileVersion 부여 스킴.
type VersioningScheme int

const (
	SchemeMajorMinorPatchTag VersioningScheme = iota
	SchemeMajorMinorPatch
	SchemeMajorMinor
	SchemeMajor
	SchemeNone
)

var schemeNames = map[VersioningScheme]string{
	SchemeMajorMinorPatchTag: "MajorMinorPatchTag", SchemeMajorMinorPatch: "MajorMinorPatch",
	SchemeMajorMinor: "MajorMinor", SchemeMajor: "Major", SchemeNone: "None",
}

func (s VersioningScheme) String() string { return schemeNames[s] }

func (s *VersioningScheme) UnmarshalYAML(b []byte) error {
	v, err := scalarString(b)
	if err != nil {
		return err
	}
	for k, n := range schemeNames {
		if n == v {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("invalid versioning scheme %q", v)
}

func (s VersioningScheme) MarshalYAML() ([]byte, error) { return []byte(s.String()), nil }

// SemanticVersionFormat: SemanticVersion 파싱 엄격도.
type SemanticVersionFormat int

const (
	FormatStrict SemanticVersionFormat = iota
	FormatLoose
)

func (f SemanticVersionFormat) String() string {
	if f == FormatLoose {
		return "Loose"
	}
	return "Strict"
}

func (f *SemanticVersionFormat) UnmarshalYAML(b []byte) error {
	v, err := scalarString(b)
	if err != nil {
		return err
	}
	switch v {
	case "Strict":
		*f = FormatStrict
	case "Loose":
		*f = FormatLoose
	default:
		return fmt.Errorf("invalid semantic-version-format %q", v)
	}
	return nil
}

func (f SemanticVersionFormat) MarshalYAML() ([]byte, error) { return []byte(f.String()), nil }

// scalarString 은 YAML 스칼라 바이트를 문자열로 디코드한다.
func scalarString(b []byte) (string, error) {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return "", err
	}
	return s, nil
}
