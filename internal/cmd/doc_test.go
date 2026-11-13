package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"task-cli/internal/filestore"
)

func TestDocCommands(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// Create a temp file to use as a document
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "architecture.pdf")
	if err := os.WriteFile(tempFile, []byte("sample pdf content"), 0644); err != nil {
		t.Fatalf("failed to create temp test file: %v", err)
	}

	tempTextFile := filepath.Join(tempDir, "notes.txt")
	if err := os.WriteFile(tempTextFile, []byte("meeting notes"), 0644); err != nil {
		t.Fatalf("failed to create temp test file: %v", err)
	}

	// Create a task to attach documents to
	task, err := ctx.Store.Add(ctx.Context(), "Design Architecture", "High level design", nil, "high")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 1. DocAdd validations
	if err := DocAdd(ctx, []string{}); err == nil {
		t.Fatal("expected error when no args provided to DocAdd, got nil")
	}

	if err := DocAdd(ctx, []string{"not-a-number", tempFile}); err == nil {
		t.Fatal("expected error for invalid task ID, got nil")
	}

	if err := DocAdd(ctx, []string{"1", "/path/that/does/not/exist/file.pdf"}); err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	if err := DocAdd(ctx, []string{"9999", tempFile}); err == nil {
		t.Fatal("expected error for non-existent task ID, got nil")
	}

	// 2. Successful DocAdd with inferred name and type (default copies file)
	if err := DocAdd(ctx, []string{"1", tempFile}); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil {
		t.Fatalf("failed to list documents from store: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].Name != "architecture.pdf" {
		t.Errorf("expected doc name 'architecture.pdf', got '%s'", docs[0].Name)
	}
	if docs[0].Type != "pdf" {
		t.Errorf("expected doc type 'pdf', got '%s'", docs[0].Type)
	}
	absPath, _ := filepath.Abs(tempFile)
	if docs[0].FilePath == absPath {
		t.Errorf("expected copied path in attachments, got original path '%s'", absPath)
	}
	if !strings.Contains(docs[0].FilePath, "architecture_") || !strings.HasSuffix(docs[0].FilePath, ".pdf") {
		t.Errorf("expected copied path to contain 'architecture_' and end with '.pdf', got '%s'", docs[0].FilePath)
	}
	if _, err := os.Stat(docs[0].FilePath); err != nil {
		t.Errorf("copied file not found on disk at '%s': %v", docs[0].FilePath, err)
	}

	// 3. Successful DocAdd with custom name and type via flags
	if err := DocAdd(ctx, []string{"1", tempTextFile, "--name", "Project Notes", "--type", "markdown"}); err != nil {
		t.Fatalf("failed to add second document with flags: %v", err)
	}

	docs, err = ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil {
		t.Fatalf("failed to list documents from store: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[1].Name != "Project Notes" || docs[1].Type != "markdown" {
		t.Errorf("unexpected custom doc data: %+v", docs[1])
	}

	// 4. DocList validations & success
	if err := DocList(ctx, []string{}); err == nil {
		t.Fatal("expected error when no task ID provided to DocList, got nil")
	}
	if err := DocList(ctx, []string{"invalid"}); err == nil {
		t.Fatal("expected error for invalid task ID in DocList, got nil")
	}
	if err := DocList(ctx, []string{"9999"}); err == nil {
		t.Fatal("expected error for non-existent task in DocList, got nil")
	}
	if err := DocList(ctx, []string{"1"}); err != nil {
		t.Fatalf("unexpected error listing documents: %v", err)
	}

	// 5. TaskView should show attached documents
	if err := TaskView(ctx, []string{"1"}); err != nil {
		t.Fatalf("unexpected error in TaskView: %v", err)
	}

	// 6. DocDelete validations & success
	if err := DocDelete(ctx, []string{}); err == nil {
		t.Fatal("expected error when no doc ID provided to DocDelete, got nil")
	}
	if err := DocDelete(ctx, []string{"invalid"}); err == nil {
		t.Fatal("expected error for invalid doc ID in DocDelete, got nil")
	}
	if err := DocDelete(ctx, []string{"9999"}); err == nil {
		t.Fatal("expected error for non-existent doc ID in DocDelete, got nil")
	}

	if err := DocDelete(ctx, []string{"1"}); err != nil {
		t.Fatalf("unexpected error deleting document: %v", err)
	}

	docsAfterDelete, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil {
		t.Fatalf("failed to list documents after delete: %v", err)
	}
	if len(docsAfterDelete) != 1 {
		t.Fatalf("expected 1 document remaining, got %d", len(docsAfterDelete))
	}
}

func TestDocAdd_Link(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "manual.pdf")
	if err := os.WriteFile(docFile, []byte("manual content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	task, err := ctx.Store.Add(ctx.Context(), "Task for link test", "Testing --link", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	if err := DocAdd(ctx, []string{taskIDStr, docFile, "--link"}); err != nil {
		t.Fatalf("DocAdd --link failed: %v", err)
	}

	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	absDocFile, _ := filepath.Abs(docFile)
	if docs[0].FilePath != absDocFile {
		t.Errorf("expected DB path to be original '%s', got '%s'", absDocFile, docs[0].FilePath)
	}

	// Verify attachments directory for this task has NO copied files
	taskAttDir, err := filestore.TaskAttachmentDir(task.ID)
	if err != nil {
		t.Fatalf("TaskAttachmentDir failed: %v", err)
	}
	if entries, err := os.ReadDir(taskAttDir); err == nil && len(entries) > 0 {
		t.Errorf("expected attachments directory to be empty or non-existent, found %d files", len(entries))
	}
}

func TestDocAdd_Copy(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "design_spec.docx")
	content := []byte("design specification content")
	if err := os.WriteFile(docFile, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	task, err := ctx.Store.Add(ctx.Context(), "Task for copy test", "Testing --copy", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	if err := DocAdd(ctx, []string{taskIDStr, docFile, "--copy"}); err != nil {
		t.Fatalf("DocAdd --copy failed: %v", err)
	}

	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	absDocFile, _ := filepath.Abs(docFile)
	if docs[0].FilePath == absDocFile {
		t.Errorf("expected copied path, got original path %s", absDocFile)
	}

	hash, err := filestore.FileShortHash(absDocFile)
	if err != nil {
		t.Fatalf("FileShortHash failed: %v", err)
	}
	expectedFilename := fmt.Sprintf("design_spec_%s.docx", hash)
	if filepath.Base(docs[0].FilePath) != expectedFilename {
		t.Errorf("expected filename %s, got %s", expectedFilename, filepath.Base(docs[0].FilePath))
	}

	// Verify physical file exists and contents match
	copiedData, err := os.ReadFile(docs[0].FilePath)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(copiedData) != string(content) {
		t.Errorf("expected copied content %q, got %q", string(content), string(copiedData))
	}
}

func TestDocAdd_LargeFile_ThresholdAndWarning(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	largeFile := filepath.Join(tempDir, "bigdata.iso")
	// Create sparse file exceeding 100MB without using actual disk blocks
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatalf("failed to create sparse file: %v", err)
	}
	largeSize := filestore.MaxCopySize + 1024*1024 // 101 MB
	if err := f.Truncate(largeSize); err != nil {
		f.Close()
		t.Fatalf("failed to truncate sparse file: %v", err)
	}
	f.Close()

	task, err := ctx.Store.Add(ctx.Context(), "Large file task", "Testing large file threshold", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	// Case 1: Without --copy (default), file > 100MB should NOT be copied, warning on Stderr
	var stderrBuf bytes.Buffer
	ctx.Stderr = &stderrBuf

	if err := DocAdd(ctx, []string{taskIDStr, largeFile}); err != nil {
		t.Fatalf("DocAdd failed: %v", err)
	}

	expectedWarning := "Warning: file exceeds 100MB; linking by reference without copying. Use --copy to force duplication.\n"
	if !strings.Contains(stderrBuf.String(), expectedWarning) {
		t.Errorf("expected stderr warning %q, got %q", expectedWarning, stderrBuf.String())
	}

	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	absLargeFile, _ := filepath.Abs(largeFile)
	if docs[0].FilePath != absLargeFile {
		t.Errorf("expected DB path to be original '%s', got '%s'", absLargeFile, docs[0].FilePath)
	}

	// Case 2: With --copy, file > 100MB is duplicated, NO warning on Stderr
	stderrBuf.Reset()
	task2, err := ctx.Store.Add(ctx.Context(), "Large file forced copy", "Testing --copy", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task 2: %v", err)
	}
	task2IDStr := fmt.Sprintf("%d", task2.ID)

	if err := DocAdd(ctx, []string{task2IDStr, largeFile, "--copy"}); err != nil {
		t.Fatalf("DocAdd with --copy failed: %v", err)
	}

	if strings.Contains(stderrBuf.String(), "Warning: file exceeds 100MB") {
		t.Errorf("unexpected warning in stderr with --copy: %q", stderrBuf.String())
	}

	docs2, err := ctx.Store.ListDocuments(ctx.Context(), task2.ID)
	if err != nil {
		t.Fatalf("ListDocuments for task2 failed: %v", err)
	}
	if len(docs2) != 1 {
		t.Fatalf("expected 1 document for task2, got %d", len(docs2))
	}
	if docs2[0].FilePath == absLargeFile {
		t.Errorf("expected copied path for task2, got original path %s", absLargeFile)
	}
	if _, err := os.Stat(docs2[0].FilePath); err != nil {
		t.Errorf("copied large file does not exist on disk: %v", err)
	}
}

func TestDocAdd_ConflictingFlags(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "doc.txt")
	_ = os.WriteFile(docFile, []byte("content"), 0644)

	task, _ := ctx.Store.Add(ctx.Context(), "Task", "Description", nil, "medium")
	taskIDStr := fmt.Sprintf("%d", task.ID)

	err := DocAdd(ctx, []string{taskIDStr, docFile, "--copy", "--link"})
	if err == nil || !strings.Contains(err.Error(), "cannot specify both --copy and --link") {
		t.Fatalf("expected error for conflicting flags, got: %v", err)
	}
}

func TestDocAdd_Rollback(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "precious_data.pdf")
	content := []byte("important document data for rollback test")
	if err := os.WriteFile(docFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	task, err := ctx.Store.Add(ctx.Context(), "Task for rollback test", "Testing rollback on DB error", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	taskAttDir, err := filestore.TaskAttachmentDir(task.ID)
	if err != nil {
		t.Fatalf("failed to resolve task attachment dir: %v", err)
	}

	// Induce SQLite failure on INSERT INTO documents via a trigger
	dbConn, err := sql.Open("sqlite", ctx.DBPath)
	if err != nil {
		t.Fatalf("failed to open direct db connection: %v", err)
	}
	defer dbConn.Close()

	_, err = dbConn.Exec("CREATE TRIGGER trigger_fail_doc BEFORE INSERT ON documents BEGIN SELECT RAISE(FAIL, 'simulated database insert error'); END;")
	if err != nil {
		t.Fatalf("failed to create fail trigger: %v", err)
	}

	// Case 1: Default copy mode - file is copied first, but DB insert fails, triggering rollback
	err = DocAdd(ctx, []string{taskIDStr, docFile})
	if err == nil {
		t.Fatal("expected DocAdd to fail due to trigger, got nil")
	}
	if !strings.Contains(err.Error(), "simulated database insert error") && !strings.Contains(err.Error(), "failed to link document") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify that the copied physical file in attachments/<task_id>/ was deleted by rollback
	if entries, err := os.ReadDir(taskAttDir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "precious_data_") {
				t.Fatalf("expected copied file to be removed by rollback, but found: %s", entry.Name())
			}
		}
	}

	// Verify that the user's original source file was NOT touched
	if _, err := os.Stat(docFile); err != nil {
		t.Fatalf("original source file was deleted or inaccessible: %v", err)
	}

	// Case 2: Link mode - verify rollback does NOT delete original source file
	err = DocAdd(ctx, []string{taskIDStr, docFile, "--link"})
	if err == nil {
		t.Fatal("expected DocAdd --link to fail due to trigger, got nil")
	}
	if _, err := os.Stat(docFile); err != nil {
		t.Fatalf("original source file was deleted during --link failure: %v", err)
	}
}

func TestDocOpen_Headless(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// Ensure headless mode
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "manual.pdf")
	if err := os.WriteFile(docFile, []byte("manual content"), 0644); err != nil {
		t.Fatalf("failed to create test doc: %v", err)
	}

	task, err := ctx.Store.Add(ctx.Context(), "Test Doc Open", "Testing doc open command", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	// Attach document
	if err := DocAdd(ctx, []string{taskIDStr, docFile}); err != nil {
		t.Fatalf("failed to attach document: %v", err)
	}

	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil || len(docs) == 0 {
		t.Fatalf("failed to list docs: %v", err)
	}
	doc := docs[0]
	docIDStr := fmt.Sprintf("%d", doc.ID)

	taskDir, err := filestore.TaskAttachmentDir(task.ID)
	if err != nil {
		t.Fatalf("failed to resolve task dir: %v", err)
	}

	// 1. Test task doc open <task_id>
	var buf bytes.Buffer
	ctx.Out = &buf
	if err := DocOpen(ctx, []string{taskIDStr}); err != nil {
		t.Fatalf("DocOpen with task ID failed: %v", err)
	}
	if !strings.Contains(buf.String(), taskDir) {
		t.Errorf("expected output to contain taskDir %q, got %q", taskDir, buf.String())
	}

	// 2. Test task doc open --doc <doc_id>
	buf.Reset()
	if err := DocOpen(ctx, []string{"--doc", docIDStr}); err != nil {
		t.Fatalf("DocOpen with --doc failed: %v", err)
	}
	if !strings.Contains(buf.String(), doc.FilePath) {
		t.Errorf("expected output to contain doc.FilePath %q, got %q", doc.FilePath, buf.String())
	}

	// 3. Test validation errors:
	// Missing args
	if err := DocOpen(ctx, []string{}); err == nil {
		t.Error("expected error for missing args, got nil")
	}

	// Non-numeric task ID
	if err := DocOpen(ctx, []string{"not-a-number"}); err == nil {
		t.Error("expected error for non-numeric task ID, got nil")
	}

	// Non-existent task ID
	if err := DocOpen(ctx, []string{"999999"}); err == nil {
		t.Error("expected error for non-existent task ID, got nil")
	}

	// Non-existent doc ID
	if err := DocOpen(ctx, []string{"--doc", "999999"}); err == nil {
		t.Error("expected error for non-existent doc ID, got nil")
	}

	// Conflicting args
	if err := DocOpen(ctx, []string{taskIDStr, "--doc", docIDStr}); err == nil {
		t.Error("expected error when both task ID and --doc specified, got nil")
	}

	// 4. Deleted task validation:
	// Soft delete task
	if err := ctx.Store.Delete(ctx.Context(), task.ID); err != nil {
		t.Fatalf("failed to delete task: %v", err)
	}

	// Opening deleted task directory directly should fail
	if err := DocOpen(ctx, []string{taskIDStr}); err == nil {
		t.Error("expected error opening deleted task, got nil")
	}

	// Opening document belonging to deleted task should fail
	if err := DocOpen(ctx, []string{"--doc", docIDStr}); err == nil {
		t.Error("expected error opening document belonging to deleted task, got nil")
	}
}

func TestDocPrune(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	docFile1 := filepath.Join(tempDir, "legit.pdf")
	_ = os.WriteFile(docFile1, []byte("legit content"), 0644)

	task, err := ctx.Store.Add(ctx.Context(), "Prune Task", "Testing doc prune", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	// Case 1: No attachments exist yet
	var buf bytes.Buffer
	ctx.Out = &buf
	if err := DocPrune(ctx, []string{}); err != nil {
		t.Fatalf("DocPrune failed on empty storage: %v", err)
	}
	if !strings.Contains(buf.String(), "No orphaned attachments found.") {
		t.Errorf("expected 'No orphaned attachments found.', got %q", buf.String())
	}

	// Add legitimate document
	if err := DocAdd(ctx, []string{taskIDStr, docFile1}); err != nil {
		t.Fatalf("failed to attach doc: %v", err)
	}
	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil || len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d (err: %v)", len(docs), err)
	}
	legitFilePath := docs[0].FilePath

	// Case 2: Only legitimate files exist, no orphans
	buf.Reset()
	if err := DocPrune(ctx, []string{}); err != nil {
		t.Fatalf("DocPrune failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No orphaned attachments found.") {
		t.Errorf("expected 'No orphaned attachments found.', got %q", buf.String())
	}
	if _, err := os.Stat(legitFilePath); err != nil {
		t.Errorf("legitimate file was deleted: %v", err)
	}

	// Plant an orphaned file inside the task attachment folder
	taskDir, err := filestore.TaskAttachmentDir(task.ID)
	if err != nil {
		t.Fatalf("failed to get task dir: %v", err)
	}
	orphanFile := filepath.Join(taskDir, "orphan_abc12345.pdf")
	if err := os.WriteFile(orphanFile, []byte("orphan content"), 0644); err != nil {
		t.Fatalf("failed to create orphan file: %v", err)
	}

	// Plant another orphaned file in a separate orphaned task folder
	orphanDir, err := filestore.TaskAttachmentDir(99999)
	if err != nil {
		t.Fatalf("failed to get orphan task dir: %v", err)
	}
	_ = os.MkdirAll(orphanDir, 0755)
	orphanFile2 := filepath.Join(orphanDir, "ghost_12345678.txt")
	if err := os.WriteFile(orphanFile2, []byte("ghost content"), 0644); err != nil {
		t.Fatalf("failed to create second orphan file: %v", err)
	}

	// Case 3: Dry run (--dry-run and -n)
	buf.Reset()
	if err := DocPrune(ctx, []string{"--dry-run"}); err != nil {
		t.Fatalf("DocPrune --dry-run failed: %v", err)
	}
	outStr := buf.String()
	if !strings.Contains(outStr, "Would prune:") || !strings.Contains(outStr, "orphan_abc12345.pdf") {
		t.Errorf("expected dry run to list orphan_abc12345.pdf, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "ghost_12345678.txt") {
		t.Errorf("expected dry run to list ghost_12345678.txt, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Dry run complete: 2 orphaned attachment files would be pruned.") {
		t.Errorf("expected dry run summary count 2, got:\n%s", outStr)
	}

	// Verify files still exist on disk after dry run
	if _, err := os.Stat(orphanFile); err != nil {
		t.Errorf("orphan file 1 was deleted during dry run: %v", err)
	}
	if _, err := os.Stat(orphanFile2); err != nil {
		t.Errorf("orphan file 2 was deleted during dry run: %v", err)
	}
	if _, err := os.Stat(legitFilePath); err != nil {
		t.Errorf("legitimate file was deleted during dry run: %v", err)
	}

	// Test -n shorthand
	buf.Reset()
	if err := DocPrune(ctx, []string{"-n"}); err != nil {
		t.Fatalf("DocPrune -n failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Dry run complete: 2 orphaned attachment files would be pruned.") {
		t.Errorf("expected -n to perform dry run, got:\n%s", buf.String())
	}

	// Case 4: Actual prune execution
	buf.Reset()
	if err := DocPrune(ctx, []string{}); err != nil {
		t.Fatalf("DocPrune actual run failed: %v", err)
	}
	outStr = buf.String()
	if !strings.Contains(outStr, "Pruned:") || !strings.Contains(outStr, "orphan_abc12345.pdf") {
		t.Errorf("expected actual prune to list orphan_abc12345.pdf, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Prune complete: 2 orphaned attachment files removed.") {
		t.Errorf("expected prune summary count 2, got:\n%s", outStr)
	}

	// Verify orphan files were deleted from disk
	if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
		t.Errorf("expected orphanFile to be deleted, got err: %v", err)
	}
	if _, err := os.Stat(orphanFile2); !os.IsNotExist(err) {
		t.Errorf("expected orphanFile2 to be deleted, got err: %v", err)
	}
	// Verify orphan folder 99999 was cleaned up
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("expected empty orphanDir to be removed, got err: %v", err)
	}

	// Verify legitimate file was PRESERVED
	if _, err := os.Stat(legitFilePath); err != nil {
		t.Errorf("legitimate file was deleted during prune: %v", err)
	}

	// Subsequent prune finds no orphans
	buf.Reset()
	if err := DocPrune(ctx, []string{}); err != nil {
		t.Fatalf("subsequent DocPrune failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No orphaned attachments found.") {
		t.Errorf("expected 'No orphaned attachments found.', got:\n%s", buf.String())
	}
}

func TestDocRouterAndAliases(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "router_spec.pdf")
	_ = os.WriteFile(docFile, []byte("router spec content"), 0644)

	task, err := ctx.Store.Add(ctx.Context(), "Router Doc Task", "Testing router", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	taskIDStr := fmt.Sprintf("%d", task.ID)

	// Add doc
	if err := DocAdd(ctx, []string{taskIDStr, docFile}); err != nil {
		t.Fatalf("failed to add doc: %v", err)
	}
	docs, err := ctx.Store.ListDocuments(ctx.Context(), task.ID)
	if err != nil || len(docs) == 0 {
		t.Fatalf("failed to list docs: %v", err)
	}
	docIDStr := fmt.Sprintf("%d", docs[0].ID)

	// 1. Root.Execute with "task doc open <task_id>"
	var buf bytes.Buffer
	ctx.Out = &buf
	err = Root.Execute(ctx, []string{"task", "doc", "open", taskIDStr})
	if err != nil {
		t.Fatalf("Root.Execute 'task doc open <task_id>' failed: %v", err)
	}
	if !strings.Contains(buf.String(), fmt.Sprintf("%d", task.ID)) {
		t.Errorf("expected output to contain task ID, got:\n%s", buf.String())
	}

	// 2. Root.Execute with "task doc open --doc <doc_id>"
	buf.Reset()
	err = Root.Execute(ctx, []string{"task", "doc", "open", "--doc", docIDStr})
	if err != nil {
		t.Fatalf("Root.Execute 'task doc open --doc' failed: %v", err)
	}
	if !strings.Contains(buf.String(), docs[0].FilePath) {
		t.Errorf("expected output to contain doc file path, got:\n%s", buf.String())
	}

	// 3. Root.Execute with "task doc prune --dry-run"
	buf.Reset()
	err = Root.Execute(ctx, []string{"task", "doc", "prune", "--dry-run"})
	if err != nil {
		t.Fatalf("Root.Execute 'task doc prune --dry-run' failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No orphaned attachments found.") {
		t.Errorf("expected 'No orphaned attachments found.', got:\n%s", buf.String())
	}

	// 4. Flat alias "doc-open" via cmd.Execute
	buf.Reset()
	handled, err := Execute(Root, ctx, "doc-open", []string{taskIDStr})
	if !handled || err != nil {
		t.Fatalf("Execute 'doc-open' failed (handled=%v): %v", handled, err)
	}
	if !strings.Contains(buf.String(), fmt.Sprintf("%d", task.ID)) {
		t.Errorf("expected 'doc-open' output to contain task ID, got:\n%s", buf.String())
	}

	// 5. Flat alias "doc-prune" via cmd.Execute
	buf.Reset()
	handled, err = Execute(Root, ctx, "doc-prune", []string{"--dry-run"})
	if !handled || err != nil {
		t.Fatalf("Execute 'doc-prune' failed (handled=%v): %v", handled, err)
	}
	if !strings.Contains(buf.String(), "No orphaned attachments found.") {
		t.Errorf("expected 'doc-prune' output to match, got:\n%s", buf.String())
	}
}
