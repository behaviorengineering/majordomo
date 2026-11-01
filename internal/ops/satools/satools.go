// Package satools builds local SA tool Docker images for Dockerfile validation.
package satools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	processrunner "github.com/behaviorengineering/majordomo/internal/ops/runner"
)

// Options configures a local SA tool image build run.
type Options struct {
	Context context.Context
	DryRun  bool
	Verbose bool
	Corp    bool
	Out     io.Writer
	// RepoRoot is the majordomo checkout (directory containing scripts/ or go.mod).
	// Empty value discovers from cwd.
	RepoRoot string
	// Runner overrides command execution (tests).
	Runner func(name string, args []string, env []string, dir string) (stdout, stderr string, err error)
}

// Run discovers SA Dockerfiles and builds each via build-copilot-image.sh.
func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
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
	if err := writeTemplate(out, "header", `SA Tool Image Builder
Mode:      {{.Mode}}
Context:   {{.Workspace}}
Tools:     {{.Tools}}
Dry-run:   {{.DryRun}}

`, map[string]any{
		"Mode":      mode,
		"Workspace": workspace,
		"Tools":     strings.Join(tools, ", "),
		"DryRun":    opts.DryRun,
	}); err != nil {
		return fmt.Errorf("write SA tool report: %w", err)
	}

	if opts.DryRun {
		for _, df := range dockerfiles {
			if err := writeTemplate(out, "dry-run", "  [dry-run] would build sa-{{.Tool}} ({{.Mode}}) from {{.Dockerfile}}\n", map[string]string{
				"Tool":       toolName(df),
				"Mode":       mode,
				"Dockerfile": df,
			}); err != nil {
				return fmt.Errorf("write SA tool report: %w", err)
			}
		}
		return nil
	}
	if opts.Runner == nil {
		if opts.Context == nil {
			return fmt.Errorf("SA tool build requires a context")
		}
		if _, ok := opts.Context.Deadline(); !ok {
			return fmt.Errorf("SA tool build requires a context deadline")
		}
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
		if err := writeTemplate(out, "build", "Building {{.Tag}} ({{.Mode}}) ...\n", map[string]string{
			"Tag":  tag,
			"Mode": mode,
		}); err != nil {
			return fmt.Errorf("write SA tool report: %w", err)
		}
		ok, output := runBuild(opts, buildSh, df, workspace, tag, tool)
		results[tool] = ok
		if err := printResult(out, tool, ok, output, opts.Verbose); err != nil {
			return fmt.Errorf("write SA tool result: %w", err)
		}
	}

	sort.Strings(names)
	passed := 0
	for _, n := range names {
		if results[n] {
			passed++
		}
	}
	if err := writeTemplate(out, "summary", "\nResults: {{.Passed}}/{{.Total}} passed\n", map[string]int{
		"Passed": passed,
		"Total":  len(names),
	}); err != nil {
		return fmt.Errorf("write SA tool summary: %w", err)
	}
	for _, n := range names {
		status := "FAIL"
		if results[n] {
			status = "PASS"
		}
		if err := writeTemplate(out, "status", "  {{.Status}}  sa-{{.Tool}}\n", map[string]string{
			"Status": status,
			"Tool":   n,
		}); err != nil {
			return fmt.Errorf("write SA tool summary: %w", err)
		}
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
	stdout, stderr, err := runCmd(opts, opts.Context, "bash", args, env, workspace, true)
	lines := strings.Split(strings.TrimRight(stdout+stderr, "\n"), "\n")
	if err == nil {
		full := "local/sa-" + tool + ":local-test"
		tagStdout, tagStderr, tagErr := runCmd(opts, opts.Context, "docker", []string{"tag", full, tag}, os.Environ(), "", false)
		lines = append(lines, strings.Split(strings.TrimRight(tagStdout+tagStderr, "\n"), "\n")...)
		if tagErr != nil {
			err = fmt.Errorf("tag image %s: %w", tag, tagErr)
		}
	}
	return err == nil, lines
}

func runCmd(opts Options, ctx context.Context, name string, args, env []string, dir string, networked bool) (string, string, error) {
	if opts.Runner != nil {
		return opts.Runner(name, args, env, dir)
	}
	result, err := processrunner.Run(ctx, processrunner.Options{
		Name:      name,
		Args:      args,
		Env:       env,
		Dir:       dir,
		Networked: networked,
	})
	return result.Stdout, result.Stderr, err
}

func printResult(out io.Writer, tool string, success bool, output []string, verbose bool) error {
	marker := "✗"
	status := "FAIL"
	if success {
		marker = "✓"
		status = "PASS"
	}
	if err := writeTemplate(out, "result", "  {{.Marker}} sa-{{.Tool}}: {{.Status}}\n", map[string]string{
		"Marker": marker,
		"Tool":   tool,
		"Status": status,
	}); err != nil {
		return err
	}
	if !success || verbose {
		for _, line := range output {
			if err := writeTemplate(out, "detail", "      {{.Line}}\n", map[string]string{"Line": line}); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeTemplate(out io.Writer, name, source string, data any) error {
	tmpl, err := template.New(name).Parse(source)
	if err != nil {
		return fmt.Errorf("parse %s report: %w", name, err)
	}
	return tmpl.Execute(out, data)
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
