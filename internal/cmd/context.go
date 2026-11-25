package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"task-cli/internal/config"
	"task-cli/internal/vault"
)

// GlobalFlags holds parsed transversal flags.
type GlobalFlags struct {
	VaultPath string
	Format    string
}

// ParseGlobalFlags extracts transversal flags (--vault, -F, --format) from args.
// It returns the parsed flags and the remaining arguments with transversal flags removed.
func ParseGlobalFlags(args []string) (GlobalFlags, []string, error) {
	var gf GlobalFlags
	var remaining []string
	var endOfFlags bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags {
			remaining = append(remaining, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			remaining = append(remaining, arg)
			continue
		}

		// --vault or --vault=<path> (hidden; for tests and isolated runs)
		if arg == "--vault" {
			if i+1 < len(args) {
				i++
				gf.VaultPath = args[i]
			} else {
				return gf, nil, errors.New("flag --vault requires an argument")
			}
			continue
		}
		if strings.HasPrefix(arg, "--vault=") {
			gf.VaultPath = strings.TrimPrefix(arg, "--vault=")
			continue
		}

		// -F, --format or -F=<val>, --format=<val>
		if arg == "-F" || arg == "--format" {
			if i+1 < len(args) {
				i++
				gf.Format = args[i]
			} else {
				return gf, nil, fmt.Errorf("flag %s requires an argument", arg)
			}
			continue
		}
		if strings.HasPrefix(arg, "-F=") {
			gf.Format = strings.TrimPrefix(arg, "-F=")
			continue
		}
		if strings.HasPrefix(arg, "--format=") {
			gf.Format = strings.TrimPrefix(arg, "--format=")
			continue
		}

		remaining = append(remaining, arg)
	}

	if gf.Format != "" {
		validated, err := validateFormat(gf.Format)
		if err != nil {
			return gf, nil, err
		}
		gf.Format = validated
	}

	return gf, remaining, nil
}

// ResolveVaultPath picks the vault folder: --vault flag, then TASK_CLI_VAULT,
// then the config file, then the default (~/.task-cli-vault).
func ResolveVaultPath(flagPath string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %w", err)
	}
	for _, p := range []string{flagPath, os.Getenv("TASK_CLI_VAULT")} {
		if strings.TrimSpace(p) != "" {
			return config.ExpandPath(p, cwd)
		}
	}
	cfgPath, err := config.Path()
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "", fmt.Errorf("config error: %w", err)
	}
	return cfg.Vault, nil
}

// ResolveContext parses global flags and opens the vault.
// It returns an initialized Context and the remaining args for dispatch.
func ResolveContext(args []string) (*Context, []string, error) {
	gf, remaining, err := ParseGlobalFlags(args)
	if err != nil {
		return nil, nil, err
	}

	ctx := &Context{VaultPath: gf.VaultPath, Format: gf.Format}

	// Commands that do not touch the vault (config must work even if the
	// configured vault path is broken)
	if len(remaining) == 0 {
		return ctx, remaining, nil
	}
	switch remaining[0] {
	case "version", "-v", "--version", "help", "-h", "--help", "config", "cfg":
		return ctx, remaining, nil
	}

	vaultPath, err := ResolveVaultPath(gf.VaultPath)
	if err != nil {
		return nil, nil, err
	}
	store, err := vault.Open(vaultPath)
	if err != nil {
		return nil, nil, err
	}
	ctx.Store = store
	ctx.VaultPath = vaultPath
	return ctx, remaining, nil
}
