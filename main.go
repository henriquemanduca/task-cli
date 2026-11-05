package main

import (
	"flag"
	"fmt"
	"os"

	"task-cli/internal/cmd"
	"task-cli/internal/db"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func printVersion() {
	fmt.Printf("task-cli version %s (commit: %s, build date: %s)\n", version, commit, date)
}

func printUsage() {
	fmt.Println("Usage: task-cli [--db <path>] <command> [arguments]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  add <title> [desc] [-f <code>]        Add a new task (optionally linked to a project via -f)")
	fmt.Println("  project-add, np <code> <name> [desc]  Create a new project")
	fmt.Println("  list [status] [-f <code>] [-a] [--done] [-d] [-t]   List tasks (default: pending; -a all, --done done, -f proj)")
	fmt.Println("  projects, lp [-d] [-t]                List all projects (-d show description, -t show dates)")
	fmt.Println("  view <id> [-t]                        View full task details (optional: -t show timestamps)")
	fmt.Println("  update <id> [title] [description]     Update a task's title, description, project (-f), priority")
	fmt.Println("  edit <id>                             Edit task description in an external editor (default: nano)")
	fmt.Println("  delete <id>                           Soft delete a task by ID")
	fmt.Println("  set-pending <id>                      Set task status to pending")
	fmt.Println("  set-in-progress <id>                  Set task status to in-progress")
	fmt.Println("  set-done <id>                         Set task status to done")
	fmt.Println("  set-abort <id> [reason]               Set task status to abort (optional reason)")
	fmt.Println("  set-prio <id> <priority>              Set task priority (low, medium, high, urgent)")
	fmt.Println("  version                               Display CLI version")
	fmt.Println("\nOptions:")
	fmt.Println("  --db <path>                           Custom SQLite database path (default: ~/.task-cli.db)")
	fmt.Println("  -v, --version                         Display CLI version")
	fmt.Println("  -h, --help                            Display this help message")
}

func getStore(customPath string) (*db.Store, error) {
	dbPath := customPath
	if dbPath == "" {
		var err error
		dbPath, err = db.DefaultDBPath()
		if err != nil {
			return nil, fmt.Errorf("failed to determine database path: %w", err)
		}
	}
	return db.NewStore(dbPath)
}

func main() {
	var showVersion bool
	var dbPathFlag string

	flag.BoolVar(&showVersion, "version", false, "Display CLI version")
	flag.BoolVar(&showVersion, "v", false, "Display CLI version (shorthand)")
	flag.StringVar(&dbPathFlag, "db", "", "Path to SQLite database file (optional)")

	flag.Usage = printUsage
	flag.Parse()

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "version":
		printVersion()
		return

	case "help":
		printUsage()
		return
	}

	store, err := getStore(dbPathFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := &cmd.Context{Store: store}
	var cmdErr error

	switch command {
	case "project-add", "np":
		cmdErr = cmd.ProjectAdd(ctx, cmdArgs)

	case "projects", "lp":
		cmdErr = cmd.ProjectList(ctx, cmdArgs)

	case "add":
		cmdErr = cmd.TaskAdd(ctx, cmdArgs)

	case "list":
		cmdErr = cmd.TaskList(ctx, cmdArgs)

	case "view", "show":
		cmdErr = cmd.TaskView(ctx, cmdArgs)

	case "update":
		cmdErr = cmd.TaskUpdate(ctx, cmdArgs)

	case "edit":
		cmdErr = cmd.TaskEdit(ctx, cmdArgs)

	case "delete":
		cmdErr = cmd.TaskDelete(ctx, cmdArgs)

	case "set-pending":
		cmdErr = cmd.TaskSetPending(ctx, cmdArgs)

	case "set-in-progress":
		cmdErr = cmd.TaskSetInProgress(ctx, cmdArgs)

	case "set-done":
		cmdErr = cmd.TaskSetDone(ctx, cmdArgs)

	case "set-abort":
		cmdErr = cmd.TaskSetAbort(ctx, cmdArgs)

	case "set-prio", "set-priority":
		cmdErr = cmd.TaskSetPriority(ctx, cmdArgs)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		os.Exit(1)
	}
}
