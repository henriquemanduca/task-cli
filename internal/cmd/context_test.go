package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedVault string
		expectedFmt   string
		expectedRem   []string
		expectErr     bool
		errContains   string
	}{
		{
			name:          "no global flags",
			args:          []string{"task", "list", "-a"},
			expectedVault: "",
			expectedFmt:   "",
			expectedRem:   []string{"task", "list", "-a"},
			expectErr:     false,
		},
		{
			name:          "--vault before command",
			args:          []string{"--vault", "/tmp/test-vault", "task", "list"},
			expectedVault: "/tmp/test-vault",
			expectedFmt:   "",
			expectedRem:   []string{"task", "list"},
			expectErr:     false,
		},
		{
			name:          "--vault=value syntax",
			args:          []string{"--vault=/tmp/eq-vault", "project", "list"},
			expectedVault: "/tmp/eq-vault",
			expectedFmt:   "",
			expectedRem:   []string{"project", "list"},
			expectErr:     false,
		},
		{
			name:          "--vault after command and positionals",
			args:          []string{"task", "add", "My Task", "--vault", "/tmp/after-vault"},
			expectedVault: "/tmp/after-vault",
			expectedFmt:   "",
			expectedRem:   []string{"task", "add", "My Task"},
			expectErr:     false,
		},
		{
			name:          "-F table before and after",
			args:          []string{"-F", "table", "task", "list"},
			expectedVault: "",
			expectedFmt:   "table",
			expectedRem:   []string{"task", "list"},
			expectErr:     false,
		},
		{
			name:          "--format=table syntax",
			args:          []string{"task", "list", "--format=table"},
			expectedVault: "",
			expectedFmt:   "table",
			expectedRem:   []string{"task", "list"},
			expectErr:     false,
		},
		{
			name:          "combined --vault and -F flags interspersed",
			args:          []string{"task", "-F", "table", "list", "--vault=/tmp/all-vault", "-a"},
			expectedVault: "/tmp/all-vault",
			expectedFmt:   "table",
			expectedRem:   []string{"task", "list", "-a"},
			expectErr:     false,
		},
		{
			name:        "missing --vault argument",
			args:        []string{"task", "list", "--vault"},
			expectErr:   true,
			errContains: "flag --vault requires an argument",
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
			name:          "stop parsing flags after --",
			args:          []string{"task", "add", "--", "-F", "table"},
			expectedVault: "",
			expectedFmt:   "",
			expectedRem:   []string{"task", "add", "--", "-F", "table"},
			expectErr:     false,
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
			if gf.VaultPath != tt.expectedVault {
				t.Errorf("VaultPath: expected %q, got %q", tt.expectedVault, gf.VaultPath)
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
	vaultPath := filepath.Join(t.TempDir(), "vault")

	// 1. ResolveContext with custom vault and format
	ctx, remaining, err := ResolveContext([]string{"--vault", vaultPath, "-F", "table", "task", "list"})
	if err != nil {
		t.Fatalf("unexpected error in ResolveContext: %v", err)
	}
	if ctx.VaultPath != vaultPath {
		t.Errorf("expected VaultPath %q, got %q", vaultPath, ctx.VaultPath)
	}
	if ctx.Format != "table" {
		t.Errorf("expected Format 'table', got %q", ctx.Format)
	}
	if ctx.Store == nil {
		t.Fatal("expected Store to be initialized in Context")
	}
	if info, err := os.Stat(vaultPath); err != nil || !info.IsDir() {
		t.Fatalf("expected vault folder to be created: %v", err)
	}
	if len(remaining) != 2 || remaining[0] != "task" || remaining[1] != "list" {
		t.Fatalf("unexpected remaining arguments: %v", remaining)
	}

	// 2. version does not open the vault
	ctxVer, remVer, err := ResolveContext([]string{"version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctxVer.Store != nil {
		t.Error("expected nil Store for 'version' command")
	}
	if len(remVer) != 1 || remVer[0] != "version" {
		t.Errorf("unexpected remaining args: %v", remVer)
	}

	// 3. empty arguments
	ctxEmpty, remEmpty, err := ResolveContext([]string{})
	if err != nil {
		t.Fatalf("unexpected error on empty args: %v", err)
	}
	if ctxEmpty.Store != nil || len(remEmpty) != 0 {
		t.Errorf("expected empty context and no store on empty args")
	}
}

func TestResolveVaultPathPrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("vault: from-config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_CLI_CONFIG", cfgPath)
	t.Setenv("TASK_CLI_VAULT", "")

	got, err := ResolveVaultPath("")
	if err != nil || got != filepath.Join(dir, "from-config") {
		t.Fatalf("config: got %q, %v", got, err)
	}

	t.Setenv("TASK_CLI_VAULT", filepath.Join(dir, "from-env"))
	got, err = ResolveVaultPath("")
	if err != nil || got != filepath.Join(dir, "from-env") {
		t.Fatalf("env: got %q, %v", got, err)
	}

	got, err = ResolveVaultPath(filepath.Join(dir, "from-flag"))
	if err != nil || got != filepath.Join(dir, "from-flag") {
		t.Fatalf("flag: got %q, %v", got, err)
	}

	if err := os.WriteFile(cfgPath, []byte("vault: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_CLI_VAULT", "")
	if _, err := ResolveVaultPath(""); err == nil || !strings.Contains(err.Error(), "config error") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestRootExecute_TransversalFlagsInjection(t *testing.T) {
	ctx := setupTestContext(t)

	// 1. Add a task to check table output
	err := Root.Execute(ctx, []string{"task", "add", "Transversal Task", "Testing global flags"})
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// 2. Resolve transversal flags anywhere, then dispatch like main does.
	resolvedCtx, remaining, err := ResolveContext([]string{"--vault", ctx.VaultPath, "task", "-F", "table", "list"})
	if err != nil {
		t.Fatalf("failed to resolve context: %v", err)
	}
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
