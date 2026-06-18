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

	"github.com/zcube/gitversion-go/internal/buildagent"
	"github.com/zcube/gitversion-go/internal/cache"
	"github.com/zcube/gitversion-go/internal/calc"
	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/exec"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/i18n"
	"github.com/zcube/gitversion-go/internal/output"
	"github.com/zcube/gitversion-go/internal/remote"
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
	logFile        string
	execHook       string
	execVersion    string
	dryRun         bool
	url            string
	username       string
	password       string
	commit         string
	dynamicRepoDir string
	// 파일 출력.
	updateAssemblyInfo bool
	ensureAssemblyInfo bool
	assemblyInfoFiles  []string
	updateProjectFiles bool
	projectFiles       []string
	updateWixFile      bool
	updatePackageFiles bool
	packageFiles       []string
	// 원본 CLI 호환용 no-op 플래그.
	nofetch, nonormalize, nocache, allowshallow bool
}

// NewRootCommand 은 루트 cobra 명령을 생성한다.
func NewRootCommand(version string) *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:           "gitversion-go [path]",
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
	f.StringVarP(&o.logFile, "log", "l", "", "Append log output to FILE (timestamped), or 'console' for stderr")
	f.StringVar(&o.execHook, "exec", "", "Extra prepare hook command to run")
	f.StringVar(&o.execVersion, "exec-version", "", "Version hook: command whose stdout overrides next-version")
	f.BoolVar(&o.dryRun, "dry-run", false, "Print hook commands without executing them")
	f.StringVar(&o.url, "url", "", "Remote repository URL to clone dynamically")
	f.StringVarP(&o.username, "username", "u", "", "Username for remote authentication")
	f.StringVarP(&o.password, "password", "p", "", "Password/token for remote authentication")
	f.StringVarP(&o.commit, "commit", "c", "", "Commit to check out after a dynamic clone")
	f.StringVar(&o.dynamicRepoDir, "dynamic-repo-location", "", "Directory for dynamic clones (default temp dir)")
	f.BoolVar(&o.updateAssemblyInfo, "updateassemblyinfo", false, "Update AssemblyInfo files (recursive search if no files given)")
	f.BoolVar(&o.ensureAssemblyInfo, "ensureassemblyinfo", false, "Create the AssemblyInfo file if it does not exist")
	f.StringArrayVar(&o.assemblyInfoFiles, "assemblyinfo-file", nil, "AssemblyInfo file to update (repeatable)")
	f.BoolVar(&o.updateProjectFiles, "updateprojectfiles", false, "Update .csproj/.vbproj/.fsproj version elements")
	f.StringArrayVar(&o.projectFiles, "project-file", nil, "Project file to update (repeatable)")
	f.BoolVar(&o.updateWixFile, "updatewixversionfile", false, "Write GitVersion_WixVersion.wxi")
	f.BoolVar(&o.updatePackageFiles, "updatepackagefiles", false, "Update package.json/Cargo.toml/pyproject.toml version")
	f.StringArrayVar(&o.packageFiles, "package-file", nil, "Package manifest to update (repeatable)")
	f.BoolVar(&o.nofetch, "nofetch", false, "Disable fetch (no-op)")
	f.BoolVar(&o.nonormalize, "nonormalize", false, "Disable normalization (no-op)")
	f.BoolVar(&o.nocache, "nocache", false, "Disable disk cache (no-op)")
	f.BoolVar(&o.allowshallow, "allowshallow", false, "Allow shallow clone (no-op)")

	return cmd
}

// dropTime 은 slog 출력에서 time 속성을 제거한다(콘솔/stderr 용, 원본의 무타임스탬프).
func dropTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}

// setupLogging 은 verbosity/diag 로 레벨을 정하고 로그 대상을 설정한다.
//
//   - logFile == ""      : stderr(타임스탬프 없음)
//   - logFile == "console": stderr(타임스탬프 없음)
//   - 그 외              : 파일에 append(타임스탬프 포함)
//
// stdout 은 항상 버전 결과 전용으로 비워 둔다. 반환값은 cleanup(파일 close).
func setupLogging(verbosity string, diag bool, logFile string) (func(), error) {
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

	cleanup := func() {}
	var h slog.Handler
	if logFile != "" && !strings.EqualFold(logFile, "console") {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return cleanup, fmt.Errorf("로그 파일 열기 실패 %s: %w", logFile, err)
		}
		cleanup = func() { _ = f.Close() }
		// 파일 로그에는 타임스탬프를 남긴다(원본 GitVersion 로그 파일과 동일).
		h = slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	} else {
		// stderr/console: 타임스탬프 없이.
		h = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level, ReplaceAttr: dropTime})
	}
	slog.SetDefault(slog.New(h))
	return cleanup, nil
}

func run(o *options, path string) error {
	i18n.Init(o.lang)
	cleanup, err := setupLogging(o.verbosity, o.diag, o.logFile)
	if err != nil {
		return err
	}
	defer cleanup()

	// --url 이 주어지면 원격 저장소를 동적으로 clone 해 그 경로를 대상으로 사용.
	target := path
	if o.url != "" {
		dest, err := remote.Prepare(&remote.Options{
			URL:      o.url,
			Branch:   o.branch,
			Username: o.username,
			Password: o.password,
			Commit:   o.commit,
			Location: o.dynamicRepoDir,
		})
		if err != nil {
			return err
		}
		target = dest
	} else if o.targetPath != "" {
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
	config.ApplyOverrides(cfg, o.overrideConfig)

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

	// 디스크 캐시: refs/HEAD/설정/override 해시 키. nocache 면 우회.
	keyInputs := append([]string{}, o.overrideConfig...)
	if o.branch != "" {
		keyInputs = append(keyInputs, "branch="+o.branch)
	}
	cfgPath := o.configPath
	if cfgPath == "" {
		cfgPath = config.Locate(workdir, workdir)
	}
	var cacheKey string
	if !o.nocache {
		cacheKey = cache.ComputeKey(repo, cfgPath, keyInputs)
	}

	var vars *output.VersionVariables
	if cacheKey != "" {
		vars = cache.Load(repo, cacheKey)
	}
	if vars == nil {
		v, err := calc.Calculate(repo, cfg, branchOverride)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T("error.calc_failed", nil), err)
		}
		vars = v
		if cacheKey != "" {
			cache.Store(repo, cacheKey, vars)
		}
	}

	// version 훅: 외부 명령 출력으로 next-version 을 덮어쓰고 재계산.
	versionCmd := o.execVersion
	if versionCmd == "" {
		versionCmd = cfg.Exec["version"]
	}
	if versionCmd != "" {
		newVer, ok, err := exec.RunVersionHook(versionCmd, vars, workdir, o.dryRun)
		if err != nil {
			return err
		}
		if ok {
			slog.Info("version 훅이 버전을 수정: " + newVer)
			cfg.NextVersion = &newVer
			vars, err = calc.Calculate(repo, cfg, branchOverride)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("error.calc_failed", nil), err)
			}
		}
	}

	// 파일 출력.
	if o.updateAssemblyInfo {
		paths, err := output.UpdateAssemblyInfo(vars, workdir, o.assemblyInfoFiles, o.ensureAssemblyInfo)
		if err != nil {
			return err
		}
		for _, p := range paths {
			slog.Info("AssemblyInfo 갱신: " + p)
		}
	}
	if o.updateProjectFiles {
		paths, err := output.UpdateProjectFiles(vars, workdir, o.projectFiles)
		if err != nil {
			return err
		}
		for _, p := range paths {
			slog.Info("프로젝트 파일 갱신: " + p)
		}
	}
	if o.updateWixFile {
		p, err := output.WriteWix(vars, workdir)
		if err != nil {
			return err
		}
		slog.Info("Wix 파일 생성: " + p)
	}
	if o.updatePackageFiles {
		paths, err := output.UpdatePackageFiles(vars, workdir, o.packageFiles)
		if err != nil {
			return err
		}
		for _, p := range paths {
			slog.Info("패키지 파일 갱신: " + p)
		}
	}

	// side-effect 훅(verify/prepare/publish/success).
	if len(cfg.Exec) > 0 || o.execHook != "" {
		if err := exec.RunHooks(cfg.Exec, o.execHook, vars, workdir, o.dryRun); err != nil {
			return err
		}
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

	rendered, err := render(o, cfg, vars)
	if err != nil {
		return err
	}
	return emit(o, rendered)
}

func render(o *options, cfg *config.GitVersionConfiguration, vars *output.VersionVariables) (string, error) {
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
			if agent := buildagent.Detect(); agent != nil {
				slog.Info("build agent detected: " + agent.Name())
				ubn := cfg == nil || cfg.UpdateBuildNumber == nil || *cfg.UpdateBuildNumber
				parts = append(parts, strings.Join(agent.WriteIntegration(vars, ubn), "\n"))
			} else {
				parts = append(parts, strings.TrimRight(vars.ToBuildServerEnv(), "\n"))
			}
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
