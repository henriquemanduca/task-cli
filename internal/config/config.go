// Package config loads the user configuration file (YAML).
//
// Location: $TASK_CLI_CONFIG, or <os.UserConfigDir()>/task-cli/config.yaml
// (~/.config/task-cli/config.yaml on Linux). A missing file means defaults.
//
//	vault: ~/.task-cli-vault
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DefaultVault is used when no vault is configured.
const DefaultVault = "~/.task-cli-vault"

// Config is the parsed configuration file.
type Config struct {
	Vault string `yaml:"vault"`
}

// Path returns the configuration file location.
func Path() (string, error) {
	if p := os.Getenv("TASK_CLI_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(dir, "task-cli", "config.yaml"), nil
}

// Load reads the config file at path. A missing file yields the defaults.
// The vault path is expanded (~, $VARS) and made absolute; relative paths are
// resolved against the config file's folder.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err == nil {
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	if strings.TrimSpace(cfg.Vault) == "" {
		cfg.Vault = DefaultVault
	}
	vault, err := ExpandPath(cfg.Vault, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	cfg.Vault = vault
	return cfg, nil
}

// ExpandPath expands ~ and environment variables and makes p absolute
// (relative paths are joined to base).
func ExpandPath(p, base string) (string, error) {
	p = os.ExpandEnv(strings.TrimSpace(p))
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding %q: %w", p, err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return filepath.Clean(p), nil
}
