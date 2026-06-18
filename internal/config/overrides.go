package config

import (
	"log/slog"
	"strconv"
	"strings"
)

// ApplyOverrides 는 `key=value` 인라인 오버라이드를 설정에 적용한다.
// 원본 cli::apply_overrides 대응. 잘못된 항목/지원하지 않는 키는 경고 후 무시한다.
func ApplyOverrides(cfg *GitVersionConfiguration, overrides []string) {
	for _, raw := range overrides {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			slog.Warn("잘못된 overrideconfig 항목(무시): " + raw)
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "tag-prefix":
			cfg.TagPrefix = &value
		case "next-version":
			cfg.NextVersion = &value
		case "label":
			cfg.Label = &value
		case "commit-date-format":
			cfg.CommitDateFormat = &value
		case "major-version-bump-message":
			cfg.MajorVersionBumpMessage = &value
		case "minor-version-bump-message":
			cfg.MinorVersionBumpMessage = &value
		case "patch-version-bump-message":
			cfg.PatchVersionBumpMessage = &value
		case "no-bump-message":
			cfg.NoBumpMessage = &value
		case "tag-pre-release-weight":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				cfg.TagPreReleaseWeight = &n
			}
		case "update-build-number":
			if b, err := strconv.ParseBool(value); err == nil {
				cfg.UpdateBuildNumber = &b
			}
		case "increment":
			if inc, ok := parseIncrementName(value); ok {
				cfg.Increment = &inc
			}
		case "mode":
			if m, ok := parseModeName(value); ok {
				cfg.Mode = &m
			}
		case "semantic-version-format":
			var f SemanticVersionFormat
			if strings.ToLower(value) == "loose" {
				f = FormatLoose
			} else {
				f = FormatStrict
			}
			cfg.SemanticVersionFormat = &f
		default:
			slog.Warn("지원하지 않는 overrideconfig 키(무시): " + key)
		}
	}
}

func parseIncrementName(v string) (IncrementStrategy, bool) {
	switch strings.ToLower(v) {
	case "major":
		return IncrementMajor, true
	case "minor":
		return IncrementMinor, true
	case "patch":
		return IncrementPatch, true
	case "none":
		return IncrementNone, true
	case "inherit":
		return IncrementInherit, true
	}
	return 0, false
}

func parseModeName(v string) (DeploymentMode, bool) {
	switch strings.ToLower(v) {
	case "continuousdelivery":
		return ContinuousDelivery, true
	case "continuousdeployment":
		return ContinuousDeployment, true
	case "manualdeployment":
		return ManualDeployment, true
	}
	return 0, false
}
