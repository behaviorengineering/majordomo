package contextdigest

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"gopkg.in/yaml.v3"
)

// BootstrapSurveyInput configures the seed-time repository survey.
type BootstrapSurveyInput struct {
	AnalysisDir    string
	EvidenceDir    string
	SourceSHA      string
	RepoID         string
	TypologyBinary string
	ModuleScope    string
	GeneratedAt    time.Time
}

// BootstrapSurveyRunner writes bootstrap evidence for a repo snapshot.
type BootstrapSurveyRunner interface {
	Survey(ctx context.Context, input BootstrapSurveyInput) error
}

// LocalBootstrapSurveyRunner uses the Typology CLI when Go modules are present.
type LocalBootstrapSurveyRunner struct{}

func (LocalBootstrapSurveyRunner) Survey(ctx context.Context, input BootstrapSurveyInput) error {
	if strings.TrimSpace(input.AnalysisDir) == "" {
		return fmt.Errorf("bootstrap survey: analysis_dir is required")
	}
	if strings.TrimSpace(input.EvidenceDir) == "" {
		return fmt.Errorf("bootstrap survey: evidence_dir is required")
	}
	if strings.TrimSpace(input.SourceSHA) == "" {
		return fmt.Errorf("bootstrap survey: source_sha is required")
	}
	if strings.TrimSpace(input.RepoID) == "" {
		return fmt.Errorf("bootstrap survey: repo_id is required")
	}
	if err := os.MkdirAll(input.EvidenceDir, 0o755); err != nil {
		return err
	}

	modules, err := discoverGoModules(input.AnalysisDir)
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		return writeFallbackSurvey(input)
	}

	moduleScope := strings.TrimSpace(input.ModuleScope)
	if moduleScope == "" {
		if len(modules) != 1 {
			return fmt.Errorf("bootstrap survey: multiple go modules found and no module scope supplied")
		}
		moduleScope = modules[0]
	}
	if !moduleScopeExists(moduleScope, modules) {
		return fmt.Errorf("bootstrap survey: module scope %q is not present in discovered modules %v", moduleScope, modules)
	}

	if strings.TrimSpace(input.TypologyBinary) == "" {
		return fmt.Errorf("bootstrap survey: Typology binary is required for Go repositories")
	}

	version, err := typologyVersion(ctx, input.TypologyBinary, input.AnalysisDir)
	if err != nil {
		return err
	}

	mode := contextstore.TypologyModeReuse
	draftLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "typology.yaml")
	confirmed := filepath.Join(input.AnalysisDir, ".typology", "typology.yaml")
	if _, err := os.Stat(confirmed); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("bootstrap survey: stat typology catalog: %w", err)
		}
		mode = contextstore.TypologyModeDiscover
		if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "discover", moduleScope); err != nil {
			return err
		}
	} else {
		if err := copyFile(confirmed, draftLocal); err != nil {
			return fmt.Errorf("bootstrap survey: stage confirmed catalog as draft: %w", err)
		}
	}

	graphOut, err := runTypologyCapture(ctx, input.TypologyBinary, input.AnalysisDir,
		"show", "graph", "--module", moduleScope, "--catalog", draftLocal)
	if err != nil {
		return err
	}
	graphPath := filepath.Join(input.EvidenceDir, "graph.txt")
	if err := os.WriteFile(graphPath, []byte(graphOut), 0o644); err != nil {
		return fmt.Errorf("bootstrap survey: write graph: %w", err)
	}

	contractsLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_contracts.md")
	if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "contracts", moduleScope,
		"--out", contractsLocal); err != nil {
		return err
	}
	contractsPath := filepath.Join(input.EvidenceDir, "package_contracts.md")
	if err := copyFile(contractsLocal, contractsPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package contracts: %w", err)
	}

	// Draft architecture stays under the analysis worktree only (Typology draft moral).
	archDraft := filepath.Join(input.AnalysisDir, "tmp", "typology", "architecture_draft.md")
	if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "architecture", moduleScope,
		"--catalog", draftLocal, "--out", archDraft); err != nil {
		return err
	}
	if _, err := os.Stat(archDraft); err != nil {
		fallbackArch := filepath.Join(input.AnalysisDir, "docs", "architecture", "typology.md")
		if copyErr := copyFile(fallbackArch, archDraft); copyErr != nil {
			return fmt.Errorf("bootstrap survey: architecture draft missing: %w", err)
		}
	}
	if err := polishTypologyArchitectureBrief(archDraft); err != nil {
		return err
	}

	manifest := contextstore.TypologyManifest{
		RepoID:               input.RepoID,
		SourceSHA:            input.SourceSHA,
		GeneratedAt:          input.GeneratedAt.UTC().Format(time.RFC3339),
		TypologyVersion:      strings.TrimSpace(version),
		Mode:                 mode,
		ModuleScope:          moduleScope,
		SnapshotPath:         "snapshot.yaml",
		ArchitecturePath:     "architecture.md",
		RefineStatus:         contextstore.TypologyRefinePending,
		GraphPath:            "graph.txt",
		PackageContractsPath: "package_contracts.md",
		ClusterProposalPath:  "cluster_proposal.md",
		RefinedSnapshotPath:  "refined_snapshot.yaml",
		JourneyPath:          "journey.md",
	}
	return writeTypologyManifest(input.EvidenceDir, manifest)
}

func writeFallbackSurvey(input BootstrapSurveyInput) error {
	architecture, err := fallbackArchitecture(input.AnalysisDir)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(input.EvidenceDir, "architecture.md"), []byte(architecture), 0o644); err != nil {
		return err
	}
	manifest := contextstore.TypologyManifest{
		RepoID:           input.RepoID,
		SourceSHA:        input.SourceSHA,
		GeneratedAt:      input.GeneratedAt.UTC().Format(time.RFC3339),
		Mode:             contextstore.TypologyModeFallback,
		ArchitecturePath: "architecture.md",
		RefineStatus:     contextstore.TypologyRefineSkipped,
	}
	return writeTypologyManifest(input.EvidenceDir, manifest)
}

func writeTypologyManifest(dir string, manifest contextstore.TypologyManifest) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal typology manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "manifest.yaml"), data, 0o644)
}

func runTypology(ctx context.Context, binary, dir, command, moduleScope string, extra ...string) error {
	args := []string{command, dir}
	if strings.TrimSpace(moduleScope) != "" {
		args = append(args, "--module", moduleScope)
	}
	args = append(args, extra...)
	_, err := runTypologyCapture(ctx, binary, dir, args...)
	return err
}

func runTypologyCapture(ctx context.Context, binary, dir string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("run typology: args required")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		command := args[0]
		if producedBootstrapOutput(dir, command, args...) {
			logf("WARN", "typology %s exited non-zero after writing output: %s", command, text)
			return text, nil
		}
		return text, fmt.Errorf("run typology %s: %w\n%s", command, err, text)
	}
	return text, nil
}

func typologyVersion(ctx context.Context, binary, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, "version")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run typology version: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func discoverGoModules(dir string) ([]string, error) {
	var modules []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".cursor", ".typology", "vendor", "node_modules":
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		rel, err := filepath.Rel(dir, filepath.Dir(path))
		if err != nil {
			return err
		}
		if rel == "." {
			rel = "."
		}
		modules = append(modules, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(modules)
	return modules, nil
}

func moduleScopeExists(scope string, modules []string) bool {
	scope = filepath.ToSlash(strings.TrimSpace(scope))
	for _, mod := range modules {
		if filepath.ToSlash(mod) == scope {
			return true
		}
	}
	return false
}

func fallbackArchitecture(dir string) (string, error) {
	var b strings.Builder
	b.WriteString("# Architecture\n\n")
	b.WriteString("Present-tense snapshot seeded without Typology.\n\n")

	readme := filepath.Join(dir, "README.md")
	if data, err := os.ReadFile(readme); err == nil {
		title, intro := firstMarkdownParagraph(string(data))
		if strings.TrimSpace(title) != "" {
			fmt.Fprintf(&b, "## README snapshot\n\n%s\n\n", strings.TrimSpace(title))
			if strings.TrimSpace(intro) != "" {
				fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(intro))
			}
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".git") || strings.HasPrefix(name, ".cursor") || strings.HasPrefix(name, ".typology") || name == "evidence" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	b.WriteString("## Top-level shape\n\n")
	for _, name := range names {
		fmt.Fprintf(&b, "- `%s`\n", name)
	}
	if len(names) == 0 {
		b.WriteString("- No tracked top-level entries were discovered.\n")
	}
	return b.String(), nil
}

func firstMarkdownParagraph(src string) (string, string) {
	sc := bufio.NewScanner(strings.NewReader(src))
	var title string
	var body []string
	seenHeading := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			if seenHeading && len(body) > 0 {
				break
			}
			continue
		}
		if !seenHeading && strings.HasPrefix(line, "#") {
			title = strings.TrimSpace(strings.TrimLeft(line, "#"))
			seenHeading = true
			continue
		}
		if seenHeading {
			body = append(body, line)
		}
	}
	return title, strings.Join(body, " ")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func producedBootstrapOutput(dir, command string, args ...string) bool {
	switch command {
	case "discover":
		_, err := os.Stat(filepath.Join(dir, "tmp", "typology", "typology.yaml"))
		return err == nil
	case "contracts":
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--out" {
				if _, err := os.Stat(args[i+1]); err == nil {
					return true
				}
			}
		}
		_, err := os.Stat(filepath.Join(dir, "tmp", "typology", "package_contracts.md"))
		return err == nil
	case "architecture":
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--out" {
				if _, err := os.Stat(args[i+1]); err == nil {
					return true
				}
			}
		}
		_, err := os.Stat(filepath.Join(dir, "docs", "architecture", "typology.md"))
		return err == nil
	case "show":
		return false
	default:
		return false
	}
}

func cloneAnalysisRepo(ctx context.Context, sourceDir string) (string, error) {
	dst, err := os.MkdirTemp("", "majordomo-typology-*")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--local", "--no-hardlinks", sourceDir, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dst)
		return "", fmt.Errorf("clone analysis repo: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return dst, nil
}
