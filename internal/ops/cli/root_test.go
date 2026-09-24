package cli_test

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/ops/cli"
)

func TestPublicCLIOmitsDigestCacheCommands(t *testing.T) {
	root := cli.NewRoot()
	cacheCmd, _, err := root.Find([]string{"cache"})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range cacheCmd.Commands() {
		names[c.Name()] = true
	}
	for _, name := range []string{
		"digest-lookup",
		"digest-store",
		"digest-push",
		"validate-digest-branch",
	} {
		if names[name] {
			t.Fatalf("public CLI must not expose cache %s", name)
		}
	}
	for _, name := range []string{"lookup", "store", "restore", "push", "precheck", "poll-get", "poll-set"} {
		if !names[name] {
			t.Fatalf("expected review/poll cache command %s", name)
		}
	}
}

func TestPublicCLIKeepsContextValidateNotDigest(t *testing.T) {
	root := cli.NewRoot()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	if names["digest"] {
		t.Fatal("public CLI must not expose digest as a root command")
	}
	ctxCmd, _, err := root.Find([]string{"context"})
	if err != nil {
		t.Fatal(err)
	}
	ctxNames := map[string]bool{}
	for _, c := range ctxCmd.Commands() {
		ctxNames[c.Name()] = true
	}
	if !ctxNames["validate"] {
		t.Fatal("context validate required")
	}
	short := ctxCmd.Short
	if !strings.Contains(short, "majordomo-context") && !strings.Contains(short, "validate") {
		t.Fatalf("context short should point operators at validate / majordomo-context, got %q", short)
	}
}
