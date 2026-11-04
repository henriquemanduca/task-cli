package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"task-cli/internal/db"
)

func setupTestContext(t *testing.T) (*Context, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "task-cli-cmd-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test_cmd.db")
	store, err := db.NewStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create test store: %v", err)
	}

	ctx := &Context{Store: store}
	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}
	return ctx, cleanup
}

func TestProjectAdd(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. Missing args
	err := ProjectAdd(ctx, []string{})
	if err == nil {
		t.Fatal("expected error for empty args, got nil")
	}

	err = ProjectAdd(ctx, []string{"PROJ"})
	if err == nil {
		t.Fatal("expected error for single arg, got nil")
	}

	// 2. Successful creation
	err = ProjectAdd(ctx, []string{"CLI", "Command Line Interface", "A cool CLI tool"})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	// 3. Duplicate code should fail
	err = ProjectAdd(ctx, []string{"CLI", "Another CLI"})
	if err == nil {
		t.Fatal("expected error for duplicate project code, got nil")
	}
}

func TestProjectList(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// List when empty
	if err := ProjectList(ctx, nil); err != nil {
		t.Fatalf("unexpected error on empty list: %v", err)
	}

	// Add projects and list
	_ = ProjectAdd(ctx, []string{"P1", "Project One"})
	_ = ProjectAdd(ctx, []string{"P2", "Project Two", "Description"})

	if err := ProjectList(ctx, nil); err != nil {
		t.Fatalf("unexpected error listing projects: %v", err)
	}
}

