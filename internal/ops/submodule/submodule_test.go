package submodule

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindParentRepoRootViaGitlink(t *testing.T) {
	calls := 0
	m := &manager{
		opts: Options{
			GitRunner: func(args []string, cwd string, check bool) (string, error) {
				calls++
				joined := strings.Join(args, " ")
				switch {
				case joined == "rev-parse --show-toplevel" && strings.HasSuffix(cwd, "parent"):
					return "/tmp/parent", nil
				case strings.HasPrefix(joined, "ls-files --stage"):
					return "160000 abcdef .majordomo", nil
				default:
					return "", nil
				}
			},
		},
		submoduleRoot: "/tmp/parent/.majordomo",
	}
	got := m.findParentRepoRoot("/tmp/parent/.majordomo")
	if got != "/tmp/parent" {
		t.Fatalf("got %q calls=%d", got, calls)
	}
	m.parentRoot = got
	m.submoduleName = m.getSubmoduleName()
	if m.submoduleName != ".majordomo" {
		t.Fatalf("name %q", m.submoduleName)
	}
}

func TestFindParentRepoRootRejectsNonSubmodule(t *testing.T) {
	m := &manager{
		opts: Options{
			GitRunner: func(args []string, cwd string, check bool) (string, error) {
				joined := strings.Join(args, " ")
				if joined == "rev-parse --show-toplevel" {
					return "/tmp/same", nil
				}
				return "", nil
			},
		},
	}
	if got := m.findParentRepoRoot("/tmp/same"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildOpsMenu(t *testing.T) {
	m := &manager{submoduleName: ".majordomo"}
	menu := m.buildOpsMenu("main")
	if !strings.Contains(menu, "Submodule : .majordomo") || !strings.Contains(menu, "1. Update") {
		t.Fatalf("menu:\n%s", menu)
	}
	_ = filepath.Separator
}

func TestRemoteGitRequiresDeadline(t *testing.T) {
	m := &manager{opts: Options{Context: context.Background()}}
	_, err := m.git([]string{"fetch", "origin"}, "/tmp", true)
	if err == nil || !strings.Contains(err.Error(), "deadline is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRetryableGitErrorClassification(t *testing.T) {
	if !retryableGitError(fmt.Errorf("fatal: unable to access: connection timed out")) {
		t.Fatal("expected timeout to be retryable")
	}
	if retryableGitError(fmt.Errorf("fatal: Authentication failed")) {
		t.Fatal("expected authentication failure not to be retryable")
	}
}
