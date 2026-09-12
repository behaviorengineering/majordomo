package contextdigest

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
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

// LocalBootstrapSurveyRunner uses the Typology CLI when Go or Python roots are present.
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

	roots, err := discoverSurveyRoots(input.AnalysisDir)
	if err != nil {
		return err
	}
	if !roots.HasGo && !roots.HasPython {
		return writeFallbackSurvey(input)
	}
	if strings.TrimSpace(input.TypologyBinary) == "" {
		return fmt.Errorf("bootstrap survey: Typology binary is required for Go or Python repositories")
	}
	if !roots.HasGo {
		return surveyWithTypologyPythonOnly(ctx, input)
	}

	modules := roots.GoModules
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

	rolesLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", packageRolesRel)
	if _, err := os.Stat(rolesLocal); err != nil {
		// contracts writes roles beside the default path under tmp/typology.
		rolesLocal = filepath.Join(input.AnalysisDir, "tmp", "typology", packageRolesRel)
		if _, err := os.Stat(rolesLocal); err != nil {
			return fmt.Errorf("bootstrap survey: package roles missing after contracts: %w", err)
		}
	}
	rolesPath := filepath.Join(input.EvidenceDir, packageRolesRel)
	if err := copyFile(rolesLocal, rolesPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package roles: %w", err)
	}

	rlmLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_rlm_context.md")
	rlmPath := filepath.Join(input.EvidenceDir, "package_rlm_context.md")
	if _, err := os.Stat(rlmLocal); err == nil {
		if err := copyFile(rlmLocal, rlmPath); err != nil {
			return fmt.Errorf("bootstrap survey: copy package RLM context: %w", err)
		}
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
	if err := prependObservedRolesBrief(archDraft, rolesPath); err != nil {
		return err
	}

	manifest := contextstore.TypologyManifest{
		RepoID:                input.RepoID,
		SourceSHA:             input.SourceSHA,
		GeneratedAt:           input.GeneratedAt.UTC().Format(time.RFC3339),
		TypologyVersion:       strings.TrimSpace(version),
		Mode:                  mode,
		ModuleScope:           moduleScope,
		SnapshotPath:          "snapshot.yaml",
		ArchitecturePath:      contextstore.TypologyArchitectureBriefPath,
		RefineStatus:          contextstore.TypologyRefinePending,
		GraphPath:             "graph.txt",
		PackageContractsPath:  "package_contracts.md",
		PackageRolesPath:      packageRolesRel,
		PackageRLMContextPath: "package_rlm_context.md",
		ClusterProposalPath:   "cluster_proposal.md",
		RefinedSnapshotPath:   refinedSnapshotRel,
		JourneyPath:           "journey.md",
	}
	return writeTypologyManifest(input.EvidenceDir, manifest)
}

func surveyWithTypologyPythonOnly(ctx context.Context, input BootstrapSurveyInput) error {
	version, err := typologyVersion(ctx, input.TypologyBinary, input.AnalysisDir)
	if err != nil {
		return err
	}

	contractsLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_contracts.md")
	if err := os.MkdirAll(filepath.Dir(contractsLocal), 0o755); err != nil {
		return err
	}
	// Empty module scope: typology Harvest detects Python roots without go.mod.
	if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "contracts", "",
		"--out", contractsLocal); err != nil {
		return err
	}

	contractsPath := filepath.Join(input.EvidenceDir, "package_contracts.md")
	if err := copyFile(contractsLocal, contractsPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package contracts: %w", err)
	}
	rolesLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", packageRolesRel)
	if _, err := os.Stat(rolesLocal); err != nil {
		return fmt.Errorf("bootstrap survey: package roles missing after contracts: %w", err)
	}
	rolesPath := filepath.Join(input.EvidenceDir, packageRolesRel)
	if err := copyFile(rolesLocal, rolesPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package roles: %w", err)
	}
	if err := ensureRolesNonEmpty(rolesPath); err != nil {
		return fmt.Errorf("bootstrap survey: python harvest: %w", err)
	}
	rlmLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_rlm_context.md")
	rlmPath := filepath.Join(input.EvidenceDir, "package_rlm_context.md")
	if _, err := os.Stat(rlmLocal); err == nil {
		if err := copyFile(rlmLocal, rlmPath); err != nil {
			return fmt.Errorf("bootstrap survey: copy package RLM context: %w", err)
		}
	}

	archDraft := filepath.Join(input.AnalysisDir, "tmp", "typology", "architecture_draft.md")
	if err := os.MkdirAll(filepath.Dir(archDraft), 0o755); err != nil {
		return err
	}
	brief := "# Architecture\n\nPresent-tense snapshot seeded from Typology Python package harvest.\n\n"
	if err := os.WriteFile(archDraft, []byte(brief), 0o644); err != nil {
		return err
	}
	if err := polishTypologyArchitectureBrief(archDraft); err != nil {
		return err
	}
	if err := prependObservedRolesBrief(archDraft, rolesPath); err != nil {
		return err
	}
	if err := copyFile(archDraft, filepath.Join(input.EvidenceDir, contextstore.TypologyArchitectureBriefPath)); err != nil {
		return fmt.Errorf("bootstrap survey: copy architecture brief: %w", err)
	}

	manifest := contextstore.TypologyManifest{
		RepoID:                input.RepoID,
		SourceSHA:             input.SourceSHA,
		GeneratedAt:           input.GeneratedAt.UTC().Format(time.RFC3339),
		TypologyVersion:       strings.TrimSpace(version),
		Mode:                  contextstore.TypologyModeDiscover,
		SnapshotPath:          "snapshot.yaml",
		ArchitecturePath:      contextstore.TypologyArchitectureBriefPath,
		RefineStatus:          contextstore.TypologyRefinePending,
		PackageContractsPath:  "package_contracts.md",
		PackageRolesPath:      packageRolesRel,
		PackageRLMContextPath: "package_rlm_context.md",
		ClusterProposalPath:   "cluster_proposal.md",
		RefinedSnapshotPath:   refinedSnapshotRel,
		JourneyPath:           "journey.md",
	}
	return writeTypologyManifest(input.EvidenceDir, manifest)
}

func ensureRolesNonEmpty(rolesPath string) error {
	doc, err := loadPackageRoles(rolesPath)
	if err != nil {
		return err
	}
	if len(doc.Packages) == 0 {
		return fmt.Errorf("%s has no packages", packageRolesRel)
	}
	return nil
}

// surveyRoots are supported language project roots under the analysis tree.
type surveyRoots struct {
	GoModules []string
	HasGo     bool
	HasPython bool
}

func discoverSurveyRoots(dir string) (surveyRoots, error) {
	modules, err := discoverGoModules(dir)
	if err != nil {
		return surveyRoots{}, err
	}
	hasPy, err := discoverPythonRoot(dir)
	if err != nil {
		return surveyRoots{}, err
	}
	return surveyRoots{
		GoModules: modules,
		HasGo:     len(modules) > 0,
		HasPython: hasPy,
	}, nil
}

func discoverPythonRoot(dir string) (bool, error) {
	found := false
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".cursor", ".typology", "vendor", "node_modules", ".venv", "venv", "__pycache__":
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		switch d.Name() {
		case "pyproject.toml", "setup.cfg", "setup.py":
			found = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

func writeFallbackSurvey(input BootstrapSurveyInput) error {
	architecture, err := fallbackArchitecture(input.AnalysisDir)
	if err != nil {
		return err
	}
	architecture = ensureTypologyArchitectureBanner(architecture)
	if err := os.WriteFile(filepath.Join(input.EvidenceDir, contextstore.TypologyArchitectureBriefPath), []byte(architecture), 0o644); err != nil {
		return err
	}
	manifest := contextstore.TypologyManifest{
		RepoID:           input.RepoID,
		SourceSHA:        input.SourceSHA,
		GeneratedAt:      input.GeneratedAt.UTC().Format(time.RFC3339),
		Mode:             contextstore.TypologyModeFallback,
		ArchitecturePath: contextstore.TypologyArchitectureBriefPath,
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
	title, intro := "", ""
	readme := filepath.Join(dir, "README.md")
	if data, err := os.ReadFile(readme); err == nil {
		title, intro = firstMarkdownParagraph(string(data))
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
	var b bytes.Buffer
	if err := fallbackArchitectureTemplate.Execute(&b, struct {
		Title string
		Intro string
		Names []string
	}{Title: strings.TrimSpace(title), Intro: strings.TrimSpace(intro), Names: names}); err != nil {
		return "", fmt.Errorf("render fallback architecture: %w", err)
	}
	return b.String(), nil
}

var fallbackArchitectureTemplate = template.Must(template.New("fallbackArchitecture").Parse(`# Architecture

Present-tense snapshot seeded without Typology.

{{if .Title}}## README snapshot

{{.Title}}

{{if .Intro}}{{.Intro}}

{{end}}{{end}}## Top-level shape

{{if .Names}}{{range .Names}}- ` + "`{{.}}`" + `
{{end}}{{else}}- No tracked top-level entries were discovered.
{{end}}`))

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
		if cleanupErr := os.RemoveAll(dst); cleanupErr != nil {
			return "", fmt.Errorf("clone analysis repo: %w; cleanup: %v\n%s", err, cleanupErr, strings.TrimSpace(string(out)))
		}
		return "", fmt.Errorf("clone analysis repo: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return dst, nil
}
