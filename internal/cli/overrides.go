package cli

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/zcube/go-gitversion/internal/config"
)

// applyOverrides 는 `key=value` 인라인 오버라이드를 설정에 적용한다.
// 원본 cli::apply_overrides 대응.
func applyOverrides(cfg *config.GitVersionConfiguration, overrides []string) error {
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
			var f config.SemanticVersionFormat
			if strings.ToLower(value) == "loose" {
				f = config.FormatLoose
			} else {
				f = config.FormatStrict
			}
			cfg.SemanticVersionFormat = &f
		default:
			slog.Warn("지원하지 않는 overrideconfig 키(무시): " + key)
		}
	}
	return nil
}

func parseIncrementName(v string) (config.IncrementStrategy, bool) {
	switch strings.ToLower(v) {
	case "major":
		return config.IncrementMajor, true
	case "minor":
		return config.IncrementMinor, true
	case "patch":
		return config.IncrementPatch, true
	case "none":
		return config.IncrementNone, true
	case "inherit":
		return config.IncrementInherit, true
	}
	return 0, false
}

func parseModeName(v string) (config.DeploymentMode, bool) {
	switch strings.ToLower(v) {
	case "continuousdelivery":
		return config.ContinuousDelivery, true
	case "continuousdeployment":
		return config.ContinuousDeployment, true
	case "manualdeployment":
		return config.ManualDeployment, true
	}
	return 0, false
}
