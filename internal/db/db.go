package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"task-cli/internal/filestore"

	_ "modernc.org/sqlite"
)

var (
	ErrProjectCodeExists = errors.New("project code already exists")
	ErrProjectNotFound   = errors.New("project not found")
	ErrTaskNotFound      = errors.New("task not found")
	ErrDocumentNotFound  = errors.New("document not found")
	ErrTaskImmutable     = errors.New("cannot modify task with status deleted or aborted")
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusAborted    Status = "aborted"
	StatusAbort      Status = StatusAborted
	StatusDeleted    Status = "deleted"
)

// ParseStatus normalizes a raw status string and reports whether it is valid.
// Legacy spellings used by older databases and imports are accepted.
func ParseStatus(raw string) (Status, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending", "todo":
		return StatusPending, true
	case "in-progress":
		return StatusInProgress, true
	case "done":
		return StatusDone, true
	case "aborted", "abort":
		return StatusAborted, true
	case "deleted":
		return StatusDeleted, true
	default:
		return "", false
	}
}

// Priority is the priority level of a task.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// ParsePriority normalizes a raw priority string and reports whether it is valid.
func ParsePriority(raw string) (Priority, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(PriorityLow):
		return PriorityLow, true
	case string(PriorityMedium):
		return PriorityMedium, true
	case string(PriorityHigh):
		return PriorityHigh, true
	case string(PriorityUrgent):
		return PriorityUrgent, true
	default:
		return "", false
	}
}

type Project struct {
	ID          int64
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Task struct {
	ID          int64
	ProjectID   *int64
	ProjectCode string
	Title       string
	Description string
	Status      Status
	Priority    Priority
	AbortReason *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type Document struct {
	ID        int64
	TaskID    int64
	Name      string
	Type      string
	FilePath  string
	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

// DefaultDBPath returns the canonical path to the SQLite database.
// It prioritizes the TASK_CLI_DB environment variable override (if set and non-empty),
// and otherwise delegates to filestore.DefaultDBPath().
func DefaultDBPath() (string, error) {
	if envPath := os.Getenv("TASK_CLI_DB"); envPath != "" {
		return envPath, nil
	}
	return filestore.DefaultDBPath()
}

// NewStore opens or creates the SQLite database and initializes the schema.
func NewStore(dbPath string) (*Store, error) {
	if err := maybeMigrateLegacyDB(dbPath); err != nil {
		return nil, err
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// modernc.org/sqlite registers the "sqlite" driver. busy_timeout makes
	// concurrent writers wait instead of failing immediately with SQLITE_BUSY.
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	dsn := dbPath + separator + "_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// A single connection avoids in-process lock contention and keeps the
	// per-connection PRAGMAs (applied through the DSN) predictable.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE COLLATE NOCASE,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		priority TEXT NOT NULL DEFAULT 'medium',
		abort_reason TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		deleted_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		file_path TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_documents_task_id ON documents(task_id);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// Clean up any trailing commas from legacy input and migrate legacy 'todo' status to 'pending', 'abort' to 'aborted'
	legacyMigrations := []string{
		`UPDATE projects SET code = TRIM(code, ', '), name = TRIM(name, ', ') WHERE code LIKE '%,';`,
		`UPDATE tasks SET status = 'pending' WHERE status = 'todo';`,
		`UPDATE tasks SET status = 'aborted' WHERE status = 'abort';`,
	}
	for _, query := range legacyMigrations {
		if _, err := db.Exec(query); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run legacy migration: %w", err)
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// AddProject creates a new project.
func (s *Store) AddProject(ctx context.Context, code, name, description string) (*Project, error) {
	code = strings.Trim(code, ", ")
	name = strings.Trim(name, ", ")
	description = strings.Trim(description, ", ")

	now := time.Now().UTC()
	query := `INSERT INTO projects (code, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, code, name, description, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrProjectCodeExists
		}
		return nil, fmt.Errorf("failed to insert project: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last inserted project ID: %w", err)
	}

	return &Project{
		ID:          id,
		Code:        code,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// ListProjects returns all projects.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	query := `SELECT id, code, name, COALESCE(description, ''), created_at, updated_at FROM projects ORDER BY id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProjectByCode returns a project by its unique code (case-insensitive and trimmed).
func (s *Store) GetProjectByCode(ctx context.Context, code string) (*Project, error) {
	cleanCode := strings.Trim(code, ", ")
	query := `SELECT id, code, name, COALESCE(description, ''), created_at, updated_at
	          FROM projects
	          WHERE code = ? COLLATE NOCASE`
	row := s.db.QueryRowContext(ctx, query, cleanCode)

	var p Project
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("failed to query project by code: %w", err)
	}
	return &p, nil
}

// GetProjectByID returns a project by its numeric ID.
func (s *Store) GetProjectByID(ctx context.Context, id int64) (*Project, error) {
	query := `SELECT id, code, name, COALESCE(description, ''), created_at, updated_at
	          FROM projects
	          WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var p Project
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("failed to query project by id: %w", err)
	}
	return &p, nil
}

// UpdateProject updates the name and description of an existing project by ID.
func (s *Store) UpdateProject(ctx context.Context, id int64, name, description string) error {
	now := time.Now().UTC()
	query := `UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, name, description, now, id)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// Add creates a new task with a title and optional description.
func (s *Store) Add(ctx context.Context, title, description string, projectID *int64, priority Priority) (*Task, error) {
	now := time.Now().UTC()
	if priority == "" {
		priority = PriorityMedium
	}
	if _, ok := ParsePriority(string(priority)); !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPriority, priority)
	}
	query := `INSERT INTO tasks (project_id, title, description, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, projectID, title, description, string(StatusPending), priority, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert task: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last inserted ID: %w", err)
	}

	return &Task{
		ID:          id,
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanTask scans a task row shared by every task query.
func scanTask(scanner rowScanner) (*Task, error) {
	var t Task
	var status string
	if err := scanner.Scan(&t.ID, &t.ProjectID, &t.ProjectCode, &t.Title, &t.Description, &status, &t.Priority, &t.AbortReason, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
		return nil, err
	}
	t.Status = Status(status)
	return &t, nil
}

// List returns active tasks (soft deleted tasks excluded), with optional status and project filters.
// projectFilter matches either project ID (if numeric) or project code (case-insensitive).
func (s *Store) List(ctx context.Context, statusFilter, projectFilter string) ([]Task, error) {
	query := `SELECT t.id, t.project_id, COALESCE(p.code, ''), t.title, COALESCE(t.description, ''),
	                 t.status, t.priority, t.abort_reason, t.created_at, t.updated_at, t.deleted_at
	          FROM tasks t
	          LEFT JOIN projects p ON t.project_id = p.id
	          WHERE t.deleted_at IS NULL`
	var args []any

	if statusFilter != "" {
		status, ok := ParseStatus(statusFilter)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, statusFilter)
		}
		query += ` AND t.status = ?`
		args = append(args, string(status))
	}

	if projectFilter != "" {
		cleanProj := strings.Trim(projectFilter, ", ")
		if projID, err := strconv.ParseInt(cleanProj, 10, 64); err == nil {
			query += ` AND (p.id = ? OR p.code = ? COLLATE NOCASE)`
			args = append(args, projID, cleanProj)
		} else {
			query += ` AND p.code = ? COLLATE NOCASE`
			args = append(args, cleanProj)
		}
	}

	query += ` ORDER BY p.code ASC, t.id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, *t)
	}

	return tasks, rows.Err()
}

// GetTaskByID returns a task by its ID (including project code).
func (s *Store) GetTaskByID(ctx context.Context, id int64) (*Task, error) {
	query := `SELECT t.id, t.project_id, COALESCE(p.code, ''), t.title, COALESCE(t.description, ''),
	                 t.status, t.priority, t.abort_reason, t.created_at, t.updated_at, t.deleted_at
	          FROM tasks t
	          LEFT JOIN projects p ON t.project_id = p.id
	          WHERE t.id = ? AND t.deleted_at IS NULL`
	row := s.db.QueryRowContext(ctx, query, id)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to query task by id: %w", err)
	}
	return t, nil
}

// mutableTaskFilter is the SQL predicate for tasks that may be modified.
// It is applied in the UPDATE itself so the check and the write are atomic.
const mutableTaskFilter = `deleted_at IS NULL AND status NOT IN ('deleted', 'aborted', 'abort')`

// CheckTaskMutable verifies whether a task exists and is in an editable state.
// Returns nil if the task is active and mutable.
// Returns ErrTaskNotFound if the task does not exist in the database.
// Returns ErrTaskImmutable if the task is deleted (deleted_at IS NOT NULL or status = 'deleted') or aborted.
func (s *Store) CheckTaskMutable(ctx context.Context, id int64) error {
	query := `SELECT status, deleted_at FROM tasks WHERE id = ?`
	var st string
	var deletedAt *time.Time
	err := s.db.QueryRowContext(ctx, query, id).Scan(&st, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTaskNotFound
		}
		return fmt.Errorf("failed to query task status: %w", err)
	}

	status, ok := ParseStatus(st)
	if deletedAt != nil || !ok || status == StatusDeleted || status == StatusAborted {
		return ErrTaskImmutable
	}

	return nil
}

// classifyMutationFailure turns a zero-row UPDATE into the precise sentinel error.
func (s *Store) classifyMutationFailure(ctx context.Context, id int64) error {
	if err := s.CheckTaskMutable(ctx, id); err != nil {
		return err
	}
	return ErrTaskNotFound
}

// Update updates the title and description of an active task.
func (s *Store) Update(ctx context.Context, id int64, title, description string) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET title = ?, description = ?, updated_at = ? WHERE id = ? AND ` + mutableTaskFilter
	res, err := s.db.ExecContext(ctx, query, title, description, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return s.classifyMutationFailure(ctx, id)
	}
	return nil
}

// UpdateTaskDetails updates title, description, project_id, and priority of an active task.
func (s *Store) UpdateTaskDetails(ctx context.Context, id int64, title, description string, projectID *int64, priority Priority) error {
	if _, ok := ParsePriority(string(priority)); !ok {
		return fmt.Errorf("%w: %q", ErrInvalidPriority, priority)
	}
	now := time.Now().UTC()
	query := `UPDATE tasks SET title = ?, description = ?, project_id = ?, priority = ?, updated_at = ? WHERE id = ? AND ` + mutableTaskFilter
	res, err := s.db.ExecContext(ctx, query, title, description, projectID, priority, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task details: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return s.classifyMutationFailure(ctx, id)
	}
	return nil
}

// UpdateDescription updates only the description of an active task.
func (s *Store) UpdateDescription(ctx context.Context, id int64, description string) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET description = ?, updated_at = ? WHERE id = ? AND ` + mutableTaskFilter
	res, err := s.db.ExecContext(ctx, query, description, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task description: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return s.classifyMutationFailure(ctx, id)
	}
	return nil
}

// UpdateStatus updates the status of an active task and optionally sets an abort reason.
func (s *Store) UpdateStatus(ctx context.Context, id int64, status Status, abortReason *string) error {
	parsed, ok := ParseStatus(string(status))
	if !ok || parsed == StatusDeleted {
		return fmt.Errorf("%w: %q", ErrInvalidStatus, status)
	}
	now := time.Now().UTC()
	query := `UPDATE tasks SET status = ?, abort_reason = ?, updated_at = ? WHERE id = ? AND ` + mutableTaskFilter
	res, err := s.db.ExecContext(ctx, query, string(parsed), abortReason, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return s.classifyMutationFailure(ctx, id)
	}
	return nil
}

// UpdatePriority updates the priority of an active task.
func (s *Store) UpdatePriority(ctx context.Context, id int64, priority Priority) error {
	parsed, ok := ParsePriority(string(priority))
	if !ok {
		return fmt.Errorf("%w: %q", ErrInvalidPriority, priority)
	}
	now := time.Now().UTC()
	query := `UPDATE tasks SET priority = ?, updated_at = ? WHERE id = ? AND ` + mutableTaskFilter
	res, err := s.db.ExecContext(ctx, query, parsed, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task priority: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return s.classifyMutationFailure(ctx, id)
	}
	return nil
}

// Delete performs a soft delete on a task by setting its deleted_at timestamp and updating its status to deleted.
func (s *Store) Delete(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET status = ?, deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.ExecContext(ctx, query, string(StatusDeleted), now, now, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete task: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// AddDocument links a new document to a task.
func (s *Store) AddDocument(ctx context.Context, taskID int64, name, docType, filePath string) (*Document, error) {
	if _, err := s.GetTaskByID(ctx, taskID); err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	docType = strings.TrimSpace(docType)
	filePath = strings.TrimSpace(filePath)

	now := time.Now().UTC()
	query := `INSERT INTO documents (task_id, name, type, file_path, created_at) VALUES (?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, taskID, name, docType, filePath, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert document: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last inserted document ID: %w", err)
	}

	return &Document{
		ID:        id,
		TaskID:    taskID,
		Name:      name,
		Type:      docType,
		FilePath:  filePath,
		CreatedAt: now,
	}, nil
}

// ListDocuments returns all documents linked to a task.
func (s *Store) ListDocuments(ctx context.Context, taskID int64) ([]Document, error) {
	query := `SELECT id, task_id, name, type, file_path, created_at FROM documents WHERE task_id = ? ORDER BY id ASC`
	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Name, &d.Type, &d.FilePath, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// GetDocumentByID returns a document by its ID.
func (s *Store) GetDocumentByID(ctx context.Context, id int64) (*Document, error) {
	query := `SELECT id, task_id, name, type, file_path, created_at FROM documents WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var d Document
	if err := row.Scan(&d.ID, &d.TaskID, &d.Name, &d.Type, &d.FilePath, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("failed to query document by id: %w", err)
	}
	return &d, nil
}

// DeleteDocument removes a document link by ID.
func (s *Store) DeleteDocument(ctx context.Context, id int64) error {
	query := `DELETE FROM documents WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// ListAllDocumentPaths returns all file paths of active documents belonging to non-deleted tasks.
func (s *Store) ListAllDocumentPaths(ctx context.Context) ([]string, error) {
	query := `SELECT d.file_path FROM documents d JOIN tasks t ON d.task_id = t.id WHERE t.deleted_at IS NULL AND t.status != 'deleted' ORDER BY d.id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query document paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan document path: %w", err)
		}
		path = strings.TrimSpace(path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed during document paths iteration: %w", err)
	}
	return paths, nil
}

func maybeMigrateLegacyDB(dbPath string) error {
	defaultPath, err := filestore.DefaultDBPath()
	if err != nil {
		return nil
	}

	cleanDBPath := filepath.Clean(dbPath)
	cleanDefault := filepath.Clean(defaultPath)
	if cleanDBPath != cleanDefault {
		return nil
	}

	if _, err := os.Stat(cleanDBPath); err == nil || !os.IsNotExist(err) {
		return nil
	}

	legacyPath, err := filestore.LegacyDBPath()
	if err != nil {
		return nil
	}
	legacyPath = filepath.Clean(legacyPath)
	if legacyPath == cleanDBPath {
		return nil
	}

	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat legacy database: %w", err)
	}

	return migrateLegacyDB(legacyPath, cleanDBPath)
}

func migrateLegacyDB(legacyPath, dbPath string) error {
	if err := filestore.CopyLegacyDB(legacyPath, dbPath); err != nil {
		cleanCorruptCopy(dbPath)
		return fmt.Errorf("failed to copy legacy database: %w", err)
	}

	checkDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		cleanCorruptCopy(dbPath)
		return fmt.Errorf("failed to open migrated database for validation: %w", err)
	}

	var checkResult string
	queryErr := checkDB.QueryRow(`PRAGMA quick_check;`).Scan(&checkResult)
	closeErr := checkDB.Close()

	if queryErr != nil {
		cleanCorruptCopy(dbPath)
		return fmt.Errorf("failed to validate migrated database: %w", queryErr)
	}
	if closeErr != nil {
		cleanCorruptCopy(dbPath)
		return fmt.Errorf("failed to close database after validation: %w", closeErr)
	}
	if checkResult != "ok" {
		cleanCorruptCopy(dbPath)
		return fmt.Errorf("migrated database integrity check failed: %s", checkResult)
	}

	if err := filestore.FinalizeLegacyDBMigration(legacyPath); err != nil {
		return fmt.Errorf("failed to finalize legacy migration: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Migrated legacy database from %s to %s (original renamed to .migrated)\n", legacyPath, dbPath)
	return nil
}

func cleanCorruptCopy(dbPath string) {
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
}
