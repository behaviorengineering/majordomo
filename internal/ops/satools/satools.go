// Package satools builds local SA tool Docker images for Dockerfile validation.
package satools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Options configures a local SA tool image build run.
type Options struct {
	DryRun  bool
	Verbose bool
	Corp    bool
	// RepoRoot is the majordomo checkout (directory containing scripts/ or go.mod).
	// Empty → discover from cwd.
	RepoRoot string
	// Runner overrides command execution (tests).
	Runner func(name string, args []string, env []string, dir string) (stdout, stderr string, err error)
}

// Run discovers SA Dockerfiles and builds each via build-copilot-image.sh.
func Run(opts Options) error {
	repoRoot, err := resolveRepoRoot(opts.RepoRoot)
	if err != nil {
		return err
	}
	workspace := workspaceRoot(repoRoot)
	saDir := saToolsDir(repoRoot)
	dockerfiles, err := discoverDockerfiles(saDir)
	if err != nil {
		return err
	}
	if len(dockerfiles) == 0 {
		return fmt.Errorf("no Dockerfiles found in %s", saDir)
	}

	if opts.Corp && !opts.DryRun {
		if os.Getenv("REGISTRY_USER") == "" || os.Getenv("REGISTRY_TOKEN") == "" || os.Getenv("PACKAGE_REGISTRY_HOST") == "" {
			return fmt.Errorf("--corp requires PACKAGE_REGISTRY_HOST, REGISTRY_USER, and REGISTRY_TOKEN")
		}
	}

	mode := "public"
	if opts.Corp {
		mode = "corp"
	}
	tools := make([]string, 0, len(dockerfiles))
	for _, df := range dockerfiles {
		tools = append(tools, toolName(df))
	}
	fmt.Printf("SA Tool Image Builder\nMode:      %s\nContext:   %s\nTools:     %s\nDry-run:   %v\n\n",
		mode, workspace, strings.Join(tools, ", "), opts.DryRun)

	if opts.DryRun {
		for _, df := range dockerfiles {
			fmt.Printf("  [dry-run] would build sa-%s (%s) from %s\n", toolName(df), mode, df)
		}
		return nil
	}

	buildSh, err := findBuildScript(repoRoot, workspace)
	if err != nil {
		return err
	}

	results := map[string]bool{}
	var names []string
	for _, df := range dockerfiles {
		tool := toolName(df)
		names = append(names, tool)
		tag := imageTag(tool)
		fmt.Printf("Building %s (%s) ...\n", tag, mode)
		ok, output := runBuild(opts, buildSh, df, workspace, tag, tool)
		results[tool] = ok
		printResult(tool, ok, output, opts.Verbose)
	}

	sort.Strings(names)
	passed := 0
	for _, n := range names {
		if results[n] {
			passed++
		}
	}
	fmt.Printf("\nResults: %d/%d passed\n", passed, len(names))
	for _, n := range names {
		status := "FAIL"
		if results[n] {
			status = "PASS"
		}
		fmt.Printf("  %s  sa-%s\n", status, n)
	}
	if passed < len(names) {
		return fmt.Errorf("%d/%d SA tool builds failed", len(names)-passed, len(names))
	}
	return nil
}

func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Clean(explicit), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "dockerfiles", "sa-tools")); err == nil {
			return dir, nil
		}
		// Vendored as .majordomo under a parent workspace.
		if filepath.Base(dir) == ".majordomo" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd, nil
}

func saToolsDir(repoRoot string) string {
	primary := filepath.Join(repoRoot, "dockerfiles", "sa-tools")
	if st, err := os.Stat(primary); err == nil && st.IsDir() {
		return primary
	}
	vendored := filepath.Join(filepath.Dir(repoRoot), ".majordomo", "dockerfiles", "sa-tools")
	if st, err := os.Stat(vendored); err == nil && st.IsDir() {
		return vendored
	}
	return primary
}

func workspaceRoot(repoRoot string) string {
	if st, err := os.Stat(filepath.Join(repoRoot, "dockerfiles", "sa-tools")); err == nil && st.IsDir() {
		return repoRoot
	}
	parent := filepath.Dir(repoRoot)
	if st, err := os.Stat(filepath.Join(parent, ".majordomo", "dockerfiles", "sa-tools")); err == nil && st.IsDir() {
		return parent
	}
	return repoRoot
}

func discoverDockerfiles(saDir string) ([]string, error) {
	entries, err := os.ReadDir(saDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".Dockerfile") {
			out = append(out, filepath.Join(saDir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}

func toolName(dockerfile string) string {
	base := filepath.Base(dockerfile)
	return strings.TrimSuffix(base, ".Dockerfile")
}

func imageTag(tool string) string {
	return "sa-" + tool + ":local-test"
}

func findBuildScript(repoRoot, workspace string) (string, error) {
	candidates := []string{
		filepath.Join(repoRoot, "pipelines", "scripts", "build-copilot-image.sh"),
		filepath.Join(workspace, ".majordomo", "pipelines", "scripts", "build-copilot-image.sh"),
		filepath.Join(workspace, "pipelines", "scripts", "build-copilot-image.sh"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("build-copilot-image.sh not found (searched under %s)", repoRoot)
}

func runBuild(opts Options, buildSh, dockerfile, workspace, tag, tool string) (bool, []string) {
	dockerfileArg := dockerfile
	if rel, err := filepath.Rel(workspace, dockerfile); err == nil {
		dockerfileArg = rel
	}
	env := append([]string{}, os.Environ()...)
	target := "public"
	if opts.Corp {
		target = "corp"
	}
	env = setEnv(env, "DOCKER_BUILD_TARGET", target)
	env = setEnv(env, "SKIP_PUSH", "true")
	if !opts.Corp {
		env = unsetEnv(env, "PACKAGE_REGISTRY_HOST")
	}
	args := []string{buildSh, "local", "sa-" + tool, "local-test", dockerfileArg}
	stdout, stderr, err := runCmd(opts, "bash", args, env, workspace)
	lines := strings.Split(strings.TrimRight(stdout+stderr, "\n"), "\n")
	if err == nil {
		full := "local/sa-" + tool + ":local-test"
		_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
	}
	return err == nil, lines
}

func runCmd(opts Options, name string, args, env []string, dir string) (string, string, error) {
	if opts.Runner != nil {
		return opts.Runner(name, args, env, dir)
	}
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func printResult(tool string, success bool, output []string, verbose bool) {
	marker := "✗"
	status := "FAIL"
	if success {
		marker = "✓"
		status = "PASS"
	}
	fmt.Printf("  %s sa-%s: %s\n", marker, tool, status)
	if !success || verbose {
		for _, line := range output {
			fmt.Printf("      %s\n", line)
		}
	}
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			out = append(out, prefix+value)
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, prefix+value)
	}
	return out
}

func unsetEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}
