package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedDB  string
		expectedFmt string
		expectedRem []string
		expectErr   bool
		errContains string
	}{
		{
			name:        "no global flags",
			args:        []string{"task", "list", "-a"},
			expectedDB:  "",
			expectedFmt: "",
			expectedRem: []string{"task", "list", "-a"},
			expectErr:   false,
		},
		{
			name:        "--db before command",
			args:        []string{"--db", "/tmp/test.db", "task", "list"},
			expectedDB:  "/tmp/test.db",
			expectedFmt: "",
			expectedRem: []string{"task", "list"},
			expectErr:   false,
		},
		{
			name:        "--db=value syntax",
			args:        []string{"--db=/tmp/eq.db", "project", "list"},
			expectedDB:  "/tmp/eq.db",
			expectedFmt: "",
			expectedRem: []string{"project", "list"},
			expectErr:   false,
		},
		{
			name:        "--db after command and positionals",
			args:        []string{"task", "add", "My Task", "--db", "/tmp/after.db"},
			expectedDB:  "/tmp/after.db",
			expectedFmt: "",
			expectedRem: []string{"task", "add", "My Task"},
			expectErr:   false,
		},
		{
			name:        "-F table before and after",
			args:        []string{"-F", "table", "task", "list"},
			expectedDB:  "",
			expectedFmt: "table",
			expectedRem: []string{"task", "list"},
			expectErr:   false,
		},
		{
			name:        "--format=table syntax",
			args:        []string{"task", "list", "--format=table"},
			expectedDB:  "",
			expectedFmt: "table",
			expectedRem: []string{"task", "list"},
			expectErr:   false,
		},
		{
			name:        "combined --db and -F flags interspersed",
			args:        []string{"task", "-F", "table", "list", "--db=/tmp/all.db", "-a"},
			expectedDB:  "/tmp/all.db",
			expectedFmt: "table",
			expectedRem: []string{"task", "list", "-a"},
			expectErr:   false,
		},
		{
			name:        "missing --db argument",
			args:        []string{"task", "list", "--db"},
			expectErr:   true,
			errContains: "flag --db requires an argument",
		},
		{
			name:        "missing -F argument",
			args:        []string{"task", "list", "-F"},
			expectErr:   true,
			errContains: "flag -F requires an argument",
		},
		{
			name:        "invalid format value",
			args:        []string{"task", "list", "-F", "invalid_fmt"},
			expectErr:   true,
			errContains: "invalid format",
		},
		{
			name:        "stop parsing flags after --",
			args:        []string{"task", "add", "--", "-F", "table"},
			expectedDB:  "",
			expectedFmt: "",
			expectedRem: []string{"task", "add", "--", "-F", "table"},
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gf, rem, err := ParseGlobalFlags(tt.args)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error %q to contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gf.DBPath != tt.expectedDB {
				t.Errorf("DBPath: expected %q, got %q", tt.expectedDB, gf.DBPath)
			}
			if gf.Format != tt.expectedFmt {
				t.Errorf("Format: expected %q, got %q", tt.expectedFmt, gf.Format)
			}
			if len(rem) != len(tt.expectedRem) {
				t.Fatalf("Remaining length: expected %d, got %d (%v vs %v)", len(tt.expectedRem), len(rem), tt.expectedRem, rem)
			}
			for i := range rem {
				if rem[i] != tt.expectedRem[i] {
					t.Errorf("Remaining[%d]: expected %q, got %q", i, tt.expectedRem[i], rem[i])
				}
			}
		})
	}
}

func TestResolveContext(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "context_test.db")

	// 1. ResolveContext with custom DB and format
	ctx, remaining, cleanup, err := ResolveContext([]string{"--db", dbPath, "-F", "table", "task", "list"})
	if err != nil {
		t.Fatalf("unexpected error in ResolveContext: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup function")
	}
	defer cleanup()

	if ctx.DBPath != dbPath {
		t.Errorf("expected DBPath %q, got %q", dbPath, ctx.DBPath)
	}
	if ctx.Format != "table" {
		t.Errorf("expected Format 'table', got %q", ctx.Format)
	}
	if ctx.Store == nil {
		t.Fatal("expected Store to be initialized in Context")
	}
	if len(remaining) != 2 || remaining[0] != "task" || remaining[1] != "list" {
		t.Fatalf("unexpected remaining arguments: %v", remaining)
	}

	// 2. ResolveContext for non-database commands like version
	ctxVer, remVer, cleanupVer, err := ResolveContext([]string{"version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleanupVer != nil {
		t.Error("expected nil cleanup for non-db command")
	}
	if ctxVer.Store != nil {
		t.Error("expected nil Store for 'version' command")
	}
	if len(remVer) != 1 || remVer[0] != "version" {
		t.Errorf("unexpected remaining args: %v", remVer)
	}

	// 3. ResolveContext for empty arguments
	ctxEmpty, remEmpty, cleanupEmpty, err := ResolveContext([]string{})
	if err != nil {
		t.Fatalf("unexpected error on empty args: %v", err)
	}
	if cleanupEmpty != nil || ctxEmpty.Store != nil || len(remEmpty) != 0 {
		t.Errorf("expected empty context and no store on empty args")
	}
}

func TestRootExecute_TransversalFlagsInjection(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()
	t.Chdir(t.TempDir())

	// 1. Add a task to check table output
	err := Root.Execute(ctx, []string{"task", "add", "Transversal Task", "Testing global flags"})
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// 2. Resolve transversal flags anywhere, then dispatch like main does.
	resolvedCtx, remaining, resolveCleanup, err := ResolveContext([]string{"--db", ctx.DBPath, "task", "-F", "table", "list"})
	if err != nil {
		t.Fatalf("failed to resolve context: %v", err)
	}
	defer resolveCleanup()

	var handled bool
	out, err := captureStdout(func() error {
		var execErr error
		handled, execErr = Execute(Root, resolvedCtx, remaining[0], remaining[1:])
		return execErr
	})
	if err != nil {
		t.Fatalf("unexpected error running 'task -F table list': %v", err)
	}
	if !handled {
		t.Fatal("expected 'task -F table list' to be handled")
	}

	// Check table headers
	if !strings.Contains(out, "PROJECT") || !strings.Contains(out, "TITLE") {
		t.Errorf("expected table format output, got:\n%s", out)
	}
	if !strings.Contains(out, "Transversal Task") {
		t.Errorf("expected task title in output, got:\n%s", out)
	}
	if resolvedCtx.Format != "table" {
		t.Errorf("expected ctx.Format to be updated to 'table', got %q", resolvedCtx.Format)
	}
}
