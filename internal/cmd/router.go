package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrUnknownCommand is returned when a command or subcommand cannot be resolved.
var ErrUnknownCommand = errors.New("unknown command")

// Command represents a node in the hierarchical command tree.
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Run         CommandFunc
	Handler     Handler
	Subcommands []*Command

	parent         *Command
	out            io.Writer
	useFlatAliases bool
}

// SetHandler configures a Handler for this command, synchronizing Run.
func (c *Command) SetHandler(h Handler) {
	c.Handler = h
	if h != nil {
		c.Run = h.Run
	} else {
		c.Run = nil
	}
}

// AddCommand registers child subcommands under this command.
func (c *Command) AddCommand(cmds ...*Command) {
	for _, sub := range cmds {
		sub.parent = c
		c.Subcommands = append(c.Subcommands, sub)
	}
}

// FindSubcommand looks up a direct child subcommand by name or alias.
func (c *Command) FindSubcommand(nameOrAlias string) *Command {
	for _, sub := range c.Subcommands {
		if sub.Name == nameOrAlias {
			return sub
		}
		for _, alias := range sub.Aliases {
			if alias == nameOrAlias {
				return sub
			}
		}
	}
	return nil
}

// FindCommand resolves a chain of names or aliases sequentially from this command.
func (c *Command) FindCommand(names ...string) *Command {
	lookupNames := names
	if c.useFlatAliases && len(names) == 1 {
		if targetPath, ok := ResolveFlatAlias(names[0]); ok {
			lookupNames = targetPath
		}
	}
	current := c
	for _, name := range lookupNames {
		current = current.FindSubcommand(name)
		if current == nil {
			return nil
		}
	}
	return current
}

// HasAlias checks if this command is known by the given alias.
func (c *Command) HasAlias(alias string) bool {
	for _, a := range c.Aliases {
		if a == alias {
			return true
		}
	}
	return false
}

// Parent returns the parent command, or nil if this is a root command.
func (c *Command) Parent() *Command {
	return c.parent
}

// CommandPath returns the full invocation path of this command (e.g. "task-cli project add").
func (c *Command) CommandPath() string {
	if c.parent != nil {
		parentPath := c.parent.CommandPath()
		if parentPath != "" {
			return parentPath + " " + c.Name
		}
	}
	return c.Name
}

// SetOutput configures the writer for help and usage output.
func (c *Command) SetOutput(w io.Writer) {
	c.out = w
}

// Out returns the configured output writer, or traverses up the parent tree, defaulting to os.Stdout.
func (c *Command) Out() io.Writer {
	if c.out != nil {
		return c.out
	}
	if c.parent != nil {
		return c.parent.Out()
	}
	return os.Stdout
}

// Resolve consumes matching subcommand names and aliases iteratively from args,
// returning the deepest matched command and any unconsumed remaining arguments.
func (c *Command) Resolve(args []string) (*Command, []string) {
	current := c
	remaining := args

	if c.useFlatAliases {
		remaining = ExpandAliases(args)
	}

	for len(remaining) > 0 {
		arg := remaining[0]
		sub := current.FindSubcommand(arg)
		if sub == nil {
			break
		}
		current = sub
		remaining = remaining[1:]
	}

	return current, remaining
}

// Execute resolves arguments against the command tree and runs the target command,
// handling hierarchical help and argument delegation. Global flags (--db/--format)
// are resolved once by ResolveContext before dispatch.
func (c *Command) Execute(ctx *Context, args []string) error {
	if ctx == nil {
		ctx = &Context{}
	}

	// Check for top-level help keyword (e.g. "help <subcommand>")
	if len(args) > 0 && args[0] == "help" {
		if len(args) == 1 {
			return c.PrintHelp(nil)
		}
		target, _ := c.Resolve(args[1:])
		return target.PrintHelp(nil)
	}

	if c.useFlatAliases {
		// Kept here for callers that inspect args before Resolve; Resolve is idempotent.
		args = ExpandAliases(args)
	}

	target, remaining := c.Resolve(args)

	// Check if help was requested via flag anywhere in remaining args
	if isHelpRequested(remaining) {
		return target.PrintHelp(nil)
	}

	// If target has no runner (parent / aggregator node)
	if target.Run == nil && target.Handler == nil {
		if len(remaining) == 0 || (len(remaining) == 1 && remaining[0] == "help") {
			return target.PrintHelp(nil)
		}
		return fmt.Errorf("%w: %s for %s", ErrUnknownCommand, remaining[0], target.CommandPath())
	}

	if ctx != nil && ctx.Out == nil && target.out != nil {
		ctx.Out = target.Out()
	}

	if target.Handler != nil {
		return target.Handler.Run(ctx, remaining)
	}
	return target.Run(ctx, remaining)
}

func isHelpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

// Help formats and returns the help text for this command.
func (c *Command) Help() string {
	var b strings.Builder

	// Usage line
	if c.Usage != "" {
		fmt.Fprintf(&b, "Usage:\n  %s\n", c.Usage)
	} else if len(c.Subcommands) > 0 {
		b.WriteString(fmt.Sprintf("Usage:\n  %s <command> [arguments]\n", c.CommandPath()))
	} else {
		b.WriteString(fmt.Sprintf("Usage:\n  %s [arguments]\n", c.CommandPath()))
	}

	// Description
	if c.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", c.Description)
	}

	// Aliases
	if len(c.Aliases) > 0 {
		fmt.Fprintf(&b, "\nAliases:\n  %s\n", strings.Join(c.Aliases, ", "))
	}

	// Subcommands
	if len(c.Subcommands) > 0 {
		b.WriteString("\nAvailable commands:\n")
		maxLen := 0
		type cmdLine struct {
			names string
			desc  string
		}
		var lines []cmdLine
		for _, sub := range c.Subcommands {
			names := sub.Name
			if len(sub.Aliases) > 0 {
				names += ", " + strings.Join(sub.Aliases, ", ")
			}
			if len(names) > maxLen {
				maxLen = len(names)
			}
			lines = append(lines, cmdLine{names: names, desc: sub.Description})
		}

		for _, line := range lines {
			padding := strings.Repeat(" ", maxLen-len(line.names)+2)
			if line.desc != "" {
				b.WriteString(fmt.Sprintf("  %s%s%s\n", line.names, padding, line.desc))
			} else {
				b.WriteString(fmt.Sprintf("  %s\n", line.names))
			}
		}
	}

	// Flags
	b.WriteString("\nFlags:\n  -h, --help  Show help for this command\n")

	return b.String()
}

// PrintHelp writes the command's help text to w, or c.Out() if w is nil.
func (c *Command) PrintHelp(w io.Writer) error {
	if w == nil {
		w = c.Out()
	}
	_, err := fmt.Fprint(w, c.Help())
	return err
}

// NewRootCommand creates the root command with all built-in CLI commands attached.
func NewRootCommand() *Command {
	root := &Command{
		Name:           "task-cli",
		Description:    "Single-binary CLI for task management backed by SQLite",
		useFlatAliases: true,
	}

	projectCmd := &Command{
		Name:        "project",
		Aliases:     []string{"proj"},
		Description: "Manage projects (creation, listing, details)",
	}
	projectCmd.AddCommand(
		&Command{
			Name:        "add",
			Aliases:     []string{"create"},
			Description: "Create a new project",
			Usage:       "task-cli project add <code> <name> [description]",
			Run:         ProjectAdd,
		},
		&Command{
			Name:        "list",
			Aliases:     []string{"ls"},
			Description: "List all projects (default: TSV; -F table for formatted view)",
			Usage:       "task-cli project list [-d] [-t] [-n] [-F <fmt>]",
			Run:         ProjectList,
		},
		&Command{
			Name:        "view",
			Aliases:     []string{"show"},
			Description: "View project details (optional: -t show timestamps)",
			Usage:       "task-cli project view <code|id> [-t]",
			Run:         ProjectView,
		},
		&Command{
			Name:        "update",
			Aliases:     []string{"edit", "mod"},
			Description: "Update project name and/or description",
			Usage:       "task-cli project update <code|id> [--name <name>] [--desc <desc>]",
			Run:         ProjectUpdate,
		},
	)

	taskCmd := &Command{
		Name:        "task",
		Aliases:     []string{"t", "tsk"},
		Description: "Manage tasks (creation, listing, update, status, doc)",
	}

	docCmd := &Command{
		Name:        "doc",
		Aliases:     []string{"docs", "d"},
		Description: "Manage task document attachments (add, list, remove, open, prune)",
	}
	docCmd.AddCommand(
		&Command{
			Name:        "add",
			Aliases:     []string{"attach"},
			Description: "Link a document to a task",
			Usage:       "task-cli task doc add <task_id> <file_path> [name] [--type <type>] [--copy] [--link]",
			Run:         DocAdd,
		},
		&Command{
			Name:        "list",
			Aliases:     []string{"ls"},
			Description: "List documents linked to a task",
			Usage:       "task-cli task doc list <task_id>",
			Run:         DocList,
		},
		&Command{
			Name:        "remove",
			Aliases:     []string{"rm", "del", "delete"},
			Description: "Remove a linked document by ID",
			Usage:       "task-cli task doc remove <doc_id>",
			Run:         DocDelete,
		},
		&Command{
			Name:        "open",
			Description: "Open task attachment directory or reveal document in file manager",
			Usage:       "task-cli task doc open <task_id> [--doc <doc_id>]",
			Run:         DocOpen,
		},
		&Command{
			Name:        "prune",
			Description: "Audit and clean orphaned attachment files",
			Usage:       "task-cli task doc prune [--dry-run]",
			Run:         DocPrune,
		},
	)

	taskCmd.AddCommand(
		docCmd,
		&Command{
			Name:        "add",
			Aliases:     []string{"create", "new"},
			Description: "Add a new task (optionally linked to a project via -p)",
			Usage:       "task-cli task add [-p <id>] [--priority <prio>] <title> [description]",
			Run:         TaskAdd,
		},
		&Command{
			Name:        "list",
			Aliases:     []string{"ls"},
			Description: "List tasks (default: TSV, pending; -F table for formatted view)",
			Usage:       "task-cli task list [status] [-p <code>] [-a] [--done] [-t] [-n] [-F <fmt>]",
			Run:         TaskList,
		},
		&Command{
			Name:        "view",
			Aliases:     []string{"show", "get"},
			Description: "View full task details (optional: -t show timestamps)",
			Usage:       "task-cli task view <id> [-t]",
			Run:         TaskView,
		},
		&Command{
			Name:        "update",
			Aliases:     []string{"mod"},
			Description: "Update a task's title, description, project (-p), priority",
			Usage:       "task-cli task update <id> [-p <id>] [--priority <prio>] [title] [description]",
			Run:         TaskUpdate,
		},
		&Command{
			Name:        "edit",
			Description: "Edit task description in an external editor (default: nano)",
			Usage:       "task-cli task edit <id>",
			Run:         TaskEdit,
		},
		&Command{
			Name:        "delete",
			Aliases:     []string{"del", "rm", "remove"},
			Description: "Soft delete a task by ID",
			Usage:       "task-cli task delete <id>",
			Run:         TaskDelete,
		},
		&Command{
			Name:        "status",
			Aliases:     []string{"set-status"},
			Description: "Set task status (pending, in-progress, done, aborted)",
			Usage:       "task-cli task status <id> <status> [reason]",
			Run:         TaskStatus,
		},
		&Command{
			Name:        "prio",
			Aliases:     []string{"priority", "set-prio", "set-priority"},
			Description: "Set task priority (low, medium, high, urgent)",
			Usage:       "task-cli task prio <id> <priority>",
			Run:         TaskSetPriority,
		},
		&Command{
			Name:        "set-pending",
			Aliases:     []string{"pending"},
			Description: "Set task status to pending",
			Usage:       "task-cli task set-pending <id>",
			Run:         TaskSetPending,
		},
		&Command{
			Name:        "set-in-progress",
			Aliases:     []string{"start"},
			Description: "Set task status to in-progress",
			Usage:       "task-cli task set-in-progress <id>",
			Run:         TaskSetInProgress,
		},
		&Command{
			Name:        "set-done",
			Aliases:     []string{"done"},
			Description: "Set task status to done",
			Usage:       "task-cli task set-done <id>",
			Run:         TaskSetDone,
		},
		&Command{
			Name:        "set-abort",
			Aliases:     []string{"abort", "aborted", "set-aborted"},
			Description: "Set task status to aborted (optional reason)",
			Usage:       "task-cli task set-abort <id> [reason]",
			Run:         TaskSetAbort,
		},
	)

	rootDocCmd := &Command{
		Name:        "doc",
		Aliases:     []string{"d"},
		Description: "Manage task document attachments (add, list, remove, open, prune)",
	}
	rootDocCmd.AddCommand(
		&Command{
			Name:        "add",
			Aliases:     []string{"attach"},
			Description: "Link a document to a task",
			Usage:       "task-cli doc add <task_id> <file_path> [name] [--type <type>] [--copy] [--link]",
			Run:         DocAdd,
		},
		&Command{
			Name:        "list",
			Aliases:     []string{"ls"},
			Description: "List documents linked to a task",
			Usage:       "task-cli doc list <task_id>",
			Run:         DocList,
		},
		&Command{
			Name:        "remove",
			Aliases:     []string{"rm", "del", "delete"},
			Description: "Remove a linked document by ID",
			Usage:       "task-cli doc remove <doc_id>",
			Run:         DocDelete,
		},
		&Command{
			Name:        "open",
			Description: "Open task attachment directory or reveal document in file manager",
			Usage:       "task-cli doc open <task_id> [--doc <doc_id>]",
			Run:         DocOpen,
		},
		&Command{
			Name:        "prune",
			Description: "Audit and clean orphaned attachment files",
			Usage:       "task-cli doc prune [--dry-run]",
			Run:         DocPrune,
		},
	)

	root.AddCommand(
		projectCmd,
		taskCmd,
		rootDocCmd,
		&Command{
			Name:        "doc-add",
			Aliases:     []string{"attach"},
			Description: "Link a document to a task",
			Run:         DocAdd,
		},
		&Command{
			Name:        "doc-list",
			Aliases:     []string{"docs"},
			Description: "List documents linked to a task",
			Run:         DocList,
		},
		&Command{
			Name:        "doc-del",
			Aliases:     []string{"doc-rm", "doc-remove"},
			Description: "Remove a linked document by ID",
			Run:         DocDelete,
		},
		&Command{
			Name:        "doc-open",
			Description: "Open task attachment directory or reveal document",
			Run:         DocOpen,
		},
		&Command{
			Name:        "doc-prune",
			Description: "Audit and clean orphaned attachment files",
			Run:         DocPrune,
		},
		&Command{
			Name:        "export",
			Aliases:     []string{"exp"},
			Description: "Export projects, tasks, and documents to JSON",
			Run:         Export,
		},
		&Command{
			Name:        "import",
			Aliases:     []string{"imp"},
			Description: "Import projects, tasks, and documents from JSON",
			Run:         Import,
		},
	)

	return root
}
