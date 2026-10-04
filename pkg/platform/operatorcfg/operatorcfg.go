// Package operatorcfg wires majordomo's optional XDG operator file to operatorconfig.
package operatorcfg

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"gopkg.in/yaml.v3"
)

const (
	// AppName is the XDG config directory and keyring service id.
	AppName = "majordomo"

	// ConfigEnv is the env var for an explicit operator config file path.
	ConfigEnv = "MAJORDOMO_CONFIG"

	// ConfigDirEnv is the env var for the control-tower config directory.
	ConfigDirEnv = "MAJORDOMO_CONFIG_DIR"

	// DefaultTowerDir is the fallback control-tower tree when nothing else is set.
	DefaultTowerDir = "majordomo-central-config"
)

//go:embed example_config.yaml
var exampleConfig []byte

// ExampleConfig returns the embedded operator config template for init.
func ExampleConfig() []byte {
	out := make([]byte, len(exampleConfig))
	copy(out, exampleConfig)
	return out
}

// OperatorFile is the typed operator overlay (not the control-tower repo YAML).
type OperatorFile struct {
	ConfigDir string                  `yaml:"config_dir"`
	Secrets   []operatorconfig.Secret `yaml:"secrets"`
}

// Bootstrap holds process-wide operator overlay state after Apply.
type Bootstrap struct {
	Path     string
	File     OperatorFile
	Applied  bool
	ApplyErr error
}

var processBootstrap Bootstrap

// BootstrapState returns the last successful Apply result for this process.
func BootstrapState() Bootstrap {
	return processBootstrap
}

// Options configures operator file discovery and secret resolution.
type Options struct {
	ConfigFlagPath string
	Keyring        operatorconfig.Keyring
	SecretFile     operatorconfig.SecretFile
}

func (o Options) libraryOpts() operatorconfig.Options {
	return operatorconfig.Options{
		App:            AppName,
		ConfigEnv:      ConfigEnv,
		ConfigFlagPath: strings.TrimSpace(o.ConfigFlagPath),
		Keyring:        o.Keyring,
		SecretFile:     o.SecretFile,
	}
}

// ResolvePath returns the operator config file path, or empty when none exists.
func ResolvePath(opts Options) (string, error) {
	return operatorconfig.ResolveConfigPath(opts.libraryOpts())
}

// LoadFile reads and decodes the operator file at path.
func LoadFile(path string) (OperatorFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OperatorFile{}, fmt.Errorf("operatorcfg: read %s: %w", path, err)
	}
	var file OperatorFile
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return OperatorFile{}, fmt.Errorf("operatorcfg: decode %s: %w", path, err)
	}
	return file, nil
}

// Apply resolves secrets from the operator file when present. Missing file is not an error.
func Apply(opts Options) (Bootstrap, error) {
	path, err := ResolvePath(opts)
	if err != nil {
		return Bootstrap{}, fmt.Errorf("operatorcfg: resolve path: %w", err)
	}
	if path == "" {
		processBootstrap = Bootstrap{}
		return processBootstrap, nil
	}
	file, err := LoadFile(path)
	if err != nil {
		return Bootstrap{}, err
	}
	libOpts := opts.libraryOpts()
	libOpts.Secrets = file.Secrets
	if err := operatorconfig.ResolveSecrets(libOpts, libOpts.Keyring); err != nil {
		return Bootstrap{}, fmt.Errorf("operatorcfg: resolve secrets: %w", err)
	}
	processBootstrap = Bootstrap{
		Path:    path,
		File:    file,
		Applied: true,
	}
	return processBootstrap, nil
}

// InitUserConfig seeds ~/.config/majordomo/config.yaml from the embedded example.
func InitUserConfig(force bool) (written bool, path string, err error) {
	opts := operatorconfig.Options{App: AppName}
	written, err = operatorconfig.InitUserConfig(opts, ExampleConfig(), force)
	if err != nil {
		return false, "", fmt.Errorf("operatorcfg: init: %w", err)
	}
	p, err := operatorconfig.UserConfigPath(AppName, "config.yaml")
	if err != nil {
		return written, "", fmt.Errorf("operatorcfg: user path: %w", err)
	}
	return written, p, nil
}

// ResolveConfigDir picks the control-tower directory.
// Order: flag, MAJORDOMO_CONFIG_DIR, operator config_dir, DefaultTowerDir.
func ResolveConfigDir(flagDir string, operatorConfigDir string) string {
	if v := strings.TrimSpace(flagDir); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(ConfigDirEnv)); v != "" {
		return v
	}
	if v := strings.TrimSpace(operatorConfigDir); v != "" {
		return v
	}
	return DefaultTowerDir
}

// EffectiveConfigDir resolves the tower dir using the in-process bootstrap when flag is empty.
func EffectiveConfigDir(flagDir string) string {
	opDir := ""
	if processBootstrap.Applied {
		opDir = processBootstrap.File.ConfigDir
	}
	return ResolveConfigDir(flagDir, opDir)
}
