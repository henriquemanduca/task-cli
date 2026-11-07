package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

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

// -----------------------------------------------------------------------------
// TaskAdd
// -----------------------------------------------------------------------------

// TaskAddArgs represents the argument schema for adding a new task.
type TaskAddArgs struct {
	Title       string
	Description string
	ProjectID   *int64
	Priority    string
}

// TaskAddResult holds the created task and associated project reference.
type TaskAddResult struct {
	Task       *db.Task
	ProjectRef string
}

// TaskAddAction encapsulates schema parsing, execution, and output formatting for task creation.
type TaskAddAction struct{}

// Parse parses flags and positional arguments for creating a task.
func (a *TaskAddAction) Parse(ctx *Context, args []string) (TaskAddArgs, error) {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	var projectStr string
	var priority string

	addCmd.StringVar(&projectStr, "p", "", "Project ID to link task to (shorthand)")
	addCmd.StringVar(&projectStr, "proj", "", "Project ID to link task to")
	addCmd.StringVar(&projectStr, "project", "", "Project ID to link task to")
	addCmd.StringVar(&priority, "priority", "medium", "Task priority (e.g. low, medium, high)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-p":         true,
		"--proj":     true,
		"--project":  true,
		"--priority": true,
	})

	if err := addCmd.Parse(flagArgs); err != nil {
		return TaskAddArgs{}, err
	}

	if len(posArgs) < 1 {
		return TaskAddArgs{}, fmt.Errorf("'title' is required.\nUsage: task-cli add [-p <id>] [--priority <prio>] <title> [description]")
	}

	var projectID *int64
	if projectStr != "" {
		id, err := strconv.ParseInt(projectStr, 10, 64)
		if err != nil {
			return TaskAddArgs{}, fmt.Errorf("invalid project ID: %s", projectStr)
		}
		projectID = &id
	}

	title := strings.TrimSpace(posArgs[0])
	var description string
	if len(posArgs) > 1 {
		description = strings.Join(posArgs[1:], " ")
	}

	return TaskAddArgs{
		Title:       title,
		Description: description,
		ProjectID:   projectID,
		Priority:    priority,
	}, nil
}

// Execute persists the new task into the database.
func (a *TaskAddAction) Execute(ctx *Context, args TaskAddArgs) (*TaskAddResult, error) {
	var projectRef string
	if args.ProjectID != nil {
		proj, err := ctx.Store.GetProjectByID(*args.ProjectID)
		if err != nil {
			if errors.Is(err, db.ErrProjectNotFound) {
				return nil, fmt.Errorf("project with ID %d not found", *args.ProjectID)
			}
			return nil, err
		}
		projectRef = proj.Code
	}

	task, err := ctx.Store.Add(args.Title, args.Description, args.ProjectID, args.Priority)
	if err != nil {
		return nil, fmt.Errorf("creating task: %w", err)
	}

	return &TaskAddResult{
		Task:       task,
		ProjectRef: projectRef,
	}, nil
}

// Format renders task creation confirmation.
func (a *TaskAddAction) Format(ctx *Context, w io.Writer, res *TaskAddResult) error {
	if res.ProjectRef != "" {
		_, err := fmt.Fprintf(w, "Task created successfully (ID: %d) in project '%s'\n", res.Task.ID, res.ProjectRef)
		return err
	}
	_, err := fmt.Fprintf(w, "Task created successfully (ID: %d)\n", res.Task.ID)
	return err
}

// TaskAddHandler is the standardized handler instance for TaskAddAction.
var TaskAddHandler = NewActionHandler(&TaskAddAction{})

// TaskAdd adiciona uma nova tarefa ao banco de dados.
func TaskAdd(ctx *Context, args []string) error {
	return TaskAddHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskList
// -----------------------------------------------------------------------------

// TaskListArgs represents the argument schema for listing tasks.
type TaskListArgs struct {
	StatusFilter string
	ProjectCode  string
	ShowTime     bool
	NoHeader     bool
	Format       string
}

// TaskListResult holds query results and formatting options for task listing.
type TaskListResult struct {
	Tasks    []db.Task
	ShowTime bool
	NoHeader bool
	Format   string
}

// TaskListAction encapsulates schema parsing, execution, and output formatting for listing tasks.
type TaskListAction struct{}

// Parse parses arguments and flags for listing tasks.
func (a *TaskListAction) Parse(ctx *Context, args []string) (TaskListArgs, error) {
	listCmd := flag.NewFlagSet("list", flag.ContinueOnError)
	var showTime bool
	var showAll bool
	var showDone bool
	var noHeader bool
	var projectCode string
	var statusFlag string
	var format string

	defaultFormat := FormatTSV
	if ctx != nil && ctx.Format != "" {
		defaultFormat = ctx.Format
	}

	listCmd.BoolVar(&showTime, "t", false, "Include creation date column")
	listCmd.BoolVar(&showTime, "time", false, "Include creation date column")
	listCmd.BoolVar(&showAll, "a", false, "List all tasks (any status)")
	listCmd.BoolVar(&showAll, "all", false, "List all tasks (any status)")
	listCmd.BoolVar(&showDone, "done", false, "List only completed (done) tasks")
	listCmd.BoolVar(&showDone, "D", false, "List only completed (done) tasks (shorthand)")
	listCmd.BoolVar(&noHeader, "n", false, "Suppress header row")
	listCmd.BoolVar(&noHeader, "no-header", false, "Suppress header row")
	listCmd.StringVar(&format, "format", defaultFormat, "Output format (tsv, table)")
	listCmd.StringVar(&format, "F", defaultFormat, "Output format (shorthand)")
	listCmd.StringVar(&projectCode, "p", "", "Filter tasks by project code (shorthand)")
	listCmd.StringVar(&projectCode, "project", "", "Filter tasks by project code")
	listCmd.StringVar(&statusFlag, "s", "", "Filter tasks by status (shorthand)")
	listCmd.StringVar(&statusFlag, "status", "", "Filter tasks by status")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-p":        true,
		"--project": true,
		"-s":        true,
		"--status":  true,
		"-F":        true,
		"--format":  true,
	})

	if err := listCmd.Parse(flagArgs); err != nil {
		return TaskListArgs{}, err
	}

	resolvedFmt, err := validateFormat(format)
	if err != nil {
		return TaskListArgs{}, err
	}

	var statusFilter string
	posStatus := strings.ToLower(strings.TrimSpace(statusFlag))
	if len(posArgs) > 0 && posStatus == "" {
		posStatus = strings.ToLower(strings.TrimSpace(posArgs[0]))
	}

	if showAll || posStatus == "all" {
		statusFilter = ""
	} else if showDone || posStatus == "done" {
		statusFilter = string(db.StatusDone)
	} else if posStatus != "" {
		switch db.Status(posStatus) {
		case db.StatusPending, db.StatusInProgress, db.StatusDone, db.StatusAbort:
			statusFilter = posStatus
		default:
			return TaskListArgs{}, fmt.Errorf("invalid status '%s'. Valid options: all, pending, in-progress, done, abort", posStatus)
		}
	} else {
		statusFilter = string(db.StatusPending)
	}

	return TaskListArgs{
		StatusFilter: statusFilter,
		ProjectCode:  projectCode,
		ShowTime:     showTime,
		NoHeader:     noHeader,
		Format:       resolvedFmt,
	}, nil
}

// Execute queries tasks from the database according to filters.
func (a *TaskListAction) Execute(ctx *Context, args TaskListArgs) (*TaskListResult, error) {
	tasks, err := ctx.Store.List(args.StatusFilter, args.ProjectCode)
	if err != nil {
		return nil, fmt.Errorf("querying tasks: %w", err)
	}

	return &TaskListResult{
		Tasks:    tasks,
		ShowTime: args.ShowTime,
		NoHeader: args.NoHeader,
		Format:   args.Format,
	}, nil
}

// Format renders the tasks list using TSV or table formatting.
func (a *TaskListAction) Format(ctx *Context, w io.Writer, res *TaskListResult) error {
	if len(res.Tasks) == 0 {
		return nil
	}

	cols := []string{"id", "project", "status", "priority", "title"}
	if res.ShowTime {
		cols = append(cols, "created_at")
	}

	var rows [][]string
	for _, t := range res.Tasks {
		proj := t.ProjectCode
		if proj == "" {
			proj = "-"
		}

		row := []string{
			fmt.Sprintf("%d", t.ID),
			proj,
			string(t.Status),
			t.Priority,
			cleanOneLine(t.Title),
		}

		if res.ShowTime {
			row = append(row, t.CreatedAt.Local().Format("2006-01-02T15:04:05"))
		}

		rows = append(rows, row)
	}

	return printOutput(w, res.Format, res.NoHeader, cols, rows)
}

// TaskListHandler is the standardized handler instance for TaskListAction.
var TaskListHandler = NewActionHandler(&TaskListAction{})

// TaskList lista tarefas filtrando por status e/ou projeto.
func TaskList(ctx *Context, args []string) error {
	return TaskListHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskView
// -----------------------------------------------------------------------------

// TaskViewArgs represents the argument schema for viewing a task.
type TaskViewArgs struct {
	ID       int64
	ShowTime bool
}

// TaskViewResult holds task details, documents, and display settings.
type TaskViewResult struct {
	Task      *db.Task
	Documents []db.Document
	ShowTime  bool
}

// TaskViewAction encapsulates schema parsing, execution, and output formatting for task details.
type TaskViewAction struct{}

// Parse parses arguments and flags for viewing a task.
func (a *TaskViewAction) Parse(ctx *Context, args []string) (TaskViewArgs, error) {
	viewCmd := flag.NewFlagSet("view", flag.ContinueOnError)
	var showTime bool
	viewCmd.BoolVar(&showTime, "t", false, "Show timestamps (created_at, updated_at)")
	viewCmd.BoolVar(&showTime, "time", false, "Show timestamps (created_at, updated_at)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	if err := viewCmd.Parse(flagArgs); err != nil {
		return TaskViewArgs{}, err
	}

	if len(posArgs) < 1 {
		return TaskViewArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli view <id> [-t]")
	}
	id, err := strconv.ParseInt(posArgs[0], 10, 64)
	if err != nil {
		return TaskViewArgs{}, fmt.Errorf("invalid task ID: %s", posArgs[0])
	}

	return TaskViewArgs{
		ID:       id,
		ShowTime: showTime,
	}, nil
}

// Execute retrieves task information and attached documents from the database.
func (a *TaskViewAction) Execute(ctx *Context, args TaskViewArgs) (*TaskViewResult, error) {
	t, err := ctx.Store.GetTaskByID(args.ID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	docs, _ := ctx.Store.ListDocuments(t.ID)

	return &TaskViewResult{
		Task:      t,
		Documents: docs,
		ShowTime:  args.ShowTime,
	}, nil
}

// Format renders the full task details view.
func (a *TaskViewAction) Format(ctx *Context, w io.Writer, res *TaskViewResult) error {
	t := res.Task
	proj := t.ProjectCode
	if proj == "" {
		proj = "-"
	}

	fmt.Fprintf(w, "Task: %04d - %s\n", t.ID, t.Title)
	fmt.Fprintf(w, "Project: %s\n", proj)
	fmt.Fprintf(w, "Status: %s\n", t.Status)
	fmt.Fprintf(w, "Priority: %s\n", t.Priority)
	if res.ShowTime {
		fmt.Fprintf(w, "Created At: %s\n", t.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(w, "Updated At: %s\n", t.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if t.AbortReason != nil && *t.AbortReason != "" {
		fmt.Fprintf(w, "Abort Note: %s\n", *t.AbortReason)
	}
	fmt.Fprintf(w, "Description:\n")
	if strings.TrimSpace(t.Description) == "" {
		fmt.Fprintln(w, "(No description provided)")
	} else {
		fmt.Fprintln(w, t.Description)
	}

	if len(res.Documents) > 0 {
		fmt.Fprintf(w, "\nAttached Documents (%d):\n", len(res.Documents))
		for _, d := range res.Documents {
			fmt.Fprintf(w, "  [%d] %s (%s): %s\n", d.ID, d.Name, d.Type, d.FilePath)
		}
	}

	return nil
}

// TaskViewHandler is the standardized handler instance for TaskViewAction.
var TaskViewHandler = NewActionHandler(&TaskViewAction{})

// TaskView exibe os detalhes completos de uma tarefa.
func TaskView(ctx *Context, args []string) error {
	return TaskViewHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskUpdate
// -----------------------------------------------------------------------------

// TaskUpdateArgs represents the argument schema for updating an existing task.
type TaskUpdateArgs struct {
	ID          int64
	Title       string
	Description string
	ProjectID   *int64
	ClearProj   bool
	Priority    string
	HasTitle    bool
	HasDesc     bool
}

// TaskUpdateResult holds the ID of the updated task.
type TaskUpdateResult struct {
	ID int64
}

// TaskUpdateAction encapsulates schema parsing, execution, and output formatting for updating tasks.
type TaskUpdateAction struct{}

// Parse parses arguments and flags for updating a task.
func (a *TaskUpdateAction) Parse(ctx *Context, args []string) (TaskUpdateArgs, error) {
	if len(args) < 1 {
		return TaskUpdateArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli update <id> [-p <id>] [--priority <prio>] [title] [description]")
	}

	updateCmd := flag.NewFlagSet("update", flag.ContinueOnError)
	var projectStr string
	var priority string
	updateCmd.StringVar(&projectStr, "p", "", "Move task to project ID (or 'none' to detach)")
	updateCmd.StringVar(&projectStr, "project", "", "Move task to project ID (or 'none' to detach)")
	updateCmd.StringVar(&priority, "priority", "", "Update task priority (e.g. low, medium, high)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-p":         true,
		"--project":  true,
		"--priority": true,
	})

	if err := updateCmd.Parse(flagArgs); err != nil {
		return TaskUpdateArgs{}, err
	}

	if len(posArgs) < 1 {
		return TaskUpdateArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli update <id> [-p <id>] [--priority <prio>] [title] [description]")
	}

	idStr := posArgs[0]
	posArgs = posArgs[1:]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return TaskUpdateArgs{}, fmt.Errorf("invalid task ID: %s", idStr)
	}

	var title string
	var hasTitle bool
	if len(posArgs) > 0 {
		title = strings.TrimSpace(posArgs[0])
		hasTitle = true
	}

	var desc string
	var hasDesc bool
	if len(posArgs) > 1 {
		desc = strings.TrimSpace(strings.Join(posArgs[1:], " "))
		hasDesc = true
	}

	var parsedProjectID *int64
	var clearProj bool
	if projectStr != "" {
		cleanProj := strings.Trim(projectStr, ", ")
		if strings.EqualFold(cleanProj, "none") || cleanProj == "-" {
			clearProj = true
		} else {
			pID, err := strconv.ParseInt(cleanProj, 10, 64)
			if err != nil {
				return TaskUpdateArgs{}, fmt.Errorf("invalid project ID: %s", projectStr)
			}
			parsedProjectID = &pID
		}
	}

	return TaskUpdateArgs{
		ID:          id,
		Title:       title,
		Description: desc,
		ProjectID:   parsedProjectID,
		ClearProj:   clearProj,
		Priority:    priority,
		HasTitle:    hasTitle,
		HasDesc:     hasDesc,
	}, nil
}

// Execute updates task details in the database.
func (a *TaskUpdateAction) Execute(ctx *Context, args TaskUpdateArgs) (*TaskUpdateResult, error) {
	existing, err := ctx.Store.GetTaskByID(args.ID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	newTitle := existing.Title
	newDesc := existing.Description
	newProjectID := existing.ProjectID
	newPriority := existing.Priority

	if args.HasTitle {
		newTitle = args.Title
	}
	if args.HasDesc {
		newDesc = args.Description
	}

	if args.ClearProj {
		newProjectID = nil
	} else if args.ProjectID != nil {
		proj, err := ctx.Store.GetProjectByID(*args.ProjectID)
		if err != nil {
			if errors.Is(err, db.ErrProjectNotFound) {
				return nil, fmt.Errorf("project with ID %d not found", *args.ProjectID)
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
		newProjectID = &proj.ID
	}

	if args.Priority != "" {
		newPriority = strings.TrimSpace(args.Priority)
	}

	if err := ctx.Store.UpdateTaskDetails(args.ID, newTitle, newDesc, newProjectID, newPriority); err != nil {
		return nil, fmt.Errorf("updating task: %w", err)
	}

	return &TaskUpdateResult{ID: args.ID}, nil
}

// Format renders the update confirmation message.
func (a *TaskUpdateAction) Format(ctx *Context, w io.Writer, res *TaskUpdateResult) error {
	_, err := fmt.Fprintf(w, "Task %d updated successfully\n", res.ID)
	return err
}

// TaskUpdateHandler is the standardized handler instance for TaskUpdateAction.
var TaskUpdateHandler = NewActionHandler(&TaskUpdateAction{})

// TaskUpdate atualiza o título, descrição, projeto ou prioridade de uma tarefa existente.
func TaskUpdate(ctx *Context, args []string) error {
	return TaskUpdateHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskEdit
// -----------------------------------------------------------------------------

// TaskEditArgs represents the argument schema for editing a task description.
type TaskEditArgs struct {
	ID int64
}

// TaskEditResult holds the task ID and whether changes were applied.
type TaskEditResult struct {
	ID      int64
	Changed bool
}

// TaskEditAction encapsulates schema parsing, execution, and output formatting for description editing.
type TaskEditAction struct{}

// Parse validates the task ID argument for editing.
func (a *TaskEditAction) Parse(ctx *Context, args []string) (TaskEditArgs, error) {
	if len(args) < 1 {
		return TaskEditArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli edit <id>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return TaskEditArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
	}

	return TaskEditArgs{ID: id}, nil
}

// Execute opens an external text editor and saves modified task descriptions.
func (a *TaskEditAction) Execute(ctx *Context, args TaskEditArgs) (*TaskEditResult, error) {
	t, err := ctx.Store.GetTaskByID(args.ID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
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
		return nil, fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(t.Description); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("writing to temporary file: %w", err)
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
		return nil, fmt.Errorf("editor '%s' exited with error: %w", editor, err)
	}

	updatedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("reading updated file: %w", err)
	}

	newDesc := strings.TrimRight(string(updatedBytes), "\r\n ")
	if newDesc == strings.TrimRight(t.Description, "\r\n ") {
		return &TaskEditResult{ID: args.ID, Changed: false}, nil
	}

	if err := ctx.Store.UpdateDescription(args.ID, newDesc); err != nil {
		return nil, fmt.Errorf("updating task description: %w", err)
	}

	return &TaskEditResult{ID: args.ID, Changed: true}, nil
}

// Format renders the result of task description editing.
func (a *TaskEditAction) Format(ctx *Context, w io.Writer, res *TaskEditResult) error {
	if !res.Changed {
		fmt.Fprintln(w, "No changes made.")
		return nil
	}
	_, err := fmt.Fprintf(w, "Task %d description updated successfully\n", res.ID)
	return err
}

// TaskEditHandler is the standardized handler instance for TaskEditAction.
var TaskEditHandler = NewActionHandler(&TaskEditAction{})

// TaskEdit abre o editor externo para editar a descrição de uma tarefa.
func TaskEdit(ctx *Context, args []string) error {
	return TaskEditHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskDelete
// -----------------------------------------------------------------------------

// TaskDeleteArgs represents the argument schema for deleting a task.
type TaskDeleteArgs struct {
	ID int64
}

// TaskDeleteResult holds the deleted task ID.
type TaskDeleteResult struct {
	ID int64
}

// TaskDeleteAction encapsulates schema parsing, execution, and output formatting for task soft deletion.
type TaskDeleteAction struct{}

// Parse parses and validates the task ID argument for deletion.
func (a *TaskDeleteAction) Parse(ctx *Context, args []string) (TaskDeleteArgs, error) {
	if len(args) < 1 {
		return TaskDeleteArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli delete <id>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return TaskDeleteArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
	}

	return TaskDeleteArgs{ID: id}, nil
}

// Execute soft-deletes the task in the database.
func (a *TaskDeleteAction) Execute(ctx *Context, args TaskDeleteArgs) (*TaskDeleteResult, error) {
	if err := ctx.Store.Delete(args.ID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found or already deleted", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &TaskDeleteResult{ID: args.ID}, nil
}

// Format renders the deletion confirmation message.
func (a *TaskDeleteAction) Format(ctx *Context, w io.Writer, res *TaskDeleteResult) error {
	_, err := fmt.Fprintf(w, "Task %d soft deleted successfully\n", res.ID)
	return err
}

// TaskDeleteHandler is the standardized handler instance for TaskDeleteAction.
var TaskDeleteHandler = NewActionHandler(&TaskDeleteAction{})

// TaskDelete remove logicamente (soft delete) uma tarefa pelo ID.
func TaskDelete(ctx *Context, args []string) error {
	return TaskDeleteHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskStatus (query & update)
// -----------------------------------------------------------------------------

// TaskStatusArgs represents the argument schema for querying or setting task status.
type TaskStatusArgs struct {
	ID      int64
	IsQuery bool
	Status  db.Status
	Reason  *string
}

// TaskStatusResult holds the outcome of a task status query or update.
type TaskStatusResult struct {
	ID      int64
	Status  db.Status
	Reason  *string
	IsQuery bool
}

// TaskStatusAction encapsulates schema parsing, execution, and output formatting for status operations.
type TaskStatusAction struct{}

// Parse parses arguments for status query or update, supporting both argument order permutations.
func (a *TaskStatusAction) Parse(ctx *Context, args []string) (TaskStatusArgs, error) {
	if len(args) < 1 {
		return TaskStatusArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task status <id> <status> [reason]")
	}

	// 1 argument: query current status
	if len(args) == 1 {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return TaskStatusArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
		}
		return TaskStatusArgs{ID: id, IsQuery: true}, nil
	}

	// 2 or more arguments: set status
	var id int64
	var rawStatus string
	var remaining []string

	id1, err1 := strconv.ParseInt(args[0], 10, 64)
	if err1 == nil {
		id = id1
		rawStatus = args[1]
		remaining = args[2:]
	} else {
		id2, err2 := strconv.ParseInt(args[1], 10, 64)
		if err2 == nil {
			id = id2
			rawStatus = args[0]
			remaining = args[2:]
		} else {
			return TaskStatusArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
		}
	}

	statusLower := strings.ToLower(strings.TrimSpace(rawStatus))
	var targetStatus db.Status
	switch statusLower {
	case "pending", "todo", "reopen":
		targetStatus = db.StatusPending
	case "in-progress", "start", "started", "progress", "doing":
		targetStatus = db.StatusInProgress
	case "done", "complete", "completed", "finish", "finished":
		targetStatus = db.StatusDone
	case "abort", "aborted", "cancel", "cancelled":
		targetStatus = db.StatusAbort
	default:
		return TaskStatusArgs{}, fmt.Errorf("invalid status '%s'. Valid options: pending, in-progress, done, abort", rawStatus)
	}

	var abortReason *string
	if targetStatus == db.StatusAbort && len(remaining) > 0 {
		reason := strings.TrimSpace(strings.Join(remaining, " "))
		if reason != "" {
			abortReason = &reason
		}
	}

	return TaskStatusArgs{
		ID:      id,
		IsQuery: false,
		Status:  targetStatus,
		Reason:  abortReason,
	}, nil
}

// Execute queries or updates the task status in the database.
func (a *TaskStatusAction) Execute(ctx *Context, args TaskStatusArgs) (*TaskStatusResult, error) {
	if args.IsQuery {
		t, err := ctx.Store.GetTaskByID(args.ID)
		if err != nil {
			if errors.Is(err, db.ErrTaskNotFound) {
				return nil, fmt.Errorf("task with ID %d not found", args.ID)
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
		return &TaskStatusResult{
			ID:      t.ID,
			Status:  t.Status,
			Reason:  t.AbortReason,
			IsQuery: true,
		}, nil
	}

	if err := ctx.Store.UpdateStatus(args.ID, args.Status, args.Reason); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &TaskStatusResult{
		ID:      args.ID,
		Status:  args.Status,
		Reason:  args.Reason,
		IsQuery: false,
	}, nil
}

// Format renders the status query or update message.
func (a *TaskStatusAction) Format(ctx *Context, w io.Writer, res *TaskStatusResult) error {
	if res.IsQuery {
		if res.Status == db.StatusAbort && res.Reason != nil && *res.Reason != "" {
			fmt.Fprintf(w, "Task %04d status: %s (reason: %s)\n", res.ID, res.Status, *res.Reason)
		} else {
			fmt.Fprintf(w, "Task %04d status: %s\n", res.ID, res.Status)
		}
		return nil
	}

	if res.Status == db.StatusAbort && res.Reason != nil {
		fmt.Fprintf(w, "Task %04d status set to 'abort' (reason: %s)\n", res.ID, *res.Reason)
	} else {
		fmt.Fprintf(w, "Task %04d status set to '%s'\n", res.ID, res.Status)
	}
	return nil
}

// TaskStatusHandler is the standardized handler instance for TaskStatusAction.
var TaskStatusHandler = NewActionHandler(&TaskStatusAction{})

// TaskStatus altera ou exibe o status de uma tarefa.
func TaskStatus(ctx *Context, args []string) error {
	return TaskStatusHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskSetPriority
// -----------------------------------------------------------------------------

// TaskSetPriorityArgs represents the argument schema for setting task priority.
type TaskSetPriorityArgs struct {
	ID       int64
	Priority string
}

// TaskSetPriorityResult holds the task ID and new priority.
type TaskSetPriorityResult struct {
	ID       int64
	Priority string
}

// TaskSetPriorityAction encapsulates schema parsing, execution, and output formatting for priority updates.
type TaskSetPriorityAction struct{}

// Parse parses arguments for setting priority.
func (a *TaskSetPriorityAction) Parse(ctx *Context, args []string) (TaskSetPriorityArgs, error) {
	if len(args) < 2 {
		return TaskSetPriorityArgs{}, fmt.Errorf("task ID and priority are required.\nUsage: task-cli set-prio <id> <priority>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return TaskSetPriorityArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
	}

	prio := strings.ToLower(strings.TrimSpace(args[1]))
	switch prio {
	case "low", "medium", "high", "urgent":
	default:
		return TaskSetPriorityArgs{}, fmt.Errorf("invalid priority '%s'. Valid options: low, medium, high, urgent", prio)
	}

	return TaskSetPriorityArgs{
		ID:       id,
		Priority: prio,
	}, nil
}

// Execute updates task priority in the database.
func (a *TaskSetPriorityAction) Execute(ctx *Context, args TaskSetPriorityArgs) (*TaskSetPriorityResult, error) {
	if err := ctx.Store.UpdatePriority(args.ID, args.Priority); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &TaskSetPriorityResult{
		ID:       args.ID,
		Priority: args.Priority,
	}, nil
}

// Format renders priority update confirmation.
func (a *TaskSetPriorityAction) Format(ctx *Context, w io.Writer, res *TaskSetPriorityResult) error {
	_, err := fmt.Fprintf(w, "Task %04d priority set to '%s'\n", res.ID, res.Priority)
	return err
}

// TaskSetPriorityHandler is the standardized handler instance for TaskSetPriorityAction.
var TaskSetPriorityHandler = NewActionHandler(&TaskSetPriorityAction{})

// TaskSetPriority define a prioridade de uma tarefa existente.
func TaskSetPriority(ctx *Context, args []string) error {
	return TaskSetPriorityHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// Direct Status Helpers (set-pending, set-in-progress, set-done, set-abort)
// -----------------------------------------------------------------------------

// TaskSetStatusArgs represents the argument schema for dedicated status transition commands.
type TaskSetStatusArgs struct {
	ID          int64
	Status      db.Status
	Reason      *string
	CommandName string
}

// TaskSetStatusResult holds status transition details.
type TaskSetStatusResult struct {
	ID     int64
	Status db.Status
	Reason *string
}

// TaskSetStatusAction encapsulates schema parsing, execution, and output formatting for dedicated status transitions.
type TaskSetStatusAction struct {
	Status      db.Status
	CommandName string
}

// Parse parses arguments for status transitions.
func (a *TaskSetStatusAction) Parse(ctx *Context, args []string) (TaskSetStatusArgs, error) {
	if a.Status == db.StatusAbort {
		if len(args) < 1 {
			return TaskSetStatusArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli set-abort <id> [reason]")
		}
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return TaskSetStatusArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
		}
		var abortReason *string
		if len(args) > 1 {
			reason := strings.TrimSpace(strings.Join(args[1:], " "))
			if reason != "" {
				abortReason = &reason
			}
		}
		return TaskSetStatusArgs{
			ID:          id,
			Status:      db.StatusAbort,
			Reason:      abortReason,
			CommandName: a.CommandName,
		}, nil
	}

	if len(args) < 1 {
		return TaskSetStatusArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli %s <id>", a.CommandName)
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return TaskSetStatusArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
	}

	return TaskSetStatusArgs{
		ID:          id,
		Status:      a.Status,
		CommandName: a.CommandName,
	}, nil
}

// Execute applies the status update in the database.
func (a *TaskSetStatusAction) Execute(ctx *Context, args TaskSetStatusArgs) (*TaskSetStatusResult, error) {
	if err := ctx.Store.UpdateStatus(args.ID, args.Status, args.Reason); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.ID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &TaskSetStatusResult{
		ID:     args.ID,
		Status: args.Status,
		Reason: args.Reason,
	}, nil
}

// Format renders the status transition confirmation.
func (a *TaskSetStatusAction) Format(ctx *Context, w io.Writer, res *TaskSetStatusResult) error {
	if res.Status == db.StatusAbort && res.Reason != nil {
		_, err := fmt.Fprintf(w, "Task %04d status set to 'abort' (reason: %s)\n", res.ID, *res.Reason)
		return err
	}
	_, err := fmt.Fprintf(w, "Task %04d status set to '%s'\n", res.ID, res.Status)
	return err
}

var (
	TaskSetPendingHandler    = NewActionHandler(&TaskSetStatusAction{Status: db.StatusPending, CommandName: "set-pending"})
	TaskSetInProgressHandler = NewActionHandler(&TaskSetStatusAction{Status: db.StatusInProgress, CommandName: "set-in-progress"})
	TaskSetDoneHandler       = NewActionHandler(&TaskSetStatusAction{Status: db.StatusDone, CommandName: "set-done"})
	TaskSetAbortHandler      = NewActionHandler(&TaskSetStatusAction{Status: db.StatusAbort, CommandName: "set-abort"})
)

// TaskSetPending define o status da tarefa como 'pending'.
func TaskSetPending(ctx *Context, args []string) error {
	return TaskSetPendingHandler.Run(ctx, args)
}

// TaskSetInProgress define o status da tarefa como 'in-progress'.
func TaskSetInProgress(ctx *Context, args []string) error {
	return TaskSetInProgressHandler.Run(ctx, args)
}

// TaskSetDone define o status da tarefa como 'done'.
func TaskSetDone(ctx *Context, args []string) error {
	return TaskSetDoneHandler.Run(ctx, args)
}

// TaskSetAbort define o status da tarefa como 'abort' com motivo opcional.
func TaskSetAbort(ctx *Context, args []string) error {
	return TaskSetAbortHandler.Run(ctx, args)
}
