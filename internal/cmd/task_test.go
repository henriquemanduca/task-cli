package cmd

import (
	"strings"
	"testing"

	"task-cli/internal/db"
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
	err = TaskAdd(ctx, []string{"-p", "1", "Buy groceries", "Milk, eggs, bread"})
	if err != nil {
		t.Fatalf("unexpected error adding task with -p: %v", err)
	}

	// Add task linked to project with -p at the END (user report case: "add Title Description -p 1")
	err = TaskAdd(ctx, []string{"Teste", "Descricao Teste", "-p", "1"})
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

	err = Root.Execute(ctx, []string{"task", "show", "1"})
	if err != nil {
		t.Fatalf("unexpected error viewing task with show alias: %v", err)
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

	// Move task to project by ID
	err = TaskUpdate(ctx, []string{"1", "-p", "1"})
	if err != nil {
		t.Fatalf("unexpected error moving task to project by ID: %v", err)
	}
	tUpdated, err := ctx.Store.GetTaskByID(1)
	if err != nil || tUpdated.ProjectID == nil || *tUpdated.ProjectID != 1 {
		t.Fatalf("expected task 1 to have ProjectID 1, got %+v (err: %v)", tUpdated, err)
	}

	// Detach task with -p none
	err = TaskUpdate(ctx, []string{"1", "-p", "none"})
	if err != nil {
		t.Fatalf("unexpected error detaching task from project: %v", err)
	}
	tUpdated, err = ctx.Store.GetTaskByID(1)
	if err != nil || tUpdated.ProjectID != nil {
		t.Fatalf("expected task 1 ProjectID to be nil, got %+v (err: %v)", tUpdated, err)
	}

	// TaskUpdate should fail with invalid project ID
	err = TaskUpdate(ctx, []string{"1", "-p", "invalid-id"})
	if err == nil {
		t.Fatal("expected error updating task with non-numeric project ID, got nil")
	}

	// TaskAdd should fail with invalid project ID
	err = TaskAdd(ctx, []string{"-p", "invalid-id", "Some Task"})
	if err == nil {
		t.Fatal("expected error adding task with non-numeric project ID, got nil")
	}

	// TaskAdd should fail with non-existent project ID
	err = TaskAdd(ctx, []string{"-p", "9999", "Some Task"})
	if err == nil {
		t.Fatal("expected error adding task with non-existent project ID, got nil")
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

	// Verify task status was set to deleted
	exportData, err := ctx.Store.ExportData(db.ExportOptions{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ExportData failed: %v", err)
	}
	var foundStatus db.Status
	var found bool
	for _, p := range exportData.Projects {
		for _, tsk := range p.Tasks {
			if tsk.ID == 1 {
				foundStatus = tsk.Status
				found = true
			}
		}
	}
	for _, tsk := range exportData.StandaloneTasks {
		if tsk.ID == 1 {
			foundStatus = tsk.Status
			found = true
		}
	}
	if !found {
		t.Fatal("deleted task 1 not found in export with deleted included")
	}
	if foundStatus != db.StatusDeleted {
		t.Fatalf("expected deleted task status to be '%s', got '%s'", db.StatusDeleted, foundStatus)
	}

	// Deleting again should return not found
	err = TaskDelete(ctx, []string{"1"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for deleted task, got %v", err)
	}
}

func TestTaskStatus(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// Create test task
	task, err := ctx.Store.Add("Task for Status Test", "Desc", nil, "medium")
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// 1. Validation errors
	if err := TaskStatus(ctx, []string{}); err == nil {
		t.Fatal("expected error for empty args in TaskStatus")
	}
	if err := TaskStatus(ctx, []string{"invalid-id"}); err == nil {
		t.Fatal("expected error for non-numeric ID in single-arg query")
	}
	if err := TaskStatus(ctx, []string{"999"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for query id 999, got %v", err)
	}
	if err := TaskStatus(ctx, []string{"invalid", "done"}); err == nil {
		t.Fatal("expected error for non-numeric ID in 2-arg set")
	}
	if err := TaskStatus(ctx, []string{"1", "invalid-status"}); err == nil {
		t.Fatal("expected error for invalid status name")
	}
	if err := TaskStatus(ctx, []string{"999", "done"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error for set id 999, got %v", err)
	}

	// 2. Query initial status
	out, err := captureStdout(func() error {
		return TaskStatus(ctx, []string{"1"})
	})
	if err != nil {
		t.Fatalf("unexpected error querying status: %v", err)
	}
	if !strings.Contains(out, "Task 0001 status: pending") {
		t.Fatalf("expected pending status, got:\n%s", out)
	}

	// 3. Set to in-progress
	err = TaskStatus(ctx, []string{"1", "in-progress"})
	if err != nil {
		t.Fatalf("unexpected error setting in-progress: %v", err)
	}
	t1, err := ctx.Store.GetTaskByID(task.ID)
	if err != nil || t1.Status != "in-progress" {
		t.Fatalf("expected status 'in-progress', got %s (err: %v)", t1.Status, err)
	}

	// 4. Set to done using reversed args order ("done", "1")
	err = TaskStatus(ctx, []string{"done", "1"})
	if err != nil {
		t.Fatalf("unexpected error setting done: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(task.ID)
	if err != nil || t1.Status != "done" {
		t.Fatalf("expected status 'done', got %s (err: %v)", t1.Status, err)
	}

	// 5. Set to abort with reason
	err = TaskStatus(ctx, []string{"1", "abort", "Scope no longer applicable"})
	if err != nil {
		t.Fatalf("unexpected error setting abort: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(task.ID)
	if err != nil || t1.Status != "abort" {
		t.Fatalf("expected status 'abort', got %s (err: %v)", t1.Status, err)
	}
	if t1.AbortReason == nil || *t1.AbortReason != "Scope no longer applicable" {
		t.Fatalf("expected abort reason, got %v", t1.AbortReason)
	}

	// Query abort status includes reason
	out, err = captureStdout(func() error {
		return TaskStatus(ctx, []string{"1"})
	})
	if err != nil {
		t.Fatalf("unexpected error querying abort status: %v", err)
	}
	if !strings.Contains(out, "Scope no longer applicable") {
		t.Fatalf("expected abort reason in query output, got:\n%s", out)
	}

	// 6. Set to pending using alias 'todo'
	err = TaskStatus(ctx, []string{"1", "todo"})
	if err != nil {
		t.Fatalf("unexpected error resetting to pending: %v", err)
	}
	t1, err = ctx.Store.GetTaskByID(task.ID)
	if err != nil || t1.Status != "pending" {
		t.Fatalf("expected status 'pending', got %s (err: %v)", t1.Status, err)
	}
}

