package cmd

import (
	"strings"
	"testing"
)

func TestFormatValidation(t *testing.T) {
	tests := []struct {
		input       string
		expectedFmt string
		shouldErr   bool
	}{
		{"", FormatTSV, false},
		{"tsv", FormatTSV, false},
		{"TSV", FormatTSV, false},
		{" table ", FormatTable, false},
		{"TABLE", FormatTable, false},
		{"json", "", true},
		{"xml", "", true},
	}

	for _, tt := range tests {
		res, err := validateFormat(tt.input)
		if tt.shouldErr && err == nil {
			t.Errorf("validateFormat(%q) expected error, got nil", tt.input)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("validateFormat(%q) unexpected error: %v", tt.input, err)
		}
		if res != tt.expectedFmt {
			t.Errorf("validateFormat(%q) = %q; expected %q", tt.input, res, tt.expectedFmt)
		}
	}
}

func TestTaskListTableFormat(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	_ = ProjectAdd(ctx, []string{"PROJ", "My Project", "Project description"})
	_ = TaskAdd(ctx, []string{"-p", "1", "First Task", "Description 1"})
	_ = TaskAdd(ctx, []string{"-p", "1", "Second Task", "Description 2"})

	// 1. Test table format output
	out, err := captureStdout(func() error {
		return TaskList(ctx, []string{"-F", "table"})
	})
	if err != nil {
		t.Fatalf("unexpected error with -F table: %v", err)
	}

	if !strings.Contains(out, "ID") || !strings.Contains(out, "PROJECT") || !strings.Contains(out, "TITLE") {
		t.Errorf("expected uppercase table headers in output, got:\n%s", out)
	}
	if !strings.Contains(out, "First Task") || !strings.Contains(out, "Second Task") {
		t.Errorf("expected task titles in table output, got:\n%s", out)
	}
	if !strings.Contains(out, "0001") || !strings.Contains(out, "0002") {
		t.Errorf("expected 4-digit formatted task IDs in table output, got:\n%s", out)
	}

	// 2. Test table format with no-header
	outNoHeader, err := captureStdout(func() error {
		return TaskList(ctx, []string{"--format", "table", "-n"})
	})
	if err != nil {
		t.Fatalf("unexpected error with --format table -n: %v", err)
	}
	if strings.Contains(outNoHeader, "PROJECT") {
		t.Errorf("expected no header in output, got:\n%s", outNoHeader)
	}
	if !strings.Contains(outNoHeader, "First Task") {
		t.Errorf("expected tasks in no-header table output, got:\n%s", outNoHeader)
	}

	// 3. Test invalid format
	err = TaskList(ctx, []string{"--format", "unknown"})
	if err == nil {
		t.Fatal("expected error with invalid format, got nil")
	}

	// 4. Test default format is TSV (contains tab)
	outTSV, err := captureStdout(func() error {
		return TaskList(ctx, []string{})
	})
	if err != nil {
		t.Fatalf("unexpected error with default format: %v", err)
	}
	if !strings.Contains(outTSV, "id\tproject\tstatus\tpriority\ttitle") {
		t.Errorf("expected TSV header row with tabs, got:\n%s", outTSV)
	}
}

func TestProjectListTableFormat(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	_ = ProjectAdd(ctx, []string{"P1", "Project One", "Description 1"})
	_ = ProjectAdd(ctx, []string{"P2", "Project Two", "Description 2"})

	// 1. Test table format output
	out, err := captureStdout(func() error {
		return ProjectList(ctx, []string{"-F", "table", "-d"})
	})
	if err != nil {
		t.Fatalf("unexpected error with -F table: %v", err)
	}

	if !strings.Contains(out, "ID") || !strings.Contains(out, "CODE") || !strings.Contains(out, "NAME") || !strings.Contains(out, "DESCRIPTION") {
		t.Errorf("expected uppercase headers in table output, got:\n%s", out)
	}
	if !strings.Contains(out, "P1") || !strings.Contains(out, "Project Two") {
		t.Errorf("expected project rows in table output, got:\n%s", out)
	}

	// 2. Test table format with -n (no header)
	outNoHeader, err := captureStdout(func() error {
		return ProjectList(ctx, []string{"--format", "table", "-n"})
	})
	if err != nil {
		t.Fatalf("unexpected error with --format table -n: %v", err)
	}
	if strings.Contains(outNoHeader, "CODE") {
		t.Errorf("expected no header in output, got:\n%s", outNoHeader)
	}

	// 3. Test invalid format
	err = ProjectList(ctx, []string{"-F", "yaml"})
	if err == nil {
		t.Fatal("expected error with invalid format on ProjectList, got nil")
	}
}
