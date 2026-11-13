package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"task-cli/internal/db"
)

func setupTestContext(t *testing.T) (*Context, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "task-cli-cmd-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("LOCALAPPDATA", tempDir)

	dbPath := filepath.Join(tempDir, "test_cmd.db")
	store, err := db.NewStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create test store: %v", err)
	}

	ctx := &Context{Store: store, DBPath: dbPath}
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

	if err := ProjectList(ctx, []string{"-t"}); err != nil {
		t.Fatalf("unexpected error listing projects with -t: %v", err)
	}

	if err := ProjectList(ctx, []string{"-d"}); err != nil {
		t.Fatalf("unexpected error listing projects with -d: %v", err)
	}

	if err := ProjectList(ctx, []string{"-d", "-t"}); err != nil {
		t.Fatalf("unexpected error listing projects with -d and -t: %v", err)
	}
}

func TestProjectView(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. Missing args
	err := ProjectView(ctx, []string{})
	if err == nil {
		t.Fatal("expected error for empty args, got nil")
	}

	// 2. Nonexistent project
	err = ProjectView(ctx, []string{"NONEXISTENT"})
	if err == nil {
		t.Fatal("expected error for nonexistent project, got nil")
	}

	// Create a project to view
	err = ProjectAdd(ctx, []string{"TKC", "Task CLI", "A Go CLI tool"})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	// 3. View by code
	out, err := captureStdout(func() error {
		return ProjectView(ctx, []string{"TKC"})
	})
	if err != nil {
		t.Fatalf("unexpected error viewing project by code: %v", err)
	}
	if !strings.Contains(out, "Project: TKC") || !strings.Contains(out, "Name: Task CLI") || !strings.Contains(out, "A Go CLI tool") {
		t.Fatalf("expected project details in output, got:\n%s", out)
	}

	// 4. View by numeric ID
	p, err := ctx.Store.GetProjectByCode(ctx.Context(), "TKC")
	if err != nil {
		t.Fatalf("unexpected error getting project: %v", err)
	}

	out, err = captureStdout(func() error {
		return ProjectView(ctx, []string{strconv.FormatInt(p.ID, 10)})
	})
	if err != nil {
		t.Fatalf("unexpected error viewing project by ID: %v", err)
	}
	if !strings.Contains(out, "Project: TKC") {
		t.Fatalf("expected project details by ID in output, got:\n%s", out)
	}

	// 5. View with timestamps (-t)
	out, err = captureStdout(func() error {
		return ProjectView(ctx, []string{"TKC", "-t"})
	})
	if err != nil {
		t.Fatalf("unexpected error viewing project with -t: %v", err)
	}
	if !strings.Contains(out, "Created:") || !strings.Contains(out, "Updated:") {
		t.Fatalf("expected timestamps in output, got:\n%s", out)
	}
}

func TestProjectHierarchicalRouting(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. project add via Root.Execute
	err := Root.Execute(ctx, []string{"project", "add", "HIER", "Hierarchical Project", "Hierarchical description"})
	if err != nil {
		t.Fatalf("unexpected error running 'project add': %v", err)
	}

	// 2. project list via alias 'proj ls'
	out, err := captureStdout(func() error {
		return Root.Execute(ctx, []string{"proj", "ls"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'proj ls': %v", err)
	}
	if !strings.Contains(out, "HIER") || !strings.Contains(out, "Hierarchical Project") {
		t.Fatalf("expected project in list output, got:\n%s", out)
	}

	// 3. project view via Root.Execute
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"project", "view", "HIER"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'project view': %v", err)
	}
	if !strings.Contains(out, "Project: HIER") || !strings.Contains(out, "Hierarchical description") {
		t.Fatalf("expected project details in view output, got:\n%s", out)
	}

	// 4. project show (alias) with -t
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"project", "show", "HIER", "-t"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'project show -t': %v", err)
	}
	if !strings.Contains(out, "Created:") {
		t.Fatalf("expected timestamps in show output, got:\n%s", out)
	}

	// 5. project with no args prints help
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"project"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'project': %v", err)
	}
	if !strings.Contains(out, "Manage projects") || !strings.Contains(out, "Available commands:") {
		t.Fatalf("expected help output for 'project', got:\n%s", out)
	}

	// 6. Execute dispatch via top-level command "project"
	handled, err := Execute(Root, ctx, "project", []string{"list"})
	if !handled || err != nil {
		t.Fatalf("expected handled=true, err=nil for Execute(Root, 'project', ['list']), got handled=%v, err=%v", handled, err)
	}
}

func TestProjectUpdate(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// Setup initial project
	_, err := ctx.Store.AddProject(ctx.Context(), "PRJ", "Initial Name", "Initial Description")
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	// 1. Missing target
	err = ProjectUpdate(ctx, []string{})
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}

	// 2. Non-existent target
	err = ProjectUpdate(ctx, []string{"NONEXISTENT", "--name", "New Name"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}

	// 3. Update name only by code
	var out string
	out, err = captureStdout(func() error {
		return ProjectUpdate(ctx, []string{"PRJ", "--name", "Updated Name"})
	})
	if err != nil {
		t.Fatalf("unexpected error updating project: %v", err)
	}
	if !strings.Contains(out, "Project 'PRJ' updated successfully") {
		t.Fatalf("expected success message, got: %s", out)
	}

	p, err := ctx.Store.GetProjectByCode(ctx.Context(), "PRJ")
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if p.Name != "Updated Name" || p.Description != "Initial Description" {
		t.Fatalf("unexpected project data: %+v", p)
	}

	// 4. Update description only by ID
	idStr := strconv.FormatInt(p.ID, 10)
	out, err = captureStdout(func() error {
		return ProjectUpdate(ctx, []string{idStr, "--desc", "Updated Description"})
	})
	if err != nil {
		t.Fatalf("unexpected error updating project by id: %v", err)
	}
	if !strings.Contains(out, "Project 'PRJ' updated successfully") {
		t.Fatalf("expected success message, got: %s", out)
	}

	p, err = ctx.Store.GetProjectByID(ctx.Context(), p.ID)
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if p.Name != "Updated Name" || p.Description != "Updated Description" {
		t.Fatalf("unexpected project data: %+v", p)
	}

	// 5. Update via Root hierarchical router with alias and flags in various positions
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"project", "mod", "--name", "Final Name", "PRJ", "--desc", "Final Description"})
	})
	if err != nil {
		t.Fatalf("unexpected error executing project update via router: %v", err)
	}
	if !strings.Contains(out, "Project 'PRJ' updated successfully") {
		t.Fatalf("expected success message, got: %s", out)
	}

	p, err = ctx.Store.GetProjectByCode(ctx.Context(), "PRJ")
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if p.Name != "Final Name" || p.Description != "Final Description" {
		t.Fatalf("unexpected project data: %+v", p)
	}
}
