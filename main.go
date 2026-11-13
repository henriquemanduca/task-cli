package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

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
	fmt.Println("Usage: task-cli [-F <format>] <command> [arguments]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  task [command]                        Manage tasks (subcommands: add, list, view, update, delete, status, prio, doc)")
	fmt.Println("  project [command]                     Manage projects (subcommands: add, list, view, update)")
	fmt.Println("  task doc [command]                    Manage task documents (subcommands: add, list, remove, open, prune)")
	fmt.Println("  task doc open <id> [--doc <id>]       Open task attachment folder or reveal document")
	fmt.Println("  task doc prune [--dry-run]            Audit and prune orphaned attachments")
	fmt.Println("  doc-add, attach <id> <path> [name]    Attach a document to a task (--copy, --link)")
	fmt.Println("  doc-list, docs <id>                   List documents linked to a task")
	fmt.Println("  doc-del, doc-remove <id>              Remove a linked document by ID")
	fmt.Println("  doc-open <id> [--doc <id>]            Open task attachment folder or reveal document")
	fmt.Println("  doc-prune [--dry-run]                 Audit and prune orphaned attachments")
	fmt.Println("  export, exp [options]                 Export projects, tasks, and documents to JSON")
	fmt.Println("  import, imp <file> [options]          Import projects, tasks, and documents from JSON")
	fmt.Println("  version                               Display CLI version")
	fmt.Println("\nGlobal Options:")
	fmt.Println("  -F, --format <format>                 Output format: tsv (default), table")
	fmt.Println("  -v, --version                         Display CLI version")
	fmt.Println("  -h, --help                            Display this help message")
	fmt.Println("\nWorkspace Config:")
	fmt.Println("  .taskcli                              .env-style defaults, searched up to $HOME")
	fmt.Println("                                        (e.g. PROJECT_CODE=<project-code>)")
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(rawArgs []string) int {
	ctx, remainingArgs, cleanup, err := cmd.ResolveContext(rawArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if cleanup != nil {
		defer cleanup()
	}

	if ctx != nil {
		sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		ctx.Ctx = sigCtx
	}

	if len(remainingArgs) == 0 {
		printUsage()
		return 0
	}

	command := remainingArgs[0]
	cmdArgs := remainingArgs[1:]

	switch command {
	case "version", "-v", "--version":
		printVersion()
		return 0

	case "help":
		if len(cmdArgs) > 0 {
			root := cmd.NewRootCommand()
			if target, _ := root.Resolve(cmdArgs); target != nil && target != root {
				target.PrintHelp(os.Stdout)
				return 0
			}
		}
		printUsage()
		return 0

	case "-h", "--help":
		printUsage()
		return 0
	}

	handled, cmdErr := cmd.Execute(cmd.NewRootCommand(), ctx, command, cmdArgs)
	if !handled {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		return 1
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		return 1
	}

	return 0
}
