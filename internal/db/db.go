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
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusAbort      Status = "abort"
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
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// Clean up any trailing commas from legacy input and migrate legacy 'todo' status to 'pending'
	_, _ = db.Exec(`UPDATE projects SET code = TRIM(code, ', '), name = TRIM(name, ', ') WHERE code LIKE '%,';`)
	_, _ = db.Exec(`UPDATE tasks SET status = 'pending' WHERE status = 'todo';`)

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
		query += ` AND t.status = ?`
		args = append(args, statusFilter)
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
		return fmt.Errorf("task with ID %d not found or deleted", id)
	}
	return nil
}

// UpdateStatus updates the status of an active task and optionally sets an abort reason.
func (s *Store) UpdateStatus(id int64, status Status, abortReason *string) error {
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
		return fmt.Errorf("task with ID %d not found or deleted", id)
	}
	return nil
}

// Delete performs a soft delete on a task by setting its deleted_at timestamp.
func (s *Store) Delete(id int64) error {
	now := time.Now().UTC()
	query := `UPDATE tasks SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := s.db.Exec(query, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete task: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("task with ID %d not found or already deleted", id)
	}
	return nil
}
