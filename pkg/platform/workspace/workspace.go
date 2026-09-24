package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Tool is one capability on Port.
type Tool int

const (
	ToolRead Tool = iota
	ToolGrep
	ToolEdit
	ToolShell
)

func (t Tool) String() string {
	switch t {
	case ToolRead:
		return "Read"
	case ToolGrep:
		return "Grep"
	case ToolEdit:
		return "Edit"
	case ToolShell:
		return "Shell"
	default:
		return fmt.Sprintf("Tool(%d)", int(t))
	}
}

// Match is one Grep hit.
type Match struct {
	Path    string
	Line    int
	Content string
}

// Port is the workspace tool surface. Paths are relative to the port root.
type Port interface {
	Read(ctx context.Context, path string) ([]byte, error)
	Grep(ctx context.Context, pattern, path string) ([]Match, error)
	Edit(ctx context.Context, path string, content []byte) error
	Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error)
}

// Allow is a per-job tool allowlist. Missing keys are denied.
type Allow map[Tool]bool

// JobAllowlists are the locked Phase 6 defaults (see PLAN).
var (
	AllowNone     = Allow{}
	AllowTechDeep = Allow{ToolRead: true, ToolGrep: true}
	AllowDigest   = Allow{ToolRead: true, ToolGrep: true, ToolEdit: true}
)

// ErrDenied is returned when a Guard blocks a tool.
var ErrDenied = fmt.Errorf("workspace: tool not allowed")

// ErrEscape is returned when a path leaves the workspace root.
var ErrEscape = fmt.Errorf("workspace: path escapes root")

// Guard wraps inner and rejects tools not in allow.
func Guard(inner Port, allow Allow) Port {
	return &guarded{inner: inner, allow: allow}
}

type guarded struct {
	inner Port
	allow Allow
}

func (g *guarded) check(t Tool) error {
	if g.allow[t] {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrDenied, t)
}

func (g *guarded) Read(ctx context.Context, path string) ([]byte, error) {
	if err := g.check(ToolRead); err != nil {
		return nil, err
	}
	return g.inner.Read(ctx, path)
}

func (g *guarded) Grep(ctx context.Context, pattern, path string) ([]Match, error) {
	if err := g.check(ToolGrep); err != nil {
		return nil, err
	}
	return g.inner.Grep(ctx, pattern, path)
}

func (g *guarded) Edit(ctx context.Context, path string, content []byte) error {
	if err := g.check(ToolEdit); err != nil {
		return err
	}
	return g.inner.Edit(ctx, path, content)
}

func (g *guarded) Shell(ctx context.Context, argv []string) ([]byte, []byte, error) {
	if err := g.check(ToolShell); err != nil {
		return nil, nil, err
	}
	return g.inner.Shell(ctx, argv)
}

// resolvePath joins root and rel and ensures the result stays under root.
func resolvePath(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("workspace: empty path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute path %q", ErrEscape, rel)
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrEscape, rel)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(absRoot, cleanRel)
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	relOut, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return "", err
	}
	if relOut == ".." || strings.HasPrefix(relOut, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrEscape, rel)
	}
	return absFull, nil
}
