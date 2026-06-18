package gogitversion_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zcube/go-gitversion/internal/buildagent"
	"github.com/zcube/go-gitversion/internal/calc"
	"github.com/zcube/go-gitversion/internal/config"
	"github.com/zcube/go-gitversion/internal/git"
)

// keepLine: UncommittedChanges(비결정적) 와 빈 줄은 비교에서 제외.
func keepLine(line string) bool {
	return line != "" && !strings.Contains(strings.ToUpper(line), "UNCOMMITTEDCHANGES")
}

func TestBuildAgentsMatchRealGitVersion(t *testing.T) {
	root := extractFixtures(t)
	agents := []string{
		"TeamCity", "AzurePipelines", "ContinuaCi", "MyGet", "Drone",
		"BitBucketPipelines", "Jenkins", "CodeBuild", "BuildKite", "SpaceAutomation",
		"EnvRun", "TravisCi", "GitLabCi", "GitHubActions",
	}

	var failures []string
	checked := 0

	for _, scenario := range []string{"buildagent_repo", "buildagent_no_ubn"} {
		repoDir := filepath.Join(root, scenario)
		if _, err := os.Stat(filepath.Join(repoDir, "expected.json")); err != nil {
			failures = append(failures, scenario+" 시나리오가 없습니다")
			continue
		}
		repo, err := git.Discover(repoDir)
		if err != nil {
			failures = append(failures, fmt.Sprintf("[%s] 저장소 오픈 실패: %v", scenario, err))
			continue
		}
		workdir := repo.Workdir()
		if workdir == "" {
			workdir = repoDir
		}
		cfg, err := config.Load("", workdir, workdir)
		if err != nil {
			failures = append(failures, fmt.Sprintf("[%s] 설정 로드 실패: %v", scenario, err))
			continue
		}
		ubn := cfg.UpdateBuildNumber == nil || *cfg.UpdateBuildNumber
		vars, err := calc.Calculate(repo, cfg, nil)
		if err != nil {
			failures = append(failures, fmt.Sprintf("[%s] 계산 실패: %v", scenario, err))
			continue
		}

		for _, agentName := range agents {
			goldenPath := filepath.Join(repoDir, "agent_"+agentName+".txt")
			goldenBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("[%s/%s] golden 파일 없음", scenario, agentName))
				continue
			}
			agent := buildagent.ByName(agentName)
			if agent == nil {
				failures = append(failures, fmt.Sprintf("[%s/%s] 알 수 없는 에이전트", scenario, agentName))
				continue
			}
			if agentName == "GitHubActions" {
				tmp := filepath.Join(t.TempDir(), "gh_env")
				t.Setenv("GITHUB_ENV", tmp)
			}
			var golden []string
			for _, l := range strings.Split(string(goldenBytes), "\n") {
				if keepLine(l) {
					golden = append(golden, l)
				}
			}
			var mine []string
			for _, l := range agent.WriteIntegration(vars, ubn) {
				if keepLine(l) {
					mine = append(mine, l)
				}
			}
			if len(mine) != len(golden) {
				failures = append(failures, fmt.Sprintf("[%s/%s] 라인 수 불일치: real=%d mine=%d", scenario, agentName, len(golden), len(mine)))
				continue
			}
			for i := range golden {
				if golden[i] != mine[i] {
					failures = append(failures, fmt.Sprintf("[%s/%s] line %d: real=%q mine=%q", scenario, agentName, i, golden[i], mine[i]))
				}
			}
			checked++
		}
	}

	// 부수효과로 생성된 properties/ps1 파일 정리.
	for _, f := range []string{"gitversion.properties", "gitversion.ps1"} {
		_ = os.Remove(f)
	}

	if len(failures) > 0 {
		t.Fatalf("%d개 에이전트 검증 중 불일치:\n%s", checked, joinLines(failures))
	}
	t.Logf("%d개 빌드에이전트 출력이 실제 GitVersion 과 일치", checked)
}
