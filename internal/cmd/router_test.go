package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRouter_SimpleAndLeafDispatch(t *testing.T) {
	executed := false
	var capturedArgs []string

	root := &Command{Name: "app"}
	root.AddCommand(&Command{
		Name:        "simple",
		Description: "Simple leaf command",
		Run: func(ctx *Context, args []string) error {
			executed = true
			capturedArgs = args
			return nil
		},
	})

	err := root.Execute(nil, []string{"simple", "arg1", "arg2"})
	if err != nil {
		t.Fatalf("unexpected error executing simple command: %v", err)
	}
	if !executed {
		t.Fatal("expected simple command to be executed")
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "arg1" || capturedArgs[1] != "arg2" {
		t.Fatalf("expected args [arg1 arg2], got %v", capturedArgs)
	}
}

func TestRouter_MultiLevelDispatch(t *testing.T) {
	var executedPath string
	var capturedArgs []string

	root := &Command{Name: "app"}
	parent := &Command{
		Name:        "group",
		Description: "Command group",
	}
	child := &Command{
		Name:        "subgroup",
		Description: "Nested subgroup",
	}
	leaf := &Command{
		Name:        "run",
		Description: "Executable leaf",
		Run: func(ctx *Context, args []string) error {
			executedPath = "app group subgroup run"
			capturedArgs = args
			return nil
		},
	}

	child.AddCommand(leaf)
	parent.AddCommand(child)
	root.AddCommand(parent)

	err := root.Execute(nil, []string{"group", "subgroup", "run", "--flag", "val", "pos1"})
	if err != nil {
		t.Fatalf("unexpected error in multi-level dispatch: %v", err)
	}
	if executedPath != "app group subgroup run" {
		t.Fatalf("expected execution path %q, got %q", "app group subgroup run", executedPath)
	}
	if len(capturedArgs) != 3 || capturedArgs[0] != "--flag" || capturedArgs[1] != "val" || capturedArgs[2] != "pos1" {
		t.Fatalf("unexpected captured args: %v", capturedArgs)
	}
}

func TestRouter_AliasResolution(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectedPath string
	}{
		{
			name:         "root level alias",
			args:         []string{"t", "sub", "leaf"},
			expectedPath: "task sub leaf",
		},
		{
			name:         "intermediate level alias",
			args:         []string{"task", "s", "leaf"},
			expectedPath: "task sub leaf",
		},
		{
			name:         "leaf level alias",
			args:         []string{"task", "sub", "l"},
			expectedPath: "task sub leaf",
		},
		{
			name:         "all aliases combined",
			args:         []string{"t", "s", "exec"},
			expectedPath: "task sub leaf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var recorded string
			root := &Command{Name: "app"}
			taskCmd := &Command{
				Name:    "task",
				Aliases: []string{"t", "tsk"},
			}
			subCmd := &Command{
				Name:    "sub",
				Aliases: []string{"s"},
			}
			leafCmd := &Command{
				Name:    "leaf",
				Aliases: []string{"l", "exec"},
				Run: func(ctx *Context, args []string) error {
					recorded = "task sub leaf"
					return nil
				},
			}

			subCmd.AddCommand(leafCmd)
			taskCmd.AddCommand(subCmd)
			root.AddCommand(taskCmd)

			err := root.Execute(nil, tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if recorded != tc.expectedPath {
				t.Fatalf("expected path %q, got %q", tc.expectedPath, recorded)
			}
		})
	}
}

func TestRouter_RemainingArguments(t *testing.T) {
	var capturedArgs []string
	root := &Command{Name: "app"}
	cmd := &Command{
		Name: "do",
		Run: func(ctx *Context, args []string) error {
			capturedArgs = args
			return nil
		},
	}
	root.AddCommand(cmd)

	inputArgs := []string{"do", "-p", "PRJ", "-a", "--format", "json", "task title", "task desc"}
	err := root.Execute(nil, inputArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"-p", "PRJ", "-a", "--format", "json", "task title", "task desc"}
	if len(capturedArgs) != len(expected) {
		t.Fatalf("expected %d args, got %d (%v)", len(expected), len(capturedArgs), capturedArgs)
	}
	for i := range expected {
		if capturedArgs[i] != expected[i] {
			t.Errorf("arg[%d] expected %q, got %q", i, expected[i], capturedArgs[i])
		}
	}
}

func TestRouter_UnknownCommands(t *testing.T) {
	root := &Command{Name: "app"}
	parent := &Command{
		Name: "parent",
	}
	parent.AddCommand(&Command{
		Name: "child",
		Run: func(ctx *Context, args []string) error {
			return nil
		},
	})
	root.AddCommand(parent)

	// Unknown at root level
	err := root.Execute(nil, []string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown root command, got nil")
	}
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("expected ErrUnknownCommand, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected error message to contain 'unknown', got: %v", err)
	}

	// Unknown subcommand under parent
	err = root.Execute(nil, []string{"parent", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("expected ErrUnknownCommand, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected error message to contain 'nonexistent', got: %v", err)
	}
}

func TestRouter_HelpHandling(t *testing.T) {
	var out bytes.Buffer
	root := &Command{
		Name:        "task-cli",
		Description: "Task CLI manager",
	}
	root.SetOutput(&out)

	projectCmd := &Command{
		Name:        "project",
		Aliases:     []string{"proj"},
		Description: "Manage projects",
	}
	addCmd := &Command{
		Name:        "add",
		Aliases:     []string{"create"},
		Description: "Create a new project",
		Run: func(ctx *Context, args []string) error {
			return nil
		},
	}
	projectCmd.AddCommand(addCmd)
	root.AddCommand(projectCmd)

	// 1. Invoking parent node without subcommands prints help
	out.Reset()
	err := root.Execute(nil, []string{"project"})
	if err != nil {
		t.Fatalf("unexpected error on parent without subcommand: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Usage:") || !strings.Contains(output, "Manage projects") {
		t.Fatalf("expected parent help output, got:\n%s", output)
	}
	if !strings.Contains(output, "add, create") || !strings.Contains(output, "Create a new project") {
		t.Fatalf("expected subcommands listed in help, got:\n%s", output)
	}

	// 2. Invoking with --help flag
	out.Reset()
	err = root.Execute(nil, []string{"project", "--help"})
	if err != nil {
		t.Fatalf("unexpected error with --help: %v", err)
	}
	if !strings.Contains(out.String(), "Manage projects") {
		t.Fatalf("expected help on --help, got:\n%s", out.String())
	}

	// 3. Invoking with -h flag
	out.Reset()
	err = root.Execute(nil, []string{"project", "-h"})
	if err != nil {
		t.Fatalf("unexpected error with -h: %v", err)
	}
	if !strings.Contains(out.String(), "Manage projects") {
		t.Fatalf("expected help on -h, got:\n%s", out.String())
	}

	// 4. Invoking "help <subcommand>"
	out.Reset()
	err = root.Execute(nil, []string{"help", "project"})
	if err != nil {
		t.Fatalf("unexpected error with help project: %v", err)
	}
	if !strings.Contains(out.String(), "Manage projects") {
		t.Fatalf("expected help on 'help project', got:\n%s", out.String())
	}

	// 5. Help for leaf command
	out.Reset()
	err = root.Execute(nil, []string{"project", "add", "--help"})
	if err != nil {
		t.Fatalf("unexpected error on leaf --help: %v", err)
	}
	if !strings.Contains(out.String(), "Create a new project") {
		t.Fatalf("expected leaf help, got:\n%s", out.String())
	}
}

func TestRouter_HandlerErrorPropagation(t *testing.T) {
	expectedErr := errors.New("custom failure")
	root := &Command{Name: "app"}
	root.AddCommand(&Command{
		Name: "fail",
		Run: func(ctx *Context, args []string) error {
			return expectedErr
		},
	})

	err := root.Execute(nil, []string{"fail"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestRouter_FindCommandAndPath(t *testing.T) {
	root := &Command{Name: "app"}
	p := &Command{Name: "p"}
	c := &Command{Name: "c"}
	l := &Command{Name: "l", Aliases: []string{"leaf"}}

	c.AddCommand(l)
	p.AddCommand(c)
	root.AddCommand(p)

	found := root.FindCommand("p", "c", "leaf")
	if found != l {
		t.Fatalf("expected to find leaf command, got %v", found)
	}

	if l.CommandPath() != "app p c l" {
		t.Fatalf("expected command path %q, got %q", "app p c l", l.CommandPath())
	}

	if root.FindCommand("p", "unknown") != nil {
		t.Fatal("expected nil for nonexistent path")
	}
}

func TestRouter_DefaultRootAndCmdExecute(t *testing.T) {
	if Root == nil {
		t.Fatal("expected Root to be non-nil")
	}

	// Verify top-level commands in Root
	expectedCmds := []string{
		"project", "task",
		"doc-add", "doc-list", "doc-del", "export", "import",
	}

	for _, name := range expectedCmds {
		sub := Root.FindSubcommand(name)
		if sub == nil {
			t.Errorf("expected command %q in default Root", name)
		}
	}

	// Verify aliases in Root
	expectedAliases := map[string]string{
		"proj":       "project",
		"t":          "task",
		"tsk":        "task",
		"attach":     "doc-add",
		"docs":       "doc-list",
		"doc-rm":     "doc-del",
		"doc-remove": "doc-del",
		"exp":        "export",
		"imp":        "import",
	}

	for alias, canonical := range expectedAliases {
		sub := Root.FindSubcommand(alias)
		if sub == nil {
			t.Errorf("expected alias %q to be found in default Root", alias)
		} else if sub.Name != canonical {
			t.Errorf("alias %q resolved to %q, expected %q", alias, sub.Name, canonical)
		}
	}

	// Verify project subcommands
	projectNode := Root.FindSubcommand("project")
	if projectNode == nil {
		t.Fatal("expected 'project' command in Root")
	}
	if projectNode.FindSubcommand("add") == nil || projectNode.FindSubcommand("create") == nil {
		t.Errorf("expected 'add' and 'create' under project")
	}
	if projectNode.FindSubcommand("list") == nil || projectNode.FindSubcommand("ls") == nil {
		t.Errorf("expected 'list' and 'ls' under project")
	}
	if projectNode.FindSubcommand("view") == nil || projectNode.FindSubcommand("show") == nil {
		t.Errorf("expected 'view' and 'show' under project")
	}

	// Verify task subcommands and aliases
	taskNode := Root.FindSubcommand("task")
	if taskNode == nil {
		t.Fatal("expected 'task' command in Root")
	}
	expectedTaskSub := []string{"add", "list", "view", "update", "edit", "delete", "status", "prio"}
	for _, sub := range expectedTaskSub {
		if taskNode.FindSubcommand(sub) == nil {
			t.Errorf("expected subcommand %q under 'task'", sub)
		}
	}
	if taskNode.FindSubcommand("create") == nil || taskNode.FindSubcommand("new") == nil {
		t.Errorf("expected 'create' and 'new' aliases under task")
	}
	if taskNode.FindSubcommand("ls") == nil {
		t.Errorf("expected 'ls' alias under task")
	}
	if taskNode.FindSubcommand("show") == nil || taskNode.FindSubcommand("get") == nil {
		t.Errorf("expected 'show' and 'get' aliases under task")
	}
	if taskNode.FindSubcommand("set-status") == nil {
		t.Errorf("expected 'set-status' alias under task")
	}
	if taskNode.FindSubcommand("priority") == nil || taskNode.FindSubcommand("set-prio") == nil {
		t.Errorf("expected 'priority' and 'set-prio' aliases under task")
	}
	if taskNode.FindSubcommand("done") == nil || taskNode.FindSubcommand("start") == nil || taskNode.FindSubcommand("abort") == nil {
		t.Errorf("expected 'done', 'start', 'abort' subcommands under task")
	}

	docNode := taskNode.FindSubcommand("doc")
	if docNode == nil {
		t.Fatal("expected 'doc' subcommand under task")
	}
	if taskNode.FindSubcommand("docs") == nil || taskNode.FindSubcommand("d") == nil {
		t.Errorf("expected 'docs' and 'd' aliases for 'doc' under task")
	}
	for _, sub := range []string{"add", "list", "remove"} {
		if docNode.FindSubcommand(sub) == nil {
			t.Errorf("expected subcommand %q under 'task doc'", sub)
		}
	}
	if docNode.FindSubcommand("attach") == nil {
		t.Errorf("expected 'attach' alias under 'task doc'")
	}
	if docNode.FindSubcommand("ls") == nil {
		t.Errorf("expected 'ls' alias under 'task doc'")
	}
	if docNode.FindSubcommand("rm") == nil || docNode.FindSubcommand("del") == nil || docNode.FindSubcommand("delete") == nil {
		t.Errorf("expected 'rm', 'del', 'delete' aliases under 'task doc'")
	}

	// Test Execute with unknown command
	handled, err := Execute(nil, "nonexistent-command-xyz", []string{})
	if handled {
		t.Fatal("expected handled=false for unknown command")
	}
	if err != nil {
		t.Fatalf("expected nil err for unknown command, got %v", err)
	}
}

func TestRouter_TaskDomainTree(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. task add via Root dispatch
	err := Root.Execute(ctx, []string{"task", "add", "Tree Root Task", "Tree Description", "--priority", "low"})
	if err != nil {
		t.Fatalf("unexpected error running 'task add': %v", err)
	}

	// 2. task list via alias 't ls'
	out, err := captureStdout(func() error {
		return Root.Execute(ctx, []string{"t", "ls"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 't ls': %v", err)
	}
	if !strings.Contains(out, "Tree Root Task") {
		t.Fatalf("expected task in list output, got:\n%s", out)
	}

	// 3. task view via alias 'tsk show'
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"tsk", "show", "1"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'tsk show 1': %v", err)
	}
	if !strings.Contains(out, "Tree Root Task") || !strings.Contains(out, "Tree Description") {
		t.Fatalf("expected task details in show output, got:\n%s", out)
	}

	// 4. task update
	err = Root.Execute(ctx, []string{"task", "update", "1", "Updated Title", "Updated Description"})
	if err != nil {
		t.Fatalf("unexpected error running 'task update': %v", err)
	}

	// 5. task prio
	err = Root.Execute(ctx, []string{"task", "prio", "1", "urgent"})
	if err != nil {
		t.Fatalf("unexpected error running 'task prio': %v", err)
	}
	task, err := ctx.Store.GetTaskByID(1)
	if err != nil || task.Priority != "urgent" {
		t.Fatalf("expected priority urgent, got %v (err: %v)", task.Priority, err)
	}

	// 6. task status query
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"task", "status", "1"})
	})
	if err != nil {
		t.Fatalf("unexpected error querying task status: %v", err)
	}
	if !strings.Contains(out, "Task 0001 status: pending") {
		t.Fatalf("expected pending status, got:\n%s", out)
	}

	// 7. task status set to in-progress
	err = Root.Execute(ctx, []string{"task", "status", "1", "in-progress"})
	if err != nil {
		t.Fatalf("unexpected error setting in-progress: %v", err)
	}
	task, err = ctx.Store.GetTaskByID(1)
	if err != nil || task.Status != "in-progress" {
		t.Fatalf("expected status in-progress, got %v", task.Status)
	}

	// 8. task status set to abort with reason
	err = Root.Execute(ctx, []string{"task", "status", "1", "abort", "Requirements changed"})
	if err != nil {
		t.Fatalf("unexpected error setting abort: %v", err)
	}
	task, err = ctx.Store.GetTaskByID(1)
	if err != nil || task.Status != "aborted" {
		t.Fatalf("expected status aborted, got %v", task.Status)
	}
	if task.AbortReason == nil || *task.AbortReason != "Requirements changed" {
		t.Fatalf("expected abort reason 'Requirements changed', got %v", task.AbortReason)
	}

	// 9. task done helper
	err = Root.Execute(ctx, []string{"task", "done", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'task done': %v", err)
	}
	task, err = ctx.Store.GetTaskByID(1)
	if err != nil || task.Status != "done" {
		t.Fatalf("expected status done, got %v", task.Status)
	}

	// 10. task delete
	err = Root.Execute(ctx, []string{"task", "delete", "1"})
	if err != nil {
		t.Fatalf("unexpected error running 'task delete': %v", err)
	}

	// 11. task without args prints help
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"task"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'task': %v", err)
	}
	if !strings.Contains(out, "Manage tasks") || !strings.Contains(out, "Available commands:") {
		t.Fatalf("expected help output for 'task', got:\n%s", out)
	}

	// 12. Execute dispatch via top-level command "task"
	handled, err := Execute(ctx, "task", []string{"list"})
	if !handled || err != nil {
		t.Fatalf("expected handled=true, err=nil for Execute('task', ['list']), got handled=%v, err=%v", handled, err)
	}

	// 13. Legacy flat commands are handled and routed via flat aliases
	handled, err = Execute(ctx, "add", []string{"Flat Add Task"})
	if !handled || err != nil {
		t.Fatalf("expected handled=true, err=nil for legacy flat 'add', got handled=%v, err=%v", handled, err)
	}

	handled, err = Execute(ctx, "list", []string{})
	if !handled || err != nil {
		t.Fatalf("expected handled=true, err=nil for legacy flat 'list', got handled=%v, err=%v", handled, err)
	}

	handled, err = Execute(ctx, "projects", []string{})
	if !handled || err != nil {
		t.Fatalf("expected handled=true, err=nil for legacy flat 'projects', got handled=%v, err=%v", handled, err)
	}
}

func TestRouter_TaskDocSubcommands(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	tempDir := t.TempDir()
	docFile1 := tempDir + "/spec.pdf"
	docFile2 := tempDir + "/notes.txt"
	if err := os.WriteFile(docFile1, []byte("pdf dummy data"), 0644); err != nil {
		t.Fatalf("failed to create temp doc file: %v", err)
	}
	if err := os.WriteFile(docFile2, []byte("notes dummy data"), 0644); err != nil {
		t.Fatalf("failed to create temp doc file 2: %v", err)
	}

	// 1. Create a task to attach to
	task, err := ctx.Store.Add("Test Doc Hierarchy", "Testing doc subcommands", nil, "medium")
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 2. task doc help when called without args
	out, err := captureStdout(func() error {
		return Root.Execute(ctx, []string{"task", "doc"})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'task doc': %v", err)
	}
	if !strings.Contains(out, "Manage task document attachments") || !strings.Contains(out, "add, attach") {
		t.Fatalf("expected doc help output, got:\n%s", out)
	}

	// 3. task doc add via full path
	taskIDStr := fmt.Sprintf("%d", task.ID)
	err = Root.Execute(ctx, []string{"task", "doc", "add", taskIDStr, docFile1, "Architecture Doc"})
	if err != nil {
		t.Fatalf("unexpected error running 'task doc add': %v", err)
	}

	// 4. task doc add via aliases 't d attach'
	err = Root.Execute(ctx, []string{"t", "d", "attach", taskIDStr, docFile2})
	if err != nil {
		t.Fatalf("unexpected error running 't d attach': %v", err)
	}

	// 5. task doc list via alias 'task doc ls'
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"task", "doc", "ls", taskIDStr})
	})
	if err != nil {
		t.Fatalf("unexpected error running 'task doc ls': %v", err)
	}
	if !strings.Contains(out, "Architecture Doc") || !strings.Contains(out, "notes.txt") {
		t.Fatalf("expected both documents listed, got:\n%s", out)
	}

	// 6. List documents via alias 't docs list'
	out, err = captureStdout(func() error {
		return Root.Execute(ctx, []string{"t", "docs", "list", taskIDStr})
	})
	if err != nil {
		t.Fatalf("unexpected error running 't docs list': %v", err)
	}
	if !strings.Contains(out, "Architecture Doc") {
		t.Fatalf("expected document listed via 't docs list', got:\n%s", out)
	}

	docs, err := ctx.Store.ListDocuments(task.ID)
	if err != nil || len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d (err: %v)", len(docs), err)
	}

	// 7. Remove first document via 'task doc remove'
	firstDocID := fmt.Sprintf("%d", docs[0].ID)
	err = Root.Execute(ctx, []string{"task", "doc", "remove", firstDocID})
	if err != nil {
		t.Fatalf("unexpected error running 'task doc remove': %v", err)
	}

	// 8. Remove second document via alias 't doc rm'
	secondDocID := fmt.Sprintf("%d", docs[1].ID)
	err = Root.Execute(ctx, []string{"t", "doc", "rm", secondDocID})
	if err != nil {
		t.Fatalf("unexpected error running 't doc rm': %v", err)
	}

	remainingDocs, err := ctx.Store.ListDocuments(task.ID)
	if err != nil || len(remainingDocs) != 0 {
		t.Fatalf("expected 0 documents remaining, got %d (err: %v)", len(remainingDocs), err)
	}
}
