package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveFlatAlias_Table(t *testing.T) {
	expectedMappings := map[string][]string{
		// Projects
		"np":          {"project", "add"},
		"project-add": {"project", "add"},
		"lp":          {"project", "list"},
		"projects":    {"project", "list"},

		// Tasks
		"add":             {"task", "add"},
		"list":            {"task", "list"},
		"ls":              {"task", "list"},
		"view":            {"task", "view"},
		"show":            {"task", "view"},
		"get":             {"task", "view"},
		"update":          {"task", "update"},
		"mod":             {"task", "update"},
		"edit":            {"task", "edit"},
		"delete":          {"task", "delete"},
		"del":             {"task", "delete"},
		"rm":              {"task", "delete"},
		"remove":          {"task", "delete"},
		"status":          {"task", "status"},
		"set-status":      {"task", "status"},
		"task-status":     {"task", "status"},
		"prio":            {"task", "prio"},
		"priority":        {"task", "prio"},
		"set-prio":        {"task", "prio"},
		"set-priority":    {"task", "prio"},
		"set-pending":     {"task", "set-pending"},
		"pending":         {"task", "set-pending"},
		"set-in-progress": {"task", "set-in-progress"},
		"start":           {"task", "set-in-progress"},
		"set-done":        {"task", "set-done"},
		"done":            {"task", "set-done"},
		"set-abort":       {"task", "set-abort"},
		"abort":           {"task", "set-abort"},
		"set-aborted":     {"task", "set-abort"},
		"aborted":         {"task", "set-abort"},

		// Documents
		"attach":     {"task", "doc", "add"},
		"doc-add":    {"task", "doc", "add"},
		"docs":       {"task", "doc", "list"},
		"doc-list":   {"task", "doc", "list"},
		"doc-del":    {"task", "doc", "remove"},
		"doc-rm":     {"task", "doc", "remove"},
		"doc-remove": {"task", "doc", "remove"},
		"doc-open":   {"doc", "open"},
		"doc-prune":  {"doc", "prune"},

		// I/O
		"export": {"export"},
		"exp":    {"export"},
		"import": {"import"},
		"imp":    {"import"},
	}

	for alias, expected := range expectedMappings {
		target, ok := ResolveFlatAlias(alias)
		if !ok {
			t.Errorf("alias %q was not found in FlatAliases", alias)
			continue
		}
		if !reflect.DeepEqual(target, expected) {
			t.Errorf("alias %q: expected %v, got %v", alias, expected, target)
		}
	}

	// Verify unknown alias
	_, ok := ResolveFlatAlias("non-existent-alias")
	if ok {
		t.Error("expected false for non-existent alias")
	}
}

func TestExpandAliases(t *testing.T) {
	// 1. Empty args
	if len(ExpandAliases(nil)) != 0 {
		t.Error("expected empty result for nil args")
	}
	if len(ExpandAliases([]string{})) != 0 {
		t.Error("expected empty result for empty args")
	}

	// 2. Unknown alias untouched
	args := []string{"unknown", "foo", "bar"}
	res := ExpandAliases(args)
	if !reflect.DeepEqual(res, args) {
		t.Errorf("expected %v, got %v", args, res)
	}

	// 3. Known alias expanded
	lpArgs := []string{"lp", "-n"}
	expanded := ExpandAliases(lpArgs)
	expected := []string{"project", "list", "-n"}
	if !reflect.DeepEqual(expanded, expected) {
		t.Errorf("expected %v, got %v", expected, expanded)
	}

	// 4. Multi-level alias expanded
	attachArgs := []string{"attach", "1", "/tmp/file.pdf"}
	attachExp := ExpandAliases(attachArgs)
	expectedAttach := []string{"task", "doc", "add", "1", "/tmp/file.pdf"}
	if !reflect.DeepEqual(attachExp, expectedAttach) {
		t.Errorf("expected %v, got %v", expectedAttach, attachExp)
	}
}

func TestRootFindCommand_WithFlatAliases(t *testing.T) {
	// Direct single alias resolution
	cmdLP := Root.FindCommand("lp")
	if cmdLP == nil || cmdLP.Name != "list" {
		t.Fatalf("expected Root.FindCommand('lp') to resolve to 'list', got %v", cmdLP)
	}

	cmdAdd := Root.FindCommand("add")
	if cmdAdd == nil || cmdAdd.Name != "add" {
		t.Fatalf("expected Root.FindCommand('add') to resolve to 'add', got %v", cmdAdd)
	}

	cmdAttach := Root.FindCommand("attach")
	if cmdAttach == nil || cmdAttach.Name != "add" || cmdAttach.parent.Name != "doc" {
		t.Fatalf("expected Root.FindCommand('attach') to resolve to 'task doc add', got %v", cmdAttach)
	}
}

func TestFlatAliases_ExecutionRetrocompatibility(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. Project creation via 'np'
	err := Root.Execute(ctx, []string{"np", "PROJ", "Retro Project", "Description for retro project"})
	if err != nil {
		t.Fatalf("unexpected error running 'np': %v", err)
	}

	// 2. Project listing via 'lp'
	var lpBuf bytes.Buffer
	ctx.Out = &lpBuf
	err = Root.Execute(ctx, []string{"lp"})
	if err != nil {
		t.Fatalf("unexpected error running 'lp': %v", err)
	}
	if !strings.Contains(lpBuf.String(), "PROJ") || !strings.Contains(lpBuf.String(), "Retro Project") {
		t.Errorf("expected project in 'lp' output, got:\n%s", lpBuf.String())
	}

	// 3. Project listing via 'projects'
	var projsBuf bytes.Buffer
	ctx.Out = &projsBuf
	err = Root.Execute(ctx, []string{"projects"})
	if err != nil {
		t.Fatalf("unexpected error running 'projects': %v", err)
	}
	if !strings.Contains(projsBuf.String(), "PROJ") {
		t.Errorf("expected project in 'projects' output, got:\n%s", projsBuf.String())
	}

	// 4. Task creation via 'add'
	var addBuf bytes.Buffer
	ctx.Out = &addBuf
	err = Root.Execute(ctx, []string{"add", "-p", "1", "Retro Task", "Retro description"})
	if err != nil {
		t.Fatalf("unexpected error running 'add': %v", err)
	}
	if !strings.Contains(addBuf.String(), "Task created successfully (ID: 1) in project 'PROJ'") {
		t.Errorf("unexpected output from 'add': %s", addBuf.String())
	}

	// 5. Task listing via 'ls' and 'list'
	var lsBuf bytes.Buffer
	ctx.Out = &lsBuf
	err = Root.Execute(ctx, []string{"ls"})
	if err != nil {
		t.Fatalf("unexpected error running 'ls': %v", err)
	}
	if !strings.Contains(lsBuf.String(), "Retro Task") {
		t.Errorf("expected task in 'ls' output, got:\n%s", lsBuf.String())
	}

	// 6. Task view via 'view', 'show', 'get'
	for _, viewAlias := range []string{"view", "show", "get"} {
		var viewBuf bytes.Buffer
		ctx.Out = &viewBuf
		err = Root.Execute(ctx, []string{viewAlias, "1"})
		if err != nil {
			t.Fatalf("unexpected error running '%s': %v", viewAlias, err)
		}
		if !strings.Contains(viewBuf.String(), "Retro Task") || !strings.Contains(viewBuf.String(), "PROJ") {
			t.Errorf("expected task details in '%s' output, got:\n%s", viewAlias, viewBuf.String())
		}
	}

	// 7. Task update via 'update' and 'mod'
	err = Root.Execute(ctx, []string{"update", "1", "Updated Retro Task"})
	if err != nil {
		t.Fatalf("unexpected error running 'update': %v", err)
	}
	err = Root.Execute(ctx, []string{"mod", "1", "Mod Retro Task"})
	if err != nil {
		t.Fatalf("unexpected error running 'mod': %v", err)
	}

	// 8. Task priority via 'prio', 'priority', 'set-prio', 'set-priority'
	for _, prioAlias := range []string{"prio", "priority", "set-prio", "set-priority"} {
		err = Root.Execute(ctx, []string{prioAlias, "1", "urgent"})
		if err != nil {
			t.Fatalf("unexpected error running '%s': %v", prioAlias, err)
		}
	}

	// 9. Status transitions via shortcuts: 'start', 'set-in-progress', 'done', 'set-done', 'abort', 'set-abort', 'pending', 'set-pending'
	err = Root.Execute(ctx, []string{"start", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'start': %v", err)
	}
	t1, _ := ctx.Store.GetTaskByID(1)
	if t1.Status != "in-progress" {
		t.Fatalf("expected in-progress, got %s", t1.Status)
	}

	err = Root.Execute(ctx, []string{"done", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'done': %v", err)
	}
	t1, _ = ctx.Store.GetTaskByID(1)
	if t1.Status != "done" {
		t.Fatalf("expected done, got %s", t1.Status)
	}

	err = Root.Execute(ctx, []string{"pending", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'pending': %v", err)
	}
	t1, _ = ctx.Store.GetTaskByID(1)
	if t1.Status != "pending" {
		t.Fatalf("expected pending, got %s", t1.Status)
	}

	err = Root.Execute(ctx, []string{"abort", "1", "No longer needed"})
	if err != nil {
		t.Fatalf("unexpected error running 'abort': %v", err)
	}
	t1, _ = ctx.Store.GetTaskByID(1)
	if t1.Status != "aborted" || t1.AbortReason == nil || *t1.AbortReason != "No longer needed" {
		t.Fatalf("expected aborted with reason, got %v", t1)
	}

	// Cannot reset an aborted task to pending
	err = Root.Execute(ctx, []string{"pending", "1"})
	if err == nil {
		t.Fatal("expected error running 'pending' on aborted task, got nil")
	}

	// 10. Document management via flat aliases: 'attach', 'docs', 'doc-rm'
	tempDir := t.TempDir()
	docPath := filepath.Join(tempDir, "doc.txt")
	_ = os.WriteFile(docPath, []byte("attachment content"), 0644)

	var attachBuf bytes.Buffer
	ctx.Out = &attachBuf
	err = Root.Execute(ctx, []string{"attach", "1", docPath, "Retro Doc"})
	if err != nil {
		t.Fatalf("unexpected error running 'attach': %v", err)
	}
	if !strings.Contains(attachBuf.String(), "Document linked successfully") {
		t.Errorf("expected attachment confirmation, got: %s", attachBuf.String())
	}

	var docsBuf bytes.Buffer
	ctx.Out = &docsBuf
	err = Root.Execute(ctx, []string{"docs", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'docs': %v", err)
	}
	if !strings.Contains(docsBuf.String(), "Retro Doc") {
		t.Errorf("expected document in 'docs' output, got:\n%s", docsBuf.String())
	}

	err = Root.Execute(ctx, []string{"doc-rm", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'doc-rm': %v", err)
	}

	// 11. Task deletion via 'delete', 'del', 'rm', 'remove'
	// Add another task to delete
	err = Root.Execute(ctx, []string{"add", "Task To Delete"})
	if err != nil {
		t.Fatalf("unexpected error creating task: %v", err)
	}
	err = Root.Execute(ctx, []string{"rm", "2"})
	if err != nil {
		t.Fatalf("unexpected error deleting task with 'rm': %v", err)
	}

	// 12. cmd.Execute dispatch with flat aliases
	handled, err := Execute(ctx, "ls", []string{})
	if !handled || err != nil {
		t.Fatalf("expected Execute('ls') to succeed: handled=%v, err=%v", handled, err)
	}

	handled, err = Execute(ctx, "lp", []string{})
	if !handled || err != nil {
		t.Fatalf("expected Execute('lp') to succeed: handled=%v, err=%v", handled, err)
	}
}

func TestFlatAliases_TransversalFlags(t *testing.T) {
	tempDir := t.TempDir()
	customDB := filepath.Join(tempDir, "transversal_alias.db")

	// Create task using 'add' with --db flag
	err := Root.Execute(nil, []string{"--db", customDB, "add", "Transversal Task", "Testing --db with flat alias"})
	if err != nil {
		t.Fatalf("unexpected error creating task via flat alias with --db: %v", err)
	}

	// Verify task in custom database using 'list' with -F table
	var buf bytes.Buffer
	ctx := &Context{Out: &buf}
	err = Root.Execute(ctx, []string{"--db", customDB, "list", "-F", "table"})
	if err != nil {
		t.Fatalf("unexpected error running 'list -F table': %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "PROJECT") || !strings.Contains(out, "TITLE") {
		t.Errorf("expected table headers in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Transversal Task") {
		t.Errorf("expected task title in output, got:\n%s", out)
	}
}
