package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"

	"task-cli/internal/db"
)

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

// splitFlagsAndPosArgs separa argumentos de flags dos argumentos posicionais,
// permitindo que flags sejam passadas antes, entre ou depois dos argumentos posicionais.
func splitFlagsAndPosArgs(args []string, valueFlags map[string]bool) ([]string, []string) {
	var flagArgs []string
	var posArgs []string
	var endOfFlags bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags {
			posArgs = append(posArgs, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if strings.Contains(arg, "=") {
				flagArgs = append(flagArgs, arg)
				continue
			}
			flagArgs = append(flagArgs, arg)
			if valueFlags[arg] {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}
	return flagArgs, posArgs
}

// TaskAdd adiciona uma nova tarefa ao banco de dados ou cria um projeto caso a flag -p seja informada.
func TaskAdd(ctx *Context, args []string) error {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	var isProject bool
	var projectCode string
	var priority string

	addCmd.BoolVar(&isProject, "p", false, "Create a project instead of a task")
	addCmd.BoolVar(&isProject, "project", false, "Create a project instead of a task")
	addCmd.StringVar(&projectCode, "f", "", "Project code to link task to (shorthand)")
	addCmd.StringVar(&projectCode, "filter", "", "Project code to link task to")
	addCmd.StringVar(&projectCode, "proj", "", "Project code to link task to")
	addCmd.StringVar(&priority, "priority", "medium", "Task priority (e.g. low, medium, high)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-f":         true,
		"--filter":   true,
		"--proj":     true,
		"--project":  true,
		"--priority": true,
	})

	if err := addCmd.Parse(flagArgs); err != nil {
		return err
	}

	if isProject {
		return ProjectAdd(ctx, posArgs)
	}

	if len(posArgs) < 1 {
		return fmt.Errorf("'title' is required.\nUsage: task-cli add [-f <code>] [--priority <prio>] <title> [description]")
	}

	title := strings.TrimSpace(posArgs[0])
	var description string
	if len(posArgs) > 1 {
		description = strings.TrimSpace(strings.Join(posArgs[1:], " "))
	}

	var projectID *int64
	var projectRef string
	if projectCode != "" {
		proj, err := ctx.Store.GetProjectByCode(projectCode)
		if err != nil {
			if errors.Is(err, db.ErrProjectNotFound) {
				return fmt.Errorf("project with code '%s' not found", projectCode)
			}
			return err
		}
		projectID = &proj.ID
		projectRef = proj.Code
	}

	task, err := ctx.Store.Add(title, description, projectID, priority)
	if err != nil {
		return fmt.Errorf("creating task: %w", err)
	}

	if projectRef != "" {
		fmt.Printf("Task created successfully (ID: %d) in project '%s'\n", task.ID, projectRef)
	} else {
		fmt.Printf("Task created successfully (ID: %d)\n", task.ID)
	}
	return nil
}

// TaskList lista tarefas filtrando por status e/ou projeto, ou lista projetos caso a flag -p seja informada.
func TaskList(ctx *Context, args []string) error {
	listCmd := flag.NewFlagSet("list", flag.ContinueOnError)
	var isProject bool
	var projectCode string
	var statusFlag string

	listCmd.BoolVar(&isProject, "p", false, "List all projects")
	listCmd.BoolVar(&isProject, "projects", false, "List all projects")
	listCmd.StringVar(&projectCode, "f", "", "Filter tasks by project code (shorthand)")
	listCmd.StringVar(&projectCode, "filter", "", "Filter tasks by project code")
	listCmd.StringVar(&projectCode, "project", "", "Filter tasks by project code")
	listCmd.StringVar(&statusFlag, "s", "", "Filter tasks by status (shorthand)")
	listCmd.StringVar(&statusFlag, "status", "", "Filter tasks by status")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-f":        true,
		"--filter":  true,
		"--project": true,
		"-s":        true,
		"--status":  true,
	})

	if err := listCmd.Parse(flagArgs); err != nil {
		return err
	}

	if isProject {
		return ProjectList(ctx, posArgs)
	}

	statusFilter := strings.ToLower(strings.TrimSpace(statusFlag))
	if len(posArgs) > 0 && statusFilter == "" {
		statusFilter = strings.ToLower(strings.TrimSpace(posArgs[0]))
	}

	if statusFilter != "" {
		switch db.Status(statusFilter) {
		case db.StatusPending, db.StatusInProgress, db.StatusDone, db.StatusAbort:
		default:
			return fmt.Errorf("invalid status '%s'. Valid options: pending, in-progress, done, abort", statusFilter)
		}
	}

	tasks, err := ctx.Store.List(statusFilter, projectCode)
	if err != nil {
		return fmt.Errorf("querying tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
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
	return w.Flush()
}

// TaskView exibe os detalhes completos de uma tarefa.
func TaskView(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli view <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	t, err := ctx.Store.GetTaskByID(id)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", id)
		}
		return fmt.Errorf("database error: %w", err)
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
	return nil
}

// TaskUpdate atualiza o título, descrição, projeto ou prioridade de uma tarefa existente.
func TaskUpdate(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli update <id> [-f <code>] [--priority <prio>] [title] [description]")
	}

	updateCmd := flag.NewFlagSet("update", flag.ContinueOnError)
	var projectCode string
	var priority string
	updateCmd.StringVar(&projectCode, "f", "", "Move task to project code (or 'none' to detach)")
	updateCmd.StringVar(&projectCode, "filter", "", "Move task to project code (or 'none' to detach)")
	updateCmd.StringVar(&projectCode, "p", "", "Move task to project code (or 'none' to detach)")
	updateCmd.StringVar(&projectCode, "project", "", "Move task to project code (or 'none' to detach)")
	updateCmd.StringVar(&priority, "priority", "", "Update task priority (e.g. low, medium, high)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-f":         true,
		"--filter":   true,
		"-p":         true,
		"--project":  true,
		"--priority": true,
	})

	if err := updateCmd.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli update <id> [-f <code>] [--priority <prio>] [title] [description]")
	}

	idStr := posArgs[0]
	posArgs = posArgs[1:]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", idStr)
	}

	existing, err := ctx.Store.GetTaskByID(id)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	newTitle := existing.Title
	newDesc := existing.Description
	newProjectID := existing.ProjectID
	newPriority := existing.Priority

	if len(posArgs) > 0 {
		newTitle = strings.TrimSpace(posArgs[0])
	}
	if len(posArgs) > 1 {
		newDesc = strings.TrimSpace(strings.Join(posArgs[1:], " "))
	}

	if projectCode != "" {
		cleanProj := strings.Trim(projectCode, ", ")
		if strings.EqualFold(cleanProj, "none") || cleanProj == "-" {
			newProjectID = nil
		} else {
			proj, err := ctx.Store.GetProjectByCode(cleanProj)
			if err != nil {
				if errors.Is(err, db.ErrProjectNotFound) {
					return fmt.Errorf("project with code '%s' not found", cleanProj)
				}
				return fmt.Errorf("database error: %w", err)
			}
			newProjectID = &proj.ID
		}
	}

	if priority != "" {
		newPriority = strings.TrimSpace(priority)
	}

	if err := ctx.Store.UpdateTaskDetails(id, newTitle, newDesc, newProjectID, newPriority); err != nil {
		return fmt.Errorf("updating task: %w", err)
	}

	fmt.Printf("Task %d updated successfully\n", id)
	return nil
}

// TaskEdit abre o editor externo para editar a descrição de uma tarefa.
func TaskEdit(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli edit <id>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	t, err := ctx.Store.GetTaskByID(id)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "nano"
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("task-cli-%d-*.md", t.ID))
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(t.Description); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing to temporary file: %w", err)
	}
	tmpFile.Close()

	editorParts := strings.Fields(editor)
	var cmd *exec.Cmd
	if len(editorParts) > 1 {
		cmdArgsList := append(editorParts[1:], tmpPath)
		cmd = exec.Command(editorParts[0], cmdArgsList...)
	} else {
		cmd = exec.Command(editor, tmpPath)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor '%s' exited with error: %w", editor, err)
	}

	updatedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading updated file: %w", err)
	}

	newDesc := strings.TrimRight(string(updatedBytes), "\r\n ")
	if newDesc == strings.TrimRight(t.Description, "\r\n ") {
		fmt.Println("No changes made.")
		return nil
	}

	if err := ctx.Store.UpdateDescription(id, newDesc); err != nil {
		return fmt.Errorf("updating task description: %w", err)
	}

	fmt.Printf("Task %d description updated successfully\n", id)
	return nil
}

// TaskDelete remove logicamente (soft delete) uma tarefa pelo ID.
func TaskDelete(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli delete <id>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	if err := ctx.Store.Delete(id); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found or already deleted", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	fmt.Printf("Task %d soft deleted successfully\n", id)
	return nil
}

func setTaskStatus(ctx *Context, args []string, status db.Status, cmdName string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli %s <id>", cmdName)
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	if err := ctx.Store.UpdateStatus(id, status, nil); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	fmt.Printf("Task %d status set to '%s'\n", id, status)
	return nil
}

// TaskSetPending define o status da tarefa como 'pending'.
func TaskSetPending(ctx *Context, args []string) error {
	return setTaskStatus(ctx, args, db.StatusPending, "set-pending")
}

// TaskSetInProgress define o status da tarefa como 'in-progress'.
func TaskSetInProgress(ctx *Context, args []string) error {
	return setTaskStatus(ctx, args, db.StatusInProgress, "set-in-progress")
}

// TaskSetDone define o status da tarefa como 'done'.
func TaskSetDone(ctx *Context, args []string) error {
	return setTaskStatus(ctx, args, db.StatusDone, "set-done")
}

// TaskSetAbort define o status da tarefa como 'abort' com motivo opcional.
func TaskSetAbort(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli set-abort <id> [reason]")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	var abortReason *string
	if len(args) > 1 {
		reason := strings.TrimSpace(strings.Join(args[1:], " "))
		if reason != "" {
			abortReason = &reason
		}
	}

	if err := ctx.Store.UpdateStatus(id, db.StatusAbort, abortReason); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	if abortReason != nil {
		fmt.Printf("Task %d status set to 'abort' (reason: %s)\n", id, *abortReason)
	} else {
		fmt.Printf("Task %d status set to 'abort'\n", id)
	}
	return nil
}



