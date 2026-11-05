package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "task-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_tasks.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// 1. Project Creation
	project, err := store.AddProject("CLI", "Task CLI Project", "Command line tool in Go")
	if err != nil {
		t.Fatalf("AddProject failed: %v", err)
	}
	if project.Code != "CLI" {
		t.Errorf("expected project code 'CLI', got %s", project.Code)
	}

	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	foundProject, err := store.GetProjectByCode("CLI")
	if err != nil {
		t.Fatalf("GetProjectByCode failed: %v", err)
	}
	if foundProject.ID != project.ID {
		t.Errorf("expected project ID %d, got %d", project.ID, foundProject.ID)
	}

	if _, err := store.GetProjectByCode("NON_EXISTENT"); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}

	// 2. Add standalone task and project task with title & description
	taskStandalone, err := store.Add("Standalone Title", "Standalone description", nil, "high")
	if err != nil {
		t.Fatalf("Add standalone task failed: %v", err)
	}
	if taskStandalone.Title != "Standalone Title" {
		t.Errorf("expected title 'Standalone Title', got %s", taskStandalone.Title)
	}
	if taskStandalone.Description != "Standalone description" {
		t.Errorf("expected description 'Standalone description', got %s", taskStandalone.Description)
	}
	if taskStandalone.ProjectID != nil {
		t.Errorf("expected nil project_id for standalone task")
	}
	if taskStandalone.Priority != "high" {
		t.Errorf("expected priority 'high', got %s", taskStandalone.Priority)
	}
	if taskStandalone.Status != StatusPending {
		t.Errorf("expected status '%s', got %s", StatusPending, taskStandalone.Status)
	}

	taskProject, err := store.Add("Project Task Title", "Project task details", &project.ID, "medium")
	if err != nil {
		t.Fatalf("Add project task failed: %v", err)
	}
	if taskProject.ProjectID == nil || *taskProject.ProjectID != project.ID {
		t.Errorf("expected project_id %d, got %v", project.ID, taskProject.ProjectID)
	}

	// 3. Update task title and description
	if err := store.Update(taskStandalone.ID, "New Standalone Title", "New description"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Update task details (change priority)
	if err := store.UpdateTaskDetails(taskStandalone.ID, "New Standalone Title", "New description", nil, "low"); err != nil {
		t.Fatalf("UpdateTaskDetails failed: %v", err)
	}

	// Update priority via UpdatePriority
	if err := store.UpdatePriority(taskStandalone.ID, "urgent"); err != nil {
		t.Fatalf("UpdatePriority failed: %v", err)
	}
	checkPrioTask, err := store.GetTaskByID(taskStandalone.ID)
	if err != nil || checkPrioTask.Priority != "urgent" {
		t.Fatalf("expected priority 'urgent', got %s (err: %v)", checkPrioTask.Priority, err)
	}

	// 4. List active tasks
	tasks, err := store.List("", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Title != "New Standalone Title" {
		t.Errorf("expected updated title 'New Standalone Title', got %s", tasks[0].Title)
	}

	// List filtered by project code
	projectTasks, err := store.List("", "CLI")
	if err != nil {
		t.Fatalf("List by project failed: %v", err)
	}
	if len(projectTasks) != 1 || projectTasks[0].ProjectCode != "CLI" {
		t.Fatalf("expected 1 task in project CLI, got %+v", projectTasks)
	}

	// 5. Update status with abort reason
	reason := "Scope changed"
	if err := store.UpdateStatus(taskProject.ID, StatusAbort, &reason); err != nil {
		t.Fatalf("UpdateStatus abort failed: %v", err)
	}

	abortedTasks, err := store.List(string(StatusAbort), "")
	if err != nil {
		t.Fatalf("List aborted tasks failed: %v", err)
	}
	if len(abortedTasks) != 1 || abortedTasks[0].AbortReason == nil || *abortedTasks[0].AbortReason != reason {
		t.Fatalf("expected 1 aborted task with reason '%s', got %+v", reason, abortedTasks)
	}

	// 6. GetTaskByID
	foundTask, err := store.GetTaskByID(taskProject.ID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if foundTask.ID != taskProject.ID || foundTask.ProjectCode != "CLI" {
		t.Errorf("unexpected task data from GetTaskByID: %+v", foundTask)
	}

	// 7. Soft delete
	if err := store.Delete(taskStandalone.ID); err != nil {
		t.Fatalf("Delete (soft delete) failed: %v", err)
	}

	if _, err := store.GetTaskByID(taskStandalone.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound for soft-deleted task, got: %v", err)
	}

	activeTasks, err := store.List("", "")
	if err != nil {
		t.Fatalf("List after soft delete failed: %v", err)
	}
	if len(activeTasks) != 1 {
		t.Fatalf("expected 1 active task after soft delete, got %d", len(activeTasks))
	}
}
