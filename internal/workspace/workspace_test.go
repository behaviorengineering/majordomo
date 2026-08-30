package workspace_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/workspace"
)

func TestResolvePathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	loc, err := workspace.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = loc.Read(context.Background(), "../outside")
	if !errors.Is(err, workspace.ErrEscape) {
		t.Fatalf("want ErrEscape, got %v", err)
	}
	_, err = loc.Read(context.Background(), "/etc/passwd")
	if !errors.Is(err, workspace.ErrEscape) {
		t.Fatalf("want ErrEscape absolute, got %v", err)
	}
}

func TestGuardBlocksShell(t *testing.T) {
	inner := workspace.NewStub()
	p := workspace.Guard(inner, workspace.AllowTechDeep)
	_, _, err := p.Shell(context.Background(), []string{"true"})
	if !errors.Is(err, workspace.ErrDenied) {
		t.Fatalf("want ErrDenied, got %v", err)
	}
	if _, err := p.Read(context.Background(), "missing"); err == nil {
		t.Fatal("expected read miss error, not deny")
	}
}

func TestLocalReadEditGrep(t *testing.T) {
	root := t.TempDir()
	loc, err := workspace.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := loc.Edit(ctx, "pkg/a.go", []byte("package pkg\nfunc Hello() {}\n")); err != nil {
		t.Fatal(err)
	}
	data, err := loc.Read(ctx, "pkg/a.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package pkg\nfunc Hello() {}\n" {
		t.Fatalf("read %q", data)
	}
	ms, err := loc.Grep(ctx, `func Hello`, "pkg")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Line != 2 {
		t.Fatalf("matches %#v", ms)
	}
}

func TestLocalShell(t *testing.T) {
	root := t.TempDir()
	loc, err := workspace.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := loc.Shell(context.Background(), []string{"pwd"})
	if err != nil {
		t.Fatal(err)
	}
	got := filepath.Clean(strings.TrimSpace(string(out)))
	want := filepath.Clean(root)
	wantEval, _ := filepath.EvalSymlinks(want)
	gotEval, _ := filepath.EvalSymlinks(got)
	if gotEval != wantEval {
		t.Fatalf("pwd=%q want %q", got, want)
	}
}

func TestStubRoundTrip(t *testing.T) {
	s := workspace.NewStub()
	ctx := context.Background()
	if err := s.Edit(ctx, "a/b.txt", []byte("one\ntwo\n")); err != nil {
		t.Fatal(err)
	}
	data, err := s.Read(ctx, "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\ntwo\n" {
		t.Fatalf("%q", data)
	}
	ms, err := s.Grep(ctx, `two`, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("%#v", ms)
	}
	if _, err := s.Read(ctx, "../x"); !errors.Is(err, workspace.ErrEscape) {
		t.Fatalf("got %v", err)
	}
}

func TestAllowNoneDeniesRead(t *testing.T) {
	p := workspace.Guard(workspace.NewStub(), workspace.AllowNone)
	_, err := p.Read(context.Background(), "x")
	if !errors.Is(err, workspace.ErrDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestDigestAllowlist(t *testing.T) {
	s := workspace.NewStub()
	p := workspace.Guard(s, workspace.AllowDigest)
	ctx := context.Background()
	if err := p.Edit(ctx, "f.md", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	_, _, err := p.Shell(ctx, []string{"true"})
	if !errors.Is(err, workspace.ErrDenied) {
		t.Fatalf("digest must deny Shell, got %v", err)
	}
}
