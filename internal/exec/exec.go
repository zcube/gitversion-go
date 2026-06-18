// Package exec 는 외부 명령 실행 훅(semantic-release exec 플러그인 유사)을 제공한다.
//
// 계산된 버전 변수를 GitVersion_* 환경변수와 {Variable}/{env:VAR} 템플릿으로 노출하고,
// 라이프사이클 훅 명령을 실행한다. version 훅은 명령의 표준출력으로 버전 정보를
// 수정(next-version 덮어쓰기 후 재계산)할 수 있다.
package exec

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/zcube/gitversion-go/internal/output"
	"github.com/zcube/gitversion-go/internal/rx"
)

// HookOrder 는 side-effect 훅 실행 순서.
var HookOrder = []string{"verify", "prepare", "publish", "success"}

var tokenRe = rx.MustCompile(`\{(?<t>[A-Za-z0-9_:]+)\}`)

// render 는 명령 문자열의 {Variable}/{env:VAR} 토큰을 치환(미지의 토큰은 보존).
func render(cmd string, m map[string]string) string {
	return tokenRe.ReplaceAllFunc(cmd, func(mt *rx.Match) string {
		t, _ := mt.Named("t")
		if envVar, ok := strings.CutPrefix(t, "env:"); ok {
			return os.Getenv(envVar)
		}
		if v, ok := m[t]; ok {
			return v
		}
		return "{" + t + "}"
	})
}

func envVars(vars *output.VersionVariables) []string {
	m := vars.ToMap()
	out := make([]string, 0, len(m))
	for _, k := range output.SortedKeys(m) {
		out = append(out, fmt.Sprintf("GitVersion_%s=%s", k, m[k]))
	}
	return out
}

// runCommand 는 쉘로 명령을 실행한다. capture 면 stdout 을 수집해 반환.
func runCommand(cmd string, vars *output.VersionVariables, workDir string, capture, dryRun bool) (string, bool, error) {
	rendered := render(cmd, vars.ToMap())
	if dryRun {
		slog.Info("[dry-run] " + rendered)
		fmt.Fprintln(os.Stderr, "[dry-run] "+rendered)
		return "", false, nil
	}
	slog.Info("실행: " + rendered)

	program, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		program, flag = "cmd", "/C"
	}
	c := exec.Command(program, flag, rendered)
	c.Dir = workDir
	c.Env = append(os.Environ(), envVars(vars)...)
	c.Stderr = os.Stderr
	if capture {
		out, err := c.Output()
		if err != nil {
			return "", false, fmt.Errorf("명령 실패 (%v): %s", err, rendered)
		}
		return string(out), true, nil
	}
	c.Stdout = os.Stdout
	if err := c.Run(); err != nil {
		return "", false, fmt.Errorf("명령 실패 (%v): %s", err, rendered)
	}
	return "", false, nil
}

// RunVersionHook 은 version 훅(또는 --exec-version)을 실행한다. stdout 의 첫 비어있지
// 않은 줄을 반환한다. 호출자는 그 값을 next-version 으로 적용해 재계산한다.
func RunVersionHook(cmd string, vars *output.VersionVariables, workDir string, dryRun bool) (string, bool, error) {
	out, ok, err := runCommand(cmd, vars, workDir, true, dryRun)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, true, nil
		}
	}
	return "", false, nil
}

// RunHooks 는 side-effect 훅(verify/prepare/publish/success)을 순서대로 실행한다.
// 실패 시 fail 훅이 있으면 실행하고 에러를 전파한다. extraPrepare 는 --exec 로 준
// 임시 prepare 명령(설정 prepare 다음에 실행).
func RunHooks(hooks map[string]string, extraPrepare string, vars *output.VersionVariables, workDir string, dryRun bool) error {
	var resultErr error
	for _, name := range HookOrder {
		if cmd, ok := hooks[name]; ok {
			if _, _, err := runCommand(cmd, vars, workDir, false, dryRun); err != nil {
				resultErr = fmt.Errorf("'%s' 훅 실패: %w", name, err)
				break
			}
		}
		if name == "prepare" && extraPrepare != "" {
			if _, _, err := runCommand(extraPrepare, vars, workDir, false, dryRun); err != nil {
				resultErr = fmt.Errorf("--exec prepare 명령 실패: %w", err)
				break
			}
		}
	}

	if resultErr != nil {
		if failCmd, ok := hooks["fail"]; ok {
			slog.Warn("fail 훅 실행")
			_, _, _ = runCommand(failCmd, vars, workDir, false, dryRun)
		}
	}
	return resultErr
}
