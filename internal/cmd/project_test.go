package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectAdd(t *testing.T) {
	ctx := setupTestContext(t)

	if err := ProjectAdd(ctx, []string{}); err == nil {
		t.Fatal("expected error for empty args")
	}
	if err := ProjectAdd(ctx, []string{"  "}); err == nil {
		t.Fatal("expected error for blank name")
	}

	out, err := captureStdout(func() error {
		return ProjectAdd(ctx, []string{"Task CLI", "A cool", "CLI tool"})
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}
	if !strings.Contains(out, "Project 'Task CLI' created successfully") {
		t.Errorf("unexpected output: %q", out)
	}
	data, err := os.ReadFile(filepath.Join(ctx.VaultPath, "Task CLI", ".project.yaml"))
	if err != nil {
		t.Fatalf("expected .project.yaml: %v", err)
	}
	if !strings.Contains(string(data), "description: A cool CLI tool") {
		t.Errorf("unexpected .project.yaml:\n%s", data)
	}

	// Description is optional, but the file is always created.
	if err := ProjectAdd(ctx, []string{"Sem descrição"}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(ctx.VaultPath, "Sem descrição", ".project.yaml"))
	if err != nil || string(data) != "# task-cli project settings\n" {
		t.Errorf("expected .project.yaml with only the header comment, got %q (%v)", data, err)
	}

	err = ProjectAdd(ctx, []string{"task cli"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate (case-insensitive) error, got %v", err)
	}

	for _, bad := range []string{"../x", "a/b", ".hidden", "line\nbreak"} {
		if err := ProjectAdd(ctx, []string{bad}); err == nil || !strings.Contains(err.Error(), "invalid project name") {
			t.Errorf("expected invalid name error for %q, got %v", bad, err)
		}
	}
}

func TestProjectList(t *testing.T) {
	ctx := setupTestContext(t)

	out, err := captureStdout(func() error { return ProjectList(ctx, nil) })
	if err != nil || out != "" {
		t.Fatalf("expected empty output for empty vault, got %q (err %v)", out, err)
	}

	_ = ProjectAdd(ctx, []string{"Zed", "zed desc"})
	_ = ProjectAdd(ctx, []string{"alpha"})
	// A plain folder created by hand is a project too (no .project.yaml needed).
	if err := os.Mkdir(filepath.Join(ctx.VaultPath, "Manual Folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hidden folders are ignored.
	for _, hidden := range []string{".git", ".trash"} {
		if err := os.Mkdir(filepath.Join(ctx.VaultPath, hidden), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	out, err = captureStdout(func() error { return ProjectList(ctx, []string{"-d"}) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "name\tdescription\nalpha\t-\nManual Folder\t-\nZed\tzed desc\n"
	if out != want {
		t.Errorf("unexpected list output:\n%q\nwant:\n%q", out, want)
	}

	out, _ = captureStdout(func() error { return ProjectList(ctx, []string{"-n"}) })
	if out != "alpha\nManual Folder\nZed\n" {
		t.Errorf("unexpected names-only output: %q", out)
	}
}

func TestProjectView(t *testing.T) {
	ctx := setupTestContext(t)
	_ = ProjectAdd(ctx, []string{"Website", "Company website"})

	if err := ProjectView(ctx, nil); err == nil {
		t.Fatal("expected error without name")
	}

	out, err := captureStdout(func() error { return ProjectView(ctx, []string{"website"}) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Project: Website\nFolder: " + filepath.Join(ctx.VaultPath, "Website") + "\nDescription:\nCompany website\n"
	if out != want {
		t.Errorf("unexpected view:\n%q\nwant:\n%q", out, want)
	}

	if err := ProjectView(ctx, []string{"Nope"}); err == nil || !strings.Contains(err.Error(), "project 'Nope' not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestProjectUpdate(t *testing.T) {
	ctx := setupTestContext(t)
	_ = ProjectAdd(ctx, []string{"App", "Old description"})
	_ = ProjectAdd(ctx, []string{"Other"})
	_ = TaskAdd(ctx, []string{"-p", "App", "Inside app"})

	if err := ProjectUpdate(ctx, []string{"App"}); err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("expected nothing to update, got %v", err)
	}

	// Rename moves the folder and its tasks.
	if err := ProjectUpdate(ctx, []string{"app", "--name", "Application"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.VaultPath, "App")); !os.IsNotExist(err) {
		t.Errorf("expected old folder to be gone")
	}
	p, err := ctx.Store.GetProject("Application")
	if err != nil || p.Description != "Old description" {
		t.Fatalf("unexpected project after rename: %+v (%v)", p, err)
	}
	task, err := ctx.Store.GetTask(1)
	if err != nil || task.ProjectName != "Application" {
		t.Errorf("expected task to follow the rename: %+v (%v)", task, err)
	}

	// Case-only rename is allowed; renaming onto another project is not.
	if err := ProjectUpdate(ctx, []string{"Application", "--name", "APPLICATION"}); err != nil {
		t.Fatalf("unexpected error on case-only rename: %v", err)
	}
	if err := ProjectUpdate(ctx, []string{"APPLICATION", "--name", "other"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists, got %v", err)
	}
	if err := ProjectUpdate(ctx, []string{"APPLICATION", "--name", "a/b"}); err == nil {
		t.Fatal("expected invalid name error")
	}

	// Description update keeps hand-written comments and keys; empty clears it.
	yamlPath := filepath.Join(ctx.VaultPath, "APPLICATION", ".project.yaml")
	if err := os.WriteFile(yamlPath, []byte("# mine\ncolor: blue\ndescription: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ProjectUpdate(ctx, []string{"APPLICATION", "--desc", "New one"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(yamlPath)
	if string(data) != "# mine\ncolor: blue\ndescription: New one\n" {
		t.Errorf("unexpected .project.yaml after update:\n%s", data)
	}
	if err := ProjectUpdate(ctx, []string{"APPLICATION", "--desc", ""}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(yamlPath)
	if string(data) != "# mine\ncolor: blue\n" {
		t.Errorf("expected description removed:\n%s", data)
	}

	// Header comment of a freshly created file survives set + clear.
	_ = ProjectAdd(ctx, []string{"Fresh"})
	_ = ProjectUpdate(ctx, []string{"Fresh", "--desc", "something"})
	_ = ProjectUpdate(ctx, []string{"Fresh", "--desc", ""})
	data, _ = os.ReadFile(filepath.Join(ctx.VaultPath, "Fresh", ".project.yaml"))
	if string(data) != "# task-cli project settings\n" {
		t.Errorf("expected header comment kept after clearing description, got %q", data)
	}

	if err := ProjectUpdate(ctx, []string{"Nope", "--desc", "x"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestProjectHierarchicalRouting(t *testing.T) {
	ctx := setupTestContext(t)

	if err := Root.Execute(ctx, []string{"proj", "create", "Back log", "Backlog items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := captureStdout(func() error { return Root.Execute(ctx, []string{"project", "ls", "-d"}) })
	if err != nil || !strings.Contains(out, "Back log\tBacklog items") {
		t.Fatalf("expected project in list, got %q (err %v)", out, err)
	}
	out, err = captureStdout(func() error { return Root.Execute(ctx, []string{"project", "show", "back log"}) })
	if err != nil || !strings.Contains(out, "Project: Back log") {
		t.Fatalf("expected project view, got %q (err %v)", out, err)
	}
}
