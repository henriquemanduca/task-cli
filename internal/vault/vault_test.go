package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	old := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = old })
	return s
}

func TestTaskFileFormat(t *testing.T) {
	s := newStore(t)
	if _, err := s.AddProject("Task CLI", "Projeto"); err != nil {
		t.Fatal(err)
	}
	task, err := s.Add("Ajustar filtro de listagem", "Corpo **md**", "task cli", PriorityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != 1 || task.ProjectName != "Task CLI" {
		t.Fatalf("unexpected task: %+v", task)
	}

	data, err := os.ReadFile(filepath.Join(s.Root, "Task CLI", "0001-ajustar-filtro-de-listagem.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := `---
status: pending
priority: high
created: 2026-11-25T10:00:00Z
updated: 2026-11-25T10:00:00Z
---
# Ajustar filtro de listagem

Corpo **md**
`
	if string(data) != want {
		t.Errorf("unexpected file:\n%s\nwant:\n%s", data, want)
	}

	proj, err := os.ReadFile(filepath.Join(s.Root, "Task CLI", ProjectFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(proj) != "# task-cli project settings\ndescription: Projeto\n" {
		t.Errorf("unexpected project file:\n%s", proj)
	}
}

func TestHandWrittenTaskFile(t *testing.T) {
	s := newStore(t)
	if err := os.Mkdir(filepath.Join(s.Root, "WORK"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Written by hand / in Obsidian: CRLF, extra keys, missing fields.
	content := "---\r\nstatus: in-progress\r\ntags: [a, b]\r\n---\r\n\r\n# Hand made\r\n\r\nSome notes\r\n"
	path := filepath.Join(s.Root, "WORK", "0042-whatever.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Files that are not tasks are ignored.
	_ = os.WriteFile(filepath.Join(s.Root, "WORK", "notes.md"), []byte("hello"), 0o644)
	_ = os.WriteFile(filepath.Join(s.Root, "README.md"), []byte("hello"), 0o644)

	task, err := s.GetTask(42)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Hand made" || task.Description != "Some notes" || task.Status != StatusInProgress || task.Priority != PriorityMedium || task.ProjectName != "WORK" {
		t.Fatalf("unexpected parsed task: %+v", task)
	}

	if next, _ := s.Add("Next", "", "", ""); next.ID != 43 {
		t.Errorf("expected next ID 43, got %d", next.ID)
	}

	// Rewriting keeps unknown keys.
	if _, err := s.SetStatus(42, StatusDone, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Root, "WORK", "0042-hand-made.md"))
	if !strings.Contains(string(data), "tags:") || !strings.Contains(string(data), "status: done") {
		t.Errorf("expected tags preserved and status updated:\n%s", data)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected old file name to be gone")
	}
}

func TestBrokenAndDuplicateFiles(t *testing.T) {
	s := newStore(t)
	_ = os.WriteFile(filepath.Join(s.Root, "0001-a.md"), []byte("---\nstatus: [\n---\n# A\n"), 0o644)
	if _, err := s.GetTask(1); err == nil || !strings.Contains(err.Error(), "0001-a.md") {
		t.Errorf("expected parse error naming the file, got %v", err)
	}

	_ = os.WriteFile(filepath.Join(s.Root, "0002-a.md"), []byte("# A\n"), 0o644)
	_ = os.WriteFile(filepath.Join(s.Root, "0002-b.md"), []byte("# B\n"), 0o644)
	if _, err := s.GetTask(2); err == nil || !strings.Contains(err.Error(), "duplicate task ID 2") {
		t.Errorf("expected duplicate error, got %v", err)
	}

	// No frontmatter at all is fine: defaults apply.
	_ = os.WriteFile(filepath.Join(s.Root, "0003.md"), []byte("# Plain\n\nbody\n"), 0o644)
	task, err := s.GetTask(3)
	if err != nil || task.Title != "Plain" || task.Status != StatusPending {
		t.Errorf("unexpected plain task: %+v, %v", task, err)
	}
}

func TestStatusAndDelete(t *testing.T) {
	s := newStore(t)
	task, _ := s.Add("T", "", "", "")

	if _, err := s.SetStatus(task.ID, StatusAborted, "why"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus(task.ID, StatusDone, ""); !errors.Is(err, ErrTaskImmutable) {
		t.Errorf("expected immutable, got %v", err)
	}

	if _, err := s.AddProject("P", ""); err != nil {
		t.Fatal(err)
	}
	other, _ := s.Add("Other", "", "P", "")
	if err := s.Delete(other.ID); err != nil {
		t.Fatal(err)
	}
	trashed := filepath.Join(s.Root, TrashDir, "P", "0002-other.md")
	data, err := os.ReadFile(trashed)
	if err != nil {
		t.Fatalf("expected task moved to trash: %v", err)
	}
	if !strings.Contains(string(data), "status: deleted") || !strings.Contains(string(data), "deleted: ") {
		t.Errorf("expected deleted marker in trashed file:\n%s", data)
	}
	if _, err := os.Stat(other.Path); !os.IsNotExist(err) {
		t.Errorf("expected original file to be gone")
	}
	if err := s.Delete(other.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected not found deleting twice, got %v", err)
	}
	// Trash is not a project, and trashed IDs are not reused even without settings.
	if projects, _ := s.ListProjects(); len(projects) != 1 {
		t.Errorf("expected only project P, got %+v", projects)
	}
	_ = os.Remove(filepath.Join(s.Root, SettingsFile))
	if next, _ := s.Add("Next", "", "", ""); next.ID != 3 {
		t.Errorf("expected ID 3 (trashed ID 2 reserved), got %d", next.ID)
	}
	if _, err := s.GetTask(other.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected deleted task to be hidden, got %v", err)
	}
	tasks, _ := s.List("", "")
	if len(tasks) != 2 || tasks[0].ID != task.ID || tasks[1].ID != 3 {
		t.Errorf("expected aborted task and task 3 listed (not the trashed one), got %+v", tasks)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Ajustar Filtro":           "ajustar-filtro",
		"  Ação: revisão / ÇÃO!! ": "acao-revisao-cao",
		"***":                      "",
		strings.Repeat("abc ", 30): strings.TrimSuffix(strings.Repeat("abc-", 12), "-"),
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSettingsNextID(t *testing.T) {
	s := newStore(t)
	settingsPath := filepath.Join(s.Root, SettingsFile)

	// First task creates the settings file.
	a, _ := s.Add("A", "", "", "")
	b, _ := s.Add("B", "", "", "")
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("unexpected IDs %d, %d", a.ID, b.ID)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil || !strings.Contains(string(data), "next_id: 3") {
		t.Fatalf("expected next_id: 3, got %q (%v)", data, err)
	}

	// Highest file removed by hand: its ID is not reused.
	if err := os.Remove(b.Path); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Add("C", "", "", ""); c.ID != 3 {
		t.Errorf("expected ID 3 (no reuse), got %d", c.ID)
	}

	// Stale counter (file copied in by hand): the scan wins.
	_ = os.WriteFile(filepath.Join(s.Root, "0010-x.md"), []byte("# X\n"), 0o644)
	if d, _ := s.Add("D", "", "", ""); d.ID != 11 {
		t.Errorf("expected ID 11 from scan, got %d", d.ID)
	}

	// Comments and unknown keys survive; counter set by hand is honored.
	custom := "# my vault\nfuture_option: true # keep me\nnext_id: 100\n"
	_ = os.WriteFile(settingsPath, []byte(custom), 0o644)
	if e, _ := s.Add("E", "", "", ""); e.ID != 100 {
		t.Errorf("expected ID 100 from settings, got %d", e.ID)
	}
	data, _ = os.ReadFile(settingsPath)
	for _, want := range []string{"# my vault", "future_option: true # keep me", "next_id: 101"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in settings:\n%s", want, data)
		}
	}

	// Settings file is not a project nor a task.
	if projects, _ := s.ListProjects(); len(projects) != 0 {
		t.Errorf("unexpected projects: %+v", projects)
	}

	_ = os.WriteFile(settingsPath, []byte("- not\n- a map\n"), 0o644)
	if _, err := s.Add("F", "", "", ""); err == nil || !strings.Contains(err.Error(), SettingsFile) {
		t.Errorf("expected settings parse error, got %v", err)
	}
}
