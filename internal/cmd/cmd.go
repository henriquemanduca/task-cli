package cmd

import (
	"task-cli/internal/db"
)

// Context armazena dependências compartilhadas entre os comandos.
type Context struct {
	Store *db.Store
}

// CommandFunc defines the function signature for subcommands.
type CommandFunc func(ctx *Context, args []string) error

// Registry maps command names and aliases to their corresponding handlers.
var Registry = map[string]CommandFunc{
	"project-add":     ProjectAdd,
	"np":              ProjectAdd,
	"projects":        ProjectList,
	"lp":              ProjectList,
	"add":             TaskAdd,
	"list":            TaskList,
	"view":            TaskView,
	"show":            TaskView,
	"update":          TaskUpdate,
	"edit":            TaskEdit,
	"delete":          TaskDelete,
	"set-pending":     TaskSetPending,
	"set-in-progress": TaskSetInProgress,
	"set-done":        TaskSetDone,
	"set-abort":       TaskSetAbort,
	"set-prio":        TaskSetPriority,
	"set-priority":    TaskSetPriority,
	"doc-add":         DocAdd,
	"attach":          DocAdd,
	"doc-list":        DocList,
	"docs":            DocList,
	"doc-del":         DocDelete,
	"doc-rm":          DocDelete,
	"doc-remove":      DocDelete,
	"export":          Export,
	"exp":             Export,
	"import":          Import,
	"imp":             Import,
}

// Execute routes a command to its registered handler.
func Execute(ctx *Context, command string, args []string) (bool, error) {
	handler, ok := Registry[command]
	if !ok {
		return false, nil
	}
	return true, handler(ctx, args)
}
