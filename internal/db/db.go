package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrProjectCodeExists = errors.New("project code already exists")
	ErrProjectNotFound   = errors.New("project not found")
	ErrTaskNotFound      = errors.New("task not found")
	ErrDocumentNotFound  = errors.New("document not found")
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
	Priority    string
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

// DefaultDBPath returns the default SQLite database path (~/.task-cli.db).
func DefaultDBPath() (string, error) {
	if envPath := os.Getenv("TASK_CLI_DB"); envPath != "" {
		return envPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "tasks.db", nil
	}
	return filepath.Join(home, ".task-cli.db"), nil
}

// NewStore opens or creates the SQLite database and initializes the schema.
func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// modernc.org/sqlite registers the "sqlite" driver
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Enable WAL mode and foreign keys
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure PRAGMAs: %w", err)
	}

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
	_, _ = db.Exec(`UPDATE projects SET code = TRIM(code, ', '), name = TRIM(name, ', ') WHERE code LIKE '%,';`)
	_, _ = db.Exec(`UPDATE tasks SET status = 'pending' WHERE status = 'todo';`)
	_, _ = db.Exec(`UPDATE tasks SET status = 'aborted' WHERE status = 'abort';`)

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// AddProject creates a new project.
func (s *Store) AddProject(code, name, description string) (*Project, error) {
	code = strings.Trim(code, ", ")
	name = strings.Trim(name, ", ")
	description = strings.Trim(description, ", ")

	now := time.Now().UTC()
	query := `INSERT INTO projects (code, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, code, name, description, now, now)
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
func (s *Store) ListProjects() ([]Project, error) {
	query := `SELECT id, code, name, COALESCE(description, ''), created_at, updated_at FROM projects ORDER BY id ASC`
	rows, err := s.db.Query(query)
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
func (s *Store) GetProjectByCode(code string) (*Project, error) {
	cleanCode := strings.Trim(code, ", ")
	query := `SELECT id, code, name, COALESCE(description, ''), created_at, updated_at
	          FROM projects
	          WHERE code = ? COLLATE NOCASE`
	row := s.db.QueryRow(query, cleanCode)

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
func (s *Store) GetProjectByID(id int64) (*Project, error) {
	query := `SELECT id, code, name, COALESCE(description, ''), created_at, updated_at
	          FROM projects
	          WHERE id = ?`
	row := s.db.QueryRow(query, id)

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
func (s *Store) UpdateProject(id int64, name, description string) error {
	now := time.Now().UTC()
	query := `UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`
	res, err := s.db.Exec(query, name, description, now, id)
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
func (s *Store) Add(title, description string, projectID *int64, priority string) (*Task, error) {
	now := time.Now().UTC()
	if priority == "" {
		priority = "medium"
	}
	query := `INSERT INTO tasks (project_id, title, description, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, projectID, title, description, string(StatusPending), priority, now, now)
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

// List returns active tasks (soft deleted tasks excluded), with optional status and project code filters.
func (s *Store) List(statusFilter, projectCodeFilter string) ([]Task, error) {
	query := `SELECT t.id, t.project_id, COALESCE(p.code, ''), t.title, COALESCE(t.description, ''),
	                 t.status, t.priority, t.abort_reason, t.created_at, t.updated_at, t.deleted_at
	          FROM tasks t
	          LEFT JOIN projects p ON t.project_id = p.id
	          WHERE t.deleted_at IS NULL`
	var args []any

	if statusFilter != "" {
		sf := statusFilter
		if sf == "abort" {
			sf = string(StatusAborted)
		}
		query += ` AND t.status = ?`
		args = append(args, sf)
	}

	if projectCodeFilter != "" {
		cleanProj := strings.Trim(projectCodeFilter, ", ")
		query += ` AND p.code = ? COLLATE NOCASE`
		args = append(args, cleanProj)
	}

	query += ` ORDER BY t.id ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var st string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.ProjectCode, &t.Title, &t.Description, &st, &t.Priority, &t.AbortReason, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		t.Status = Status(st)
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// GetTaskByID returns a task by its ID (including project code).
func (s *Store) GetTaskByID(id int64) (*Task, error) {
	query := `SELECT t.id, t.project_id, COALESCE(p.code, ''), t.title, COALESCE(t.description, ''),
	                 t.status, t.priority, t.abort_reason, t.created_at, t.updated_at, t.deleted_at
	          FROM tasks t
	          LEFT JOIN projects p ON t.project_id = p.id
	          WHERE t.id = ? AND t.deleted_at IS NULL`
	row := s.db.QueryRow(query, id)

	var t Task
	var st string
	if err := row.Scan(&t.ID, &t.ProjectID, &t.ProjectCode, &t.Title, &t.Description, &st, &t.Priority, &t.AbortReason, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to query task by id: %w", err)
	}
	t.Status = Status(st)
	return &t, nil
}

// Update updates the title and description of an active task.
func (s *Store) Update(id int64, title, description string) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET title = ?, description = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, title, description, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
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

// UpdateTaskDetails updates title, description, project_id, and priority of an active task.
func (s *Store) UpdateTaskDetails(id int64, title, description string, projectID *int64, priority string) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET title = ?, description = ?, project_id = ?, priority = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, title, description, projectID, priority, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task details: %w", err)
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

// UpdateDescription updates only the description of an active task.
func (s *Store) UpdateDescription(id int64, description string) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET description = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, description, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task description: %w", err)
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

// UpdateStatus updates the status of an active task and optionally sets an abort reason.
func (s *Store) UpdateStatus(id int64, status Status, abortReason *string) error {
	if status == "abort" {
		status = StatusAborted
	}
	now := time.Now().UTC()
	query := `UPDATE tasks SET status = ?, abort_reason = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, string(status), abortReason, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
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

// UpdatePriority updates the priority of an active task.
func (s *Store) UpdatePriority(id int64, priority string) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET priority = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, priority, now, id)
	if err != nil {
		return fmt.Errorf("failed to update task priority: %w", err)
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

// Delete performs a soft delete on a task by setting its deleted_at timestamp and updating its status to deleted.
func (s *Store) Delete(id int64) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET status = ?, deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, string(StatusDeleted), now, now, id)
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
func (s *Store) AddDocument(taskID int64, name, docType, filePath string) (*Document, error) {
	if _, err := s.GetTaskByID(taskID); err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	docType = strings.TrimSpace(docType)
	filePath = strings.TrimSpace(filePath)

	now := time.Now().UTC()
	query := `INSERT INTO documents (task_id, name, type, file_path, created_at) VALUES (?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, taskID, name, docType, filePath, now)
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
func (s *Store) ListDocuments(taskID int64) ([]Document, error) {
	query := `SELECT id, task_id, name, type, file_path, created_at FROM documents WHERE task_id = ? ORDER BY id ASC`
	rows, err := s.db.Query(query, taskID)
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
func (s *Store) GetDocumentByID(id int64) (*Document, error) {
	query := `SELECT id, task_id, name, type, file_path, created_at FROM documents WHERE id = ?`
	row := s.db.QueryRow(query, id)

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
func (s *Store) DeleteDocument(id int64) error {
	query := `DELETE FROM documents WHERE id = ?`
	res, err := s.db.Exec(query, id)
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
