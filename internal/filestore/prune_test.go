package filestore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPruneAttachments_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "non_existent_attachments")

	pruned, err := pruneAttachmentsDir(root, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pruned != nil {
		t.Fatalf("expected nil pruned list for non-existent directory, got %v", pruned)
	}
}

func TestPruneAttachments_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	attRoot := filepath.Join(tempDir, "attachments")

	task10Dir := filepath.Join(attRoot, "10")
	task20Dir := filepath.Join(attRoot, "20")
	if err := os.MkdirAll(task10Dir, 0755); err != nil {
		t.Fatalf("failed to create task 10 dir: %v", err)
	}
	if err := os.MkdirAll(task20Dir, 0755); err != nil {
		t.Fatalf("failed to create task 20 dir: %v", err)
	}

	active1 := filepath.Join(task10Dir, "active1.pdf")
	orphan1 := filepath.Join(task10Dir, "orphan1.txt")
	orphan2 := filepath.Join(task20Dir, "orphan2.jpg")
	orphan3 := filepath.Join(task20Dir, "orphan3.png")

	for _, path := range []string{active1, orphan1, orphan2, orphan3} {
		if err := os.WriteFile(path, []byte("attachment data"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", path, err)
		}
	}

	absActive1, _ := filepath.Abs(active1)
	absOrphan1, _ := filepath.Abs(orphan1)
	absOrphan2, _ := filepath.Abs(orphan2)
	absOrphan3, _ := filepath.Abs(orphan3)

	pruned, err := pruneAttachmentsDir(attRoot, []string{absActive1}, true)
	if err != nil {
		t.Fatalf("unexpected error during dry run: %v", err)
	}

	expectedPruned := []string{absOrphan1, absOrphan2, absOrphan3}
	if !reflect.DeepEqual(pruned, expectedPruned) {
		t.Errorf("expected pruned %v, got %v", expectedPruned, pruned)
	}

	// Verify all files and directories still exist
	for _, p := range []string{active1, orphan1, orphan2, orphan3} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file %s should not have been deleted in dry-run mode", p)
		}
	}
	for _, d := range []string{task10Dir, task20Dir} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("directory %s should not have been removed in dry-run mode", d)
		}
	}
}

func TestPruneAttachments_Execution(t *testing.T) {
	tempDir := t.TempDir()

	attRoot := filepath.Join(tempDir, "attachments")

	task10Dir := filepath.Join(attRoot, "10")
	task20Dir := filepath.Join(attRoot, "20")
	if err := os.MkdirAll(task10Dir, 0755); err != nil {
		t.Fatalf("failed to create task 10 dir: %v", err)
	}
	if err := os.MkdirAll(task20Dir, 0755); err != nil {
		t.Fatalf("failed to create task 20 dir: %v", err)
	}

	active1 := filepath.Join(task10Dir, "active1.pdf")
	orphan1 := filepath.Join(task10Dir, "orphan1.txt")
	orphan2 := filepath.Join(task20Dir, "orphan2.jpg")
	orphan3 := filepath.Join(task20Dir, "orphan3.png")

	for _, path := range []string{active1, orphan1, orphan2, orphan3} {
		if err := os.WriteFile(path, []byte("attachment data"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", path, err)
		}
	}

	absActive1, _ := filepath.Abs(active1)
	absOrphan1, _ := filepath.Abs(orphan1)
	absOrphan2, _ := filepath.Abs(orphan2)
	absOrphan3, _ := filepath.Abs(orphan3)

	pruned, err := pruneAttachmentsDir(attRoot, []string{absActive1}, false)
	if err != nil {
		t.Fatalf("unexpected error during prune execution: %v", err)
	}

	expectedPruned := []string{absOrphan1, absOrphan2, absOrphan3}
	if !reflect.DeepEqual(pruned, expectedPruned) {
		t.Errorf("expected pruned %v, got %v", expectedPruned, pruned)
	}

	// Active file must still exist
	if _, err := os.Stat(active1); err != nil {
		t.Errorf("active file %s was unexpectedly deleted", active1)
	}

	// Orphan files must be deleted
	for _, p := range []string{orphan1, orphan2, orphan3} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("orphan file %s was not deleted", p)
		}
	}

	// Directory 10 must still exist (contains active1.pdf)
	if _, err := os.Stat(task10Dir); err != nil {
		t.Errorf("directory %s should still exist as it contains active files", task10Dir)
	}

	// Directory 20 must be removed (became empty)
	if _, err := os.Stat(task20Dir); !os.IsNotExist(err) {
		t.Errorf("empty directory %s should have been removed", task20Dir)
	}
}

func TestPruneAttachments_AllOrphans(t *testing.T) {
	tempDir := t.TempDir()

	attRoot := filepath.Join(tempDir, "attachments")

	task30Dir := filepath.Join(attRoot, "30")
	if err := os.MkdirAll(task30Dir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	file1 := filepath.Join(task30Dir, "file1.txt")
	file2 := filepath.Join(task30Dir, "file2.txt")
	_ = os.WriteFile(file1, []byte("data"), 0644)
	_ = os.WriteFile(file2, []byte("data"), 0644)

	pruned, err := pruneAttachmentsDir(attRoot, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pruned) != 2 {
		t.Fatalf("expected 2 pruned files, got %d", len(pruned))
	}

	// Directory 30 should be removed
	if _, err := os.Stat(task30Dir); !os.IsNotExist(err) {
		t.Errorf("directory %s should have been removed", task30Dir)
	}

	// Root attachments directory should still exist
	if _, err := os.Stat(attRoot); err != nil {
		t.Errorf("root attachments directory %s should still exist", attRoot)
	}
}

func TestPruneAttachments_PathNormalization(t *testing.T) {
	tempDir := t.TempDir()

	attRoot := filepath.Join(tempDir, "attachments")

	task40Dir := filepath.Join(attRoot, "40")
	if err := os.MkdirAll(task40Dir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	file1 := filepath.Join(task40Dir, "active.txt")
	_ = os.WriteFile(file1, []byte("data"), 0644)

	// Pass redundant slashes and relative-style components
	unnormalizedPath := task40Dir + "/./../40//active.txt"

	pruned, err := pruneAttachmentsDir(attRoot, []string{unnormalizedPath, ""}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pruned) != 0 {
		t.Errorf("expected 0 pruned files, got %v", pruned)
	}

	if _, err := os.Stat(file1); err != nil {
		t.Errorf("active file %s should still exist", file1)
	}
}

func TestPruneAttachments_NotADirectory(t *testing.T) {
	tempDir := t.TempDir()

	filePath := filepath.Join(tempDir, "not_a_dir")
	if err := os.WriteFile(filePath, []byte("regular file"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	_, err := pruneAttachmentsDir(filePath, nil, false)
	if err == nil {
		t.Fatal("expected error when attachments root is a file, got nil")
	}
}

func TestPruneAttachments_IntegrationDefaultDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	attRoot, err := AttachmentsDir()
	if err != nil {
		t.Fatalf("AttachmentsDir failed: %v", err)
	}

	// Verify before attachments root exists
	pruned, err := PruneAttachments(nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pruned != nil {
		t.Fatalf("expected nil pruned for non-existent root, got %v", pruned)
	}

	// Create attachments root with an orphan file
	taskDir := filepath.Join(attRoot, "99")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	orphan := filepath.Join(taskDir, "orphan.dat")
	if err := os.WriteFile(orphan, []byte("orphan"), 0644); err != nil {
		t.Fatalf("failed to create orphan: %v", err)
	}

	absOrphan, _ := filepath.Abs(orphan)
	pruned, err = PruneAttachments(nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pruned) != 1 || pruned[0] != absOrphan {
		t.Errorf("expected [%s], got %v", absOrphan, pruned)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan should be deleted")
	}
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Errorf("empty task dir should be removed")
	}
}
