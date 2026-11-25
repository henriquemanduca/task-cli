package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Missing file → default vault in $HOME.
	cfg, err := Load(path)
	if err != nil || cfg.Vault != filepath.Join(home, ".task-cli-vault") {
		t.Fatalf("default: got %+v, %v", cfg, err)
	}

	cases := map[string]string{
		"vault: ~/notes/tasks\n":   filepath.Join(home, "notes", "tasks"),
		"vault: /abs/vault\n":      "/abs/vault",
		"vault: rel\n":             filepath.Join(dir, "rel"),
		"vault: $HOME/env-vault\n": filepath.Join(home, "env-vault"),
		"# only a comment\n":       filepath.Join(home, ".task-cli-vault"),
		"":                         filepath.Join(home, ".task-cli-vault"),
	}
	for content, want := range cases {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil || cfg.Vault != want {
			t.Errorf("%q: got %+v, %v; want %s", content, cfg, err, want)
		}
	}

	if err := os.WriteFile(path, []byte("valt: typo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "valt") {
		t.Errorf("expected unknown key error, got %v", err)
	}
}

func TestPath(t *testing.T) {
	t.Setenv("TASK_CLI_CONFIG", "/custom/config.yaml")
	if p, _ := Path(); p != "/custom/config.yaml" {
		t.Errorf("env override: %s", p)
	}
	t.Setenv("TASK_CLI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if p, _ := Path(); p != "/xdg/task-cli/config.yaml" {
		t.Errorf("xdg: %s", p)
	}
}
