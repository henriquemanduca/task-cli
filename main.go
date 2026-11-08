package main

import (
	"fmt"
	"os"

	"task-cli/internal/cmd"
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
	fmt.Println("Usage: task-cli [--db <path>] [-F <format>] <command> [arguments]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  task [command]                        Manage tasks (subcommands: add, list, view, update, delete, status, prio, doc)")
	fmt.Println("  project [command]                     Manage projects (subcommands: add, list, view, update)")
	fmt.Println("  task doc [command]                    Manage task documents (subcommands: add, list, remove)")
	fmt.Println("  doc-add, attach <id> <path> [name]    Link a document to a task")
	fmt.Println("  doc-list, docs <id>                   List documents linked to a task")
	fmt.Println("  doc-del, doc-remove <id>              Remove a linked document by ID")
	fmt.Println("  export, exp [options]                 Export projects, tasks, and documents to JSON")
	fmt.Println("  import, imp <file> [options]          Import projects, tasks, and documents from JSON")
	fmt.Println("  version                               Display CLI version")
	fmt.Println("\nGlobal Options:")
	fmt.Println("  --db <path>                           Custom SQLite database path (default: ~/.task-cli.db)")
	fmt.Println("  -F, --format <format>                 Output format: tsv (default), table")
	fmt.Println("  -v, --version                         Display CLI version")
	fmt.Println("  -h, --help                            Display this help message")
}

func main() {
	rawArgs := os.Args[1:]

	ctx, remainingArgs, cleanup, err := cmd.ResolveContext(rawArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if cleanup != nil {
		defer cleanup()
	}

	if len(remainingArgs) == 0 {
		printUsage()
		os.Exit(0)
	}

	command := remainingArgs[0]
	cmdArgs := remainingArgs[1:]

	switch command {
	case "version", "-v", "--version":
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

	case "-h", "--help":
		printUsage()
		return
	}

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
