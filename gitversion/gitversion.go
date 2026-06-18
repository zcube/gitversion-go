// Package gitversion 은 Git 히스토리로부터 시맨틱 버전을 계산하는 공개 API 다.
//
// GitVersion(.NET)의 Go 포트로, 출력은 실제 GitVersion 6.x 와 차등 테스트로 검증된다.
// 외부 프로그램은 이 패키지를 임포트해 라이브러리로 사용할 수 있다.
//
//	v, err := gitversion.Compute(".")
//	if err != nil { ... }
//	fmt.Println(v.FullSemVer)            // 예: "1.2.3-beta.1+5"
//	fmt.Println(v.ToMap()["SemVer"])     // 단일 변수
//	j, _ := v.ToJSON()                   // GitVersion 호환 JSON
//
// 설정 입력(우선순위 Config > ConfigYAML > ConfigPath/자동탐색):
//
//	// 1) 인라인 YAML(GitVersion.yml 과 동일 형식)
//	v, _ = gitversion.Calculate(gitversion.Options{
//	    Path:       ".",
//	    ConfigYAML: []byte("workflow: GitHubFlow/v1\nnext-version: 2.0.0\n"),
//	})
//
//	// 2) 설정 객체(프리셋에서 만들어 조정)
//	cfg := gitversion.DefaultConfig("GitFlow")
//	nv := "3.0.0"; cfg.NextVersion = &nv
//	v, _ = gitversion.Calculate(gitversion.Options{Path: ".", Config: cfg})
package gitversion

import (
	"os"

	"github.com/goccy/go-yaml"

	"github.com/zcube/gitversion-go/internal/cache"
	"github.com/zcube/gitversion-go/internal/calc"
	"github.com/zcube/gitversion-go/internal/config"
	"github.com/zcube/gitversion-go/internal/git"
	"github.com/zcube/gitversion-go/internal/output"
)

// Variables 는 계산된 GitVersion 출력 변수(Major/Minor/Patch/FullSemVer/...).
//
// 메서드: ToMap()(map[string]string), ToJSON()(string,error),
// ShowVariable(name)(string,error), FormatTemplate(tmpl, getenv)(string,error).
type Variables = output.VersionVariables

// Configuration 은 GitVersion 설정 모델. DefaultConfig/ParseConfig 로 만든다.
type Configuration = config.GitVersionConfiguration

// Options 는 버전 계산 옵션.
type Options struct {
	// Path 는 저장소 경로(.git 포함 디렉터리 또는 그 하위). 비면 ".".
	Path string

	// 설정 입력(우선순위: Config > ConfigYAML > ConfigPath/자동탐색).
	// Config 는 완성된 설정 객체(프리셋+병합 완료본)를 그대로 사용한다.
	Config *Configuration
	// ConfigYAML 은 GitVersion.yml 과 동일 형식의 인라인 설정. 워크플로 기본값과 병합된다.
	ConfigYAML []byte
	// ConfigPath 는 명시 설정 파일 경로. 비면 작업 디렉터리에서 자동 탐색.
	ConfigPath string

	// Branch 는 현재 체크아웃 대신 계산할 브랜치(선택).
	Branch string
	// Overrides 는 인라인 설정 오버라이드(예: "next-version=2.0.0").
	Overrides []string
	// NoCache 가 true 면 디스크 캐시를 읽지/쓰지 않는다.
	NoCache bool
}

// Compute 는 경로만으로 버전을 계산하는 간편 함수.
func Compute(path string) (*Variables, error) {
	return Calculate(Options{Path: path})
}

// Calculate 는 옵션에 따라 버전을 계산해 출력 변수를 반환한다.
func Calculate(opts Options) (*Variables, error) {
	path := opts.Path
	if path == "" {
		path = "."
	}

	repo, err := git.Discover(path)
	if err != nil {
		return nil, err
	}
	workdir := repo.Workdir()
	if workdir == "" {
		workdir = path
	}

	// 설정 결정: Config > ConfigYAML > ConfigPath/자동탐색.
	var cfg *config.GitVersionConfiguration
	var cfgContent []byte // 캐시 키용 설정 내용
	switch {
	case opts.Config != nil:
		cfg = opts.Config
		cfgContent, _ = yaml.Marshal(cfg)
	case opts.ConfigYAML != nil:
		cfg, err = config.ParseYAML(opts.ConfigYAML, workdir)
		if err != nil {
			return nil, err
		}
		cfgContent = opts.ConfigYAML
	default:
		cfg, err = config.Load(opts.ConfigPath, workdir, workdir)
		if err != nil {
			return nil, err
		}
		cfgPath := opts.ConfigPath
		if cfgPath == "" {
			cfgPath = config.Locate(workdir, workdir)
		}
		if cfgPath != "" {
			cfgContent, _ = os.ReadFile(cfgPath)
		}
	}

	config.ApplyOverrides(cfg, opts.Overrides)

	var branchOverride *string
	if opts.Branch != "" {
		branchOverride = &opts.Branch
	}

	if opts.NoCache {
		return calc.Calculate(repo, cfg, branchOverride)
	}

	keyInputs := append([]string(nil), opts.Overrides...)
	if opts.Branch != "" {
		keyInputs = append(keyInputs, "branch="+opts.Branch)
	}
	key := cache.ComputeKeyContent(repo, cfgContent, keyInputs)
	if v := cache.Load(repo, key); v != nil {
		return v, nil
	}
	v, err := calc.Calculate(repo, cfg, branchOverride)
	if err != nil {
		return nil, err
	}
	cache.Store(repo, key, v)
	return v, nil
}

// DefaultConfig 은 워크플로 프리셋 기본 설정을 반환한다.
// workflow: "GitFlow"(기본) | "GitHubFlow" | "TrunkBased"/"Mainline".
func DefaultConfig(workflow string) *Configuration {
	if workflow == "" {
		return config.GitFlow()
	}
	return config.ForWorkflow(&workflow)
}

// ParseConfig 는 GitVersion.yml 형식의 YAML 을 워크플로 기본값과 병합해 설정 객체로 만든다.
func ParseConfig(yamlData []byte) (*Configuration, error) {
	return config.ParseYAML(yamlData, "")
}

// LoadConfiguration 은 작업 디렉터리 기준으로 effective 설정을 로드한다(파일 자동 탐색).
func LoadConfiguration(workDir, configPath string) (*Configuration, error) {
	return config.Load(configPath, workDir, workDir)
}
