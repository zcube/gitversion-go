// Package buildagent 는 빌드에이전트(CI) 통합을 제공한다.
//
// 원본 GitVersion.BuildAgents 의 각 에이전트를 옮긴다. 환경변수로 현재 CI 를 감지하고,
// 변수/빌드번호를 해당 CI 의 형식으로 출력한다. update_build_number 가 false 면
// 빌드번호 설정을 생략한다(원본 BuildAgentBase.WriteIntegration).
package buildagent

import (
	"fmt"
	"os"
	"strings"

	"github.com/zcube/go-gitversion/internal/output"
)

// escapeValue 는 TeamCity/MyGet service message 값 이스케이프.
func escapeValue(v string) string {
	r := strings.NewReplacer(
		"|", "||",
		"'", "|'",
		"[", "|[",
		"]", "|]",
		"\r", "|r",
		"\n", "|n",
	)
	return r.Replace(v)
}

// Agent 는 빌드에이전트. 메서드는 함수 필드로 표현하며, nil 이면 기본 동작을 쓴다.
type Agent struct {
	name             string
	setBuildNumber   func(vars *output.VersionVariables) string
	setOutputVar     func(name, value string) []string
	writeIntegration func(a *Agent, vars *output.VersionVariables, ubn bool) []string
}

// Name 은 원본 클래스명(GetType().Name)과 동일.
func (a *Agent) Name() string { return a.name }

// SetBuildNumber 는 빌드번호 설정 출력(기본 FullSemVer).
func (a *Agent) SetBuildNumber(vars *output.VersionVariables) string {
	if a.setBuildNumber != nil {
		return a.setBuildNumber(vars)
	}
	return vars.FullSemVer
}

// SetOutputVariable 는 단일 변수 출력 라인들.
func (a *Agent) SetOutputVariable(name, value string) []string {
	if a.setOutputVar != nil {
		return a.setOutputVar(name, value)
	}
	return nil
}

// WriteIntegration 은 전체 통합 출력(로그 라인 포함).
func (a *Agent) WriteIntegration(vars *output.VersionVariables, ubn bool) []string {
	if a.writeIntegration != nil {
		return a.writeIntegration(a, vars, ubn)
	}
	return baseIntegration(a, vars, ubn)
}

// baseIntegration 은 기본 WriteIntegration 동작(원본 BuildAgentBase).
func baseIntegration(a *Agent, vars *output.VersionVariables, ubn bool) []string {
	var out []string
	if ubn {
		out = append(out, fmt.Sprintf("Set Build Number for '%s'.", a.name))
		bn := a.SetBuildNumber(vars)
		if bn != "" {
			out = append(out, bn)
		}
	}
	out = append(out, fmt.Sprintf("Set Output Variables for '%s'.", a.name))
	m := vars.ToMap()
	for _, key := range output.SortedKeys(m) {
		out = append(out, a.SetOutputVariable(key, m[key])...)
	}
	return out
}

func keyValueLine(name, value string) []string {
	return []string{fmt.Sprintf("GitVersion_%s=%s", name, value)}
}

func sortedMap(vars *output.VersionVariables) (map[string]string, []string) {
	m := vars.ToMap()
	return m, output.SortedKeys(m)
}

func writePropertiesFile(vars *output.VersionVariables) {
	m, keys := sortedMap(vars)
	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("GitVersion_%s=%s", k, m[k]))
	}
	_ = os.WriteFile("gitversion.properties", []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func replaceAzureVars(buildNumber string, vars *output.VersionVariables) string {
	m, keys := sortedMap(vars)
	out := buildNumber
	for _, k := range keys {
		out = strings.ReplaceAll(out, fmt.Sprintf("$(GITVERSION_%s)", k), m[k])
		out = strings.ReplaceAll(out, fmt.Sprintf("$(GITVERSION.%s)", k), m[k])
	}
	return out
}

func teamCity() *Agent {
	return &Agent{
		name: "TeamCity",
		setBuildNumber: func(v *output.VersionVariables) string {
			return fmt.Sprintf("##teamcity[buildNumber '%s']", escapeValue(v.FullSemVer))
		},
		setOutputVar: func(name, value string) []string {
			e := escapeValue(value)
			return []string{
				fmt.Sprintf("##teamcity[setParameter name='GitVersion.%s' value='%s']", name, e),
				fmt.Sprintf("##teamcity[setParameter name='system.GitVersion.%s' value='%s']", name, e),
			}
		},
	}
}

func myGet() *Agent {
	return &Agent{
		name: "MyGet",
		setBuildNumber: func(v *output.VersionVariables) string {
			return fmt.Sprintf("##myget[buildNumber '%s']", escapeValue(v.FullSemVer))
		},
		setOutputVar: func(name, value string) []string {
			return []string{fmt.Sprintf("##myget[setParameter name='GitVersion.%s' value='%s']", name, escapeValue(value))}
		},
	}
}

func azurePipelines() *Agent {
	return &Agent{
		name: "AzurePipelines",
		setBuildNumber: func(v *output.VersionVariables) string {
			bn, ok := os.LookupEnv("BUILD_BUILDNUMBER")
			if ok && strings.TrimSpace(bn) != "" {
				replaced := replaceAzureVars(bn, v)
				if replaced != bn {
					return "##vso[build.updatebuildnumber]" + replaced
				}
				val := strings.TrimSuffix(v.FullSemVer, "+0")
				return "##vso[build.updatebuildnumber]" + val
			}
			return v.FullSemVer
		},
		setOutputVar: func(name, value string) []string {
			return []string{
				fmt.Sprintf("##vso[task.setvariable variable=GitVersion.%s]%s", name, value),
				fmt.Sprintf("##vso[task.setvariable variable=GitVersion.%s;isOutput=true]%s", name, value),
			}
		},
	}
}

func continuaCi() *Agent {
	return &Agent{
		name: "ContinuaCi",
		setBuildNumber: func(v *output.VersionVariables) string {
			return fmt.Sprintf("@@continua[setBuildVersion value='%s']", v.FullSemVer)
		},
		setOutputVar: func(name, value string) []string {
			return []string{fmt.Sprintf("@@continua[setVariable name='GitVersion_%s' value='%s' skipIfNotDefined='true']", name, value)}
		},
	}
}

func envRun() *Agent {
	return &Agent{
		name: "EnvRun",
		setOutputVar: func(name, value string) []string {
			return []string{fmt.Sprintf("@@envrun[set name='GitVersion_%s' value='%s']", name, value)}
		},
	}
}

func travisCi() *Agent {
	return &Agent{name: "TravisCi", setOutputVar: keyValueLine}
}

func drone() *Agent {
	return &Agent{name: "Drone", setOutputVar: keyValueLine}
}

func propertiesAgent(name string) *Agent {
	return &Agent{
		name:         name,
		setOutputVar: keyValueLine,
		writeIntegration: func(a *Agent, vars *output.VersionVariables, ubn bool) []string {
			out := baseIntegration(a, vars, ubn)
			if a.name == "GitLabCi" {
				out = append(out, "Outputting variables to 'gitversion.properties' ... ")
				writePropertiesFile(vars)
			} else {
				writePropertiesFile(vars)
				out = append(out, "Outputting variables to 'gitversion.properties' ... ")
			}
			return out
		},
	}
}

func bitBucketPipelines() *Agent {
	return &Agent{
		name: "BitBucketPipelines",
		setOutputVar: func(name, value string) []string {
			return []string{fmt.Sprintf("GITVERSION_%s=%s", strings.ToUpper(name), value)}
		},
		writeIntegration: func(a *Agent, vars *output.VersionVariables, ubn bool) []string {
			out := baseIntegration(a, vars, ubn)
			const pf = "gitversion.properties"
			const ps1 = "gitversion.ps1"
			m, keys := sortedMap(vars)
			var exports []string
			for _, k := range keys {
				exports = append(exports, fmt.Sprintf("export GITVERSION_%s=%s", strings.ToUpper(k), m[k]))
			}
			_ = os.WriteFile(pf, []byte(strings.Join(exports, "\n")+"\n"), 0o644)
			out = append(out,
				fmt.Sprintf("Outputting variables to '%s' for Bash,", pf),
				fmt.Sprintf("and to '%s' for Powershell ... ", ps1),
				"To import the file into your build environment, add the following line to your build step:",
				"Bash:",
				fmt.Sprintf("  - source %s", pf),
				"Powershell:",
				fmt.Sprintf("  - . .\\%s", ps1),
				"",
				"To reuse the file across build steps, add the file as a build artifact:",
				"Bash:",
				"  artifacts:",
				fmt.Sprintf("    - %s", pf),
				"Powershell:",
				"  artifacts:",
				fmt.Sprintf("    - %s", ps1),
			)
			return out
		},
	}
}

func gitHubActions() *Agent {
	return &Agent{
		name:           "GitHubActions",
		setBuildNumber: func(v *output.VersionVariables) string { return "" },
		setOutputVar:   func(name, value string) []string { return nil },
		writeIntegration: func(a *Agent, vars *output.VersionVariables, ubn bool) []string {
			out := baseIntegration(a, vars, ubn)
			path, ok := os.LookupEnv("GITHUB_ENV")
			if ok {
				out = append(out, fmt.Sprintf("Writing version variables to $GITHUB_ENV file for '%s'.", a.name))
				m, keys := sortedMap(vars)
				var lines []string
				for _, k := range keys {
					if m[k] != "" {
						lines = append(lines, fmt.Sprintf("GitVersion_%s=%s", k, m[k]))
					}
				}
				if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
					fmt.Fprintln(f, strings.Join(lines, "\n"))
					f.Close()
				}
			} else {
				out = append(out, "Unable to write GitVersion variables to $GITHUB_ENV because the environment variable is not set.")
			}
			return out
		},
	}
}

func logOnly(name string) *Agent {
	return &Agent{
		name:           name,
		setBuildNumber: func(v *output.VersionVariables) string { return "" },
		setOutputVar:   func(name, value string) []string { return nil },
	}
}

// appVeyorBuildNumberBody / appVeyorOutputVariableBody 는 원본 AppVeyor 가 보내는
// REST 요청 body(JSON) 형식이다(PUT api/build, POST api/build/variables). 실제 전송은
// 환경 의존이라 하지 않고 형식만 단위 테스트로 검증한다.
func appVeyorBuildNumberBody(vars *output.VersionVariables, buildNumber string) string {
	return fmt.Sprintf(`{"version":"%s.build.%s"}`, vars.FullSemVer, buildNumber)
}

func appVeyorOutputVariableBody(name, value string) string {
	return fmt.Sprintf(`{"name":"GitVersion_%s","value":"%s"}`, name, value)
}

func appVeyor() *Agent {
	return &Agent{
		name: "AppVeyor",
		setBuildNumber: func(v *output.VersionVariables) string {
			return fmt.Sprintf("Set AppVeyor build number to '%s'.", v.FullSemVer)
		},
		setOutputVar: func(name, value string) []string {
			return []string{fmt.Sprintf("Adding Environment Variable. name='GitVersion_%s' value='%s']", name, value)}
		},
	}
}

// ByName 은 에이전트 이름으로 인스턴스 생성. 알 수 없으면 nil.
func ByName(name string) *Agent {
	switch name {
	case "TeamCity":
		return teamCity()
	case "MyGet":
		return myGet()
	case "AzurePipelines":
		return azurePipelines()
	case "ContinuaCi":
		return continuaCi()
	case "EnvRun":
		return envRun()
	case "TravisCI", "TravisCi":
		return travisCi()
	case "Drone":
		return drone()
	case "GitLabCi":
		return propertiesAgent("GitLabCi")
	case "Jenkins":
		return propertiesAgent("Jenkins")
	case "CodeBuild":
		return propertiesAgent("CodeBuild")
	case "BitBucketPipelines":
		return bitBucketPipelines()
	case "GitHubActions":
		return gitHubActions()
	case "BuildKite":
		return logOnly("BuildKite")
	case "SpaceAutomation":
		return logOnly("SpaceAutomation")
	case "AppVeyor":
		return appVeyor()
	default:
		return nil
	}
}

// Detect 는 환경변수로 현재 빌드에이전트 감지(원본 등록 순서와 유사).
func Detect() *Agent {
	has := func(k string) bool {
		v, ok := os.LookupEnv(k)
		return ok && v != ""
	}
	switch {
	case has("TEAMCITY_VERSION"):
		return teamCity()
	case has("TF_BUILD"):
		return azurePipelines()
	case has("GITHUB_ACTIONS"):
		return gitHubActions()
	case has("GITLAB_CI"):
		return propertiesAgent("GitLabCi")
	case has("JENKINS_URL"):
		return propertiesAgent("Jenkins")
	case has("CODEBUILD_WEBHOOK_HEAD_REF"):
		return propertiesAgent("CodeBuild")
	case has("TRAVIS"):
		return travisCi()
	case has("DRONE"):
		return drone()
	case has("APPVEYOR"):
		return appVeyor()
	case has("ENVRUN_DATABASE"):
		return envRun()
	case has("ContinuaCI.Version"):
		return continuaCi()
	case has("BITBUCKET_WORKSPACE"):
		return bitBucketPipelines()
	case has("BUILDKITE"):
		return logOnly("BuildKite")
	case has("JB_SPACE_PROJECT_KEY"):
		return logOnly("SpaceAutomation")
	default:
		if v, ok := os.LookupEnv("BuildRunner"); ok && strings.EqualFold(v, "MyGet") {
			return myGet()
		}
		return nil
	}
}
