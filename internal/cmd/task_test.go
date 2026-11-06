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

	// Add project via ProjectAdd
	err = ProjectAdd(ctx, []string{"PROJ", "My Project", "Project description"})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	// TaskAdd should reject -f flag
	err = TaskAdd(ctx, []string{"-f", "PROJ2", "Another Project"})
	if err == nil {
		t.Fatal("expected error when passing -f to TaskAdd, got nil")
	}

	// Add task linked to project via "add -p" (flag first)
	err = TaskAdd(ctx, []string{"-p", "PROJ", "Buy groceries", "Milk, eggs, bread"})
	if err != nil {
		t.Fatalf("unexpected error adding task with -p: %v", err)
	}

	// Add task linked to project with -p at the END (user report case: "add Title Description -p PROJ")
	err = TaskAdd(ctx, []string{"Teste", "Descricao Teste", "-p", "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error adding task with -p at end: %v", err)
	}
	tAdded, err := ctx.Store.GetTaskByID(2)
	if err != nil {
		t.Fatalf("failed to retrieve task 2: %v", err)
	}
	if tAdded.Title != "Teste" {
		t.Errorf("expected title 'Teste', got '%s'", tAdded.Title)
	}
	if tAdded.Description != "Descricao Teste" {
		t.Errorf("expected description 'Descricao Teste', got '%s'", tAdded.Description)
	}
	if tAdded.ProjectCode != "PROJ" {
		t.Errorf("expected project code 'PROJ', got '%s'", tAdded.ProjectCode)
	}

	// 2. TaskList
	// TaskList should reject -f flag
	err = TaskList(ctx, []string{"-f", "X"})
	if err == nil {
		t.Fatal("expected error when passing -f to TaskList, got nil")
	}

	// List tasks (all) with and without -t and -d
	err = TaskList(ctx, []string{})
	if err != nil {
		t.Fatalf("unexpected error listing tasks: %v", err)
	}
	err = TaskList(ctx, []string{"-t"})
	if err != nil {
		t.Fatalf("unexpected error listing tasks with -t: %v", err)
	}
	// TaskList should reject -d flag (descriptions are only viewable via view/show)
	err = TaskList(ctx, []string{"-d"})
	if err == nil {
		t.Fatal("expected error when passing -d to TaskList, got nil")
	}

	// List tasks filtered by project via "list -p"
	err = TaskList(ctx, []string{"-p", "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error listing tasks filtered by project -p: %v", err)
	}

	// List tasks filtered by status and project: "list pending -p PROJ"
	err = TaskList(ctx, []string{"pending", "-p", "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error listing tasks filtered by status and project: %v", err)
	}

	// List tasks with -a / --all / all
	err = TaskList(ctx, []string{"-a"})
	if err != nil {
		t.Fatalf("unexpected error listing all tasks with -a: %v", err)
	}
	err = TaskList(ctx, []string{"--all"})
	if err != nil {
		t.Fatalf("unexpected error listing all tasks with --all: %v", err)
	}
	err = TaskList(ctx, []string{"all"})
	if err != nil {
		t.Fatalf("unexpected error listing all tasks with 'all': %v", err)
	}

	// List done tasks with --done
	err = TaskList(ctx, []string{"--done"})
	if err != nil {
		t.Fatalf("unexpected error listing done tasks with --done: %v", err)
	}

	err = TaskList(ctx, []string{"--status", "invalid-status"})
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}

	// 3. TaskView and show alias (with and without -t)
	err = TaskView(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error viewing task: %v", err)
	}

	showHandler, ok := Registry["show"]
	if !ok {
		t.Fatal("expected 'show' command to be registered in Registry")
	}
	err = showHandler(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("unexpected error viewing task with show: %v", err)
	}

	err = TaskView(ctx, []string{"1", "-t"})
	if err != nil {
		t.Fatalf("unexpected error viewing task with -t: %v", err)
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

	// Verify task 1 appears in --done list
	err = TaskList(ctx, []string{"--done"})
	if err != nil {
		t.Fatalf("unexpected error listing done tasks: %v", err)
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

	// 7. TaskSetPriority
	err = TaskSetPriority(ctx, []string{"1", "urgent"})
	if err != nil {
		t.Fatalf("unexpected error setting priority: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(1)
	if err != nil || t1.Priority != "urgent" {
		t.Fatalf("expected priority 'urgent', got %s (err: %v)", t1.Priority, err)
	}

	// Validation errors for set-prio
	if err := TaskSetPriority(ctx, []string{}); err == nil {
		t.Fatal("expected error for empty args in set-prio")
	}
	if err := TaskSetPriority(ctx, []string{"1"}); err == nil {
		t.Fatal("expected error for missing priority in set-prio")
	}
	if err := TaskSetPriority(ctx, []string{"1", "invalid-prio"}); err == nil {
		t.Fatal("expected error for invalid priority value")
	}
	if err := TaskSetPriority(ctx, []string{"999", "high"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for id 999, got %v", err)
	}

	// 8. TaskDelete
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

