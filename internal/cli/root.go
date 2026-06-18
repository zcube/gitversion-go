// Package cli 는 cobra/fang 기반 명령줄 인터페이스를 제공한다.
//
// 원본 GitVersion.App/ArgumentParser.cs 의 주요 옵션을 옮긴다.
package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/zcube/go-gitversion/internal/calc"
	"github.com/zcube/go-gitversion/internal/config"
	"github.com/zcube/go-gitversion/internal/git"
	"github.com/zcube/go-gitversion/internal/i18n"
	"github.com/zcube/go-gitversion/internal/output"
)

type options struct {
	targetPath     string
	outputs        []string
	outputFile     string
	showVariable   string
	format         string
	configPath     string
	showConfig     bool
	overrideConfig []string
	branch         string
	lang           string
	verbosity      string
	diag           bool
	// 원본 CLI 호환용 no-op 플래그.
	nofetch, nonormalize, nocache, allowshallow bool
}

// NewRootCommand 은 루트 cobra 명령을 생성한다.
func NewRootCommand(version string) *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:           "gitversion [path]",
		Short:         "Calculate a semantic version from Git history",
		Version:       version,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			return run(o, path)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.targetPath, "targetpath", "", "Repository path (position-independent)")
	f.StringSliceVar(&o.outputs, "output", []string{"json"}, "Output format: json, dot-env, build-server, file")
	f.StringVar(&o.outputFile, "outputfile", "", "Write the result to a file")
	f.StringVarP(&o.showVariable, "showvariable", "v", "", "Print a single variable only (e.g. -v SemVer)")
	f.StringVar(&o.format, "format", "", `Print using a format string (e.g. "{Major}.{Minor}")`)
	f.StringVar(&o.configPath, "config", "", "Config file path")
	f.BoolVar(&o.showConfig, "showconfig", false, "Print the effective config as YAML and exit")
	f.StringArrayVar(&o.overrideConfig, "overrideconfig", nil, "Inline config override (key=value). May be repeated")
	f.StringVarP(&o.branch, "branch", "b", "", "Branch to compute for (instead of the current checkout)")
	f.StringVar(&o.lang, "lang", "", "Output language (en/ko/ja/zh)")
	f.StringVar(&o.verbosity, "verbosity", "normal", "Log verbosity: quiet, minimal, normal, verbose, diagnostic")
	f.BoolVar(&o.diag, "diag", false, "Diagnostic mode (debug logging)")
	f.BoolVar(&o.nofetch, "nofetch", false, "Disable fetch (no-op)")
	f.BoolVar(&o.nonormalize, "nonormalize", false, "Disable normalization (no-op)")
	f.BoolVar(&o.nocache, "nocache", false, "Disable disk cache (no-op)")
	f.BoolVar(&o.allowshallow, "allowshallow", false, "Allow shallow clone (no-op)")

	return cmd
}

func setupLogging(verbosity string, diag bool) {
	level := slog.LevelInfo
	switch strings.ToLower(verbosity) {
	case "quiet":
		level = slog.LevelError
	case "minimal":
		level = slog.LevelWarn
	case "normal":
		level = slog.LevelInfo
	case "verbose", "diagnostic":
		level = slog.LevelDebug
	}
	if diag {
		level = slog.LevelDebug
	}
	// stdout 은 버전 결과 전용으로 비워 두고, 로그는 stderr 로.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

func run(o *options, path string) error {
	i18n.Init(o.lang)
	setupLogging(o.verbosity, o.diag)

	target := path
	if o.targetPath != "" {
		target = o.targetPath
	}
	if abs, err := filepath.Abs(target); err == nil {
		target = abs
	}
	slog.Debug(i18n.T("log.target_path", map[string]interface{}{"path": target}))

	repo, err := git.Discover(target)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("error.git_open", nil), err)
	}
	workdir := repo.Workdir()
	if workdir == "" {
		workdir = target
	}

	cfg, err := config.Load(o.configPath, workdir, workdir)
	if err != nil {
		return err
	}
	if err := applyOverrides(cfg, o.overrideConfig); err != nil {
		return err
	}

	if o.showConfig {
		b, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
		return nil
	}

	var branchOverride *string
	if o.branch != "" {
		branchOverride = &o.branch
	}
	vars, err := calc.Calculate(repo, cfg, branchOverride)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("error.calc_failed", nil), err)
	}

	if o.showVariable != "" {
		out, err := vars.ShowVariable(o.showVariable)
		if err != nil {
			return err
		}
		return emit(o, out)
	}
	if o.format != "" {
		out, err := vars.FormatTemplate(o.format, os.Getenv)
		if err != nil {
			return err
		}
		return emit(o, out)
	}

	rendered, err := render(o, vars)
	if err != nil {
		return err
	}
	return emit(o, rendered)
}

func render(o *options, vars *output.VersionVariables) (string, error) {
	var parts []string
	for _, fmtName := range o.outputs {
		switch strings.ToLower(strings.TrimSpace(fmtName)) {
		case "json", "file":
			j, err := vars.ToJSON()
			if err != nil {
				return "", err
			}
			parts = append(parts, j)
		case "dot-env", "dotenv":
			parts = append(parts, strings.TrimRight(vars.ToDotEnv(), "\n"))
		case "build-server", "buildserver":
			parts = append(parts, strings.TrimRight(vars.ToBuildServerEnv(), "\n"))
		default:
			return "", fmt.Errorf("알 수 없는 출력 형식: %s", fmtName)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func emit(o *options, content string) error {
	if o.outputFile != "" {
		data := content
		if !strings.HasSuffix(data, "\n") {
			data += "\n"
		}
		if err := os.WriteFile(o.outputFile, []byte(data), 0o644); err != nil {
			return err
		}
		slog.Info(i18n.T("log.result_written", map[string]interface{}{"path": o.outputFile}))
		return nil
	}
	fmt.Println(content)
	return nil
}
