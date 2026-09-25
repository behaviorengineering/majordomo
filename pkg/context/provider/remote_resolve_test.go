package contextprovider

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type fakeGitRunner struct {
	mu       sync.Mutex
	calls    []string
	lsRemote string
	headSHA  string
	isRepo   bool
}

func (f *fakeGitRunner) record(args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(args, " "))
}

func (f *fakeGitRunner) Run(_ context.Context, dir, _, _ string, args ...string) (string, error) {
	f.record(append([]string{"run", dir}, args...)...)
	if len(args) >= 1 && args[0] == "clone" {
		return "", nil
	}
	if len(args) >= 1 && args[0] == "fetch" {
		return "", nil
	}
	if len(args) >= 2 && args[0] == "checkout" {
		f.headSHA = args[len(args)-1]
		return "", nil
	}
	return "", nil
}

func (f *fakeGitRunner) RunTrim(_ context.Context, dir, _, _ string, args ...string) (string, error) {
	f.record(append([]string{"trim", dir}, args...)...)
	if len(args) >= 2 && args[0] == "ls-remote" {
		if f.lsRemote == "" {
			return "", errors.New("no branch")
		}
		return f.lsRemote, nil
	}
	if len(args) >= 1 && args[0] == "rev-parse" {
		return f.headSHA, nil
	}
	return "", nil
}

func (f *fakeGitRunner) AllowFail(context.Context, string, string, string, ...string) (string, int) {
	return "", 1
}

func (f *fakeGitRunner) IsRepo(string) bool {
	return f.isRepo
}

func TestResolveRemoteUsesRequestContextSHA(t *testing.T) {
	fake := &fakeGitRunner{isRepo: true, headSHA: "aaa1111"}
	cfg := GitRemoteConfig{
		WorkDir:  "/work",
		CloneURL: "https://example.com/r.git",
		RepoID:   "demo",
		Git:      fake,
	}
	dir, sha, err := ResolveRemoteContextDir(context.Background(), cfg, Request{
		RepoID:     "demo",
		ContextSHA: "bbb2222",
	})
	if err != nil || dir == "" || sha != "bbb2222" {
		t.Fatalf("ResolveRemoteContextDir = %q %q err=%v", dir, sha, err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, c := range fake.calls {
		if strings.Contains(c, "ls-remote") {
			t.Fatalf("unexpected ls-remote when ContextSHA set: %q", c)
		}
	}
	if fake.headSHA != "bbb2222" {
		t.Fatalf("head after pin = %q", fake.headSHA)
	}
}

func TestResolveRemotePinsStaleCache(t *testing.T) {
	fake := &fakeGitRunner{
		isRepo:   true,
		lsRemote: "ccc3333 refs/heads/majordomo-context/demo",
		headSHA:  "old9999",
	}
	cfg := GitRemoteConfig{
		WorkDir:  "/work",
		CloneURL: "https://example.com/r.git",
		RepoID:   "demo",
		Git:      fake,
	}
	dir, sha, err := ResolveRemoteContextDir(context.Background(), cfg, Request{RepoID: "demo"})
	if err != nil || sha != "ccc3333" {
		t.Fatalf("dir=%q sha=%q err=%v", dir, sha, err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var sawFetch, sawCheckout bool
	for _, c := range fake.calls {
		if strings.Contains(c, "fetch") && strings.Contains(c, "ccc3333") {
			sawFetch = true
		}
		if strings.Contains(c, "checkout") && strings.Contains(c, "ccc3333") {
			sawCheckout = true
		}
	}
	if !sawFetch || !sawCheckout {
		t.Fatalf("calls=%v", fake.calls)
	}
}

func TestResolveRemoteMissingBranch(t *testing.T) {
	fake := &fakeGitRunner{lsRemote: ""}
	cfg := GitRemoteConfig{
		WorkDir:  "/work",
		CloneURL: "https://example.com/r.git",
		RepoID:   "demo",
		Git:      fake,
	}
	_, _, err := ResolveRemoteContextDir(context.Background(), cfg, Request{RepoID: "demo"})
	if !errors.Is(err, ErrNoContext) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRemoteBranchHead(t *testing.T) {
	if got := parseRemoteBranchHead("deadbeef refs/heads/majordomo-context/x"); got != "deadbeef" {
		t.Fatalf("got %q", got)
	}
}
