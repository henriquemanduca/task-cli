package cmd

import (
	"io"
	"os"

	"task-cli/internal/db"
)

// Context armazena dependências compartilhadas entre os comandos.
type Context struct {
	Store  *db.Store
	DBPath string
	Format string
	Out    io.Writer
}

// OutWriter returns the configured output writer for the context,
// or defaults to os.Stdout if unset.
func (c *Context) OutWriter() io.Writer {
	if c != nil && c.Out != nil {
		return c.Out
	}
	return os.Stdout
}

// CommandFunc defines the function signature for subcommands.
type CommandFunc func(ctx *Context, args []string) error

// Registry maps command names and aliases to their corresponding handlers.
var Registry = map[string]CommandFunc{
	"np":              ProjectAdd,
	"project-add":     ProjectAdd,
	"lp":              ProjectList,
	"projects":        ProjectList,
	"add":             TaskAdd,
	"list":            TaskList,
	"ls":              TaskList,
	"view":            TaskView,
	"show":            TaskView,
	"get":             TaskView,
	"update":          TaskUpdate,
	"mod":             TaskUpdate,
	"edit":            TaskEdit,
	"delete":          TaskDelete,
	"del":             TaskDelete,
	"rm":              TaskDelete,
	"remove":          TaskDelete,
	"status":          TaskStatus,
	"set-status":      TaskStatus,
	"task-status":     TaskStatus,
	"prio":            TaskSetPriority,
	"priority":        TaskSetPriority,
	"set-prio":        TaskSetPriority,
	"set-priority":    TaskSetPriority,
	"set-pending":     TaskSetPending,
	"pending":         TaskSetPending,
	"set-in-progress": TaskSetInProgress,
	"start":           TaskSetInProgress,
	"set-done":        TaskSetDone,
	"done":            TaskSetDone,
	"set-abort":       TaskSetAbort,
	"abort":           TaskSetAbort,
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
	if Root != nil {
		if targetPath, ok := ResolveFlatAlias(command); ok {
			return true, Root.Execute(ctx, append(targetPath, args...))
		}

		target := Root.FindSubcommand(command)
		if target != nil {
			return true, Root.Execute(ctx, append([]string{command}, args...))
		}
	}

	handler, ok := Registry[command]
	if !ok {
		return false, nil
	}
	return true, handler(ctx, args)
}
