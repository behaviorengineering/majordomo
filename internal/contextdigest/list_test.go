package contextdigest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListTargetsSkipsGeneric(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte("scm: github\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generic := `scm: generic
repository:
  id: offline
  cloneUrl: https://example.com/offline.git
`
	github := `scm: github
repository:
  id: demo
  owner: acme
  name: demo
  cloneUrl: https://github.com/acme/demo.git
`
	if err := os.WriteFile(filepath.Join(dir, "offline.yaml"), []byte(generic), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(github), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListTargets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 || got.Repos[0].RepoID != "demo" {
		t.Fatalf("repos=%+v", got.Repos)
	}
}
