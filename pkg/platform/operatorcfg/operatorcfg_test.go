package operatorcfg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/majordomo/pkg/platform/operatorcfg"
	"github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
)

func TestResolveConfigDirPrecedence(t *testing.T) {
	t.Setenv(operatorcfg.ConfigDirEnv, "from-env")
	got := operatorcfg.ResolveConfigDir("from-flag", "from-operator")
	if got != "from-flag" {
		t.Fatalf("flag: got %q", got)
	}
	got = operatorcfg.ResolveConfigDir("", "from-operator")
	if got != "from-env" {
		t.Fatalf("env: got %q", got)
	}
	t.Setenv(operatorcfg.ConfigDirEnv, "")
	got = operatorcfg.ResolveConfigDir("", "from-operator")
	if got != "from-operator" {
		t.Fatalf("operator: got %q", got)
	}
	got = operatorcfg.ResolveConfigDir("", "")
	if got != operatorcfg.DefaultTowerDir {
		t.Fatalf("default: got %q", got)
	}
}

func TestApplyNoFileNoError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	b, err := operatorcfg.Apply(operatorcfg.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if b.Applied {
		t.Fatal("expected no apply when file missing")
	}
}

func TestApplyResolvesRequiredSecretFromKeyring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "majordomo")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "config.yaml")
	const secretName = "TEST_OPERATORCFG_TOKEN"
	if err := os.WriteFile(path, []byte("secrets:\n  - env: "+secretName+"\n    required: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(operatorcfg.ConfigEnv, path)
	kr := operatorconfig.NewMemKeyring()
	if err := kr.Set(operatorcfg.AppName, secretName, "  trimmed-value  "); err != nil {
		t.Fatal(err)
	}
	_, err := operatorcfg.Apply(operatorcfg.Options{Keyring: kr})
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(secretName); got != "trimmed-value" {
		t.Fatalf("env %q = %q", secretName, got)
	}
}

func TestInitUserConfigCreatesExample(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	written, path, err := operatorcfg.InitUserConfig(false)
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected written on first init")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	examplePath := path + ".example"
	if _, err := os.Stat(examplePath); err != nil {
		t.Fatal(err)
	}
	written2, _, err := operatorcfg.InitUserConfig(false)
	if err != nil {
		t.Fatal(err)
	}
	if written2 {
		t.Fatal("expected no overwrite without force")
	}
}
