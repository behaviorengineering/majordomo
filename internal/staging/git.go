package staging

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GitError is a non-zero git exit.
type GitError struct {
	Args string
	Err  string
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s failed: %s", e.Args, e.Err)
}

// GitRunner runs git in a working directory.
type GitRunner struct {
	Dir     string
	Timeout time.Duration
}

func (g *GitRunner) timeout() time.Duration {
	if g.Timeout > 0 {
		return g.Timeout
	}
	return GitTimeout
}

func (g *GitRunner) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &GitError{Args: strings.Join(args, " "), Err: strings.TrimSpace(stderr.String())}
	}
	return stdout.String(), nil
}

func (g *GitRunner) runAllowFail(args ...string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return stdout.String(), ee.ExitCode()
		}
		return stdout.String(), 1
	}
	return stdout.String(), 0
}

// SetupGitResult holds git setup outputs.
type SetupGitResult struct {
	Refspec     string
	RepoRoot    string
	AllFiles    []string
	FileStatus  map[string]string
	StatusPairs [][2]string
	ExtraExcl   []*regexp.Regexp
}

// SetupGit configures safe.directory, validates merge-base, lists changed files.
func SetupGit(baseBranch, stagingDir, workDir string) (*SetupGitResult, error) {
	g := &GitRunner{Dir: workDir}
	extra := GetSubmoduleExclusions(g)

	cwd := workDir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fatalf("cannot get cwd: %v", err)
		}
	}
	// Best-effort safe.directory (may fail without write access to global gitconfig).
	_ = exec.Command("git", "config", "--global", "--add", "safe.directory", cwd).Run()

	refspec := fmt.Sprintf("origin/%s...HEAD", baseBranch)
	logf("INFO", "%s", strings.Repeat("=", 50))
	logf("INFO", "majordomo prep")
	logf("INFO", "%s", strings.Repeat("=", 50))
	logf("INFO", "Base branch:  %s", baseBranch)
	logf("INFO", "Refspec:      %s", refspec)
	logf("INFO", "Staging dir:  %s", stagingDir)

	shallowOut, _ := g.runAllowFail("rev-parse", "--is-shallow-repository")
	isShallow := strings.TrimSpace(shallowOut) == "true"
	logf("INFO", "Shallow clone: %v", isShallow)

	mbOut, mbCode := g.runAllowFail("merge-base", "origin/"+baseBranch, "HEAD")
	if mbCode != 0 {
		return nil, fatalf(
			"No common ancestor found between 'origin/%s' and HEAD. "+
				"The feature branch and '%s' have disconnected git histories. "+
				"Ensure '%s' in the target repo was pushed from the same origin "+
				"as the feature branch so a merge base exists.",
			baseBranch, baseBranch, baseBranch,
		)
	}
	logf("INFO", "Merge base: %s", strings.TrimSpace(mbOut))

	repoRootOut, err := g.run("rev-parse", "--show-toplevel")
	if err != nil {
		logf("ERROR", "%v", err)
		return nil, fatalf("%v", err)
	}
	repoRoot := strings.TrimSpace(repoRootOut)

	changedRaw, err := g.run("diff", "-z", "--name-status", refspec)
	if err != nil {
		logf("ERROR", "%v", err)
		return nil, fatalf("%v", err)
	}

	statusPairs := ParseNameStatus(changedRaw)
	allFiles := make([]string, 0, len(statusPairs))
	fileStatus := make(map[string]string, len(statusPairs))
	for _, p := range statusPairs {
		allFiles = append(allFiles, p[1])
		fileStatus[p[1]] = p[0]
	}

	logf("INFO", "Raw diff -z --name-status output (%d files):", len(allFiles))
	if len(allFiles) == 0 {
		logf("INFO", "  (empty)")
	} else {
		for _, p := range statusPairs {
			logf("INFO", "  [%s] %s", p[0], p[1])
		}
	}
	if len(allFiles) == 0 {
		logf("WARN", "No changes detected against origin/%s", baseBranch)
		return nil, ErrNothingToReview
	}

	return &SetupGitResult{
		Refspec:     refspec,
		RepoRoot:    repoRoot,
		AllFiles:    allFiles,
		FileStatus:  fileStatus,
		StatusPairs: statusPairs,
		ExtraExcl:   extra,
	}, nil
}

// GetSubmoduleExclusions parses git submodule status --cached.
func GetSubmoduleExclusions(g *GitRunner) []*regexp.Regexp {
	out, code := g.runAllowFail("submodule", "status", "--cached")
	if code != 0 || strings.TrimSpace(out) == "" {
		return nil
	}
	return ParseSubmoduleStatusLines(out)
}

func (g *GitRunner) diff(refspec, file string) (string, error) {
	return g.run("diff", refspec, "--", file)
}

func (g *GitRunner) showHEAD(file string) string {
	out, code := g.runAllowFail("show", "HEAD:"+filepath.ToSlash(file))
	if code != 0 {
		return ""
	}
	return out
}

// IsExcludedWithExtra checks static + dynamic exclusion patterns.
func IsExcludedWithExtra(file string, extra []*regexp.Regexp) bool {
	if IsExcluded(file) {
		return true
	}
	for _, p := range extra {
		if p.MatchString(file) {
			return true
		}
	}
	return false
}
