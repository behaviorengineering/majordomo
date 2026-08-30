package reviewrun

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func authConfigArgs(token, scm string) []string {
	if token == "" {
		return nil
	}
	var header string
	switch strings.ToLower(strings.TrimSpace(scm)) {
	case "gitlab":
		header = "PRIVATE-TOKEN: " + token
	default:
		header = "Authorization: Bearer " + token
	}
	return []string{"-c", "http.extraHeader=" + header}
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

func shaMatch(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	if got == want {
		return true
	}
	if len(want) >= 7 && strings.HasPrefix(got, want) {
		return true
	}
	if len(got) >= 7 && strings.HasPrefix(want, got) {
		return true
	}
	return false
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

func splitOwnerName(cloneURL string) (owner, name string) {
	path := strings.TrimSuffix(cloneURL, ".git")
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "http://")
	if i := strings.Index(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}
