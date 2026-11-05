package cmd

import (
	"os"
	"path/filepath"
	"testing"
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
	task, err := ctx.Store.Add("Design Architecture", "High level design", nil, "high")
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

	// 2. Successful DocAdd with inferred name and type
	if err := DocAdd(ctx, []string{"1", tempFile}); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	docs, err := ctx.Store.ListDocuments(task.ID)
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
	if docs[0].FilePath != absPath {
		t.Errorf("expected absolute path '%s', got '%s'", absPath, docs[0].FilePath)
	}

	// 3. Successful DocAdd with custom name and type via flags
	if err := DocAdd(ctx, []string{"1", tempTextFile, "--name", "Project Notes", "--type", "markdown"}); err != nil {
		t.Fatalf("failed to add second document with flags: %v", err)
	}

	docs, err = ctx.Store.ListDocuments(task.ID)
	if err != nil {
		t.Fatalf("failed to list documents: %v", err)
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

	docsAfterDelete, err := ctx.Store.ListDocuments(task.ID)
	if err != nil {
		t.Fatalf("failed to list documents after delete: %v", err)
	}
	if len(docsAfterDelete) != 1 {
		t.Fatalf("expected 1 document remaining, got %d", len(docsAfterDelete))
	}
}

