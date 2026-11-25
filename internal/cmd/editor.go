package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openInEditor opens path in $EDITOR, then $VISUAL, falling back to nano.
// The editor value may include arguments (e.g. "code --wait").
func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "nano"
	}
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor '%s' exited with error: %w", editor, err)
	}
	return nil
}
