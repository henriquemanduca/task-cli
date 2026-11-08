package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "task-cli-transfer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "transfer_test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

func TestStoreExportData(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. Create Projects
	projA, err := store.AddProject("PROJ_A", "Project Alpha", "First test project")
	if err != nil {
		t.Fatalf("failed to create projA: %v", err)
	}
	projB, err := store.AddProject("PROJ_B", "Project Beta", "Second test project")
	if err != nil {
		t.Fatalf("failed to create projB: %v", err)
	}

	// 2. Create Tasks for Project A
	tA1, err := store.Add("Task A1", "Description A1", &projA.ID, "high")
	if err != nil {
		t.Fatalf("failed to add task A1: %v", err)
	}
	// Add documents to tA1
	doc1, err := store.AddDocument(tA1.ID, "Specification", "pdf", "/files/spec.pdf")
	if err != nil {
		t.Fatalf("failed to add doc1: %v", err)
	}
	doc2, err := store.AddDocument(tA1.ID, "Notes", "txt", "/files/notes.txt")
	if err != nil {
		t.Fatalf("failed to add doc2: %v", err)
	}

	tA2, err := store.Add("Task A2", "Description A2", &projA.ID, "medium")
	if err != nil {
		t.Fatalf("failed to add task A2: %v", err)
	}
	if err := store.UpdateStatus(tA2.ID, StatusDone, nil); err != nil {
		t.Fatalf("failed to set task A2 to done: %v", err)
	}

	tA3, err := store.Add("Task A3", "Description A3", &projA.ID, "urgent")
	if err != nil {
		t.Fatalf("failed to add task A3: %v", err)
	}
	if err := store.Delete(tA3.ID); err != nil {
		t.Fatalf("failed to soft delete task A3: %v", err)
	}

	// 3. Create Tasks for Project B
	tB1, err := store.Add("Task B1", "Description B1", &projB.ID, "low")
	if err != nil {
		t.Fatalf("failed to add task B1: %v", err)
	}
	reason := "Requirements cancelled"
	if err := store.UpdateStatus(tB1.ID, StatusAbort, &reason); err != nil {
		t.Fatalf("failed to set task B1 to abort: %v", err)
	}

	// 4. Create Standalone Tasks
	tS1, err := store.Add("Standalone Task 1", "Desc S1", nil, "medium")
	if err != nil {
		t.Fatalf("failed to add standalone task 1: %v", err)
	}
	doc3, err := store.AddDocument(tS1.ID, "Standalone Doc", "md", "/files/readme.md")
	if err != nil {
		t.Fatalf("failed to add doc3: %v", err)
	}

	tS2, err := store.Add("Standalone Task 2", "Desc S2", nil, "low")
	if err != nil {
		t.Fatalf("failed to add standalone task 2: %v", err)
	}
	if err := store.Delete(tS2.ID); err != nil {
		t.Fatalf("failed to delete standalone task 2: %v", err)
	}

	t.Run("Full export default options", func(t *testing.T) {
		data, err := store.ExportData(ExportOptions{})
		if err != nil {
			t.Fatalf("ExportData failed: %v", err)
		}

		if data.Version != CurrentSchemaVersion {
			t.Errorf("expected version %d, got %d", CurrentSchemaVersion, data.Version)
		}
		if data.ExportedAt.IsZero() {
			t.Errorf("expected ExportedAt to be set")
		}
		if len(data.Projects) != 2 {
			t.Fatalf("expected 2 projects, got %d", len(data.Projects))
		}

		// Find PROJ_A
		var expA *ProjectTransfer
		for i := range data.Projects {
			if data.Projects[i].Code == "PROJ_A" {
				expA = &data.Projects[i]
				break
			}
		}
		if expA == nil {
			t.Fatalf("PROJ_A not found in export")
		}
		if expA.Name != "Project Alpha" {
			t.Errorf("expected Name 'Project Alpha', got %s", expA.Name)
		}
		// Deleted task tA3 must be excluded by default
		if len(expA.Tasks) != 2 {
			t.Fatalf("expected 2 tasks in PROJ_A, got %d", len(expA.Tasks))
		}
		if expA.Tasks[0].ID != tA1.ID || expA.Tasks[0].Title != "Task A1" {
			t.Errorf("unexpected task 0: %+v", expA.Tasks[0])
		}
		if len(expA.Tasks[0].Documents) != 2 {
			t.Fatalf("expected 2 documents in tA1, got %d", len(expA.Tasks[0].Documents))
		}
		if expA.Tasks[0].Documents[0].Name != doc1.Name || expA.Tasks[0].Documents[1].Name != doc2.Name {
			t.Errorf("unexpected documents in tA1: %+v", expA.Tasks[0].Documents)
		}

		// Check standalone tasks (deleted tS2 excluded by default)
		if len(data.StandaloneTasks) != 1 {
			t.Fatalf("expected 1 standalone task, got %d", len(data.StandaloneTasks))
		}
		if data.StandaloneTasks[0].ID != tS1.ID {
			t.Errorf("expected standalone task ID %d, got %d", tS1.ID, data.StandaloneTasks[0].ID)
		}
		if len(data.StandaloneTasks[0].Documents) != 1 || data.StandaloneTasks[0].Documents[0].Name != doc3.Name {
			t.Errorf("unexpected standalone task documents: %+v", data.StandaloneTasks[0].Documents)
		}
	})

	t.Run("Export filtered by project code", func(t *testing.T) {
		data, err := store.ExportData(ExportOptions{ProjectCode: "PROJ_A"})
		if err != nil {
			t.Fatalf("ExportData failed: %v", err)
		}
		if len(data.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(data.Projects))
		}
		if data.Projects[0].Code != "PROJ_A" {
			t.Errorf("expected project code PROJ_A, got %s", data.Projects[0].Code)
		}
		if len(data.StandaloneTasks) != 0 {
			t.Errorf("expected 0 standalone tasks when filtering by project, got %d", len(data.StandaloneTasks))
		}
	})

	t.Run("Export filtered by non-existent project", func(t *testing.T) {
		_, err := store.ExportData(ExportOptions{ProjectCode: "UNKNOWN"})
		if !errors.Is(err, ErrProjectNotFound) {
			t.Errorf("expected ErrProjectNotFound, got %v", err)
		}
	})

	t.Run("Export filtered by status", func(t *testing.T) {
		data, err := store.ExportData(ExportOptions{Status: "pending"})
		if err != nil {
			t.Fatalf("ExportData failed: %v", err)
		}

		for _, p := range data.Projects {
			for _, task := range p.Tasks {
				if task.Status != StatusPending {
					t.Errorf("expected status pending, got %s", task.Status)
				}
			}
		}
		for _, task := range data.StandaloneTasks {
			if task.Status != StatusPending {
				t.Errorf("expected standalone status pending, got %s", task.Status)
			}
		}
	})

	t.Run("Export with include deleted", func(t *testing.T) {
		data, err := store.ExportData(ExportOptions{IncludeDeleted: true})
		if err != nil {
			t.Fatalf("ExportData failed: %v", err)
		}

		var expA *ProjectTransfer
		for i := range data.Projects {
			if data.Projects[i].Code == "PROJ_A" {
				expA = &data.Projects[i]
				break
			}
		}
		if expA == nil {
			t.Fatalf("PROJ_A not found in export")
		}
		// With deleted, should be 3 tasks
		if len(expA.Tasks) != 3 {
			t.Fatalf("expected 3 tasks in PROJ_A with deleted included, got %d", len(expA.Tasks))
		}
		var foundDeletedA3 bool
		for _, task := range expA.Tasks {
			if task.ID == tA3.ID {
				foundDeletedA3 = true
				if task.Status != StatusDeleted {
					t.Errorf("expected task A3 status '%s', got '%s'", StatusDeleted, task.Status)
				}
			}
		}
		if !foundDeletedA3 {
			t.Errorf("task A3 not found in exported tasks")
		}

		// Standalone tasks should be 2
		if len(data.StandaloneTasks) != 2 {
			t.Fatalf("expected 2 standalone tasks with deleted included, got %d", len(data.StandaloneTasks))
		}
		var foundDeletedS2 bool
		for _, task := range data.StandaloneTasks {
			if task.ID == tS2.ID {
				foundDeletedS2 = true
				if task.Status != StatusDeleted {
					t.Errorf("expected standalone task 2 status '%s', got '%s'", StatusDeleted, task.Status)
				}
			}
		}
		if !foundDeletedS2 {
			t.Errorf("standalone task 2 not found in exported standalone tasks")
		}
	})
}

func TestStoreImportData(t *testing.T) {
	t.Run("Import basic fixture", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		fixture := &TransferData{
			Version:    CurrentSchemaVersion,
			ExportedAt: time.Now().UTC(),
			Projects: []ProjectTransfer{
				{
					Code:        "IMP",
					Name:        "Imported Project",
					Description: "Imported project description",
					Tasks: []TaskTransfer{
						{
							Title:       "Task 1",
							Description: "Desc 1",
							Status:      StatusPending,
							Priority:    "high",
							Documents: []DocumentTransfer{
								{
									Name:     "Doc 1",
									Type:     "pdf",
									FilePath: "/tmp/doc1.pdf",
								},
							},
						},
						{
							Title:       "Task 2",
							Description: "Desc 2",
							Status:      StatusDone,
							Priority:    "medium",
						},
					},
				},
			},
			StandaloneTasks: []TaskTransfer{
				{
					Title:       "Standalone Import",
					Description: "Standalone desc",
					Status:      StatusInProgress,
					Priority:    "urgent",
				},
			},
		}

		res, err := store.ImportData(fixture, ImportOptions{})
		if err != nil {
			t.Fatalf("ImportData failed: %v", err)
		}

		if res.ProjectsImported != 1 {
			t.Errorf("expected 1 project imported, got %d", res.ProjectsImported)
		}
		if res.TasksImported != 3 {
			t.Errorf("expected 3 tasks imported, got %d", res.TasksImported)
		}
		if res.DocumentsImported != 1 {
			t.Errorf("expected 1 document imported, got %d", res.DocumentsImported)
		}
		if res.ProjectsSkipped != 0 || res.TasksSkipped != 0 {
			t.Errorf("expected 0 skipped, got %d projs, %d tasks", res.ProjectsSkipped, res.TasksSkipped)
		}

		// Verify in DB
		p, err := store.GetProjectByCode("IMP")
		if err != nil {
			t.Fatalf("GetProjectByCode failed: %v", err)
		}
		if p.Name != "Imported Project" {
			t.Errorf("expected project name 'Imported Project', got %s", p.Name)
		}

		tasks, err := store.List("", "IMP")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks in project IMP, got %d", len(tasks))
		}

		standaloneTasks, err := store.List("", "")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(standaloneTasks) != 3 { // 2 project + 1 standalone
			t.Fatalf("expected 3 total active tasks, got %d", len(standaloneTasks))
		}

		// Check document on Task 1
		var task1ID int64
		for _, tItem := range tasks {
			if tItem.Title == "Task 1" {
				task1ID = tItem.ID
				break
			}
		}
		docs, err := store.ListDocuments(task1ID)
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		if len(docs) != 1 || docs[0].Name != "Doc 1" {
			t.Fatalf("unexpected documents on Task 1: %+v", docs)
		}
	})

	t.Run("Duplicate project - Skip vs Overwrite", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		// Pre-populate project
		_, err := store.AddProject("DUP", "Original Name", "Original Description")
		if err != nil {
			t.Fatalf("AddProject failed: %v", err)
		}

		fixture := &TransferData{
			Version:    CurrentSchemaVersion,
			ExportedAt: time.Now().UTC(),
			Projects: []ProjectTransfer{
				{
					Code:        "DUP",
					Name:        "New Name",
					Description: "New Description",
					Tasks: []TaskTransfer{
						{
							Title:    "New Task under DUP",
							Status:   StatusPending,
							Priority: "medium",
						},
					},
				},
			},
		}

		// 1. Without Overwrite: project metadata should NOT change, but task should be imported
		resSkip, err := store.ImportData(fixture, ImportOptions{Overwrite: false})
		if err != nil {
			t.Fatalf("ImportData without overwrite failed: %v", err)
		}
		if resSkip.ProjectsSkipped != 1 {
			t.Errorf("expected 1 project skipped, got %d", resSkip.ProjectsSkipped)
		}
		if resSkip.ProjectsUpdated != 0 {
			t.Errorf("expected 0 projects updated, got %d", resSkip.ProjectsUpdated)
		}
		if resSkip.TasksImported != 1 {
			t.Errorf("expected 1 task imported, got %d", resSkip.TasksImported)
		}

		proj, err := store.GetProjectByCode("DUP")
		if err != nil {
			t.Fatalf("GetProjectByCode failed: %v", err)
		}
		if proj.Name != "Original Name" || proj.Description != "Original Description" {
			t.Errorf("project metadata should not change on skip: got %s, %s", proj.Name, proj.Description)
		}

		// 2. With Overwrite: project metadata should update
		fixture.Projects[0].Name = "Updated Name"
		fixture.Projects[0].Description = "Updated Description"
		resOverwrite, err := store.ImportData(fixture, ImportOptions{Overwrite: true})
		if err != nil {
			t.Fatalf("ImportData with overwrite failed: %v", err)
		}
		if resOverwrite.ProjectsUpdated != 1 {
			t.Errorf("expected 1 project updated, got %d", resOverwrite.ProjectsUpdated)
		}
		if resOverwrite.ProjectsSkipped != 0 {
			t.Errorf("expected 0 projects skipped, got %d", resOverwrite.ProjectsSkipped)
		}

		projUpdated, err := store.GetProjectByCode("DUP")
		if err != nil {
			t.Fatalf("GetProjectByCode failed: %v", err)
		}
		if projUpdated.Name != "Updated Name" || projUpdated.Description != "Updated Description" {
			t.Errorf("project metadata should be updated: got %s, %s", projUpdated.Name, projUpdated.Description)
		}
	})

	t.Run("Document deduplication - Skip vs ForceDocs", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		fixtureWithDuplicates := &TransferData{
			Version:    CurrentSchemaVersion,
			ExportedAt: time.Now().UTC(),
			StandaloneTasks: []TaskTransfer{
				{
					Title:    "Doc Dedup Task",
					Status:   StatusPending,
					Priority: "low",
					Documents: []DocumentTransfer{
						{Name: "Doc A", FilePath: "/same/path.pdf"},
						{Name: "Doc B", FilePath: "/same/path.pdf"},
					},
				},
			},
		}

		// Default (ForceDocs = false): second document with same file_path should be skipped
		res, err := store.ImportData(fixtureWithDuplicates, ImportOptions{ForceDocs: false})
		if err != nil {
			t.Fatalf("ImportData failed: %v", err)
		}
		if res.DocumentsImported != 1 {
			t.Errorf("expected 1 doc imported, got %d", res.DocumentsImported)
		}
		if res.DocumentsSkipped != 1 {
			t.Errorf("expected 1 doc skipped, got %d", res.DocumentsSkipped)
		}

		// ForceDocs = true: both should be imported
		resForce, err := store.ImportData(fixtureWithDuplicates, ImportOptions{ForceDocs: true})
		if err != nil {
			t.Fatalf("ImportData with ForceDocs failed: %v", err)
		}
		if resForce.DocumentsImported != 2 {
			t.Errorf("expected 2 docs imported, got %d", resForce.DocumentsImported)
		}
		if resForce.DocumentsSkipped != 0 {
			t.Errorf("expected 0 docs skipped, got %d", resForce.DocumentsSkipped)
		}
	})

	t.Run("Validation errors and Strict mode rollback", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		// 1. Invalid schema version
		badVersion := &TransferData{Version: 99}
		_, err := store.ImportData(badVersion, ImportOptions{})
		if !errors.Is(err, ErrInvalidSchemaVersion) {
			t.Errorf("expected ErrInvalidSchemaVersion, got %v", err)
		}

		// 2. Invalid status in strict mode -> rollback
		badStatusData := &TransferData{
			Version: CurrentSchemaVersion,
			Projects: []ProjectTransfer{
				{
					Code: "VALID_PROJ",
					Name: "Valid Proj",
					Tasks: []TaskTransfer{
						{Title: "Task 1", Status: StatusPending, Priority: "medium"},
						{Title: "Task 2 with bad status", Status: "bad_status", Priority: "medium"},
					},
				},
			},
		}
		_, err = store.ImportData(badStatusData, ImportOptions{Strict: true})
		if !errors.Is(err, ErrInvalidStatus) {
			t.Errorf("expected ErrInvalidStatus, got %v", err)
		}

		// Verify rollback: VALID_PROJ should NOT exist in DB!
		if _, err := store.GetProjectByCode("VALID_PROJ"); !errors.Is(err, ErrProjectNotFound) {
			t.Errorf("expected project to be rolled back, got %v", err)
		}

		// 3. Invalid status in non-strict mode -> skip invalid task, keep valid ones
		resNonStrict, err := store.ImportData(badStatusData, ImportOptions{Strict: false})
		if err != nil {
			t.Fatalf("expected non-strict mode to succeed with warnings, got %v", err)
		}
		if resNonStrict.TasksImported != 1 {
			t.Errorf("expected 1 task imported, got %d", resNonStrict.TasksImported)
		}
		if resNonStrict.TasksSkipped != 1 {
			t.Errorf("expected 1 task skipped, got %d", resNonStrict.TasksSkipped)
		}
		if len(resNonStrict.Warnings) != 1 {
			t.Errorf("expected 1 warning, got %d", len(resNonStrict.Warnings))
		}

		// 4. Invalid priority in strict mode
		badPriorityData := &TransferData{
			Version: CurrentSchemaVersion,
			StandaloneTasks: []TaskTransfer{
				{Title: "Bad Prio Task", Status: StatusPending, Priority: "extreme"},
			},
		}
		_, err = store.ImportData(badPriorityData, ImportOptions{Strict: true})
		if !errors.Is(err, ErrInvalidPriority) {
			t.Errorf("expected ErrInvalidPriority, got %v", err)
		}

		// 5. Valid status 'deleted' in strict mode
		deletedStatusData := &TransferData{
			Version: CurrentSchemaVersion,
			StandaloneTasks: []TaskTransfer{
				{Title: "Deleted Task", Status: StatusDeleted, Priority: "low"},
			},
		}
		resDeleted, err := store.ImportData(deletedStatusData, ImportOptions{Strict: true})
		if err != nil {
			t.Fatalf("expected importing task with StatusDeleted to succeed, got %v", err)
		}
		if resDeleted.TasksImported != 1 {
			t.Errorf("expected 1 task imported, got %d", resDeleted.TasksImported)
		}
	})

	t.Run("Dry-run does not persist changes", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		fixture := &TransferData{
			Version: CurrentSchemaVersion,
			Projects: []ProjectTransfer{
				{
					Code: "DRY",
					Name: "Dry Run Project",
					Tasks: []TaskTransfer{
						{Title: "Dry Task", Status: StatusPending, Priority: "low"},
					},
				},
			},
		}

		res, err := store.ImportData(fixture, ImportOptions{DryRun: true})
		if err != nil {
			t.Fatalf("ImportData in dry-run failed: %v", err)
		}
		if res.ProjectsImported != 1 || res.TasksImported != 1 {
			t.Errorf("expected 1 project and 1 task in stats, got %d, %d", res.ProjectsImported, res.TasksImported)
		}

		// Database should still be empty
		projs, err := store.ListProjects()
		if err != nil {
			t.Fatalf("ListProjects failed: %v", err)
		}
		if len(projs) != 0 {
			t.Fatalf("expected 0 projects in DB after dry run, got %d", len(projs))
		}
	})
}
