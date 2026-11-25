package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEditor installs an $EDITOR script that runs the given shell snippet on "$1".
func fakeEditor(t *testing.T, snippet string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+snippet+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", script)
}

func TestConfigEdit(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sub", "config.yaml")
	t.Setenv("TASK_CLI_CONFIG", cfgPath)
	t.Setenv("HOME", dir)
	ctx := &Context{}

	// Missing file: created from template, editor opened, defaults reported.
	fakeEditor(t, `true`)
	out, err := captureStdout(func() error { return Root.Execute(ctx, []string{"config"}) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Config created: "+cfgPath) || !strings.Contains(out, "Vault: "+filepath.Join(dir, ".task-cli-vault")) {
		t.Errorf("unexpected output: %q", out)
	}
	data, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(data), "# task-cli configuration") {
		t.Errorf("expected template, got:\n%s", data)
	}

	// No changes.
	out, _ = captureStdout(func() error { return ConfigEdit(ctx, nil) })
	if out != "No changes made.\n" {
		t.Errorf("expected no changes, got %q", out)
	}

	// Edit keeps comments and reports the new vault.
	fakeEditor(t, `sed -i 's|^vault: .*|vault: ~/notes/tasks|' "$1"`)
	out, err = captureStdout(func() error { return ConfigEdit(ctx, nil) })
	if err != nil || !strings.Contains(out, "Config saved: ") || !strings.Contains(out, "Vault: "+filepath.Join(dir, "notes", "tasks")) {
		t.Fatalf("unexpected result: %q, %v", out, err)
	}

	// Invalid edit is reported (file kept so it can be fixed).
	fakeEditor(t, `echo 'valt: typo' >> "$1"`)
	err = ConfigEdit(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "config file is invalid") || !strings.Contains(err.Error(), "valt") {
		t.Fatalf("expected validation error, got %v", err)
	}

	if err := ConfigEdit(ctx, []string{"vault"}); err == nil {
		t.Error("expected error for extra arguments")
	}
}

func TestResolveContextConfigSkipsVault(t *testing.T) {
	// A broken config must not prevent opening it for editing.
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("vault: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_CLI_CONFIG", cfgPath)
	t.Setenv("TASK_CLI_VAULT", "")

	ctx, rem, err := ResolveContext([]string{"config"})
	if err != nil || ctx.Store != nil || rem[0] != "config" {
		t.Fatalf("expected config to skip vault resolution, got ctx=%+v rem=%v err=%v", ctx, rem, err)
	}
	if _, _, err := ResolveContext([]string{"task", "list"}); err == nil {
		t.Fatal("expected config error for vault commands")
	}
}
