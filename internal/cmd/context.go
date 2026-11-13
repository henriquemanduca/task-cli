package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"task-cli/internal/config"
	"task-cli/internal/db"
)

// GlobalFlags holds parsed transversal flags.
type GlobalFlags struct {
	DBPath string
	Format string
}

// ParseGlobalFlags extracts transversal flags (--db, -F, --format) from args.
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

		// --db or --db=<path>
		if arg == "--db" {
			if i+1 < len(args) {
				i++
				gf.DBPath = args[i]
			} else {
				return gf, nil, errors.New("flag --db requires an argument")
			}
			continue
		}
		if strings.HasPrefix(arg, "--db=") {
			gf.DBPath = strings.TrimPrefix(arg, "--db=")
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

// ResolveStore opens the SQLite database using customPath or defaults.
func ResolveStore(customPath string) (*db.Store, string, error) {
	dbPath := customPath
	if dbPath == "" {
		var err error
		dbPath, err = db.DefaultDBPath()
		if err != nil {
			return nil, "", fmt.Errorf("failed to determine database path: %w", err)
		}
	}
	store, err := db.NewStore(dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("database error: %w", err)
	}
	return store, dbPath, nil
}

// ResolveContext unifies parsing of global flags, dependency injection, and Store setup.
// It returns an initialized Context, remaining args for dispatch, a cleanup function, and any error.
func ResolveContext(args []string) (*Context, []string, func(), error) {
	gf, remaining, err := ParseGlobalFlags(args)
	if err != nil {
		return nil, nil, nil, err
	}

	ctx := &Context{
		DBPath: gf.DBPath,
		Format: gf.Format,
		Ctx:    context.Background(),
	}

	// Commands that do not require database connection
	if len(remaining) == 0 {
		return ctx, remaining, nil, nil
	}
	cmdName := remaining[0]
	if cmdName == "version" || cmdName == "-v" || cmdName == "--version" {
		return ctx, remaining, nil, nil
	}
	if cmdName == "help" || cmdName == "-h" || cmdName == "--help" {
		return ctx, remaining, nil, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to determine working directory: %w", err)
	}
	cfg, _, err := config.LoadFromDir(cwd)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("workspace config error: %w", err)
	}
	ctx.Config = cfg

	store, resolvedPath, err := ResolveStore(gf.DBPath)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx.Store = store
	ctx.DBPath = resolvedPath

	cleanup := func() {
		if ctx.Store != nil {
			_ = ctx.Store.Close()
		}
	}

	return ctx, remaining, cleanup, nil
}
