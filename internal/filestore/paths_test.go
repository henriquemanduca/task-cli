package filestore

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBaseDir(t *testing.T) {
	tempDir := t.TempDir()
	fakeHome := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(fakeHome, 0755); err != nil {
		t.Fatalf("failed to create fake home: %v", err)
	}

	t.Run("host OS default", func(t *testing.T) {
		base, err := BaseDir()
		if err != nil {
			t.Fatalf("unexpected error from BaseDir(): %v", err)
		}
		if base == "" {
			t.Fatal("expected non-empty BaseDir()")
		}
		if filepath.Base(base) != "task-cli" {
			t.Errorf("expected base dir to end with 'task-cli', got %q", base)
		}
	})

	t.Run("linux with XDG_DATA_HOME set", func(t *testing.T) {
		xdgDir := filepath.Join(tempDir, "xdg_share")
		t.Setenv("XDG_DATA_HOME", xdgDir)
		t.Setenv("HOME", fakeHome)

		base, err := baseDirForOS("linux")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(xdgDir, "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})

	t.Run("linux with XDG_DATA_HOME unset fallback", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", fakeHome)

		base, err := baseDirForOS("linux")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(fakeHome, ".local", "share", "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})

	t.Run("freebsd unix fallback", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", fakeHome)

		base, err := baseDirForOS("freebsd")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(fakeHome, ".local", "share", "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})

	t.Run("darwin macOS path", func(t *testing.T) {
		t.Setenv("HOME", fakeHome)

		base, err := baseDirForOS("darwin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(fakeHome, "Library", "Application Support", "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})

	t.Run("windows with LOCALAPPDATA", func(t *testing.T) {
		winAppData := filepath.Join(tempDir, "localappdata")
		t.Setenv("LOCALAPPDATA", winAppData)
		t.Setenv("USERPROFILE", "")

		base, err := baseDirForOS("windows")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(winAppData, "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})

	t.Run("windows fallback to USERPROFILE", func(t *testing.T) {
		winProfile := filepath.Join(tempDir, "userprofile")
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("USERPROFILE", winProfile)

		base, err := baseDirForOS("windows")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(winProfile, "AppData", "Local", "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})

	t.Run("windows fallback to HOME", func(t *testing.T) {
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("USERPROFILE", "")
		t.Setenv("HOME", fakeHome)

		base, err := baseDirForOS("windows")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(fakeHome, "AppData", "Local", "task-cli")
		if base != expected {
			t.Errorf("expected %q, got %q", expected, base)
		}
	})
}

func TestDefaultDBPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	dbPath, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	baseDir, err := BaseDir()
	if err != nil {
		t.Fatalf("unexpected error from BaseDir: %v", err)
	}

	expected := filepath.Join(baseDir, "task-cli.db")
	if dbPath != expected {
		t.Errorf("expected %q, got %q", expected, dbPath)
	}
}

func TestLegacyDBPath(t *testing.T) {
	fakeHome := filepath.Join(t.TempDir(), "fakehome")
	t.Setenv("HOME", fakeHome)

	legacyPath, err := LegacyDBPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(fakeHome, ".task-cli.db")
	if legacyPath != expected {
		t.Errorf("expected %q, got %q", expected, legacyPath)
	}
}

func TestAttachmentsDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	attDir, err := AttachmentsDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	baseDir, err := BaseDir()
	if err != nil {
		t.Fatalf("unexpected error from BaseDir: %v", err)
	}

	expected := filepath.Join(baseDir, "attachments")
	if attDir != expected {
		t.Errorf("expected %q, got %q", expected, attDir)
	}
}

func TestTaskAttachmentDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	taskID := int64(42)
	attDir, err := TaskAttachmentDir(taskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	baseDir, err := BaseDir()
	if err != nil {
		t.Fatalf("unexpected error from BaseDir: %v", err)
	}

	expected := filepath.Join(baseDir, "attachments", "42")
	if attDir != expected {
		t.Errorf("expected %q, got %q", expected, attDir)
	}
}

func TestCopyLegacyDB(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "legacy.db")
	dstPath := filepath.Join(tempDir, "nested", "data", "task-cli.db")

	dummyContent := []byte("SQLite format 3\x00dummy-database-content-for-testing")
	if err := os.WriteFile(srcPath, dummyContent, 0600); err != nil {
		t.Fatalf("failed to create dummy legacy db: %v", err)
	}

	// Test successful copy with directory creation
	if err := CopyLegacyDB(srcPath, dstPath); err != nil {
		t.Fatalf("CopyLegacyDB failed: %v", err)
	}

	// Verify content integrity
	copiedContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if !bytes.Equal(copiedContent, dummyContent) {
		t.Fatalf("content mismatch: expected %q, got %q", dummyContent, copiedContent)
	}

	// Verify file permissions (on Unix-like platforms)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dstPath)
		if err != nil {
			t.Fatalf("failed to stat copied file: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0644 {
			t.Errorf("expected file mode 0644, got %o", perm)
		}
	}

	// Test copying non-existent source
	nonExistent := filepath.Join(tempDir, "missing.db")
	err = CopyLegacyDB(nonExistent, filepath.Join(tempDir, "out.db"))
	if err == nil {
		t.Fatal("expected error when source does not exist, got nil")
	}
}

func TestFinalizeLegacyDBMigration(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "legacy.db")

	content := []byte("legacy-db-content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := FinalizeLegacyDBMigration(srcPath); err != nil {
		t.Fatalf("FinalizeLegacyDBMigration failed: %v", err)
	}

	// Verify src no longer exists
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("expected original file %q to be gone, stat err: %v", srcPath, err)
	}

	// Verify migrated file exists with identical content
	migratedPath := srcPath + ".migrated"
	migratedContent, err := os.ReadFile(migratedPath)
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}
	if !bytes.Equal(migratedContent, content) {
		t.Fatalf("content mismatch in migrated file: expected %q, got %q", content, migratedContent)
	}

	// Test error on non-existent file
	err = FinalizeLegacyDBMigration(filepath.Join(tempDir, "does-not-exist.db"))
	if err == nil {
		t.Fatal("expected error when finalizing non-existent file, got nil")
	}
}
