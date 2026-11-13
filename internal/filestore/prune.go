package filestore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PruneAttachments scans the centralized attachments repository (<data_root>/attachments/),
// identifies orphaned files that are not referenced in activePaths, and deletes them
// unless dryRun is true.
//
// When dryRun is false, it also removes empty task directories (<data_root>/attachments/<task_id>/)
// left behind after file deletion.
//
// Returns the list of pruned file paths.
func PruneAttachments(activePaths []string, dryRun bool) (pruned []string, err error) {
	root, err := AttachmentsDir()
	if err != nil {
		return nil, err
	}
	return pruneAttachmentsDir(root, activePaths, dryRun)
}

func pruneAttachmentsDir(attachmentsRoot string, activePaths []string, dryRun bool) ([]string, error) {
	st, err := os.Stat(attachmentsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to access attachments directory: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("attachments root %q is not a directory", attachmentsRoot)
	}

	activeMap := make(map[string]bool, len(activePaths))
	for _, p := range activePaths {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = filepath.Clean(p)
		} else {
			abs = filepath.Clean(abs)
		}
		activeMap[abs] = true
	}

	var pruned []string

	err = filepath.Walk(attachmentsRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		absFile, err := filepath.Abs(path)
		if err != nil {
			absFile = filepath.Clean(path)
		} else {
			absFile = filepath.Clean(absFile)
		}

		if !activeMap[absFile] {
			pruned = append(pruned, absFile)
			if !dryRun {
				if err := os.Remove(absFile); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to remove orphan attachment %q: %w", absFile, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !dryRun {
		entries, err := os.ReadDir(attachmentsRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachments directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			taskDir := filepath.Join(attachmentsRoot, entry.Name())
			subEntries, err := os.ReadDir(taskDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read task attachment directory %q: %w", taskDir, err)
			}
			if len(subEntries) != 0 {
				continue
			}
			if err := os.Remove(taskDir); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to remove empty task directory %q: %w", taskDir, err)
			}
		}
	}

	sort.Strings(pruned)
	return pruned, nil
}
