package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"task-cli/internal/db"
)

func captureStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	cmdErr := f()
	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	return out, cmdErr
}

func runWithStdin(input string, f func() error) error {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(input))
		_ = w.Close()
	}()
	defer func() {
		_ = r.Close()
		os.Stdin = oldStdin
	}()
	return f()
}

func TestExportCmd(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// Seed data
	proj, err := ctx.Store.AddProject(ctx.Context(), "TEST", "Test Project", "Test project description")
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	task1, err := ctx.Store.Add(ctx.Context(), "Active Task", "Task 1 description", &proj.ID, "high")
	if err != nil {
		t.Fatalf("failed to add task 1: %v", err)
	}
	_, err = ctx.Store.AddDocument(ctx.Context(), task1.ID, "Doc1", "pdf", "/tmp/doc1.pdf")
	if err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	task2, err := ctx.Store.Add(ctx.Context(), "Deleted Task", "Task 2 description", &proj.ID, "low")
	if err != nil {
		t.Fatalf("failed to add task 2: %v", err)
	}
	if err := ctx.Store.Delete(ctx.Context(), task2.ID); err != nil {
		t.Fatalf("failed to delete task 2: %v", err)
	}

	standalone, err := ctx.Store.Add(ctx.Context(), "Standalone Task", "Standalone description", nil, "medium")
	if err != nil {
		t.Fatalf("failed to add standalone task: %v", err)
	}

	t.Run("Export to stdout default", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return Export(ctx, []string{})
		})
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		var data db.TransferData
		if err := json.Unmarshal([]byte(out), &data); err != nil {
			t.Fatalf("failed to parse stdout as JSON: %v. Output: %s", err, out)
		}

		if data.Version != db.CurrentSchemaVersion {
			t.Errorf("expected version %d, got %d", db.CurrentSchemaVersion, data.Version)
		}
		if len(data.Projects) != 1 || data.Projects[0].Code != "TEST" {
			t.Fatalf("unexpected projects in export: %+v", data.Projects)
		}
		// Deleted task excluded by default
		if len(data.Projects[0].Tasks) != 1 || data.Projects[0].Tasks[0].Title != "Active Task" {
			t.Fatalf("unexpected tasks in project: %+v", data.Projects[0].Tasks)
		}
		if len(data.Projects[0].Tasks[0].Documents) != 1 {
			t.Fatalf("expected 1 document in task, got %d", len(data.Projects[0].Tasks[0].Documents))
		}
		if len(data.StandaloneTasks) != 1 || data.StandaloneTasks[0].ID != standalone.ID {
			t.Fatalf("unexpected standalone tasks: %+v", data.StandaloneTasks)
		}
	})

	t.Run("Export to file with --out and --pretty", func(t *testing.T) {
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "exported.json")

		out, err := captureStdout(func() error {
			return Export(ctx, []string{"--out", outPath, "--pretty"})
		})
		if err != nil {
			t.Fatalf("Export to file failed: %v", err)
		}

		if !strings.Contains(out, "Data exported successfully") {
			t.Errorf("expected success message in stdout, got: %s", out)
		}

		raw, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("failed to read exported file: %v", err)
		}

		if !strings.Contains(string(raw), "\n  \"version\":") {
			t.Errorf("expected indented JSON, got: %s", string(raw))
		}

		var data db.TransferData
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatalf("failed to unmarshal exported file: %v", err)
		}
		if len(data.Projects) != 1 {
			t.Errorf("expected 1 project in file, got %d", len(data.Projects))
		}
	})

	t.Run("Export with -f project filter", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return Export(ctx, []string{"-f", "TEST"})
		})
		if err != nil {
			t.Fatalf("Export with filter failed: %v", err)
		}

		var data db.TransferData
		if err := json.Unmarshal([]byte(out), &data); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if len(data.Projects) != 1 {
			t.Errorf("expected 1 project, got %d", len(data.Projects))
		}
		if len(data.StandaloneTasks) != 0 {
			t.Errorf("expected 0 standalone tasks when filtering by project, got %d", len(data.StandaloneTasks))
		}
	})

	t.Run("Export with --include-deleted", func(t *testing.T) {
		out, err := captureStdout(func() error {
			return Export(ctx, []string{"--include-deleted"})
		})
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		var data db.TransferData
		if err := json.Unmarshal([]byte(out), &data); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if len(data.Projects[0].Tasks) != 2 {
			t.Fatalf("expected 2 tasks in project with --include-deleted, got %d", len(data.Projects[0].Tasks))
		}
		var deletedFound bool
		for _, tsk := range data.Projects[0].Tasks {
			if tsk.ID == task2.ID {
				deletedFound = true
				if tsk.Status != db.StatusDeleted {
					t.Errorf("expected deleted task status '%s', got '%s'", db.StatusDeleted, tsk.Status)
				}
			}
		}
		if !deletedFound {
			t.Errorf("expected deleted task %d in exported tasks", task2.ID)
		}
	})

	t.Run("Export with --dry-run", func(t *testing.T) {
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "dry_run.json")

		out, err := captureStdout(func() error {
			return Export(ctx, []string{"--dry-run", "--out", outPath})
		})
		if err != nil {
			t.Fatalf("Export dry-run failed: %v", err)
		}

		if !strings.Contains(out, "[dry-run] Export:") {
			t.Errorf("expected dry-run message, got: %s", out)
		}

		if _, err := os.Stat(outPath); !os.IsNotExist(err) {
			t.Errorf("expected file not to exist in dry-run, got err: %v", err)
		}
	})

	t.Run("Export with -q quiet mode", func(t *testing.T) {
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "quiet.json")

		out, err := captureStdout(func() error {
			return Export(ctx, []string{"--out", outPath, "-q"})
		})
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("expected empty stdout in quiet mode with --out, got: %s", out)
		}
	})
}

func TestImportCmd(t *testing.T) {
	fixtureJSON := `{
		"version": 1,
		"exported_at": "2026-11-05T19:08:40Z",
		"projects": [
			{
				"code": "IMP_CLI",
				"name": "CLI Import Project",
				"description": "Original Description",
				"tasks": [
					{
						"title": "Imported Task 1",
						"description": "Task 1 desc",
						"status": "pending",
						"priority": "high",
						"documents": [
							{
								"name": "Doc A",
								"type": "txt",
								"file_path": "/path/to/docA.txt"
							}
						]
					}
				]
			}
		],
		"standalone_tasks": [
			{
				"title": "Imported Standalone",
				"description": "Standalone desc",
				"status": "done",
				"priority": "low"
			}
		]
	}`

	t.Run("Import from file argument", func(t *testing.T) {
		ctx, cleanup := setupTestContext(t)
		defer cleanup()

		tempFile := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(tempFile, []byte(fixtureJSON), 0644); err != nil {
			t.Fatalf("failed to write fixture file: %v", err)
		}

		out, err := captureStdout(func() error {
			return Import(ctx, []string{tempFile})
		})
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if !strings.Contains(out, "Importados: 1 projetos, 2 tarefas, 1 documentos.") {
			t.Errorf("unexpected output summary: %s", out)
		}

		// Verify in DB
		p, err := ctx.Store.GetProjectByCode(ctx.Context(), "IMP_CLI")
		if err != nil {
			t.Fatalf("project not found in DB: %v", err)
		}
		if p.Name != "CLI Import Project" {
			t.Errorf("unexpected project name: %s", p.Name)
		}
	})

	t.Run("Import from stdin", func(t *testing.T) {
		ctx, cleanup := setupTestContext(t)
		defer cleanup()

		var out string
		err := runWithStdin(fixtureJSON, func() error {
			var cmdErr error
			out, cmdErr = captureStdout(func() error {
				return Import(ctx, []string{"-"})
			})
			return cmdErr
		})
		if err != nil {
			t.Fatalf("Import from stdin failed: %v", err)
		}

		if !strings.Contains(out, "Importados: 1 projetos, 2 tarefas, 1 documentos.") {
			t.Errorf("unexpected output summary: %s", out)
		}
	})

	t.Run("Import with --dry-run", func(t *testing.T) {
		ctx, cleanup := setupTestContext(t)
		defer cleanup()

		tempFile := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(tempFile, []byte(fixtureJSON), 0644); err != nil {
			t.Fatalf("failed to write fixture file: %v", err)
		}

		out, err := captureStdout(func() error {
			return Import(ctx, []string{tempFile, "--dry-run"})
		})
		if err != nil {
			t.Fatalf("Import dry-run failed: %v", err)
		}

		if !strings.Contains(out, "[dry-run] Importados: 1 projetos, 2 tarefas, 1 documentos.") {
			t.Errorf("expected dry-run prefix in summary, got: %s", out)
		}

		// Project must NOT exist in DB
		_, err = ctx.Store.GetProjectByCode(ctx.Context(), "IMP_CLI")
		if !strings.Contains(err.Error(), "project not found") {
			t.Errorf("expected project not found in dry run, got: %v", err)
		}
	})

	t.Run("Import with --overwrite", func(t *testing.T) {
		ctx, cleanup := setupTestContext(t)
		defer cleanup()

		// Pre-create project with same code
		_, err := ctx.Store.AddProject(ctx.Context(), "IMP_CLI", "Old Name", "Old Description")
		if err != nil {
			t.Fatalf("failed to add pre-existing project: %v", err)
		}

		tempFile := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(tempFile, []byte(fixtureJSON), 0644); err != nil {
			t.Fatalf("failed to write fixture file: %v", err)
		}

		out, err := captureStdout(func() error {
			return Import(ctx, []string{tempFile, "--overwrite"})
		})
		if err != nil {
			t.Fatalf("Import with overwrite failed: %v", err)
		}

		if !strings.Contains(out, "Importados: 1 projetos (0 novos, 1 atualizados)") {
			t.Errorf("expected overwrite summary, got: %s", out)
		}

		p, err := ctx.Store.GetProjectByCode(ctx.Context(), "IMP_CLI")
		if err != nil {
			t.Fatalf("GetProjectByCode failed: %v", err)
		}
		if p.Name != "CLI Import Project" || p.Description != "Original Description" {
			t.Errorf("expected project to be overwritten with new name/desc, got: %s / %s", p.Name, p.Description)
		}
	})

	t.Run("Import with -q quiet mode", func(t *testing.T) {
		ctx, cleanup := setupTestContext(t)
		defer cleanup()

		tempFile := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(tempFile, []byte(fixtureJSON), 0644); err != nil {
			t.Fatalf("failed to write fixture file: %v", err)
		}

		out, err := captureStdout(func() error {
			return Import(ctx, []string{tempFile, "-q"})
		})
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if strings.TrimSpace(out) != "" {
			t.Errorf("expected empty output in quiet mode, got: %s", out)
		}
	})

	t.Run("Import non-existent file error", func(t *testing.T) {
		ctx, cleanup := setupTestContext(t)
		defer cleanup()

		err := Import(ctx, []string{"/non/existent/path.json"})
		if err == nil {
			t.Fatal("expected error for non-existent file, got nil")
		}
	})
}
