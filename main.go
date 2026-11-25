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
	fmt.Println("Usage: task-cli [-F <format>] <command> [arguments]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  task [command]                        Manage tasks (subcommands: add, list, view, update, edit, delete, status, prio, start, done, abort)")
	fmt.Println("  project [command]                     Manage projects (subcommands: add, list, view, update)")
	fmt.Println("  config                                Open the config file in $EDITOR")
	fmt.Println("  help [command]                        Show help for a command")
	fmt.Println("  version                               Display CLI version")
	fmt.Println("\nGlobal Options:")
	fmt.Println("  -F, --format <format>                 Output format: tsv (default), table")
	fmt.Println("  -v, --version                         Display CLI version")
	fmt.Println("  -h, --help                            Display this help message")
	fmt.Println("\nStorage:")
	fmt.Println("  Tasks are Markdown files in a vault folder (projects are subfolders).")
	fmt.Println("  Config: ~/.config/task-cli/config.yaml  (vault: ~/.task-cli-vault)")
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(rawArgs []string) int {
	ctx, remainingArgs, err := cmd.ResolveContext(rawArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
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
