package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllAndCredentialEnv(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n  interval: 5m\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "payments-api.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: payments-api
  cloneUrl: https://github.com/acme/payments-api.git
trigger:
  poll: true
`), 0o644)
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Repository.ID != "payments-api" {
		t.Fatalf("%#v", all)
	}
	if got := CredentialEnvName("payments-api"); got != "MAJORDOMO_CREDENTIAL__PAYMENTS_API" {
		t.Fatalf("got %s", got)
	}
}
