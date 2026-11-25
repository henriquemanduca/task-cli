package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskAdd(t *testing.T) {
	ctx := setupTestContext(t)
	_ = ProjectAdd(ctx, []string{"PROJ", "My Project"})

	if err := TaskAdd(ctx, []string{}); err == nil {
		t.Fatal("expected error without title")
	}
	if err := TaskAdd(ctx, []string{"-f", "X", "Title"}); err == nil {
		t.Fatal("expected error for unknown flag -f")
	}

	out, err := captureStdout(func() error {
		return TaskAdd(ctx, []string{"-p", "proj", "Buy groceries", "Milk, eggs, bread"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Task created successfully (ID: 1) in project 'PROJ'") {
		t.Errorf("unexpected output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(ctx.VaultPath, "PROJ", "0001-buy-groceries.md")); err != nil {
		t.Fatalf("expected task file in project folder: %v", err)
	}

	// Flag after positionals, no project → vault root.
	if err := TaskAdd(ctx, []string{"Standalone", "desc", "--priority", "urgent"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task, err := ctx.Store.GetTask(2)
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectName != "" || task.Priority != "urgent" || task.Description != "desc" {
		t.Errorf("unexpected task: %+v", task)
	}
	if filepath.Dir(task.Path) != ctx.VaultPath {
		t.Errorf("expected task in vault root, got %s", task.Path)
	}

	if err := TaskAdd(ctx, []string{"-p", "NOPE", "x"}); err == nil || !strings.Contains(err.Error(), "project 'NOPE' not found") {
		t.Fatalf("expected project not found, got %v", err)
	}
	if err := TaskAdd(ctx, []string{"--priority", "critical", "x"}); err == nil || !strings.Contains(err.Error(), "invalid priority") {
		t.Fatalf("expected invalid priority, got %v", err)
	}
}

func TestTaskList(t *testing.T) {
	ctx := setupTestContext(t)
	_ = ProjectAdd(ctx, []string{"BETA", "Beta"})
	_ = ProjectAdd(ctx, []string{"ALPHA", "Alpha"})
	_ = TaskAdd(ctx, []string{"-p", "BETA", "Beta task"})   // 1
	_ = TaskAdd(ctx, []string{"-p", "ALPHA", "Alpha task"}) // 2
	_ = TaskAdd(ctx, []string{"Root task"})                 // 3
	_ = TaskAdd(ctx, []string{"-p", "ALPHA", "Done task"})  // 4
	_ = TaskSetDone(ctx, []string{"4"})
	_ = TaskDelete(ctx, []string{"3"})

	out, err := captureStdout(func() error { return TaskList(ctx, nil) })
	if err != nil {
		t.Fatal(err)
	}
	want := "id\tproject\tstatus\tpriority\ttitle\n" +
		"0002\tALPHA\tpending\tmedium\tAlpha task\n" +
		"0001\tBETA\tpending\tmedium\tBeta task\n"
	if out != want {
		t.Errorf("default list:\n%q\nwant:\n%q", out, want)
	}

	out, _ = captureStdout(func() error { return TaskList(ctx, []string{"--done", "-n"}) })
	if out != "0004\tALPHA\tdone\tmedium\tDone task\n" {
		t.Errorf("done list: %q", out)
	}

	out, _ = captureStdout(func() error { return TaskList(ctx, []string{"-a", "-p", "alpha", "-n"}) })
	if strings.Count(out, "\n") != 2 || strings.Contains(out, "BETA") {
		t.Errorf("project filter: %q", out)
	}

	out, _ = captureStdout(func() error { return TaskList(ctx, []string{"in-progress"}) })
	if out != "" {
		t.Errorf("expected no output (not even header) for empty result, got %q", out)
	}

	if err := TaskList(ctx, []string{"deleted"}); err == nil {
		t.Fatal("expected error listing 'deleted' status")
	}
}

func TestTaskViewUpdateAndMove(t *testing.T) {
	ctx := setupTestContext(t)
	_ = ProjectAdd(ctx, []string{"A", "Project A"})
	_ = ProjectAdd(ctx, []string{"B", "Project B"})
	_ = TaskAdd(ctx, []string{"-p", "A", "Original title", "Original desc"})

	out, err := captureStdout(func() error { return TaskView(ctx, []string{"1", "-t"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Task: 0001", "Project: A", "Title: Original title", "Status: pending", "Created At:", "File: ", "Original desc"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in view:\n%s", want, out)
		}
	}

	if err := TaskUpdate(ctx, []string{"1", "-p", "b", "--priority", "high", "New title"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task, _ := ctx.Store.GetTask(1)
	if task.Title != "New title" || task.Description != "Original desc" || task.ProjectName != "B" || task.Priority != "high" {
		t.Errorf("unexpected task after update: %+v", task)
	}
	if task.Path != filepath.Join(ctx.VaultPath, "B", "0001-new-title.md") {
		t.Errorf("unexpected path after move/rename: %s", task.Path)
	}
	if _, err := os.Stat(filepath.Join(ctx.VaultPath, "A", "0001-original-title.md")); !os.IsNotExist(err) {
		t.Errorf("expected old file to be removed, stat err = %v", err)
	}

	if err := TaskUpdate(ctx, []string{"1", "-p", "none"}); err != nil {
		t.Fatal(err)
	}
	task, _ = ctx.Store.GetTask(1)
	if task.ProjectName != "" || filepath.Dir(task.Path) != ctx.VaultPath {
		t.Errorf("expected task detached to vault root: %+v", task)
	}

	if err := TaskUpdate(ctx, []string{"1", "-p", "NOPE"}); err == nil || !strings.Contains(err.Error(), "project 'NOPE' not found") {
		t.Fatalf("expected project not found, got %v", err)
	}
	if err := TaskUpdate(ctx, []string{"99", "x"}); err == nil || !strings.Contains(err.Error(), "task with ID 99 not found") {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := TaskView(ctx, []string{"abc"}); err == nil || !strings.Contains(err.Error(), "invalid task ID") {
		t.Fatalf("expected invalid ID, got %v", err)
	}
}

func TestTaskStatus(t *testing.T) {
	ctx := setupTestContext(t)
	_ = TaskAdd(ctx, []string{"Status task"})

	out, _ := captureStdout(func() error { return TaskStatus(ctx, []string{"1"}) })
	if out != "Task 0001 status: pending\n" {
		t.Errorf("query: %q", out)
	}

	// Reversed argument order and status synonyms.
	if err := TaskStatus(ctx, []string{"doing", "1"}); err != nil {
		t.Fatal(err)
	}
	if task, _ := ctx.Store.GetTask(1); task.Status != "in-progress" {
		t.Errorf("expected in-progress, got %s", task.Status)
	}

	if err := TaskStatus(ctx, []string{"1", "deleted"}); err == nil {
		t.Fatal("expected error setting status deleted")
	}

	out, err := captureStdout(func() error { return TaskSetAbort(ctx, []string{"1", "Out", "of", "scope"}) })
	if err != nil {
		t.Fatal(err)
	}
	if out != "Task 0001 status set to 'aborted' (reason: Out of scope)\n" {
		t.Errorf("abort: %q", out)
	}
	out, _ = captureStdout(func() error { return TaskStatus(ctx, []string{"1"}) })
	if out != "Task 0001 status: aborted (reason: Out of scope)\n" {
		t.Errorf("query aborted: %q", out)
	}
}

func TestImmutableTasks(t *testing.T) {
	ctx := setupTestContext(t)
	_ = TaskAdd(ctx, []string{"To abort"})  // 1
	_ = TaskAdd(ctx, []string{"To delete"}) // 2
	_ = TaskSetAbort(ctx, []string{"1"})
	_ = TaskDelete(ctx, []string{"2"})

	for _, run := range []func() error{
		func() error { return TaskUpdate(ctx, []string{"1", "x"}) },
		func() error { return TaskSetPriority(ctx, []string{"1", "low"}) },
		func() error { return TaskSetPending(ctx, []string{"1"}) },
		func() error { return TaskEdit(ctx, []string{"1"}) },
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "deleted or aborted") {
			t.Errorf("expected immutable error for aborted task, got %v", err)
		}
	}

	if err := TaskSetDone(ctx, []string{"2"}); err == nil {
		t.Error("expected error changing deleted task")
	}
	if err := TaskView(ctx, []string{"2"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected deleted task to be hidden from view, got %v", err)
	}
	if err := TaskDelete(ctx, []string{"2"}); err == nil || !strings.Contains(err.Error(), "already deleted") {
		t.Errorf("expected already deleted error, got %v", err)
	}

	// Deleted task keeps its file and ID, so the next task does not reuse it.
	_ = TaskAdd(ctx, []string{"Next"})
	if _, err := ctx.Store.GetTask(3); err != nil {
		t.Errorf("expected new task to get ID 3: %v", err)
	}
}

func TestTaskEditOpensFile(t *testing.T) {
	ctx := setupTestContext(t)
	_ = TaskAdd(ctx, []string{"Edit me", "old body"})

	// Fake editor: rewrite the title heading and body of the file it receives.
	script := filepath.Join(t.TempDir(), "editor.sh")
	content := "#!/bin/sh\nsed -i 's/^# Edit me$/# Edited title/; s/^old body$/new body/' \"$1\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", script)

	out, err := captureStdout(func() error { return TaskEdit(ctx, []string{"1"}) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Task 1 updated successfully") {
		t.Errorf("unexpected output: %q", out)
	}
	task, _ := ctx.Store.GetTask(1)
	if task.Title != "Edited title" || task.Description != "new body" {
		t.Errorf("unexpected task after edit: %+v", task)
	}
	if filepath.Base(task.Path) != "0001-edited-title.md" {
		t.Errorf("expected file renamed after title change, got %s", task.Path)
	}

	// Editor that changes nothing.
	t.Setenv("EDITOR", "true")
	out, _ = captureStdout(func() error { return TaskEdit(ctx, []string{"1"}) })
	if !strings.Contains(out, "No changes made.") {
		t.Errorf("expected no changes, got %q", out)
	}
}
