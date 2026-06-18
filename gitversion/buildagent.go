package gitversion

import "github.com/zcube/gitversion-go/internal/buildagent"

// BuildAgent 는 빌드에이전트(CI) 통합 어댑터. 메서드: Name(), SetBuildNumber(*Variables),
// SetOutputVariable(name, value)([]string), WriteIntegration(*Variables, updateBuildNumber)([]string).
type BuildAgent = buildagent.Agent

// DetectBuildAgent 는 환경변수로 현재 CI 를 감지한다. 감지되지 않으면 nil.
func DetectBuildAgent() *BuildAgent {
	return buildagent.Detect()
}

// BuildAgentByName 은 이름으로 빌드에이전트를 생성한다(TeamCity/AzurePipelines/GitHubActions
// /GitLabCi/Jenkins/AppVeyor/TravisCi/Drone/CodeBuild/ContinuaCi/EnvRun/MyGet/
// BitBucketPipelines/BuildKite/SpaceAutomation). 알 수 없으면 nil.
func BuildAgentByName(name string) *BuildAgent {
	return buildagent.ByName(name)
}

// BuildServerOutput 은 현재 감지된 CI 형식으로 변수 통합 출력 라인을 만든다.
// CI 가 감지되지 않으면 (nil, false). updateBuildNumber 는 빌드번호 갱신 명령 포함 여부.
func BuildServerOutput(v *Variables, updateBuildNumber bool) ([]string, bool) {
	agent := buildagent.Detect()
	if agent == nil {
		return nil, false
	}
	return agent.WriteIntegration(v, updateBuildNumber), true
}
