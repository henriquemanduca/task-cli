// Package filestore manages centralized file storage, default directory resolution,
// attachment partitioning, and file duplication for task-cli.
package filestore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// BaseDir returns the canonical platform-specific base data directory for task-cli:
// - Linux/BSD: $XDG_DATA_HOME/task-cli (or ~/.local/share/task-cli if unset/empty)
// - macOS: ~/Library/Application Support/task-cli
// - Windows: %LOCALAPPDATA%\task-cli (or %USERPROFILE%\AppData\Local\task-cli)
func BaseDir() (string, error) {
	return baseDirForOS(runtime.GOOS)
}

func baseDirForOS(goos string) (string, error) {
	switch goos {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "task-cli"), nil

	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "task-cli"), nil
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			return filepath.Join(userProfile, "AppData", "Local", "task-cli"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(home, "AppData", "Local", "task-cli"), nil

	default: // linux, bsd, and other unix-like
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "task-cli"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "task-cli"), nil
	}
}

// DefaultDBPath returns the canonical path to the primary SQLite database
// (<baseDir>/task-cli.db).
func DefaultDBPath() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "task-cli.db"), nil
}

// LegacyDBPath returns the legacy SQLite database path (~/.task-cli.db).
func LegacyDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".task-cli.db"), nil
}

// AttachmentsDir returns the canonical base directory path for attachments (<baseDir>/attachments).
func AttachmentsDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "attachments"), nil
}

// TaskAttachmentDir returns the directory path for attachments belonging to taskID
// (<baseDir>/attachments/<taskID>).
func TaskAttachmentDir(taskID int64) (string, error) {
	attDir, err := AttachmentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(attDir, fmt.Sprintf("%d", taskID)), nil
}

// CopyLegacyDB copies the legacy SQLite database file from src to dst with fsync.
// It creates the parent directory of dst (mode 0755) if it does not exist.
func CopyLegacyDB(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create destination database: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy database content: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination database: %w", err)
	}

	if err := dstFile.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set destination database permissions: %w", err)
	}

	return nil
}

// FinalizeLegacyDBMigration renames the legacy database file to src + ".migrated".
func FinalizeLegacyDBMigration(src string) error {
	dst := src + ".migrated"
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("failed to finalize legacy migration: %w", err)
	}
	return nil
}
