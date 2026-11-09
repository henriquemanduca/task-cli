package filestore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type openerFunc func(name string, args ...string) error

var (
	runOpener   openerFunc = defaultRunOpener
	currentGOOS            = runtime.GOOS
)

func defaultRunOpener(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(1 * time.Second):
		return nil
	}
}

func hasGraphicDisplay() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// OpenFolder opens a directory in the desktop file manager.
// If running in a headless environment, or if the opener command fails,
// it falls back to returning the exact absolute path with a trailing newline.
func OpenFolder(path string) (stdoutOutput string, err error) {
	return openPath(path, false)
}

// OpenFile opens or reveals a file in the desktop file manager.
// If running in a headless environment, or if the opener command fails,
// it falls back to returning the exact absolute path with a trailing newline.
func OpenFile(path string) (stdoutOutput string, err error) {
	return openPath(path, true)
}

func openPath(path string, isFile bool) (string, error) {
	if path == "" {
		return "", errors.New("path cannot be empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %q: %w", path, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("path %q not found or inaccessible: %w", absPath, err)
	}

	fallbackOutput := fmt.Sprintf("%s\n", absPath)

	switch currentGOOS {
	case "linux":
		if !hasGraphicDisplay() {
			return fallbackOutput, nil
		}
		if err := runOpener("xdg-open", absPath); err != nil {
			return fallbackOutput, nil
		}
		return "", nil

	case "darwin":
		args := []string{absPath}
		if isFile {
			args = []string{"-R", absPath}
		}
		if err := runOpener("open", args...); err != nil {
			return fallbackOutput, nil
		}
		return "", nil

	case "windows":
		target := absPath
		if isFile {
			target = "/select," + absPath
		}
		if err := runOpener("explorer.exe", target); err != nil {
			return fallbackOutput, nil
		}
		return "", nil

	default:
		if hasGraphicDisplay() {
			if err := runOpener("xdg-open", absPath); err == nil {
				return "", nil
			}
		}
		return fallbackOutput, nil
	}
}
