package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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

	foundByID, err := store.GetProjectByID(project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID failed: %v", err)
	}
	if foundByID.Code != project.Code {
		t.Errorf("expected project code %s, got %s", project.Code, foundByID.Code)
	}

	if _, err := store.GetProjectByID(99999); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound for nonexistent ID, got %v", err)
	}

	// Update project
	if err := store.UpdateProject(project.ID, "CLI Tools Updated", "Updated desc"); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	updatedProj, err := store.GetProjectByID(project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID failed after update: %v", err)
	}
	if updatedProj.Name != "CLI Tools Updated" || updatedProj.Description != "Updated desc" {
		t.Errorf("unexpected updated project data: %+v", updatedProj)
	}
	if err := store.UpdateProject(99999, "None", "None"); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound for nonexistent ID update, got %v", err)
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
	if err := store.UpdateStatus(taskProject.ID, StatusAborted, &reason); err != nil {
		t.Fatalf("UpdateStatus abort failed: %v", err)
	}

	abortedTasks, err := store.List(string(StatusAborted), "")
	if err != nil {
		t.Fatalf("List aborted tasks failed: %v", err)
	}
	if len(abortedTasks) != 1 || abortedTasks[0].AbortReason == nil || *abortedTasks[0].AbortReason != reason {
		t.Fatalf("expected 1 aborted task with reason '%s', got %+v", reason, abortedTasks)
	}
	if abortedTasks[0].Status != StatusAborted {
		t.Fatalf("expected status '%s', got '%s'", StatusAborted, abortedTasks[0].Status)
	}

	// Verify querying with legacy 'abort' string also works
	legacyList, err := store.List("abort", "")
	if err != nil {
		t.Fatalf("List with legacy 'abort' filter failed: %v", err)
	}
	if len(legacyList) != 1 || legacyList[0].ID != taskProject.ID {
		t.Fatalf("expected 1 task with legacy filter, got %d", len(legacyList))
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

	// Verify status is changed to deleted and deleted_at is set in the database
	var deletedStatus string
	var deletedAt *time.Time
	err = store.db.QueryRow("SELECT status, deleted_at FROM tasks WHERE id = ?", taskStandalone.ID).Scan(&deletedStatus, &deletedAt)
	if err != nil {
		t.Fatalf("failed to query deleted task: %v", err)
	}
	if deletedStatus != string(StatusDeleted) {
		t.Errorf("expected status '%s' after delete, got '%s'", StatusDeleted, deletedStatus)
	}
	if deletedAt == nil {
		t.Errorf("expected non-nil deleted_at timestamp")
	}

	activeTasks, err := store.List("", "")
	if err != nil {
		t.Fatalf("List after soft delete failed: %v", err)
	}
	if len(activeTasks) != 1 {
		t.Fatalf("expected 1 active task after soft delete, got %d", len(activeTasks))
	}

	// 8. Documents operations
	// Adding document to soft-deleted task should fail
	if _, err := store.AddDocument(taskStandalone.ID, "spec.pdf", "pdf", "/tmp/spec.pdf"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound when adding doc to soft-deleted task, got: %v", err)
	}

	// Add document to active task
	doc1, err := store.AddDocument(taskProject.ID, "spec.pdf", "pdf", "/home/user/spec.pdf")
	if err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if doc1.Name != "spec.pdf" || doc1.Type != "pdf" || doc1.FilePath != "/home/user/spec.pdf" {
		t.Errorf("unexpected document data: %+v", doc1)
	}

	doc2, err := store.AddDocument(taskProject.ID, "diagram.png", "image/png", "/home/user/diagram.png")
	if err != nil {
		t.Fatalf("AddDocument 2 failed: %v", err)
	}

	// List documents
	docs, err := store.ListDocuments(taskProject.ID)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}

	// GetDocumentByID
	foundDoc, err := store.GetDocumentByID(doc1.ID)
	if err != nil {
		t.Fatalf("GetDocumentByID failed: %v", err)
	}
	if foundDoc.Name != doc1.Name || foundDoc.FilePath != doc1.FilePath {
		t.Errorf("expected doc matching %+v, got %+v", doc1, foundDoc)
	}

	if _, err := store.GetDocumentByID(99999); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("expected ErrDocumentNotFound, got %v", err)
	}

	// Delete document
	if err := store.DeleteDocument(doc1.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}
	if _, err := store.GetDocumentByID(doc1.ID); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("expected ErrDocumentNotFound after deletion, got %v", err)
	}
	docsAfterDel, err := store.ListDocuments(taskProject.ID)
	if err != nil {
		t.Fatalf("ListDocuments after delete failed: %v", err)
	}
	if len(docsAfterDel) != 1 || docsAfterDel[0].ID != doc2.ID {
		t.Fatalf("expected 1 document (%d) remaining, got %+v", doc2.ID, docsAfterDel)
	}
}

func TestLegacyMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "task-cli-migration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "migration.db")

	// Initialize store
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("initial NewStore failed: %v", err)
	}

	// Insert task with legacy 'abort' status and legacy 'todo' status directly into SQLite
	_, err = store.db.Exec(`INSERT INTO tasks (title, description, status, priority, created_at, updated_at) VALUES ('Legacy Abort Task', 'Desc', 'abort', 'medium', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	if err != nil {
		store.Close()
		t.Fatalf("failed to insert legacy abort task: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO tasks (title, description, status, priority, created_at, updated_at) VALUES ('Legacy Todo Task', 'Desc', 'todo', 'medium', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	if err != nil {
		store.Close()
		t.Fatalf("failed to insert legacy todo task: %v", err)
	}
	store.Close()

	// Reopen store; NewStore must execute legacy migration
	reopenedStore, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("reopened NewStore failed: %v", err)
	}
	defer reopenedStore.Close()

	tasks, err := reopenedStore.List("", "")
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}

	var foundAborted, foundPending bool
	for _, task := range tasks {
		if task.Title == "Legacy Abort Task" {
			foundAborted = true
			if task.Status != StatusAborted {
				t.Errorf("expected legacy abort task to have status '%s', got '%s'", StatusAborted, task.Status)
			}
		}
		if task.Title == "Legacy Todo Task" {
			foundPending = true
			if task.Status != StatusPending {
				t.Errorf("expected legacy todo task to have status '%s', got '%s'", StatusPending, task.Status)
			}
		}
	}

	if !foundAborted {
		t.Errorf("legacy abort task not found")
	}
	if !foundPending {
		t.Errorf("legacy todo task not found")
	}
}
