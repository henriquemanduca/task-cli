// Package config loads workspace-level defaults from a .taskcli file.
//
// The file uses a .env-style format: one KEY=VALUE per line, blank lines and
// lines starting with # are ignored, and an optional "export " prefix is
// accepted. The current supported key is PROJECT_CODE, which sets the
// default project code for task creation and listing.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the workspace configuration file name.
const FileName = ".taskcli"

// KeyProject is the configuration key that sets the default project code.
const KeyProject = "PROJECT_CODE"

// Config holds workspace-level defaults loaded from .taskcli.
type Config struct {
	// Project is the default project code used by task add and task list.
	Project string
}

// LoadFromDir finds the nearest .taskcli from startDir upward, stopping at the
// user's home directory (inclusive). It returns an empty Config when no file is
// found. The second return value is the path of the loaded file, if any.
func LoadFromDir(startDir string) (*Config, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	return loadFromDir(startDir, homeDir)
}

func loadFromDir(startDir, stopDir string) (*Config, string, error) {
	dir := normalizeDir(startDir)
	stop := normalizeDir(stopDir)

	for {
		path := filepath.Join(dir, FileName)
		info, err := os.Stat(path)
		switch {
		case err == nil && !info.IsDir():
			cfg, err := LoadFile(path)
			if err != nil {
				return nil, "", err
			}
			return cfg, path, nil
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return nil, "", fmt.Errorf("checking %s: %w", path, err)
		}

		if stop != "" && dir == stop {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return &Config{}, "", nil
}

// LoadFile reads and parses a .taskcli file.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	cfg, err := parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func parse(r io.Reader) (*Config, error) {
	cfg := &Config{}
	scanner := bufio.NewScanner(r)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", lineNo)
		}

		key = strings.TrimSpace(key)
		value := parseValue(rawValue)

		switch key {
		case KeyProject:
			cfg.Project = value
		default:
			return nil, fmt.Errorf("line %d: unknown key %q", lineNo, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return cfg, nil
}

func parseValue(raw string) string {
	value := strings.TrimSpace(raw)
	if unquoted, ok := unquote(value); ok {
		return unquoted
	}

	// Support inline comments only when preceded by whitespace, so values
	// containing # (e.g. URLs) are preserved.
	for _, marker := range []string{" #", "\t#"} {
		if i := strings.Index(value, marker); i >= 0 {
			value = value[:i]
		}
	}
	value = strings.TrimSpace(value)
	if unquoted, ok := unquote(value); ok {
		return unquoted
	}
	return value
}

func unquote(value string) (string, bool) {
	if len(value) < 2 {
		return "", false
	}
	first, last := value[0], value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1], true
	}
	return "", false
}

func normalizeDir(dir string) string {
	if dir == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
}
