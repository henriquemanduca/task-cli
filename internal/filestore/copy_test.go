package filestore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileShortHash(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("empty file sha256 prefix", func(t *testing.T) {
		emptyPath := filepath.Join(tempDir, "empty.txt")
		if err := os.WriteFile(emptyPath, nil, 0644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		hash, err := FileShortHash(emptyPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// SHA-256 of empty string is e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
		expectedPrefix := "e3b0c442"
		if hash != expectedPrefix {
			t.Errorf("expected hash prefix %q, got %q", expectedPrefix, hash)
		}
	})

	t.Run("known content sha256 prefix", func(t *testing.T) {
		contentPath := filepath.Join(tempDir, "hello.txt")
		data := []byte("Hello, task-cli attachment storage test!")
		if err := os.WriteFile(contentPath, data, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		hash, err := FileShortHash(contentPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		h := sha256.Sum256(data)
		expected := hex.EncodeToString(h[:])[:8]
		if hash != expected {
			t.Errorf("expected hash %q, got %q", expected, hash)
		}
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		_, err := FileShortHash(filepath.Join(tempDir, "non_existent.bin"))
		if err == nil {
			t.Fatal("expected error for non-existent file, got nil")
		}
	})
}

func TestCloneFileOrCopy(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source.dat")
	dstPath := filepath.Join(tempDir, "dest.dat")

	content := bytes.Repeat([]byte("Reliable CoW or fallback streaming copy verification.\n"), 100)
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := cloneFileOrCopy(srcPath, dstPath); err != nil {
		t.Fatalf("cloneFileOrCopy failed: %v", err)
	}

	copiedData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}

	if !bytes.Equal(copiedData, content) {
		t.Errorf("content mismatch between source and destination")
	}

	st, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}
	if st.Mode().Perm() != 0644 {
		t.Errorf("expected permissions 0644, got %v", st.Mode().Perm())
	}
}

func TestSaveAttachment(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	taskID := int64(42)
	expectedDestDir := filepath.Join(tempDir, "task-cli", "attachments", "42")

	t.Run("normal copy with extension and hash verification", func(t *testing.T) {
		content := []byte("Attachment test content for PDF simulation.")
		srcPath := filepath.Join(tempDir, "invoice.pdf")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		h := sha256.Sum256(content)
		shortHash := hex.EncodeToString(h[:])[:8]
		expectedFilename := fmt.Sprintf("invoice_%s.pdf", shortHash)
		expectedSavedPath := filepath.Join(expectedDestDir, expectedFilename)

		savedPath, copied, err := SaveAttachment(srcPath, taskID, false, false)
		if err != nil {
			t.Fatalf("SaveAttachment failed: %v", err)
		}

		if !copied {
			t.Errorf("expected copied=true, got false")
		}

		if savedPath != expectedSavedPath {
			t.Errorf("expected savedPath=%q, got %q", expectedSavedPath, savedPath)
		}

		savedContent, err := os.ReadFile(savedPath)
		if err != nil {
			t.Fatalf("failed to read saved attachment: %v", err)
		}
		if !bytes.Equal(savedContent, content) {
			t.Errorf("saved content does not match original source content")
		}
	})

	t.Run("normal copy without extension", func(t *testing.T) {
		content := []byte("Plain text without extension.")
		srcPath := filepath.Join(tempDir, "README")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		h := sha256.Sum256(content)
		shortHash := hex.EncodeToString(h[:])[:8]
		expectedFilename := fmt.Sprintf("README_%s", shortHash)

		savedPath, copied, err := SaveAttachment(srcPath, taskID, false, false)
		if err != nil {
			t.Fatalf("SaveAttachment failed: %v", err)
		}
		if !copied {
			t.Errorf("expected copied=true, got false")
		}
		if filepath.Base(savedPath) != expectedFilename {
			t.Errorf("expected filename %q, got %q", expectedFilename, filepath.Base(savedPath))
		}
	})

	t.Run("compound extension e.g. tar.gz", func(t *testing.T) {
		content := []byte("Tar archive payload.")
		srcPath := filepath.Join(tempDir, "bundle.tar.gz")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		h := sha256.Sum256(content)
		shortHash := hex.EncodeToString(h[:])[:8]
		expectedFilename := fmt.Sprintf("bundle.tar_%s.gz", shortHash)

		savedPath, copied, err := SaveAttachment(srcPath, taskID, false, false)
		if err != nil {
			t.Fatalf("SaveAttachment failed: %v", err)
		}
		if !copied {
			t.Errorf("expected copied=true, got false")
		}
		if filepath.Base(savedPath) != expectedFilename {
			t.Errorf("expected filename %q, got %q", expectedFilename, filepath.Base(savedPath))
		}
	})

	t.Run("hidden file e.g. .bashrc", func(t *testing.T) {
		content := []byte("export FOO=BAR\n")
		srcPath := filepath.Join(tempDir, ".bashrc")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		h := sha256.Sum256(content)
		shortHash := hex.EncodeToString(h[:])[:8]
		expectedFilename := fmt.Sprintf(".bashrc_%s", shortHash)

		savedPath, copied, err := SaveAttachment(srcPath, taskID, false, false)
		if err != nil {
			t.Fatalf("SaveAttachment failed: %v", err)
		}
		if !copied {
			t.Errorf("expected copied=true, got false")
		}
		if filepath.Base(savedPath) != expectedFilename {
			t.Errorf("expected filename %q, got %q", expectedFilename, filepath.Base(savedPath))
		}
	})

	t.Run("forceLink=true skips duplication", func(t *testing.T) {
		content := []byte("Source file to remain linked by external pointer.")
		srcPath := filepath.Join(tempDir, "external.doc")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		absSrc, _ := filepath.Abs(srcPath)
		savedPath, copied, err := SaveAttachment(srcPath, 999, false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if copied {
			t.Errorf("expected copied=false for forceLink=true")
		}
		if savedPath != absSrc {
			t.Errorf("expected savedPath=%q, got %q", absSrc, savedPath)
		}

		// Verify task 999 directory was not created
		linkTaskDir := filepath.Join(tempDir, "task-cli", "attachments", "999")
		if _, err := os.Stat(linkTaskDir); !os.IsNotExist(err) {
			t.Errorf("expected linkTaskDir to not exist, got err=%v", err)
		}
	})

	t.Run("file larger than threshold without forceCopy links by reference", func(t *testing.T) {

		content := bytes.Repeat([]byte("A"), 200)
		srcPath := filepath.Join(tempDir, "large_doc.bin")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		absSrc, _ := filepath.Abs(srcPath)
		savedPath, copied, err := saveAttachment(srcPath, 100, false, false, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if copied {
			t.Errorf("expected copied=false when size exceeds threshold without forceCopy")
		}
		if savedPath != absSrc {
			t.Errorf("expected savedPath=%q, got %q", absSrc, savedPath)
		}
	})

	t.Run("file larger than threshold with forceCopy duplicates file", func(t *testing.T) {

		content := bytes.Repeat([]byte("B"), 200)
		srcPath := filepath.Join(tempDir, "forced_large_doc.bin")
		if err := os.WriteFile(srcPath, content, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		savedPath, copied, err := saveAttachment(srcPath, 101, true, false, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !copied {
			t.Errorf("expected copied=true when forceCopy=true")
		}
		if !strings.HasPrefix(savedPath, filepath.Join(tempDir, "task-cli", "attachments", "101")) {
			t.Errorf("expected savedPath in task directory, got %q", savedPath)
		}
		copiedContent, err := os.ReadFile(savedPath)
		if err != nil {
			t.Fatalf("failed to read copied file: %v", err)
		}
		if !bytes.Equal(copiedContent, content) {
			t.Errorf("copied content mismatch")
		}
	})

	t.Run("sparse file exceeding actual 100MB MaxCopySize links by reference", func(t *testing.T) {
		sparsePath := filepath.Join(tempDir, "sparse_101mb.dat")
		f, err := os.Create(sparsePath)
		if err != nil {
			t.Fatalf("failed to create sparse file: %v", err)
		}
		if err := f.Truncate(MaxCopySize + 1024); err != nil {
			f.Close()
			t.Fatalf("failed to truncate sparse file: %v", err)
		}
		f.Close()

		absSrc, _ := filepath.Abs(sparsePath)
		savedPath, copied, err := SaveAttachment(sparsePath, 102, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if copied {
			t.Errorf("expected copied=false for >100MB without forceCopy")
		}
		if savedPath != absSrc {
			t.Errorf("expected savedPath=%q, got %q", absSrc, savedPath)
		}
	})

	t.Run("non-existent source returns error", func(t *testing.T) {
		_, _, err := SaveAttachment(filepath.Join(tempDir, "missing.file"), taskID, false, false)
		if err == nil {
			t.Fatal("expected error for non-existent source, got nil")
		}
	})

	t.Run("directory source returns error", func(t *testing.T) {
		dirPath := filepath.Join(tempDir, "subfolder")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		_, _, err := SaveAttachment(dirPath, taskID, false, false)
		if err == nil {
			t.Fatal("expected error for directory source, got nil")
		}
	})
}

func TestRemoveFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("existing file removed successfully", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "to_delete.tmp")
		if err := os.WriteFile(filePath, []byte("delete me"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		if err := RemoveFile(filePath); err != nil {
			t.Fatalf("unexpected error from RemoveFile: %v", err)
		}

		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("expected file to be deleted, stat returned: %v", err)
		}
	})

	t.Run("non-existent file returns nil without error", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "never_existed.tmp")
		if err := RemoveFile(filePath); err != nil {
			t.Errorf("expected nil for non-existent file, got %v", err)
		}
	})

	t.Run("empty string returns nil without error", func(t *testing.T) {
		if err := RemoveFile(""); err != nil {
			t.Errorf("expected nil for empty filePath, got %v", err)
		}
	})
}
