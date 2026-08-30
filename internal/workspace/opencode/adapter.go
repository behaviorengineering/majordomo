// Package opencode is the OpenCode CLI adapter for workspace.Port.
// Not wired into orchestrate yet; review Judge is in-process strop.
// When Shell is used, child env goes through aigateway.ChildEnv.
package opencode

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
	"github.com/behaviorengineering/majordomo/internal/workspace"
)

// ErrNotImplemented means this adapter method is not built yet.
var ErrNotImplemented = fmt.Errorf("workspace/opencode: not implemented")

// Adapter will shell out to the OpenCode binary for tool calls.
// Today it embeds Local for Read/Grep/Edit. Shell runs argv with Bifrost
// ChildEnv so any OpenCode-mediated tools cannot bypass the gateway.
type Adapter struct {
	Local *workspace.Local
	Bin   string // open code binary name; default "opencode" when wired
}

// New wraps a Local checkout. Bin may be empty until Shell is implemented.
func New(root, bin string) (*Adapter, error) {
	loc, err := workspace.NewLocal(root)
	if err != nil {
		return nil, err
	}
	if bin == "" {
		bin = "opencode"
	}
	return &Adapter{Local: loc, Bin: bin}, nil
}

func (a *Adapter) Read(ctx context.Context, path string) ([]byte, error) {
	return a.Local.Read(ctx, path)
}

func (a *Adapter) Grep(ctx context.Context, pattern, path string) ([]workspace.Match, error) {
	return a.Local.Grep(ctx, pattern, path)
}

func (a *Adapter) Edit(ctx context.Context, path string, content []byte) error {
	return a.Local.Edit(ctx, path, content)
}

// ChildEnv returns parent rewritten for OpenCode (gateway loopback, keys stripped).
func (a *Adapter) ChildEnv(parent []string) ([]string, error) {
	if parent == nil {
		parent = os.Environ()
	}
	return aigateway.PrepareChildEnv(parent)
}

// Shell runs argv with gateway ChildEnv. Prefer OpenCode-mediated tools later;
// today this is a gated host shell for the checkout.
func (a *Adapter) Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error) {
	if a == nil {
		return nil, nil, fmt.Errorf("workspace/opencode: nil adapter")
	}
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("workspace/opencode: empty argv")
	}
	env, err := a.ChildEnv(os.Environ())
	if err != nil {
		return nil, nil, fmt.Errorf("workspace/opencode: child env: %w", err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	if a.Local != nil {
		cmd.Dir = a.Local.Root
	}
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return out, nil, fmt.Errorf("workspace/opencode shell: %w", runErr)
	}
	return out, nil, nil
}

// Ensure Adapter implements workspace.Port.
var _ workspace.Port = (*Adapter)(nil)
