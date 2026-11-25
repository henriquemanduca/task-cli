package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"task-cli/internal/config"
)

const configTemplate = `# task-cli configuration
#
# vault: folder that holds the tasks (projects are subfolders).
#        Accepts ~, $VARS and paths relative to this file.
vault: ` + config.DefaultVault + `
`

// ConfigEdit opens the config file in the external editor, creating it from a
// template when missing, and validates it after the editor exits.
func ConfigEdit(ctx *Context, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q.\nUsage: task-cli config", args[0])
	}
	path, err := config.Path()
	if err != nil {
		return err
	}

	before, err := os.ReadFile(path)
	created := false
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating config folder: %w", err)
		}
		if err := os.WriteFile(path, []byte(configTemplate), 0o644); err != nil {
			return fmt.Errorf("creating config file: %w", err)
		}
		before, created = []byte(configTemplate), true
	} else if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := openInEditor(path); err != nil {
		return err
	}

	after, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("config file is invalid, fix it with 'task-cli config': %w", err)
	}

	w := ctx.OutWriter()
	switch {
	case created:
		fmt.Fprintf(w, "Config created: %s\n", path)
	case string(before) == string(after):
		fmt.Fprintln(w, "No changes made.")
		return nil
	default:
		fmt.Fprintf(w, "Config saved: %s\n", path)
	}
	_, err = fmt.Fprintf(w, "Vault: %s\n", cfg.Vault)
	return err
}
