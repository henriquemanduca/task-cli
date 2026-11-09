# cmd

## Overview
Package `cmd` implements the command-line interface, hierarchical router, transversal flag processing, flat alias resolution, and command action handlers for tasks, projects, document attachments, and data transfer.

## Interface
- `alias.go:FlatAliases`
- `alias.go:ResolveFlatAlias`
- `alias.go:ExpandAliases`
- `cmd.go:Context`
- `cmd.go:Context.DefaultProject`
- `cmd.go:CommandFunc`
- `cmd.go:Execute`
- `context.go:GlobalFlags`
- `context.go:ParseGlobalFlags`
- `context.go:ResolveStore`
- `context.go:ResolveContext`
- `doc.go:DocAddAction`, `doc.go:DocAddHandler`, `doc.go:DocAdd`
- `doc.go:DocListAction`, `doc.go:DocListHandler`, `doc.go:DocList`
- `doc.go:DocDeleteAction`, `doc.go:DocDeleteHandler`, `doc.go:DocDelete`
- `doc.go:DocOpenAction`, `doc.go:DocOpenHandler`, `doc.go:DocOpen`
- `doc.go:DocPruneAction`, `doc.go:DocPruneHandler`, `doc.go:DocPrune`
- `format.go:FormatTSV`, `format.go:FormatTable`
- `handler.go:Handler`, `handler.go:HandlerFunc`, `handler.go:Action`, `handler.go:ActionHandler`, `handler.go:NewActionHandler`
- `io.go:ExportAction`, `io.go:ExportHandler`, `io.go:Export`
- `io.go:ImportAction`, `io.go:ImportHandler`, `io.go:Import`
- `project.go:ProjectAddAction`, `project.go:ProjectAddHandler`, `project.go:ProjectAdd`
- `project.go:ProjectListAction`, `project.go:ProjectListHandler`, `project.go:ProjectList`
- `project.go:ProjectViewAction`, `project.go:ProjectViewHandler`, `project.go:ProjectView`
- `project.go:ProjectUpdateAction`, `project.go:ProjectUpdateHandler`, `project.go:ProjectUpdate`
- `router.go:ErrUnknownCommand`, `router.go:Command`, `router.go:NewRootCommand`, `router.go:Root`
- `task.go:TaskAddAction`, `task.go:TaskAddHandler`, `task.go:TaskAdd`
- `task.go:TaskListAction`, `task.go:TaskListHandler`, `task.go:TaskList`
- `task.go:TaskViewAction`, `task.go:TaskViewHandler`, `task.go:TaskView`
- `task.go:TaskUpdateAction`, `task.go:TaskUpdateHandler`, `task.go:TaskUpdate`
- `task.go:TaskEditAction`, `task.go:TaskEditHandler`, `task.go:TaskEdit`
- `task.go:TaskDeleteAction`, `task.go:TaskDeleteHandler`, `task.go:TaskDelete`
- `task.go:TaskStatusAction`, `task.go:TaskStatusHandler`, `task.go:TaskStatus`
- `task.go:TaskSetPriorityAction`, `task.go:TaskSetPriorityHandler`, `task.go:TaskSetPriority`
- `task.go:TaskSetPendingHandler`, `task.go:TaskSetInProgressHandler`, `task.go:TaskSetDoneHandler`, `task.go:TaskSetAbortHandler`
- `task.go:TaskSetPending`, `task.go:TaskSetInProgress`, `task.go:TaskSetDone`, `task.go:TaskSetAbort`

## Invariants
- `doc.go:DocAddAction.Parse`: Requires at least 2 positional arguments (task ID and file path).
- `doc.go:DocAddAction.Parse`: Document file path must resolve to an existing accessible file via `os.Stat`.
- `doc.go:DocAddAction.Parse`: Accepts optional `--copy` and `--link` boolean flags (mutually exclusive).
- `doc.go:DocListAction.Parse`: Requires at least 1 positional argument parsed as an `int64` task ID.
- `doc.go:DocDeleteAction.Parse`: Requires at least 1 positional argument parsed as an `int64` document ID.
- `doc.go:DocOpenAction.Parse`: Requires at least 1 positional argument parsed as an `int64` task ID, or `--doc` flag parsed as an `int64` document ID (mutually exclusive).
- `doc.go:DocOpenAction.Execute`: Rejects non-existent or soft-deleted tasks/documents, returning an explanatory user error.
- `doc.go:DocPruneAction.Parse`: Accepts optional `--dry-run` (`-n`) boolean flag.
- `format.go:validateFormat`: Output formats are strictly restricted to `tsv` and `table` (defaults to `tsv` when empty).
- `io.go:ImportAction.Parse`: Input data must be non-empty JSON provided via file path positional argument or standard input.
- `project.go:ProjectAddAction.Parse`: Requires at least 2 positional arguments: project code and name.
- `project.go:ProjectViewAction.Parse`: Requires at least 1 positional argument for project code or `int64` ID.
- `project.go:ProjectUpdateAction.Parse`: Requires at least 1 positional argument for project code or `int64` ID.
- `task.go:TaskAddAction.Parse`: Requires at least 1 positional argument for task title.
- `task.go:TaskAddAction.Parse`, `task.go:TaskListAction.Parse`: When no project flag is explicitly provided, use `Context.DefaultProject()` from `.taskcli`; an explicit `-p ""` disables the default.
- `task.go:TaskAddAction.Execute`: Resolves the configured project code to its ID (case-insensitive) and fails if the project does not exist.
- `task.go:TaskListAction.Parse`: Valid task status filters are: `all`, `pending`, `in-progress`, `done`, `aborted` (defaults to `pending`, accepts legacy `abort`).
- `task.go:TaskViewAction.Parse`, `task.go:TaskUpdateAction.Parse`, `task.go:TaskEditAction.Parse`, `task.go:TaskDeleteAction.Parse`: Require at least 1 positional argument parsed as an `int64` task ID.
- `task.go:TaskStatusAction.Parse`: Valid status options are `pending`, `in-progress`, `done`, `aborted` (accepting recognized synonyms like `todo`, `reopen`, `start`, `doing`, `complete`, `finish`, `abort`, `cancel`).
- `task.go:TaskSetPriorityAction.Parse`: Priority must be one of: `low`, `medium`, `high`, `urgent`.
- `task.go:TaskSetStatusAction.Parse`: Requires at least 1 positional argument parsed as an `int64` task ID.
- `task.go:TaskUpdateAction.Execute`, `task.go:TaskEditAction.Execute`, `task.go:TaskStatusAction.Execute`, `task.go:TaskSetPriorityAction.Execute`, `task.go:TaskSetStatusAction.Execute`: Reject updates and status transitions on tasks with status 'deleted' or 'aborted' (or soft-deleted tasks), returning an explanatory user error.

## Side Effects
- `context.go:ResolveContext`, `context.go:ResolveStore`: Reads the nearest `.taskcli` via `config.LoadFromDir` (except for `version`/`help`), then opens or initializes the SQLite database connection on disk via `db.NewStore` and returns a cleanup callback.
- `doc.go:DocAddAction.Execute`: Duplicates document file into `<data_root>/attachments/<task_id>/<stem>_<hash>.<ext>` via `filestore.SaveAttachment` (unless `--link` is specified or file exceeds 100MB without `--copy`), inserts record in SQLite via `Store.AddDocument`, and automatically deletes the newly created copy on DB insertion failure (rollback).
- `doc.go:DocDeleteAction.Execute`: Deletes document record from SQLite database via `Store.DeleteDocument`.
- `doc.go:DocOpenAction.Execute`: Creates task directory via `os.MkdirAll` if missing; launches system file manager via `filestore.OpenFile` / `filestore.OpenFolder` or prints fallback path.
- `doc.go:DocPruneAction.Execute`: Deletes orphaned attachment files from disk and removes empty task folders via `filestore.PruneAttachments` (unless in dry-run mode).
- `io.go:ExportAction.Execute`: Writes exported JSON data to disk via `os.WriteFile` if `-o`/`--out` flag is provided.
- `io.go:ImportAction.Parse`: Reads file data from disk via `os.ReadFile` or standard input via `io.ReadAll`.
- `io.go:ImportAction.Execute`: Modifies database by inserting tasks, inserting/updating projects, and attaching documents via `Store.ImportData`.
- `io.go:ImportAction.Format`: Writes import warnings to `os.Stderr` and result summary to output writer.
- `project.go:ProjectAddAction.Execute`: Inserts project record into SQLite database via `Store.AddProject`.
- `project.go:ProjectUpdateAction.Execute`: Updates project name and/or description in SQLite database via `Store.UpdateProject`.
- `task.go:TaskAddAction.Execute`: Inserts task record into SQLite database via `Store.Add`.
- `task.go:TaskUpdateAction.Execute`: Updates task metadata in SQLite database via `Store.UpdateTaskDetails`.
- `task.go:TaskEditAction.Execute`: Creates temporary markdown file via `os.CreateTemp`, launches interactive editor process (`$EDITOR`, `$VISUAL`, or `nano`) attached to standard I/O, and updates task description in database via `Store.UpdateDescription`.
- `task.go:TaskDeleteAction.Execute`: Performs soft delete on task record via `Store.Delete`.
- `task.go:TaskStatusAction.Execute`, `task.go:TaskSetStatusAction.Execute`: Updates task status and abort reason in database via `Store.UpdateStatus`.
- `task.go:TaskSetPriorityAction.Execute`: Updates task priority in database via `Store.UpdatePriority`.
- `format.go:printOutput`: Writes formatted TSV or table records to provided `io.Writer`.

## Verification
- Run: `go test ./internal/cmd`
