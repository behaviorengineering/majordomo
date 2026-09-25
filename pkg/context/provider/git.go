package contextprovider

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/behaviorengineering/majordomo/pkg/forge/githttps"
)

func authConfigArgs(token, scm string) []string {
	return githttps.ExtraHeaderArgs(token, scm)
}

func git(dir, token, scm string, args ...string) (string, error) {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, authConfigArgs(token, scm)...)
	if strings.TrimSpace(dir) != "" {
		cmdArgs = append(cmdArgs, "-C", dir)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func gitTrim(dir, token, scm string, args ...string) (string, error) {
	out, err := git(dir, token, scm, args...)
	return strings.TrimSpace(out), err
}

func gitAllowFail(dir, token, scm string, args ...string) (string, int) {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, authConfigArgs(token, scm)...)
	if strings.TrimSpace(dir) != "" {
		cmdArgs = append(cmdArgs, "-C", dir)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout.String(), ee.ExitCode()
	}
	return stdout.String(), 1
}

func isGitRepo(dir string) bool {
	_, code := gitAllowFail(dir, "", "", "rev-parse", "--is-inside-work-tree")
	return code == 0
}
