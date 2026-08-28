// Package opencode is the OpenCode CLI adapter for workspace.Port.
// Not wired into orchestrate yet; review still uses agent-dispatch.sh.
package opencode

import (
	"context"
	"fmt"

	"github.com/behaviorengineering/majordomo/internal/workspace"
)

// ErrNotImplemented means this adapter method is not built yet.
var ErrNotImplemented = fmt.Errorf("workspace/opencode: not implemented")

// Adapter will shell out to the OpenCode binary for tool calls.
// Today it embeds Local for Read/Grep/Edit and leaves Shell unimplemented
// until Judge wiring needs OpenCode-mediated tools.
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

// Shell is reserved for OpenCode-mediated command execution.
func (a *Adapter) Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error) {
	return nil, nil, fmt.Errorf("%w: Shell via %s", ErrNotImplemented, a.Bin)
}

// Ensure Adapter implements workspace.Port.
var _ workspace.Port = (*Adapter)(nil)
