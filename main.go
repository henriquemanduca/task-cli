package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

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
	fmt.Println("  p-add <code> <name> [description]     Create a new project")
	fmt.Println("  p-list                                List all projects")
	fmt.Println("  add [-p <code>] <title> [desc]        Add a new task (optionally linked to a project)")
	fmt.Println("  list [status] [-p <code>]             List tasks (optional filter: pending, in-progress, done, abort)")
	fmt.Println("  view <id>                             View full task details and multiline description")
	fmt.Println("  update <id> <title> [description]     Update a task's title and description")
	fmt.Println("  delete <id>                           Soft delete a task by ID")
	fmt.Println("  set-pending <id>                      Set task status to pending")
	fmt.Println("  set-in-progress <id>                  Set task status to in-progress")
	fmt.Println("  set-done <id>                         Set task status to done")
	fmt.Println("  set-abort <id> [reason]               Set task status to abort (optional reason)")
	fmt.Println("  version                               Display CLI version")
	fmt.Println("\nOptions:")
	fmt.Println("  --db <path>                           Custom SQLite database path (default: ~/.task-cli.db)")
	fmt.Println("  -v, --version                         Display CLI version")
	fmt.Println("  -h, --help                            Display this help message")
}

func summarizeText(text string, maxLen int) string {
	clean := strings.ReplaceAll(text, "\r\n", " ")
	clean = strings.ReplaceAll(clean, "\n", " ")
	clean = strings.ReplaceAll(clean, "\t", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		return "-"
	}
	runes := []rune(clean)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return clean
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

	case "help":
		printUsage()

	case "p-add", "project-add":
		if len(cmdArgs) < 2 {
			fmt.Fprintln(os.Stderr, "Error: 'code' and 'name' are required.")
			fmt.Fprintln(os.Stderr, "Usage: task-cli p-add <code> <name> [description]")
			os.Exit(1)
		}

		code := strings.TrimSpace(cmdArgs[0])
		name := strings.TrimSpace(cmdArgs[1])
		var description string
		if len(cmdArgs) > 2 {
			description = strings.TrimSpace(strings.Join(cmdArgs[2:], " "))
		}

		store, err := getStore(dbPathFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		project, err := store.AddProject(code, name, description)
		if err != nil {
			if errors.Is(err, db.ErrProjectCodeExists) {
				fmt.Fprintf(os.Stderr, "Error: project with code '%s' already exists\n", code)
			} else {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(1)
		}

		fmt.Printf("Project '%s' created successfully (ID: %d)\n", project.Code, project.ID)

	case "p-list", "project-list", "projects":
		store, err := getStore(dbPathFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		projects, err := store.ListProjects()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
			os.Exit(1)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tCODE\tNAME\tDESCRIPTION\tCREATED AT")
		fmt.Fprintln(w, "--\t----\t----\t-----------\t----------")
		for _, p := range projects {
			desc := p.Description
			if desc == "" {
				desc = "-"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
				p.ID,
				p.Code,
				p.Name,
				desc,
				p.CreatedAt.Local().Format("2006-01-02 15:04"),
			)
		}
		w.Flush()

	case "add":
		addCmd := flag.NewFlagSet("add", flag.ExitOnError)
		var projectCode string
		var priority string
		addCmd.StringVar(&projectCode, "p", "", "Project code to link task to (shorthand)")
		addCmd.StringVar(&projectCode, "project", "", "Project code to link task to")
		addCmd.StringVar(&priority, "priority", "medium", "Task priority (e.g. low, medium, high)")

		if err := addCmd.Parse(cmdArgs); err != nil {
			os.Exit(1)
		}

		posArgs := addCmd.Args()
		if len(posArgs) < 1 {
			fmt.Fprintln(os.Stderr, "Error: 'title' is required.")
			fmt.Fprintln(os.Stderr, "Usage: task-cli add [-p <code>] [--priority <prio>] <title> [description]")
			os.Exit(1)
		}

		title := strings.TrimSpace(posArgs[0])
		var description string
		if len(posArgs) > 1 {
			description = strings.TrimSpace(strings.Join(posArgs[1:], " "))
		}

		store, err := getStore(dbPathFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		var projectID *int64
		var projectRef string
		if projectCode != "" {
			proj, err := store.GetProjectByCode(projectCode)
			if err != nil {
				if errors.Is(err, db.ErrProjectNotFound) {
					fmt.Fprintf(os.Stderr, "Error: project with code '%s' not found\n", projectCode)
				} else {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				os.Exit(1)
			}
			projectID = &proj.ID
			projectRef = proj.Code
		}

		task, err := store.Add(title, description, projectID, priority)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating task: %v\n", err)
			os.Exit(1)
		}

		if projectRef != "" {
			fmt.Printf("Task created successfully (ID: %d) in project '%s'\n", task.ID, projectRef)
		} else {
			fmt.Printf("Task created successfully (ID: %d)\n", task.ID)
		}

	case "list":
		listCmd := flag.NewFlagSet("list", flag.ExitOnError)
		var projectCode string
		var statusFlag string
		listCmd.StringVar(&projectCode, "p", "", "Filter tasks by project code (shorthand)")
		listCmd.StringVar(&projectCode, "project", "", "Filter tasks by project code")
		listCmd.StringVar(&statusFlag, "s", "", "Filter tasks by status (shorthand)")
		listCmd.StringVar(&statusFlag, "status", "", "Filter tasks by status")

		if err := listCmd.Parse(cmdArgs); err != nil {
			os.Exit(1)
		}

		statusFilter := strings.ToLower(strings.TrimSpace(statusFlag))
		posArgs := listCmd.Args()
		if len(posArgs) > 0 && statusFilter == "" {
			statusFilter = strings.ToLower(strings.TrimSpace(posArgs[0]))
		}

		if statusFilter != "" {
			switch db.Status(statusFilter) {
			case db.StatusPending, db.StatusInProgress, db.StatusDone, db.StatusAbort:
			default:
				fmt.Fprintf(os.Stderr, "Invalid status '%s'. Valid options: pending, in-progress, done, abort\n", statusFilter)
				os.Exit(1)
			}
		}

		store, err := getStore(dbPathFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		tasks, err := store.List(statusFilter, projectCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying tasks: %v\n", err)
			os.Exit(1)
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tPROJECT\tSTATUS\tPRIO\tTITLE\tDESCRIPTION\tCREATED AT")
		fmt.Fprintln(w, "--\t-------\t------\t----\t-----\t-----------\t----------")
		for _, t := range tasks {
			proj := t.ProjectCode
			if proj == "" {
				proj = "-"
			}
			desc := t.Description
			if t.AbortReason != nil && *t.AbortReason != "" {
				desc = fmt.Sprintf("%s (reason: %s)", desc, *t.AbortReason)
			}
			descSummary := summarizeText(desc, 50)
			titleSummary := summarizeText(t.Title, 40)

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				t.ID,
				proj,
				t.Status,
				t.Priority,
				titleSummary,
				descSummary,
				t.CreatedAt.Local().Format("2006-01-02 15:04"),
			)
		}
		w.Flush()

	case "view", "show":
		if len(cmdArgs) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: task-cli view <id>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(cmdArgs[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", cmdArgs[0])
			os.Exit(1)
		}

		store, err := getStore(dbPathFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		t, err := store.GetTaskByID(id)
		if err != nil {
			if errors.Is(err, db.ErrTaskNotFound) {
				fmt.Fprintf(os.Stderr, "Error: task with ID %d not found\n", id)
			} else {
				fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
			}
			os.Exit(1)
		}

		proj := t.ProjectCode
		if proj == "" {
			proj = "-"
		}

		fmt.Printf("Task #%d: %s\n", t.ID, t.Title)
		fmt.Printf("--------------------------------------------------\n")
		fmt.Printf("Project:    %s\n", proj)
		fmt.Printf("Status:     %s\n", t.Status)
		fmt.Printf("Priority:   %s\n", t.Priority)
		fmt.Printf("Created At: %s\n", t.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated At: %s\n", t.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
		if t.AbortReason != nil && *t.AbortReason != "" {
			fmt.Printf("Abort Note: %s\n", *t.AbortReason)
		}
		fmt.Printf("\nDescription:\n")
		if strings.TrimSpace(t.Description) == "" {
			fmt.Println("(No description provided)")
		} else {
			fmt.Println(t.Description)
		}

	case "update":
		fmt.Printf("[TODO] Command 'update' called with args: %v (not implemented yet)\n", cmdArgs)

	case "delete":
		fmt.Printf("[TODO] Command 'delete' called with args: %v (not implemented yet)\n", cmdArgs)

	case "set-pending":
		fmt.Printf("[TODO] Command 'set-pending' called with args: %v (not implemented yet)\n", cmdArgs)

	case "set-in-progress":
		fmt.Printf("[TODO] Command 'set-in-progress' called with args: %v (not implemented yet)\n", cmdArgs)

	case "set-done":
		fmt.Printf("[TODO] Command 'set-done' called with args: %v (not implemented yet)\n", cmdArgs)

	case "set-abort":
		fmt.Printf("[TODO] Command 'set-abort' called with args: %v (not implemented yet)\n", cmdArgs)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}
