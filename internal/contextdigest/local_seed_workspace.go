package contextdigest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	localSeedSchemaVersion = 1

	LocalStageSurvey       = "survey"
	LocalStageCatalog      = "catalog"
	LocalStageRefine       = "refine" // legacy alias for catalog (one release)
	LocalStageIntervention = "intervention"
	LocalStageStory        = "story"

	localWorkspaceManifest = "workspace.yaml"
	localWorkspaceLock     = "workspace.lock"
	localContextRel        = "context"
	localAnalysisRel       = "analysis"
	localCacheRel          = "inference-cache"
	localWorkStoryRel      = "work-story"
	localDiffRel           = "local.diff"
)

// LocalSeedManifest is the versioned workspace.yaml for filesystem-only seeds.
type LocalSeedManifest struct {
	SchemaVersion   int    `yaml:"schema_version"`
	RepoID          string `yaml:"repo_id"`
	SourceSHA       string `yaml:"source_sha"`
	ModuleScope     string `yaml:"module_scope,omitempty"`
	TypologyVersion string `yaml:"typology_version,omitempty"`
	CompletedStage  string `yaml:"completed_stage,omitempty"`
	CurrentStage    string `yaml:"current_stage,omitempty"`
	CreatedAt       string `yaml:"created_at"`
	UpdatedAt       string `yaml:"updated_at"`
	LastError       string `yaml:"last_error,omitempty"`
}

// LocalSeedWorkspace is a durable filesystem workspace for local digest seeding.
type LocalSeedWorkspace struct {
	Root     string
	Manifest LocalSeedManifest
	lockPath string
	locked   bool
}

// OpenLocalSeedWorkspace creates or reopens a local seed workspace under root.
func OpenLocalSeedWorkspace(root, repoID, sourceSHA, moduleScope string, allowSourceMove bool, now time.Time) (*LocalSeedWorkspace, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("local seed dir is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("local seed mkdir: %w", err)
	}
	ws := &LocalSeedWorkspace{Root: root, lockPath: filepath.Join(root, localWorkspaceLock)}
	if err := ws.acquireLock(now); err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(root, localWorkspaceManifest)
	_, err := os.Stat(manifestPath)
	switch {
	case os.IsNotExist(err):
		ws.Manifest = LocalSeedManifest{
			SchemaVersion: localSeedSchemaVersion,
			RepoID:        repoID,
			SourceSHA:     sourceSHA,
			ModuleScope:   moduleScope,
			CreatedAt:     now.UTC().Format(time.RFC3339),
			UpdatedAt:     now.UTC().Format(time.RFC3339),
		}
		if err := ws.ensureLayout(); err != nil {
			_ = ws.Release()
			return nil, err
		}
		if err := ws.writeManifest(); err != nil {
			_ = ws.Release()
			return nil, err
		}
		return ws, nil
	case err != nil:
		_ = ws.Release()
		return nil, fmt.Errorf("stat workspace.yaml: %w", err)
	}
	if err := ws.loadManifest(); err != nil {
		_ = ws.Release()
		return nil, err
	}
	if err := ws.validateIdentity(repoID, sourceSHA, moduleScope, allowSourceMove); err != nil {
		_ = ws.Release()
		return nil, err
	}
	if err := ws.ensureLayout(); err != nil {
		_ = ws.Release()
		return nil, err
	}
	return ws, nil
}

func (ws *LocalSeedWorkspace) ContextDir() string {
	return filepath.Join(ws.Root, localContextRel)
}

func (ws *LocalSeedWorkspace) AnalysisDir() string {
	return filepath.Join(ws.Root, localAnalysisRel)
}

func (ws *LocalSeedWorkspace) CacheDir() string {
	return filepath.Join(ws.Root, localCacheRel)
}

func (ws *LocalSeedWorkspace) WorkStoryDir() string {
	return filepath.Join(ws.Root, localWorkStoryRel)
}

func (ws *LocalSeedWorkspace) DiffPath() string {
	return filepath.Join(ws.Root, localDiffRel)
}

func (ws *LocalSeedWorkspace) IsNew() bool {
	return strings.TrimSpace(ws.Manifest.CompletedStage) == ""
}

func (ws *LocalSeedWorkspace) ensureLayout() error {
	for _, rel := range []string{localContextRel, localAnalysisRel, localCacheRel, localWorkStoryRel} {
		if err := os.MkdirAll(filepath.Join(ws.Root, rel), 0o755); err != nil {
			return fmt.Errorf("local seed layout %s: %w", rel, err)
		}
	}
	return nil
}

func (ws *LocalSeedWorkspace) loadManifest() error {
	raw, err := os.ReadFile(filepath.Join(ws.Root, localWorkspaceManifest))
	if err != nil {
		return fmt.Errorf("read workspace.yaml: %w (repair or delete the workspace and start fresh)", err)
	}
	var m LocalSeedManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse workspace.yaml: %w (corrupt manifest; repair or delete the workspace)", err)
	}
	if m.SchemaVersion != localSeedSchemaVersion {
		return fmt.Errorf("workspace.yaml schema_version %d unsupported (want %d)", m.SchemaVersion, localSeedSchemaVersion)
	}
	if strings.TrimSpace(m.RepoID) == "" || strings.TrimSpace(m.SourceSHA) == "" {
		return fmt.Errorf("workspace.yaml missing repo_id or source_sha (corrupt; repair or delete)")
	}
	ws.Manifest = m
	return nil
}

func (ws *LocalSeedWorkspace) writeManifest() error {
	ws.Manifest.SchemaVersion = localSeedSchemaVersion
	raw, err := yaml.Marshal(&ws.Manifest)
	if err != nil {
		return fmt.Errorf("encode workspace.yaml: %w", err)
	}
	return writeFileAtomic(filepath.Join(ws.Root, localWorkspaceManifest), raw)
}

func (ws *LocalSeedWorkspace) validateIdentity(repoID, sourceSHA, moduleScope string, allowSourceMove bool) error {
	if ws.Manifest.RepoID != repoID {
		return fmt.Errorf("local seed workspace repo_id=%q does not match --repo-id %q", ws.Manifest.RepoID, repoID)
	}
	wantScope := strings.TrimSpace(moduleScope)
	haveScope := strings.TrimSpace(ws.Manifest.ModuleScope)
	if haveScope != "" && wantScope != "" && haveScope != wantScope {
		return fmt.Errorf("local seed workspace module_scope=%q does not match --module-scope %q", haveScope, wantScope)
	}
	if ws.Manifest.SourceSHA != sourceSHA {
		if !allowSourceMove {
			return fmt.Errorf("local seed workspace source_sha=%s does not match workdir HEAD %s (pass --allow-source-move to retarget, or use a new --local-seed-dir)",
				shortSHA(ws.Manifest.SourceSHA), shortSHA(sourceSHA))
		}
		ws.Manifest.SourceSHA = sourceSHA
		ws.Manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := ws.writeManifest(); err != nil {
			return err
		}
		logf("WARN", "local seed source_sha retargeted to %s (--allow-source-move)", shortSHA(sourceSHA))
	}
	if haveScope == "" && wantScope != "" {
		ws.Manifest.ModuleScope = wantScope
	}
	return nil
}

// BeginStage records current_stage without advancing completed_stage.
func (ws *LocalSeedWorkspace) BeginStage(stage string, now time.Time) error {
	ws.Manifest.CurrentStage = stage
	ws.Manifest.LastError = ""
	ws.Manifest.UpdatedAt = now.UTC().Format(time.RFC3339)
	return ws.writeManifest()
}

// SaveCheckpoint advances completed_stage after a successful stage.
func (ws *LocalSeedWorkspace) SaveCheckpoint(stage string, now time.Time) error {
	ws.Manifest.CompletedStage = stage
	ws.Manifest.CurrentStage = ""
	ws.Manifest.LastError = ""
	ws.Manifest.UpdatedAt = now.UTC().Format(time.RFC3339)
	return ws.writeManifest()
}

// RecordFailure stores last_error without advancing completed_stage.
func (ws *LocalSeedWorkspace) RecordFailure(stage string, stageErr error, now time.Time) error {
	ws.Manifest.CurrentStage = stage
	ws.Manifest.LastError = stageErr.Error()
	ws.Manifest.UpdatedAt = now.UTC().Format(time.RFC3339)
	return ws.writeManifest()
}

// PersistAnalysisDrafts copies refine inputs from analysisDir into the workspace analysis/ tree.
func (ws *LocalSeedWorkspace) PersistAnalysisDrafts(analysisDir string) error {
	dstRoot := ws.AnalysisDir()
	if err := os.MkdirAll(filepath.Join(dstRoot, "tmp", "typology"), 0o755); err != nil {
		return err
	}
	for _, rel := range []string{analysisDraftCatalogRel, analysisDraftArchRel} {
		src := filepath.Join(analysisDir, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(dstRoot, rel)); err != nil {
			return fmt.Errorf("persist analysis %s: %w", rel, err)
		}
	}
	readme := filepath.Join(analysisDir, "README.md")
	if _, err := os.Stat(readme); err == nil {
		if err := copyFile(readme, filepath.Join(dstRoot, "README.md")); err != nil {
			return fmt.Errorf("persist analysis README: %w", err)
		}
	}
	return nil
}

// RestoreAnalysisDrafts copies persisted drafts into a fresh analysis clone.
func (ws *LocalSeedWorkspace) RestoreAnalysisDrafts(analysisDir string) error {
	srcRoot := ws.AnalysisDir()
	for _, rel := range []string{analysisDraftCatalogRel, analysisDraftArchRel, "README.md"} {
		src := filepath.Join(srcRoot, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(analysisDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("restore analysis %s: %w", rel, err)
		}
	}
	return nil
}

// WriteLocalDiff writes a simple tree listing / content marker for operator inspection.
func (ws *LocalSeedWorkspace) WriteLocalDiff(content string) error {
	return writeFileAtomic(ws.DiffPath(), []byte(content))
}

// Release unlocks the workspace.
func (ws *LocalSeedWorkspace) Release() error {
	if ws == nil || !ws.locked {
		return nil
	}
	ws.locked = false
	return os.Remove(ws.lockPath)
}

type localSeedLock struct {
	PID int    `json:"pid"`
	At  string `json:"at"`
}

func (ws *LocalSeedWorkspace) acquireLock(now time.Time) error {
	payload, err := json.Marshal(localSeedLock{PID: os.Getpid(), At: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(ws.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		_, werr := f.Write(payload)
		_ = f.Close()
		if werr != nil {
			_ = os.Remove(ws.lockPath)
			return werr
		}
		ws.locked = true
		return nil
	}
	if !os.IsExist(err) {
		return fmt.Errorf("local seed lock: %w", err)
	}
	raw, rerr := os.ReadFile(ws.lockPath)
	if rerr != nil {
		return fmt.Errorf("local seed lock busy and unreadable: %w", rerr)
	}
	var existing localSeedLock
	_ = json.Unmarshal(raw, &existing)
	if existing.PID > 0 && processAlive(existing.PID) {
		return fmt.Errorf("local seed workspace locked by pid %d (started %s)", existing.PID, existing.At)
	}
	// Stale lock: remove and retry once.
	_ = os.Remove(ws.lockPath)
	f, err = os.OpenFile(ws.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("local seed lock retry: %w", err)
	}
	_, werr := f.Write(payload)
	_ = f.Close()
	if werr != nil {
		_ = os.Remove(ws.lockPath)
		return werr
	}
	ws.locked = true
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := p.Signal(syscall.Signal(0)); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
		return true
	}
	return false
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func stageOrder(stage string) int {
	switch normalizeResumeStage(stage) {
	case LocalStageSurvey:
		return 1
	case LocalStageCatalog:
		return 2
	case LocalStageIntervention:
		return 3
	case LocalStageStory:
		return 4
	default:
		return 0
	}
}

func localStageReady(completed, want string) error {
	want = normalizeResumeStage(want)
	completed = normalizeResumeStage(completed)
	switch want {
	case "", LocalStageSurvey:
		return nil
	case LocalStageCatalog:
		if stageOrder(completed) < stageOrder(LocalStageSurvey) {
			return fmt.Errorf("local seed resume from catalog requires completed survey (have %q)", completed)
		}
		return nil
	case LocalStageIntervention:
		if stageOrder(completed) < stageOrder(LocalStageCatalog) {
			return fmt.Errorf("local seed resume from intervention requires completed catalog (have %q)", completed)
		}
		return nil
	case LocalStageStory:
		if stageOrder(completed) < stageOrder(LocalStageCatalog) {
			return fmt.Errorf("local seed resume from story requires completed catalog (have %q)", completed)
		}
		return nil
	default:
		return fmt.Errorf("unsupported local from-stage %q", want)
	}
}
