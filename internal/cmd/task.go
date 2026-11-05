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

func cleanOneLine(text string) string {
	clean := strings.ReplaceAll(text, "\r\n", " ")
	clean = strings.ReplaceAll(clean, "\n", " ")
	clean = strings.ReplaceAll(clean, "\t", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		return "-"
	}
	return clean
}

func summarizeText(text string, maxLen int) string {
	clean := cleanOneLine(text)
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

// TaskAdd adiciona uma nova tarefa ao banco de dados.
func TaskAdd(ctx *Context, args []string) error {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	var projectCode string
	var priority string

	addCmd.StringVar(&projectCode, "f", "", "Project code to link task to (shorthand)")
	addCmd.StringVar(&projectCode, "filter", "", "Project code to link task to")
	addCmd.StringVar(&projectCode, "proj", "", "Project code to link task to")
	addCmd.StringVar(&priority, "priority", "medium", "Task priority (e.g. low, medium, high)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-f":         true,
		"--filter":   true,
		"--proj":     true,
		"--priority": true,
	})

	if err := addCmd.Parse(flagArgs); err != nil {
		return err
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

// TaskList lista tarefas filtrando por status e/ou projeto.
// Por padrão, lista apenas tarefas com status 'pending'.
func TaskList(ctx *Context, args []string) error {
	listCmd := flag.NewFlagSet("list", flag.ContinueOnError)
	var showTime bool
	var showDesc bool
	var showAll bool
	var showDone bool
	var projectCode string
	var statusFlag string

	listCmd.BoolVar(&showTime, "t", false, "Show creation dates")
	listCmd.BoolVar(&showTime, "time", false, "Show creation dates")
	listCmd.BoolVar(&showDesc, "d", false, "Show task descriptions")
	listCmd.BoolVar(&showDesc, "desc", false, "Show task descriptions")
	listCmd.BoolVar(&showDesc, "description", false, "Show task descriptions")
	listCmd.BoolVar(&showAll, "a", false, "List all tasks (any status)")
	listCmd.BoolVar(&showAll, "all", false, "List all tasks (any status)")
	listCmd.BoolVar(&showDone, "done", false, "List only completed (done) tasks")
	listCmd.BoolVar(&showDone, "D", false, "List only completed (done) tasks (shorthand)")
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

	var statusFilter string
	posStatus := strings.ToLower(strings.TrimSpace(statusFlag))
	if len(posArgs) > 0 && posStatus == "" {
		posStatus = strings.ToLower(strings.TrimSpace(posArgs[0]))
	}

	if showAll || posStatus == "all" {
		statusFilter = "" // vazio lista todas no Store.List
	} else if showDone || posStatus == "done" {
		statusFilter = string(db.StatusDone)
	} else if posStatus != "" {
		switch db.Status(posStatus) {
		case db.StatusPending, db.StatusInProgress, db.StatusDone, db.StatusAbort:
			statusFilter = posStatus
		default:
			return fmt.Errorf("invalid status '%s'. Valid options: all, pending, in-progress, done, abort", posStatus)
		}
	} else {
		// Padrão: somente tarefas pendentes
		statusFilter = string(db.StatusPending)
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
	headers := []string{"ID", "PROJECT", "STATUS", "PRIO", "TITLE"}
	dividers := []string{"----", "-------", "------", "----", "-----"}
	if showDesc {
		headers = append(headers, "DESCRIPTION")
		dividers = append(dividers, "-----------")
	}
	if showTime {
		headers = append(headers, "CREATED AT")
		dividers = append(dividers, "----------")
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dividers, "\t"))

	for _, t := range tasks {
		proj := t.ProjectCode
		if proj == "" {
			proj = "-"
		}
		titleClean := cleanOneLine(t.Title)

		row := []string{
			fmt.Sprintf("%04d", t.ID),
			proj,
			string(t.Status),
			t.Priority,
			titleClean,
		}

		if showDesc {
			desc := t.Description
			if t.AbortReason != nil && *t.AbortReason != "" {
				desc = fmt.Sprintf("%s (reason: %s)", desc, *t.AbortReason)
			}
			row = append(row, summarizeText(desc, 50))
		}

		if showTime {
			row = append(row, t.CreatedAt.Local().Format("2006-01-02 15:04"))
		}

		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

// TaskView exibe os detalhes completos de uma tarefa.
func TaskView(ctx *Context, args []string) error {
	viewCmd := flag.NewFlagSet("view", flag.ContinueOnError)
	var showTime bool
	viewCmd.BoolVar(&showTime, "t", false, "Show timestamps (created_at, updated_at)")
	viewCmd.BoolVar(&showTime, "time", false, "Show timestamps (created_at, updated_at)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	if err := viewCmd.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli view <id> [-t]")
	}
	id, err := strconv.ParseInt(posArgs[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", posArgs[0])
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

	fmt.Printf("Task #%04d: %s\n", t.ID, t.Title)
	fmt.Printf("--------------------------------------------------\n")
	fmt.Printf("Project:    %s\n", proj)
	fmt.Printf("Status:     %s\n", t.Status)
	fmt.Printf("Priority:   %s\n", t.Priority)
	if showTime {
		fmt.Printf("Created At: %s\n", t.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated At: %s\n", t.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
	}
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

	fmt.Printf("Task %04d status set to '%s'\n", id, status)
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
		fmt.Printf("Task %04d status set to 'abort' (reason: %s)\n", id, *abortReason)
	} else {
		fmt.Printf("Task %04d status set to 'abort'\n", id)
	}
	return nil
}

// TaskSetPriority define a prioridade de uma tarefa existente (low, medium, high, urgent).
func TaskSetPriority(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("task ID and priority are required.\nUsage: task-cli set-prio <id> <priority>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	prio := strings.ToLower(strings.TrimSpace(args[1]))
	switch prio {
	case "low", "medium", "high", "urgent":
	default:
		return fmt.Errorf("invalid priority '%s'. Valid options: low, medium, high, urgent", prio)
	}

	if err := ctx.Store.UpdatePriority(id, prio); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	fmt.Printf("Task %04d priority set to '%s'\n", id, prio)
	return nil
}
