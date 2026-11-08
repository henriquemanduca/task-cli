# db

## Overview
Package `db` provides pure-Go SQLite storage and schema management for tasks, projects, and document attachments via `modernc.org/sqlite`. It handles connection initialization, schema migration, CRUD operations, and JSON transfer serialization/deserialization.

## Interface
- `db.go:ErrProjectCodeExists`
- `db.go:ErrProjectNotFound`
- `db.go:ErrTaskNotFound`
- `db.go:ErrDocumentNotFound`
- `db.go:Status` (`StatusPending`, `StatusInProgress`, `StatusDone`, `StatusAbort`, `StatusDeleted`)
- `db.go:Project`
- `db.go:Task`
- `db.go:Document`
- `db.go:Store`
- `db.go:DefaultDBPath`
- `db.go:NewStore`
- `db.go:Store.Close`
- `db.go:Store.AddProject`
- `db.go:Store.ListProjects`
- `db.go:Store.GetProjectByCode`
- `db.go:Store.GetProjectByID`
- `db.go:Store.UpdateProject`
- `db.go:Store.Add`
- `db.go:Store.List`
- `db.go:Store.GetTaskByID`
- `db.go:Store.Update`
- `db.go:Store.UpdateTaskDetails`
- `db.go:Store.UpdateDescription`
- `db.go:Store.UpdateStatus`
- `db.go:Store.UpdatePriority`
- `db.go:Store.Delete`
- `db.go:Store.AddDocument`
- `db.go:Store.ListDocuments`
- `db.go:Store.GetDocumentByID`
- `db.go:Store.DeleteDocument`
- `transfer.go:CurrentSchemaVersion`
- `transfer.go:ErrInvalidSchemaVersion`
- `transfer.go:ErrInvalidStatus`
- `transfer.go:ErrInvalidPriority`
- `transfer.go:DocumentTransfer`
- `transfer.go:TaskTransfer`
- `transfer.go:ProjectTransfer`
- `transfer.go:TransferData`
- `transfer.go:ExportOptions`
- `transfer.go:ImportOptions`
- `transfer.go:ImportResult`
- `transfer.go:Store.ExportData`
- `transfer.go:Store.ImportData`

## Invariants
- `db.go:schema`: `projects.code` is `NOT NULL UNIQUE COLLATE NOCASE`.
- `db.go:schema`: `tasks.project_id` foreign key references `projects(id) ON DELETE SET NULL`.
- `db.go:schema`: `tasks.status` is `NOT NULL DEFAULT 'pending'`.
- `db.go:schema`: `tasks.priority` is `NOT NULL DEFAULT 'medium'`.
- `db.go:schema`: `documents.task_id` is `NOT NULL` references `tasks(id) ON DELETE CASCADE`.
- `db.go:List`, `db.go:GetTaskByID`: Soft-deleted tasks (`deleted_at IS NOT NULL`) are excluded from queries.
- `db.go:Update`, `db.go:UpdateTaskDetails`, `db.go:UpdateDescription`, `db.go:UpdateStatus`, `db.go:UpdatePriority`, `db.go:Delete`: Restrict mutations to active tasks (`deleted_at IS NULL`).
- `transfer.go:ImportData`: `data.Version` must equal `CurrentSchemaVersion` (version 1).
- `transfer.go:isValidStatus`: Status must be `pending`, `in-progress`, `done`, `abort`, or `deleted`.
- `transfer.go:isValidPriority`: Priority must be `low`, `medium`, `high`, or `urgent`.
- `transfer.go:opts.Strict`: When strict mode is enabled, enforces non-empty task title, valid status, valid priority, and non-empty project code during import.

## Side Effects
- `db.go:DefaultDBPath`: Reads environment variable `TASK_CLI_DB` and queries user home directory via `os.UserHomeDir`.
- `db.go:NewStore`: Creates directory path via `os.MkdirAll`, opens SQLite database file, configures `PRAGMA journal_mode=WAL` and `foreign_keys=ON`, executes DDL schema migrations, and updates legacy records.
- `db.go:Store.Close`: Closes underlying SQLite database connection.
- `db.go:Store.AddProject`: Inserts project record into SQLite `projects` table.
- `db.go:Store.UpdateProject`: Updates project name, description, and updated_at in SQLite `projects` table.
- `db.go:Store.Add`: Inserts task record into SQLite `tasks` table.
- `db.go:Store.Update`, `db.go:Store.UpdateTaskDetails`, `db.go:Store.UpdateDescription`, `db.go:Store.UpdateStatus`, `db.go:Store.UpdatePriority`: Modifies task records in SQLite `tasks` table and updates `updated_at`.
- `db.go:Store.Delete`: Performs soft delete by populating `deleted_at`, updating status to `deleted`, and updating `updated_at` in SQLite `tasks` table.
- `db.go:Store.AddDocument`: Inserts document path record into SQLite `documents` table.
- `db.go:Store.DeleteDocument`: Deletes document record from SQLite `documents` table.
- `transfer.go:Store.ImportData`: Executes transaction mutating `projects`, `tasks`, and `documents` tables, rolling back on error or if `opts.DryRun` is true.

## Verification
- Run: `go test ./internal/db`
