package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"task-cli/internal/filestore"
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

	// List filtered by project ID
	projectTasksByID, err := store.List("", fmt.Sprintf("%d", project.ID))
	if err != nil {
		t.Fatalf("List by project ID failed: %v", err)
	}
	if len(projectTasksByID) != 1 || projectTasksByID[0].ProjectCode != "CLI" {
		t.Fatalf("expected 1 task in project CLI by ID, got %+v", projectTasksByID)
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

func TestImmutableTasks_AbortedAndDeleted(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_immutable.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// 1. Aborted task cannot change status or be updated
	task1, err := store.Add("Task 1", "Desc 1", nil, "medium")
	if err != nil {
		t.Fatalf("failed to add task 1: %v", err)
	}
	abortReason := "Requirements dropped"
	if err := store.UpdateStatus(task1.ID, StatusAborted, &abortReason); err != nil {
		t.Fatalf("failed to abort task 1: %v", err)
	}

	if err := store.CheckTaskMutable(task1.ID); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from CheckTaskMutable, got: %v", err)
	}

	// Attempt status changes on aborted task
	for _, targetStatus := range []Status{StatusPending, StatusInProgress, StatusDone, StatusAborted} {
		err := store.UpdateStatus(task1.ID, targetStatus, nil)
		if !errors.Is(err, ErrTaskImmutable) {
			t.Errorf("expected ErrTaskImmutable when changing aborted task to %s, got: %v", targetStatus, err)
		}
	}

	// Attempt updates on aborted task
	if err := store.Update(task1.ID, "New Title", "New Desc"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from Update on aborted task, got: %v", err)
	}
	if err := store.UpdateTaskDetails(task1.ID, "New Title", "New Desc", nil, "high"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from UpdateTaskDetails on aborted task, got: %v", err)
	}
	if err := store.UpdateDescription(task1.ID, "New Desc"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from UpdateDescription on aborted task, got: %v", err)
	}
	if err := store.UpdatePriority(task1.ID, "urgent"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from UpdatePriority on aborted task, got: %v", err)
	}

	// 2. Deleted task cannot change status or be updated
	task2, err := store.Add("Task 2", "Desc 2", nil, "low")
	if err != nil {
		t.Fatalf("failed to add task 2: %v", err)
	}
	if err := store.Delete(task2.ID); err != nil {
		t.Fatalf("failed to delete task 2: %v", err)
	}

	if err := store.CheckTaskMutable(task2.ID); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from CheckTaskMutable on deleted task, got: %v", err)
	}

	// Attempt status changes on deleted task
	for _, targetStatus := range []Status{StatusPending, StatusInProgress, StatusDone, StatusAborted} {
		err := store.UpdateStatus(task2.ID, targetStatus, nil)
		if !errors.Is(err, ErrTaskImmutable) {
			t.Errorf("expected ErrTaskImmutable when changing deleted task to %s, got: %v", targetStatus, err)
		}
	}

	// Attempt updates on deleted task
	if err := store.Update(task2.ID, "New Title", "New Desc"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from Update on deleted task, got: %v", err)
	}
	if err := store.UpdateTaskDetails(task2.ID, "New Title", "New Desc", nil, "high"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from UpdateTaskDetails on deleted task, got: %v", err)
	}
	if err := store.UpdateDescription(task2.ID, "New Desc"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from UpdateDescription on deleted task, got: %v", err)
	}
	if err := store.UpdatePriority(task2.ID, "urgent"); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected ErrTaskImmutable from UpdatePriority on deleted task, got: %v", err)
	}

	// 3. Non-existent task returns ErrTaskNotFound
	nonExistentID := int64(99999)
	if err := store.CheckTaskMutable(nonExistentID); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound from CheckTaskMutable, got: %v", err)
	}
	if err := store.UpdateStatus(nonExistentID, StatusDone, nil); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound from UpdateStatus, got: %v", err)
	}
	if err := store.Update(nonExistentID, "Title", "Desc"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound from Update, got: %v", err)
	}
	if err := store.UpdateTaskDetails(nonExistentID, "Title", "Desc", nil, "low"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound from UpdateTaskDetails, got: %v", err)
	}
	if err := store.UpdateDescription(nonExistentID, "Desc"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound from UpdateDescription, got: %v", err)
	}
	if err := store.UpdatePriority(nonExistentID, "high"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound from UpdatePriority, got: %v", err)
	}
}

func TestDefaultDBPath(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("with TASK_CLI_DB override", func(t *testing.T) {
		customPath := filepath.Join(tempDir, "override.db")
		t.Setenv("TASK_CLI_DB", customPath)

		got, err := DefaultDBPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != customPath {
			t.Errorf("expected %q, got %q", customPath, got)
		}
	})

	t.Run("without TASK_CLI_DB delegates to filestore", func(t *testing.T) {
		t.Setenv("TASK_CLI_DB", "")
		expected, err := filestore.DefaultDBPath()
		if err != nil {
			t.Fatalf("filestore.DefaultDBPath failed: %v", err)
		}

		got, err := DefaultDBPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestListAllDocumentPaths(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "docs_test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Initially empty
	paths, err := store.ListAllDocumentPaths()
	if err != nil {
		t.Fatalf("ListAllDocumentPaths failed on empty db: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected 0 paths, got %v", paths)
	}

	// Create tasks
	task1, err := store.Add("Task 1 (Active)", "Desc 1", nil, "medium")
	if err != nil {
		t.Fatalf("failed to add task 1: %v", err)
	}
	task2, err := store.Add("Task 2 (To be deleted)", "Desc 2", nil, "low")
	if err != nil {
		t.Fatalf("failed to add task 2: %v", err)
	}
	task3, err := store.Add("Task 3 (Aborted)", "Desc 3", nil, "high")
	if err != nil {
		t.Fatalf("failed to add task 3: %v", err)
	}

	// Add documents
	_, err = store.AddDocument(task1.ID, "doc1", "pdf", "/storage/attachments/1/doc1.pdf")
	if err != nil {
		t.Fatalf("failed to add doc1: %v", err)
	}
	doc2, err := store.AddDocument(task1.ID, "doc2", "png", "/storage/attachments/1/doc2.png")
	if err != nil {
		t.Fatalf("failed to add doc2: %v", err)
	}
	_, err = store.AddDocument(task2.ID, "doc3", "txt", "/storage/attachments/2/doc3.txt")
	if err != nil {
		t.Fatalf("failed to add doc3: %v", err)
	}
	_, err = store.AddDocument(task3.ID, "doc4", "md", "/storage/attachments/3/doc4.md")
	if err != nil {
		t.Fatalf("failed to add doc4: %v", err)
	}
	// Add an empty/whitespace path to test filtering of empty paths
	_, err = store.AddDocument(task1.ID, "empty_doc", "bin", "   ")
	if err != nil {
		t.Fatalf("failed to add empty doc: %v", err)
	}

	// Abort task 3 (aborted tasks are active in terms of document retention, not deleted)
	abortReason := "cancelled"
	if err := store.UpdateStatus(task3.ID, StatusAborted, &abortReason); err != nil {
		t.Fatalf("failed to abort task 3: %v", err)
	}

	// Check all active before deletions
	paths, err = store.ListAllDocumentPaths()
	if err != nil {
		t.Fatalf("ListAllDocumentPaths failed: %v", err)
	}
	expected := []string{
		"/storage/attachments/1/doc1.pdf",
		"/storage/attachments/1/doc2.png",
		"/storage/attachments/2/doc3.txt",
		"/storage/attachments/3/doc4.md",
	}
	if !reflect.DeepEqual(paths, expected) {
		t.Fatalf("expected paths %v, got %v", expected, paths)
	}

	// 1. Soft-delete task2 -> its documents should no longer be returned
	if err := store.Delete(task2.ID); err != nil {
		t.Fatalf("failed to delete task 2: %v", err)
	}

	// 2. Delete doc2 directly -> should no longer be returned
	if err := store.DeleteDocument(doc2.ID); err != nil {
		t.Fatalf("failed to delete doc2: %v", err)
	}

	pathsAfter, err := store.ListAllDocumentPaths()
	if err != nil {
		t.Fatalf("ListAllDocumentPaths failed after deletion: %v", err)
	}
	expectedAfter := []string{
		"/storage/attachments/1/doc1.pdf",
		"/storage/attachments/3/doc4.md",
	}
	if !reflect.DeepEqual(pathsAfter, expectedAfter) {
		t.Fatalf("expected paths after deletion %v, got %v", expectedAfter, pathsAfter)
	}
}

func TestLegacyDBMigration(t *testing.T) {
	t.Run("successful migration on default path", func(t *testing.T) {
		fakeHome := t.TempDir()
		fakeXDG := filepath.Join(fakeHome, "local_share")
		t.Setenv("HOME", fakeHome)
		t.Setenv("USERPROFILE", fakeHome)
		t.Setenv("XDG_DATA_HOME", fakeXDG)
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("TASK_CLI_DB", "")

		legacyPath, err := filestore.LegacyDBPath()
		if err != nil {
			t.Fatalf("LegacyDBPath failed: %v", err)
		}
		defaultPath, err := filestore.DefaultDBPath()
		if err != nil {
			t.Fatalf("DefaultDBPath failed: %v", err)
		}

		// Create legacy DB with sample data
		legacyStore, err := NewStore(legacyPath)
		if err != nil {
			t.Fatalf("failed to create legacy store: %v", err)
		}
		proj, err := legacyStore.AddProject("LEGACY", "Legacy Project", "From legacy db")
		if err != nil {
			t.Fatalf("failed to add legacy project: %v", err)
		}
		task, err := legacyStore.Add("Legacy Task", "Legacy Desc", &proj.ID, "high")
		if err != nil {
			t.Fatalf("failed to add legacy task: %v", err)
		}
		_, err = legacyStore.AddDocument(task.ID, "legacy_doc", "txt", "/legacy/doc.txt")
		if err != nil {
			t.Fatalf("failed to add legacy doc: %v", err)
		}
		if err := legacyStore.Close(); err != nil {
			t.Fatalf("failed to close legacy store: %v", err)
		}

		// Ensure legacy DB exists, default DB does NOT exist, and .migrated does not exist
		if _, err := os.Stat(legacyPath); err != nil {
			t.Fatalf("expected legacy DB to exist: %v", err)
		}
		if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
			t.Fatalf("expected default DB to NOT exist yet")
		}
		if _, err := os.Stat(legacyPath + ".migrated"); !os.IsNotExist(err) {
			t.Fatalf("expected .migrated to NOT exist yet")
		}

		// Now initialize store via DefaultDBPath
		newStore, err := NewStore(defaultPath)
		if err != nil {
			t.Fatalf("NewStore failed on defaultPath with legacy DB: %v", err)
		}
		defer newStore.Close()

		// Verify file state
		if _, err := os.Stat(defaultPath); err != nil {
			t.Fatalf("expected default DB to exist after migration: %v", err)
		}
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("expected original legacy DB to be moved/renamed: %v", err)
		}
		if _, err := os.Stat(legacyPath + ".migrated"); err != nil {
			t.Fatalf("expected legacy DB to be renamed to .migrated: %v", err)
		}

		// Verify data in migrated store
		loadedProj, err := newStore.GetProjectByCode("LEGACY")
		if err != nil {
			t.Fatalf("failed to find migrated project: %v", err)
		}
		if loadedProj.Name != "Legacy Project" {
			t.Errorf("expected project name 'Legacy Project', got %s", loadedProj.Name)
		}

		loadedTasks, err := newStore.List("", "LEGACY")
		if err != nil {
			t.Fatalf("failed to list migrated tasks: %v", err)
		}
		if len(loadedTasks) != 1 || loadedTasks[0].Title != "Legacy Task" {
			t.Fatalf("unexpected migrated tasks: %+v", loadedTasks)
		}

		docPaths, err := newStore.ListAllDocumentPaths()
		if err != nil {
			t.Fatalf("failed to list doc paths: %v", err)
		}
		if len(docPaths) != 1 || docPaths[0] != "/legacy/doc.txt" {
			t.Fatalf("unexpected migrated doc paths: %+v", docPaths)
		}
	})

	t.Run("corrupted legacy DB aborts without touching original", func(t *testing.T) {
		fakeHome := t.TempDir()
		fakeXDG := filepath.Join(fakeHome, "local_share")
		t.Setenv("HOME", fakeHome)
		t.Setenv("USERPROFILE", fakeHome)
		t.Setenv("XDG_DATA_HOME", fakeXDG)
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("TASK_CLI_DB", "")

		legacyPath, err := filestore.LegacyDBPath()
		if err != nil {
			t.Fatalf("LegacyDBPath failed: %v", err)
		}
		defaultPath, err := filestore.DefaultDBPath()
		if err != nil {
			t.Fatalf("DefaultDBPath failed: %v", err)
		}

		// Write corrupt data to legacyPath
		corruptContent := []byte("this is corrupted garbage, not a sqlite database header")
		if err := os.WriteFile(legacyPath, corruptContent, 0644); err != nil {
			t.Fatalf("failed to write corrupt legacy db: %v", err)
		}

		// NewStore on defaultPath should fail
		store, err := NewStore(defaultPath)
		if err == nil {
			store.Close()
			t.Fatalf("expected NewStore to fail on corrupted legacy database")
		}

		// Original legacy file must still exist and be intact
		readBack, err := os.ReadFile(legacyPath)
		if err != nil {
			t.Fatalf("failed to read back legacy file: %v", err)
		}
		if string(readBack) != string(corruptContent) {
			t.Fatalf("corrupted legacy file was modified! Expected %q, got %q", string(corruptContent), string(readBack))
		}

		// .migrated must NOT exist
		if _, err := os.Stat(legacyPath + ".migrated"); !os.IsNotExist(err) {
			t.Errorf("expected .migrated to NOT exist")
		}

		// Destination default DB must have been cleaned up
		if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
			t.Errorf("expected destination %q to be cleaned up after validation failure", defaultPath)
		}
	})

	t.Run("skips migration when dbPath is not default path", func(t *testing.T) {
		fakeHome := t.TempDir()
		t.Setenv("HOME", fakeHome)
		t.Setenv("USERPROFILE", fakeHome)
		t.Setenv("TASK_CLI_DB", "")

		legacyPath, _ := filestore.LegacyDBPath()
		_ = os.WriteFile(legacyPath, []byte("legacy-content"), 0644)

		customPath := filepath.Join(fakeHome, "custom_non_default.db")
		store, err := NewStore(customPath)
		if err != nil {
			t.Fatalf("NewStore failed: %v", err)
		}
		store.Close()

		// Legacy file must be untouched
		if _, err := os.Stat(legacyPath); err != nil {
			t.Errorf("expected legacy file to remain untouched: %v", err)
		}
		if _, err := os.Stat(legacyPath + ".migrated"); !os.IsNotExist(err) {
			t.Errorf("expected .migrated to NOT exist")
		}
	})

	t.Run("skips migration when default DB already exists", func(t *testing.T) {
		fakeHome := t.TempDir()
		fakeXDG := filepath.Join(fakeHome, "local_share")
		t.Setenv("HOME", fakeHome)
		t.Setenv("USERPROFILE", fakeHome)
		t.Setenv("XDG_DATA_HOME", fakeXDG)
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("TASK_CLI_DB", "")

		defaultPath, err := filestore.DefaultDBPath()
		if err != nil {
			t.Fatalf("DefaultDBPath failed: %v", err)
		}

		// 1. Create default DB first
		store1, err := NewStore(defaultPath)
		if err != nil {
			t.Fatalf("failed to create default DB: %v", err)
		}
		store1.Close()

		// 2. Create legacy file
		legacyPath, err := filestore.LegacyDBPath()
		if err != nil {
			t.Fatalf("LegacyDBPath failed: %v", err)
		}
		legacyContent := []byte("original legacy content")
		if err := os.WriteFile(legacyPath, legacyContent, 0644); err != nil {
			t.Fatalf("failed to write legacy file: %v", err)
		}

		// 3. Open default DB again
		store2, err := NewStore(defaultPath)
		if err != nil {
			t.Fatalf("failed to reopen default DB: %v", err)
		}
		store2.Close()

		// 4. Legacy file must be untouched
		readBack, err := os.ReadFile(legacyPath)
		if err != nil {
			t.Fatalf("failed to read legacy file: %v", err)
		}
		if string(readBack) != string(legacyContent) {
			t.Errorf("legacy file was modified")
		}
		if _, err := os.Stat(legacyPath + ".migrated"); !os.IsNotExist(err) {
			t.Errorf("expected .migrated to NOT exist")
		}
	})
}

func TestTaskListOrderByProjectCode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "order_test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	projZ, err := store.AddProject("ZETA", "Project Zeta", "")
	if err != nil {
		t.Fatalf("AddProject ZETA failed: %v", err)
	}
	projA, err := store.AddProject("ALPHA", "Project Alpha", "")
	if err != nil {
		t.Fatalf("AddProject ALPHA failed: %v", err)
	}
	projB, err := store.AddProject("BETA", "Project Beta", "")
	if err != nil {
		t.Fatalf("AddProject BETA failed: %v", err)
	}

	// 1. Task in ZETA
	t1, err := store.Add("Task Zeta 1", "", &projZ.ID, "medium")
	if err != nil {
		t.Fatalf("Add task 1 failed: %v", err)
	}
	// 2. Standalone task without project
	t2, err := store.Add("Task Standalone 1", "", nil, "medium")
	if err != nil {
		t.Fatalf("Add task 2 failed: %v", err)
	}
	// 3. Task in ALPHA
	t3, err := store.Add("Task Alpha 1", "", &projA.ID, "medium")
	if err != nil {
		t.Fatalf("Add task 3 failed: %v", err)
	}
	// 4. Task in BETA
	t4, err := store.Add("Task Beta 1", "", &projB.ID, "medium")
	if err != nil {
		t.Fatalf("Add task 4 failed: %v", err)
	}
	// 5. Another task in ALPHA
	t5, err := store.Add("Task Alpha 2", "", &projA.ID, "medium")
	if err != nil {
		t.Fatalf("Add task 5 failed: %v", err)
	}
	// 6. Another standalone task
	t6, err := store.Add("Task Standalone 2", "", nil, "medium")
	if err != nil {
		t.Fatalf("Add task 6 failed: %v", err)
	}

	tasks, err := store.List("", "")
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}

	if len(tasks) != 6 {
		t.Fatalf("expected 6 tasks, got %d", len(tasks))
	}

	expectedIDs := []int64{t2.ID, t6.ID, t3.ID, t5.ID, t4.ID, t1.ID}
	for i, expectedID := range expectedIDs {
		if tasks[i].ID != expectedID {
			t.Errorf("at index %d: expected task ID %d, got %d (project %q)",
				i, expectedID, tasks[i].ID, tasks[i].ProjectCode)
		}
	}
}

