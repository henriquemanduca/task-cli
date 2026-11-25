package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"task-cli/internal/vault"
)

// parseTaskID parses a numeric task ID argument.
func parseTaskID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid task ID: %s", raw)
	}
	return id, nil
}

// taskError turns vault sentinel errors into user-facing messages.
// action describes the attempted change (e.g. "update"), used for immutable tasks.
func taskError(id int64, action string, err error) error {
	switch {
	case errors.Is(err, vault.ErrTaskNotFound):
		return fmt.Errorf("task with ID %d not found", id)
	case errors.Is(err, vault.ErrTaskImmutable):
		return fmt.Errorf("cannot %s task with ID %d: task is deleted or aborted", action, id)
	case errors.Is(err, vault.ErrProjectNotFound):
		return fmt.Errorf("project not found: %w", err)
	default:
		return err
	}
}

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
	ProjectName string
	Priority    vault.Priority
}

// TaskAddResult holds the created task and associated project reference.
type TaskAddResult struct {
	Task *vault.Task
}

// TaskAddAction encapsulates schema parsing, execution, and output formatting for task creation.
type TaskAddAction struct{}

// Parse parses flags and positional arguments for creating a task.
func (a *TaskAddAction) Parse(ctx *Context, args []string) (TaskAddArgs, error) {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	var projectName string
	var priority string

	addCmd.StringVar(&projectName, "p", "", "Project name to create the task in (shorthand)")
	addCmd.StringVar(&projectName, "proj", "", "Project name to create the task in")
	addCmd.StringVar(&projectName, "project", "", "Project name to create the task in")
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
		return TaskAddArgs{}, fmt.Errorf("'title' is required.\nUsage: task-cli task add [-p <project>] [--priority <prio>] <title> [description]")
	}

	title := strings.TrimSpace(posArgs[0])
	var description string
	if len(posArgs) > 1 {
		description = strings.Join(posArgs[1:], " ")
	}

	parsedPriority, ok := vault.ParsePriority(priority)
	if !ok {
		return TaskAddArgs{}, fmt.Errorf("invalid priority '%s'. Valid options: low, medium, high, urgent", priority)
	}

	return TaskAddArgs{
		Title:       title,
		Description: description,
		ProjectName: strings.TrimSpace(projectName),
		Priority:    parsedPriority,
	}, nil
}

// Execute writes the new task file into the vault.
func (a *TaskAddAction) Execute(ctx *Context, args TaskAddArgs) (*TaskAddResult, error) {
	task, err := ctx.Store.Add(args.Title, args.Description, args.ProjectName, args.Priority)
	if err != nil {
		if errors.Is(err, vault.ErrProjectNotFound) {
			return nil, fmt.Errorf("project '%s' not found", args.ProjectName)
		}
		return nil, fmt.Errorf("creating task: %w", err)
	}
	return &TaskAddResult{Task: task}, nil
}

// Format renders task creation confirmation.
func (a *TaskAddAction) Format(ctx *Context, w io.Writer, res *TaskAddResult) error {
	if res.Task.ProjectName != "" {
		_, err := fmt.Fprintf(w, "Task created successfully (ID: %d) in project '%s'\n", res.Task.ID, res.Task.ProjectName)
		return err
	}
	_, err := fmt.Fprintf(w, "Task created successfully (ID: %d)\n", res.Task.ID)
	return err
}

// TaskAddHandler is the standardized handler instance for TaskAddAction.
var TaskAddHandler = NewActionHandler(&TaskAddAction{})

// TaskAdd adiciona uma nova tarefa ao vault.
func TaskAdd(ctx *Context, args []string) error {
	return TaskAddHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// TaskList
// -----------------------------------------------------------------------------

// TaskListArgs represents the argument schema for listing tasks.
type TaskListArgs struct {
	StatusFilter string
	ProjectName  string
	ShowTime     bool
	NoHeader     bool
	Format       string
}

// TaskListResult holds query results and formatting options for task listing.
type TaskListResult struct {
	Tasks    []vault.Task
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
	var projectName string
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
	listCmd.StringVar(&projectName, "p", "", "Filter tasks by project name (shorthand)")
	listCmd.StringVar(&projectName, "project", "", "Filter tasks by project name")
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
		statusFilter = string(vault.StatusDone)
	} else if posStatus != "" {
		status, ok := vault.ParseStatus(posStatus)
		if !ok || status == vault.StatusDeleted {
			return TaskListArgs{}, fmt.Errorf("invalid status '%s'. Valid options: all, pending, in-progress, done, aborted", posStatus)
		}
		statusFilter = string(status)
	} else {
		statusFilter = string(vault.StatusPending)
	}

	return TaskListArgs{
		StatusFilter: statusFilter,
		ProjectName:  projectName,
		ShowTime:     showTime,
		NoHeader:     noHeader,
		Format:       resolvedFmt,
	}, nil
}

// Execute reads tasks from the vault according to filters.
func (a *TaskListAction) Execute(ctx *Context, args TaskListArgs) (*TaskListResult, error) {
	tasks, err := ctx.Store.List(args.StatusFilter, args.ProjectName)
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
		proj := t.ProjectName
		if proj == "" {
			proj = "-"
		}

		row := []string{
			fmt.Sprintf("%04d", t.ID),
			proj,
			string(t.Status),
			string(t.Priority),
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
	Task     *vault.Task
	ShowTime bool
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
		return TaskViewArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task view <id> [-t]")
	}
	id, err := parseTaskID(posArgs[0])
	if err != nil {
		return TaskViewArgs{}, err
	}

	return TaskViewArgs{
		ID:       id,
		ShowTime: showTime,
	}, nil
}

// Execute reads the task file from the vault.
func (a *TaskViewAction) Execute(ctx *Context, args TaskViewArgs) (*TaskViewResult, error) {
	t, err := ctx.Store.GetTask(args.ID)
	if err != nil {
		return nil, taskError(args.ID, "view", err)
	}
	return &TaskViewResult{Task: t, ShowTime: args.ShowTime}, nil
}

// Format renders the full task details view.
func (a *TaskViewAction) Format(ctx *Context, w io.Writer, res *TaskViewResult) error {
	t := res.Task
	proj := t.ProjectName
	if proj == "" {
		proj = "-"
	}

	fmt.Fprintf(w, "Task: %04d\n", t.ID)
	fmt.Fprintf(w, "Project: %s\n", proj)
	fmt.Fprintf(w, "Title: %s\n", t.Title)
	fmt.Fprintf(w, "Status: %s\n", t.Status)
	fmt.Fprintf(w, "Priority: %s\n", t.Priority)
	if res.ShowTime {
		fmt.Fprintf(w, "Created At: %s\n", t.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(w, "Updated At: %s\n", t.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if t.AbortReason != "" {
		fmt.Fprintf(w, "Abort Note: %s\n", t.AbortReason)
	}
	fmt.Fprintf(w, "File: %s\n", t.Path)
	fmt.Fprintf(w, "Description:\n")
	if strings.TrimSpace(t.Description) == "" {
		fmt.Fprintln(w, "(No description provided)")
	} else {
		fmt.Fprintln(w, t.Description)
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
	ProjectName string
	ClearProj   bool
	Priority    vault.Priority
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
		return TaskUpdateArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task update <id> [-p <project|none>] [--priority <prio>] [title] [description]")
	}

	updateCmd := flag.NewFlagSet("update", flag.ContinueOnError)
	var projectStr string
	var priority string
	updateCmd.StringVar(&projectStr, "p", "", "Move task to project name (or 'none' to detach)")
	updateCmd.StringVar(&projectStr, "proj", "", "Move task to project name (or 'none' to detach)")
	updateCmd.StringVar(&projectStr, "project", "", "Move task to project name (or 'none' to detach)")
	updateCmd.StringVar(&priority, "priority", "", "Update task priority (e.g. low, medium, high)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-p":         true,
		"--proj":     true,
		"--project":  true,
		"--priority": true,
	})

	if err := updateCmd.Parse(flagArgs); err != nil {
		return TaskUpdateArgs{}, err
	}

	if len(posArgs) < 1 {
		return TaskUpdateArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task update <id> [-p <project|none>] [--priority <prio>] [title] [description]")
	}

	id, err := parseTaskID(posArgs[0])
	if err != nil {
		return TaskUpdateArgs{}, err
	}
	posArgs = posArgs[1:]

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

	var projectName string
	var clearProj bool
	if cleanProj := strings.TrimSpace(projectStr); cleanProj != "" {
		if strings.EqualFold(cleanProj, "none") || cleanProj == "-" {
			clearProj = true
		} else {
			projectName = cleanProj
		}
	}

	var parsedPriority vault.Priority
	if priority != "" {
		var ok bool
		parsedPriority, ok = vault.ParsePriority(priority)
		if !ok {
			return TaskUpdateArgs{}, fmt.Errorf("invalid priority '%s'. Valid options: low, medium, high, urgent", priority)
		}
	}

	return TaskUpdateArgs{
		ID:          id,
		Title:       title,
		Description: desc,
		ProjectName: projectName,
		ClearProj:   clearProj,
		Priority:    parsedPriority,
		HasTitle:    hasTitle,
		HasDesc:     hasDesc,
	}, nil
}

// Execute updates the task file (renaming/moving it when title or project change).
func (a *TaskUpdateAction) Execute(ctx *Context, args TaskUpdateArgs) (*TaskUpdateResult, error) {
	_, err := ctx.Store.UpdateTask(args.ID, func(t *vault.Task) error {
		if args.HasTitle {
			t.Title = args.Title
		}
		if args.HasDesc {
			t.Description = args.Description
		}
		if args.ClearProj {
			t.ProjectName = ""
		} else if args.ProjectName != "" {
			t.ProjectName = args.ProjectName
		}
		if args.Priority != "" {
			t.Priority = args.Priority
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, vault.ErrProjectNotFound) {
			return nil, fmt.Errorf("project '%s' not found", args.ProjectName)
		}
		return nil, taskError(args.ID, "update", err)
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
		return TaskEditArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task edit <id>")
	}

	id, err := parseTaskID(args[0])
	if err != nil {
		return TaskEditArgs{}, err
	}

	return TaskEditArgs{ID: id}, nil
}

// Execute opens the task's Markdown file directly in the user's editor.
func (a *TaskEditAction) Execute(ctx *Context, args TaskEditArgs) (*TaskEditResult, error) {
	t, err := ctx.Store.GetMutableTask(args.ID)
	if err != nil {
		return nil, taskError(args.ID, "edit", err)
	}

	before, err := os.ReadFile(t.Path)
	if err != nil {
		return nil, fmt.Errorf("reading task file: %w", err)
	}

	if err := openInEditor(t.Path); err != nil {
		return nil, err
	}

	after, err := os.ReadFile(t.Path)
	if err != nil {
		return nil, fmt.Errorf("reading task file: %w", err)
	}
	if string(before) == string(after) {
		return &TaskEditResult{ID: args.ID, Changed: false}, nil
	}

	// Re-save through the vault to validate the file, bump "updated" and
	// rename it if the title heading changed.
	if _, err := ctx.Store.UpdateTask(args.ID, func(*vault.Task) error { return nil }); err != nil {
		return nil, fmt.Errorf("task file saved but could not be re-read (%s): %w", t.Path, err)
	}
	return &TaskEditResult{ID: args.ID, Changed: true}, nil
}

// Format renders the result of task description editing.
func (a *TaskEditAction) Format(ctx *Context, w io.Writer, res *TaskEditResult) error {
	if !res.Changed {
		fmt.Fprintln(w, "No changes made.")
		return nil
	}
	_, err := fmt.Fprintf(w, "Task %d updated successfully\n", res.ID)
	return err
}

// TaskEditHandler is the standardized handler instance for TaskEditAction.
var TaskEditHandler = NewActionHandler(&TaskEditAction{})

// TaskEdit abre o arquivo .md da tarefa no editor externo.
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
		return TaskDeleteArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task delete <id>")
	}

	id, err := parseTaskID(args[0])
	if err != nil {
		return TaskDeleteArgs{}, err
	}

	return TaskDeleteArgs{ID: id}, nil
}

// Execute marks the task as deleted and moves it to <vault>/.trash.
func (a *TaskDeleteAction) Execute(ctx *Context, args TaskDeleteArgs) (*TaskDeleteResult, error) {
	if err := ctx.Store.Delete(args.ID); err != nil {
		if errors.Is(err, vault.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found or already deleted", args.ID)
		}
		return nil, err
	}

	return &TaskDeleteResult{ID: args.ID}, nil
}

// Format renders the deletion confirmation message.
func (a *TaskDeleteAction) Format(ctx *Context, w io.Writer, res *TaskDeleteResult) error {
	_, err := fmt.Fprintf(w, "Task %d moved to trash\n", res.ID)
	return err
}

// TaskDeleteHandler is the standardized handler instance for TaskDeleteAction.
var TaskDeleteHandler = NewActionHandler(&TaskDeleteAction{})

// TaskDelete move a tarefa para a lixeira do vault (.trash).
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
	Status  vault.Status
	Reason  string
}

// TaskStatusResult holds the outcome of a task status query or update.
type TaskStatusResult struct {
	ID      int64
	Status  vault.Status
	Reason  string
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
		id, err := parseTaskID(args[0])
		if err != nil {
			return TaskStatusArgs{}, err
		}
		return TaskStatusArgs{ID: id, IsQuery: true}, nil
	}

	// 2 or more arguments: set status
	var id int64
	var rawStatus string
	var remaining []string

	id1, err1 := parseTaskID(args[0])
	if err1 == nil {
		id = id1
		rawStatus = args[1]
		remaining = args[2:]
	} else {
		id2, err2 := parseTaskID(args[1])
		if err2 == nil {
			id = id2
			rawStatus = args[0]
			remaining = args[2:]
		} else {
			return TaskStatusArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
		}
	}

	statusLower := strings.ToLower(strings.TrimSpace(rawStatus))
	switch statusLower {
	case "todo", "reopen":
		statusLower = string(vault.StatusPending)
	case "start", "started", "progress", "doing":
		statusLower = string(vault.StatusInProgress)
	case "complete", "completed", "finish", "finished":
		statusLower = string(vault.StatusDone)
	case "cancel", "cancelled":
		statusLower = string(vault.StatusAborted)
	}
	targetStatus, ok := vault.ParseStatus(statusLower)
	if !ok || targetStatus == vault.StatusDeleted {
		return TaskStatusArgs{}, fmt.Errorf("invalid status '%s'. Valid options: pending, in-progress, done, aborted", rawStatus)
	}

	var abortReason string
	if targetStatus == vault.StatusAborted {
		abortReason = strings.TrimSpace(strings.Join(remaining, " "))
	}

	return TaskStatusArgs{
		ID:      id,
		IsQuery: false,
		Status:  targetStatus,
		Reason:  abortReason,
	}, nil
}

// Execute queries or updates the task status.
func (a *TaskStatusAction) Execute(ctx *Context, args TaskStatusArgs) (*TaskStatusResult, error) {
	if args.IsQuery {
		t, err := ctx.Store.GetTask(args.ID)
		if err != nil {
			return nil, taskError(args.ID, "view", err)
		}
		return &TaskStatusResult{ID: t.ID, Status: t.Status, Reason: t.AbortReason, IsQuery: true}, nil
	}

	if _, err := ctx.Store.SetStatus(args.ID, args.Status, args.Reason); err != nil {
		return nil, taskError(args.ID, "change status of", err)
	}
	return &TaskStatusResult{ID: args.ID, Status: args.Status, Reason: args.Reason}, nil
}

// Format renders the status query or update message.
func (a *TaskStatusAction) Format(ctx *Context, w io.Writer, res *TaskStatusResult) error {
	if res.IsQuery {
		if res.Status == vault.StatusAborted && res.Reason != "" {
			fmt.Fprintf(w, "Task %04d status: %s (reason: %s)\n", res.ID, res.Status, res.Reason)
		} else {
			fmt.Fprintf(w, "Task %04d status: %s\n", res.ID, res.Status)
		}
		return nil
	}

	if res.Status == vault.StatusAborted && res.Reason != "" {
		fmt.Fprintf(w, "Task %04d status set to 'aborted' (reason: %s)\n", res.ID, res.Reason)
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
	Priority vault.Priority
}

// TaskSetPriorityResult holds the task ID and new priority.
type TaskSetPriorityResult struct {
	ID       int64
	Priority vault.Priority
}

// TaskSetPriorityAction encapsulates schema parsing, execution, and output formatting for priority updates.
type TaskSetPriorityAction struct{}

// Parse parses arguments for setting priority.
func (a *TaskSetPriorityAction) Parse(ctx *Context, args []string) (TaskSetPriorityArgs, error) {
	if len(args) < 2 {
		return TaskSetPriorityArgs{}, fmt.Errorf("task ID and priority are required.\nUsage: task-cli task prio <id> <priority>")
	}

	id, err := parseTaskID(args[0])
	if err != nil {
		return TaskSetPriorityArgs{}, err
	}

	prio, ok := vault.ParsePriority(args[1])
	if !ok {
		return TaskSetPriorityArgs{}, fmt.Errorf("invalid priority '%s'. Valid options: low, medium, high, urgent", args[1])
	}

	return TaskSetPriorityArgs{
		ID:       id,
		Priority: prio,
	}, nil
}

// Execute updates the task priority.
func (a *TaskSetPriorityAction) Execute(ctx *Context, args TaskSetPriorityArgs) (*TaskSetPriorityResult, error) {
	_, err := ctx.Store.UpdateTask(args.ID, func(t *vault.Task) error {
		t.Priority = args.Priority
		return nil
	})
	if err != nil {
		return nil, taskError(args.ID, "update priority of", err)
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
	Status      vault.Status
	Reason      string
	CommandName string
}

// TaskSetStatusResult holds status transition details.
type TaskSetStatusResult struct {
	ID     int64
	Status vault.Status
	Reason string
}

// TaskSetStatusAction encapsulates schema parsing, execution, and output formatting for dedicated status transitions.
type TaskSetStatusAction struct {
	Status      vault.Status
	CommandName string
}

// Parse parses arguments for status transitions.
func (a *TaskSetStatusAction) Parse(ctx *Context, args []string) (TaskSetStatusArgs, error) {
	if a.Status == vault.StatusAborted {
		if len(args) < 1 {
			return TaskSetStatusArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task set-abort <id> [reason]")
		}
		id, err := parseTaskID(args[0])
		if err != nil {
			return TaskSetStatusArgs{}, err
		}
		return TaskSetStatusArgs{
			ID:          id,
			Status:      vault.StatusAborted,
			Reason:      strings.TrimSpace(strings.Join(args[1:], " ")),
			CommandName: a.CommandName,
		}, nil
	}

	if len(args) < 1 {
		return TaskSetStatusArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task %s <id>", a.CommandName)
	}

	id, err := parseTaskID(args[0])
	if err != nil {
		return TaskSetStatusArgs{}, err
	}

	return TaskSetStatusArgs{
		ID:          id,
		Status:      a.Status,
		CommandName: a.CommandName,
	}, nil
}

// Execute applies the status update.
func (a *TaskSetStatusAction) Execute(ctx *Context, args TaskSetStatusArgs) (*TaskSetStatusResult, error) {
	if _, err := ctx.Store.SetStatus(args.ID, args.Status, args.Reason); err != nil {
		return nil, taskError(args.ID, "change status of", err)
	}

	return &TaskSetStatusResult{
		ID:     args.ID,
		Status: args.Status,
		Reason: args.Reason,
	}, nil
}

// Format renders the status transition confirmation.
func (a *TaskSetStatusAction) Format(ctx *Context, w io.Writer, res *TaskSetStatusResult) error {
	if res.Status == vault.StatusAborted && res.Reason != "" {
		_, err := fmt.Fprintf(w, "Task %04d status set to 'aborted' (reason: %s)\n", res.ID, res.Reason)
		return err
	}
	_, err := fmt.Fprintf(w, "Task %04d status set to '%s'\n", res.ID, res.Status)
	return err
}

var (
	TaskSetPendingHandler    = NewActionHandler(&TaskSetStatusAction{Status: vault.StatusPending, CommandName: "set-pending"})
	TaskSetInProgressHandler = NewActionHandler(&TaskSetStatusAction{Status: vault.StatusInProgress, CommandName: "set-in-progress"})
	TaskSetDoneHandler       = NewActionHandler(&TaskSetStatusAction{Status: vault.StatusDone, CommandName: "set-done"})
	TaskSetAbortHandler      = NewActionHandler(&TaskSetStatusAction{Status: vault.StatusAborted, CommandName: "set-abort"})
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

// TaskSetAbort define o status da tarefa como 'aborted' com motivo opcional.
func TaskSetAbort(ctx *Context, args []string) error {
	return TaskSetAbortHandler.Run(ctx, args)
}
