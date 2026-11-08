package db

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// CurrentSchemaVersion defines the supported JSON transfer schema version.
const CurrentSchemaVersion = 1

var (
	ErrInvalidSchemaVersion = errors.New("invalid or unsupported schema version")
	ErrInvalidStatus        = errors.New("invalid task status")
	ErrInvalidPriority      = errors.New("invalid task priority")
)

// DocumentTransfer represents the JSON serialization model for a document.
type DocumentTransfer struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	FilePath  string    `json:"file_path"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskTransfer represents the JSON serialization model for a task.
type TaskTransfer struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      Status             `json:"status"`
	Priority    string             `json:"priority"`
	AbortReason *string            `json:"abort_reason"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	DeletedAt   *time.Time         `json:"deleted_at"`
	Documents   []DocumentTransfer `json:"documents"`
}

// ProjectTransfer represents the JSON serialization model for a project.
type ProjectTransfer struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Tasks       []TaskTransfer `json:"tasks"`
}

// TransferData represents the root JSON schema for import and export.
type TransferData struct {
	Version         int               `json:"version"`
	ExportedAt      time.Time         `json:"exported_at"`
	Projects        []ProjectTransfer `json:"projects"`
	StandaloneTasks []TaskTransfer    `json:"standalone_tasks"`
}

// ExportOptions specifies filtering options for exporting data.
type ExportOptions struct {
	ProjectCode    string
	Status         string
	IncludeDeleted bool
}

// ImportOptions specifies flags and behavior for importing data.
type ImportOptions struct {
	Overwrite bool
	Strict    bool
	ForceDocs bool
	DryRun    bool
}

// ImportResult contains statistics about the completed import operation.
type ImportResult struct {
	ProjectsImported  int      `json:"projects_imported"`
	ProjectsUpdated   int      `json:"projects_updated"`
	ProjectsSkipped   int      `json:"projects_skipped"`
	TasksImported     int      `json:"tasks_imported"`
	TasksSkipped      int      `json:"tasks_skipped"`
	DocumentsImported int      `json:"documents_imported"`
	DocumentsSkipped  int      `json:"documents_skipped"`
	Warnings          []string `json:"warnings,omitempty"`
}

func isValidStatus(s Status) bool {
	switch strings.ToLower(strings.TrimSpace(string(s))) {
	case string(StatusPending), string(StatusInProgress), string(StatusDone), string(StatusAbort), string(StatusDeleted):
		return true
	default:
		return false
	}
}

func isValidPriority(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "low", "medium", "high", "urgent":
		return true
	default:
		return false
	}
}

// ExportData exports projects, tasks, and documents according to the given options.
func (s *Store) ExportData(opts ExportOptions) (*TransferData, error) {
	data := &TransferData{
		Version:         CurrentSchemaVersion,
		ExportedAt:      time.Now().UTC(),
		Projects:        make([]ProjectTransfer, 0),
		StandaloneTasks: make([]TaskTransfer, 0),
	}

	opts.ProjectCode = strings.Trim(opts.ProjectCode, ", ")
	opts.Status = strings.TrimSpace(opts.Status)

	var targetProjects []Project
	if opts.ProjectCode != "" {
		p, err := s.GetProjectByCode(opts.ProjectCode)
		if err != nil {
			return nil, err
		}
		targetProjects = append(targetProjects, *p)
	} else {
		var err error
		targetProjects, err = s.ListProjects()
		if err != nil {
			return nil, fmt.Errorf("failed to list projects for export: %w", err)
		}
	}

	// Fetch all documents indexed by task ID
	docRows, err := s.db.Query(`SELECT task_id, name, type, file_path, created_at FROM documents ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents for export: %w", err)
	}
	defer docRows.Close()

	docsByTask := make(map[int64][]DocumentTransfer)
	for docRows.Next() {
		var taskID int64
		var d DocumentTransfer
		if err := docRows.Scan(&taskID, &d.Name, &d.Type, &d.FilePath, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan document for export: %w", err)
		}
		docsByTask[taskID] = append(docsByTask[taskID], d)
	}
	if err := docRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading documents: %w", err)
	}

	// Build task query
	query := `SELECT t.id, t.project_id, COALESCE(p.code, ''), t.title, COALESCE(t.description, ''),
	                 t.status, t.priority, t.abort_reason, t.created_at, t.updated_at, t.deleted_at
	          FROM tasks t
	          LEFT JOIN projects p ON t.project_id = p.id
	          WHERE 1=1`
	var args []any

	if !opts.IncludeDeleted {
		query += ` AND t.deleted_at IS NULL`
	}
	if opts.Status != "" {
		query += ` AND t.status = ?`
		args = append(args, opts.Status)
	}
	if opts.ProjectCode != "" {
		query += ` AND p.code = ? COLLATE NOCASE`
		args = append(args, opts.ProjectCode)
	}
	query += ` ORDER BY t.id ASC`

	taskRows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks for export: %w", err)
	}
	defer taskRows.Close()

	projectTasks := make(map[int64][]TaskTransfer)
	for taskRows.Next() {
		var t Task
		var st string
		if err := taskRows.Scan(&t.ID, &t.ProjectID, &t.ProjectCode, &t.Title, &t.Description, &st, &t.Priority, &t.AbortReason, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task for export: %w", err)
		}
		t.Status = Status(st)

		docs := docsByTask[t.ID]
		if docs == nil {
			docs = make([]DocumentTransfer, 0)
		}

		tt := TaskTransfer{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Status:      t.Status,
			Priority:    t.Priority,
			AbortReason: t.AbortReason,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
			DeletedAt:   t.DeletedAt,
			Documents:   docs,
		}

		if t.ProjectID != nil {
			projectTasks[*t.ProjectID] = append(projectTasks[*t.ProjectID], tt)
		} else {
			data.StandaloneTasks = append(data.StandaloneTasks, tt)
		}
	}
	if err := taskRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading tasks: %w", err)
	}

	for _, p := range targetProjects {
		tasks := projectTasks[p.ID]
		if tasks == nil {
			tasks = make([]TaskTransfer, 0)
		}
		data.Projects = append(data.Projects, ProjectTransfer{
			Code:        p.Code,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
			Tasks:       tasks,
		})
	}

	return data, nil
}

// ImportData imports projects, tasks, and documents from a TransferData struct.
func (s *Store) ImportData(data *TransferData, opts ImportOptions) (*ImportResult, error) {
	if data == nil {
		return nil, errors.New("cannot import nil transfer data")
	}

	if data.Version != CurrentSchemaVersion {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidSchemaVersion, CurrentSchemaVersion, data.Version)
	}

	res := &ImportResult{
		Warnings: make([]string, 0),
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	importTask := func(t TaskTransfer, projectID *int64) error {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			if opts.Strict {
				return errors.New("task title cannot be empty")
			}
			res.Warnings = append(res.Warnings, "skipped task with empty title")
			res.TasksSkipped++
			return nil
		}

		if !isValidStatus(t.Status) {
			if opts.Strict {
				return fmt.Errorf("%w: %q", ErrInvalidStatus, t.Status)
			}
			res.Warnings = append(res.Warnings, fmt.Sprintf("task %q skipped: %v (%s)", title, ErrInvalidStatus, t.Status))
			res.TasksSkipped++
			return nil
		}
		st := Status(strings.ToLower(strings.TrimSpace(string(t.Status))))

		prio := strings.ToLower(strings.TrimSpace(t.Priority))
		if !isValidPriority(prio) {
			if opts.Strict {
				return fmt.Errorf("%w: %q", ErrInvalidPriority, t.Priority)
			}
			res.Warnings = append(res.Warnings, fmt.Sprintf("task %q skipped: %v (%s)", title, ErrInvalidPriority, t.Priority))
			res.TasksSkipped++
			return nil
		}

		createdAt := t.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		updatedAt := t.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = createdAt
		}

		taskRes, err := tx.Exec(
			`INSERT INTO tasks (project_id, title, description, status, priority, abort_reason, created_at, updated_at, deleted_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, title, t.Description, string(st), prio, t.AbortReason, createdAt, updatedAt, t.DeletedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert task %q: %w", title, err)
		}

		taskID, err := taskRes.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get inserted task ID: %w", err)
		}
		res.TasksImported++

		seenDocs := make(map[string]bool)
		for _, doc := range t.Documents {
			docPath := strings.TrimSpace(doc.FilePath)
			if docPath == "" {
				continue
			}
			if !opts.ForceDocs && seenDocs[docPath] {
				res.DocumentsSkipped++
				continue
			}
			seenDocs[docPath] = true

			docName := strings.TrimSpace(doc.Name)
			if docName == "" {
				docName = filepath.Base(docPath)
			}
			docType := strings.TrimSpace(doc.Type)
			if docType == "" {
				ext := strings.TrimPrefix(filepath.Ext(docPath), ".")
				if ext == "" {
					docType = "unknown"
				} else {
					docType = strings.ToLower(ext)
				}
			}
			docCreatedAt := doc.CreatedAt
			if docCreatedAt.IsZero() {
				docCreatedAt = time.Now().UTC()
			}

			_, err := tx.Exec(
				`INSERT INTO documents (task_id, name, type, file_path, created_at) VALUES (?, ?, ?, ?, ?)`,
				taskID, docName, docType, docPath, docCreatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to insert document %q: %w", docName, err)
			}
			res.DocumentsImported++
		}

		return nil
	}

	for _, proj := range data.Projects {
		code := strings.Trim(proj.Code, ", ")
		if code == "" {
			if opts.Strict {
				return nil, errors.New("project code cannot be empty")
			}
			res.Warnings = append(res.Warnings, "skipped project with empty code")
			res.ProjectsSkipped++
			continue
		}

		name := strings.Trim(proj.Name, ", ")
		if name == "" {
			name = code
		}

		var projectID int64
		var existingName, existingDesc string
		err := tx.QueryRow(`SELECT id, name, COALESCE(description, '') FROM projects WHERE code = ? COLLATE NOCASE`, code).Scan(&projectID, &existingName, &existingDesc)

		if err == nil {
			// Project exists
			if opts.Overwrite {
				projUpdated := proj.UpdatedAt
				if projUpdated.IsZero() {
					projUpdated = time.Now().UTC()
				}
				_, err := tx.Exec(`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`, name, proj.Description, projUpdated, projectID)
				if err != nil {
					return nil, fmt.Errorf("failed to update project %s: %w", code, err)
				}
				res.ProjectsUpdated++
			} else {
				res.ProjectsSkipped++
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			projCreated := proj.CreatedAt
			if projCreated.IsZero() {
				projCreated = time.Now().UTC()
			}
			projUpdated := proj.UpdatedAt
			if projUpdated.IsZero() {
				projUpdated = projCreated
			}
			insertRes, err := tx.Exec(`INSERT INTO projects (code, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, code, name, proj.Description, projCreated, projUpdated)
			if err != nil {
				return nil, fmt.Errorf("failed to insert project %s: %w", code, err)
			}
			projectID, err = insertRes.LastInsertId()
			if err != nil {
				return nil, fmt.Errorf("failed to get inserted project ID: %w", err)
			}
			res.ProjectsImported++
		} else {
			return nil, fmt.Errorf("failed to check existing project %s: %w", code, err)
		}

		for _, t := range proj.Tasks {
			if err := importTask(t, &projectID); err != nil {
				return nil, err
			}
		}
	}

	for _, t := range data.StandaloneTasks {
		if err := importTask(t, nil); err != nil {
			return nil, err
		}
	}

	if opts.DryRun {
		_ = tx.Rollback()
		tx = nil
		return res, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx = nil

	return res, nil
}
