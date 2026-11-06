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
	fmt.Println("  task [command]                        Manage tasks (subcommands: add, list, view, update, delete, status, prio)")
	fmt.Println("  project [command]                     Manage projects (subcommands: add, list, view)")
	fmt.Println("  add <title> [desc] [-p <code>]        Add a new task (alias for 'task add')")
	fmt.Println("  list [status] [-p <code>] [-a] [--done] [-t] [-n] [-F <fmt>]         List tasks (default: TSV, pending; alias for 'task list')")
	fmt.Println("  view, show <id> [-t]                  View full task details (alias for 'task view')")
	fmt.Println("  update <id> [title] [description]     Update a task's title, description, project (-p), priority (alias for 'task update')")
	fmt.Println("  edit <id>                             Edit task description in an external editor (alias for 'task edit')")
	fmt.Println("  delete <id>                           Soft delete a task by ID (alias for 'task delete')")
	fmt.Println("  set-pending <id>                      Set task status to pending (alias for 'task status <id> pending')")
	fmt.Println("  set-in-progress <id>                  Set task status to in-progress (alias for 'task status <id> in-progress')")
	fmt.Println("  set-done <id>                         Set task status to done (alias for 'task status <id> done')")
	fmt.Println("  set-abort <id> [reason]               Set task status to abort (alias for 'task status <id> abort')")
	fmt.Println("  set-prio <id> <priority>              Set task priority (alias for 'task prio <id> <priority>')")
	fmt.Println("  doc-add, attach <id> <path> [name]    Link a document to a task")
	fmt.Println("  doc-list, docs <id>                   List documents linked to a task")
	fmt.Println("  doc-del, doc-remove <id>              Remove a linked document by ID")
	fmt.Println("  export, exp [options]                 Export projects, tasks, and documents to JSON")
	fmt.Println("  import, imp <file> [options]          Import projects, tasks, and documents from JSON")
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
		if len(cmdArgs) > 0 {
			if target, _ := cmd.Root.Resolve(cmdArgs); target != nil && target != cmd.Root {
				target.PrintHelp(os.Stdout)
				return
			}
		}
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
	handled, cmdErr := cmd.Execute(ctx, command, cmdArgs)
	if !handled {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		os.Exit(1)
	}
}
