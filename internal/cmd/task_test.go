package cmd

import (
	"strings"
	"testing"
)

func TestTaskCommands(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. TaskAdd validation & creation
	err := TaskAdd(ctx, []string{})
	if err == nil {
		t.Fatal("expected error when adding task without title, got nil")
	}

	// Add project via "add -p"
	err = TaskAdd(ctx, []string{"-p", "PROJ", "My Project", "Project description"})
	if err != nil {
		t.Fatalf("unexpected error creating project via add -p: %v", err)
	}

	// Add task linked to project via "add -f"
	err = TaskAdd(ctx, []string{"-f", "PROJ", "Buy groceries", "Milk, eggs, bread"})
	if err != nil {
		t.Fatalf("unexpected error adding task with -f: %v", err)
	}

	// 2. TaskList
	// List projects via "list -p"
	err = TaskList(ctx, []string{"-p"})
	if err != nil {
		t.Fatalf("unexpected error listing projects via list -p: %v", err)
	}

	// List tasks (all)
	err = TaskList(ctx, []string{})
	if err != nil {
		t.Fatalf("unexpected error listing tasks: %v", err)
	}

	// List tasks filtered by project via "list -f"
	err = TaskList(ctx, []string{"-f", "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error listing tasks filtered by project -f: %v", err)
	}

	// List tasks filtered by status and project: "list pending -f PROJ"
	err = TaskList(ctx, []string{"pending", "-f", "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error listing tasks filtered by status and project: %v", err)
	}

	err = TaskList(ctx, []string{"--status", "invalid-status"})
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}

	// 3. TaskView
	err = TaskView(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error viewing task: %v", err)
	}

	err = TaskView(ctx, []string{"999"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for non-existent task, got %v", err)
	}

	// 4. TaskUpdate
	err = TaskUpdate(ctx, []string{"1", "--priority", "high", "Buy fresh groceries"})
	if err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	// 5. TaskSetInProgress and TaskSetPending
	err = TaskSetInProgress(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error setting in-progress: %v", err)
	}
	t1, err := ctx.Store.GetTaskByID(1)
	if err != nil || t1.Status != "in-progress" {
		t.Fatalf("expected status 'in-progress', got %s (err: %v)", t1.Status, err)
	}

	err = TaskSetPending(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error setting pending: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(1)
	if err != nil || t1.Status != "pending" {
		t.Fatalf("expected status 'pending', got %s (err: %v)", t1.Status, err)
	}

	// Validation errors
	if err := TaskSetInProgress(ctx, []string{}); err == nil {
		t.Fatal("expected error for empty args in set-in-progress")
	}
	if err := TaskSetPending(ctx, []string{"abc"}); err == nil {
		t.Fatal("expected error for non-numeric ID in set-pending")
	}
	if err := TaskSetInProgress(ctx, []string{"999"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for id 999, got %v", err)
	}

	// 6. TaskSetDone and TaskSetAbort
	err = TaskSetDone(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error setting done: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(1)
	if err != nil || t1.Status != "done" {
		t.Fatalf("expected status 'done', got %s (err: %v)", t1.Status, err)
	}

	err = TaskSetAbort(ctx, []string{"1", "No longer required"})
	if err != nil {
		t.Fatalf("unexpected error setting abort: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(1)
	if err != nil || t1.Status != "abort" {
		t.Fatalf("expected status 'abort', got %s (err: %v)", t1.Status, err)
	}
	if t1.AbortReason == nil || *t1.AbortReason != "No longer required" {
		t.Fatalf("expected abort reason 'No longer required', got %v", t1.AbortReason)
	}

	// Validation errors for set-abort and set-done
	if err := TaskSetDone(ctx, []string{}); err == nil {
		t.Fatal("expected error for empty args in set-done")
	}
	if err := TaskSetAbort(ctx, []string{}); err == nil {
		t.Fatal("expected error for empty args in set-abort")
	}
	if err := TaskSetAbort(ctx, []string{"invalid-id"}); err == nil {
		t.Fatal("expected error for non-numeric ID in set-abort")
	}

	// 7. TaskDelete
	err = TaskDelete(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error deleting task: %v", err)
	}

	// Deleting again should return not found
	err = TaskDelete(ctx, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for deleted task, got %v", err)
	}
}

