package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"task-cli/internal/config"
)

func TestTaskDefaultsFromWorkspaceConfig(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()
	ctx.Config = &config.Config{Project: "PROJ"}

	if err := ProjectAdd(ctx, []string{"PROJ", "Configured Project"}); err != nil {
		t.Fatalf("failed to create configured project: %v", err)
	}
	if err := ProjectAdd(ctx, []string{"OTHER", "Other Project"}); err != nil {
		t.Fatalf("failed to create other project: %v", err)
	}

	// task add uses the configured project when -p is omitted.
	if err := TaskAdd(ctx, []string{"Configured task"}); err != nil {
		t.Fatalf("unexpected error adding task with configured project: %v", err)
	}
	task, err := ctx.Store.GetTaskByID(1)
	if err != nil {
		t.Fatalf("failed to retrieve task 1: %v", err)
	}
	if task.ProjectCode != "PROJ" {
		t.Errorf("expected configured project PROJ, got %q", task.ProjectCode)
	}

	// An explicit -p overrides the configured project.
	if err := TaskAdd(ctx, []string{"-p", "2", "Explicit task"}); err != nil {
		t.Fatalf("unexpected error adding task with explicit project: %v", err)
	}
	task, err = ctx.Store.GetTaskByID(2)
	if err != nil {
		t.Fatalf("failed to retrieve task 2: %v", err)
	}
	if task.ProjectCode != "OTHER" {
		t.Errorf("expected explicit project OTHER, got %q", task.ProjectCode)
	}

	// task list defaults to the configured project.
	out, err := captureStdout(func() error {
		return TaskList(ctx, []string{"-a"})
	})
	if err != nil {
		t.Fatalf("unexpected error listing tasks: %v", err)
	}
	if !strings.Contains(out, "Configured task") {
		t.Errorf("expected configured task in output, got:\n%s", out)
	}
	if strings.Contains(out, "Explicit task") {
		t.Errorf("did not expect task from another project, got:\n%s", out)
	}

	// An explicit -p overrides the configured project in listings.
	out, err = captureStdout(func() error {
		return TaskList(ctx, []string{"-a", "-p", "OTHER"})
	})
	if err != nil {
		t.Fatalf("unexpected error listing tasks with explicit project: %v", err)
	}
	if strings.Contains(out, "Configured task") || !strings.Contains(out, "Explicit task") {
		t.Errorf("expected only explicit task in output, got:\n%s", out)
	}

	// An explicit empty -p disables the workspace default for this invocation.
	out, err = captureStdout(func() error {
		return TaskList(ctx, []string{"-a", "-p", ""})
	})
	if err != nil {
		t.Fatalf("unexpected error listing tasks without project filter: %v", err)
	}
	if !strings.Contains(out, "Configured task") || !strings.Contains(out, "Explicit task") {
		t.Errorf("expected both tasks in output, got:\n%s", out)
	}
}

func TestTaskAddConfiguredProjectNotFound(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()
	ctx.Config = &config.Config{Project: "MISSING"}

	err := TaskAdd(ctx, []string{"Orphan task"})
	if err == nil {
		t.Fatal("expected error for missing configured project, got nil")
	}
	if !strings.Contains(err.Error(), "MISSING") || !strings.Contains(err.Error(), config.FileName) {
		t.Errorf("expected error mentioning the project and config file, got: %v", err)
	}
}

func TestResolveContextLoadsWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, config.FileName)
	if err := os.WriteFile(configPath, []byte("PROJECT_CODE=CFG\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Chdir(dir)

	ctx, _, cleanup, err := ResolveContext([]string{"--db", filepath.Join(dir, "test.db"), "task", "list"})
	if err != nil {
		t.Fatalf("unexpected error in ResolveContext: %v", err)
	}
	defer cleanup()

	if got := ctx.DefaultProject(); got != "CFG" {
		t.Errorf("expected configured project CFG, got %q", got)
	}
}

func TestResolveContextSkipsConfigForVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("invalid line\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Chdir(dir)

	ctx, _, cleanup, err := ResolveContext([]string{"version"})
	if err != nil {
		t.Fatalf("version should ignore config errors, got: %v", err)
	}
	if cleanup != nil {
		t.Error("expected nil cleanup for version")
	}
	if ctx.Config != nil {
		t.Error("expected config not to be loaded for version")
	}
}

func TestResolveContextReportsConfigError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("invalid line\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Chdir(dir)

	_, _, _, err := ResolveContext([]string{"--db", filepath.Join(dir, "test.db"), "task", "list"})
	if err == nil {
		t.Fatal("expected error for malformed config, got nil")
	}
	if !strings.Contains(err.Error(), "workspace config error") {
		t.Errorf("expected workspace config error, got: %v", err)
	}
}
