// Package vault stores projects and tasks as plain folders and Markdown files.
//
// Layout:
//
//	<vault>/
//	  .task-cli.yaml              vault settings (next_id)
//	  0004-task-without-project.md
//	  <Project name>/             project folder (folder name = project name)
//	    .project.yaml             project settings (optional description)
//	    0003-some-task.md         task file: <id>-<slug>.md
//	  .trash/[<Project name>/]    soft-deleted tasks
//
// Task files carry YAML frontmatter (status, priority, dates) followed by a
// "# Title" heading and a free-form Markdown description.
package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.yaml.in/yaml/v3"
)

var (
	ErrProjectExists      = errors.New("project already exists")
	ErrProjectNotFound    = errors.New("project not found")
	ErrInvalidProjectName = errors.New("invalid project name")
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskImmutable      = errors.New("cannot modify task with status deleted or aborted")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrInvalidPriority    = errors.New("invalid priority")
)

// ProjectFile is the hidden settings file inside a project folder.
const ProjectFile = ".project.yaml"

// TrashDir holds soft-deleted tasks (<vault>/.trash/[<project>/]<id>-<slug>.md).
// Hidden folders are never treated as projects, so trashed tasks drop out of
// every query while their IDs stay reserved.
const TrashDir = ".trash"

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusAborted    Status = "aborted"
	StatusDeleted    Status = "deleted"
)

// ParseStatus normalizes a raw status string and reports whether it is valid.
func ParseStatus(raw string) (Status, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending":
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

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// ParsePriority normalizes a raw priority string and reports whether it is valid.
func ParsePriority(raw string) (Priority, bool) {
	switch p := Priority(strings.ToLower(strings.TrimSpace(raw))); p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return p, true
	default:
		return "", false
	}
}

// Project is a folder in the vault root; its name is the folder name.
type Project struct {
	Name        string
	Description string
	Path        string // project folder
}

type Task struct {
	ID          int64
	ProjectName string
	Title       string
	Description string
	Status      Status
	Priority    Priority
	AbortReason string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	Path        string // task file

	// extra keeps unknown frontmatter keys (e.g. tags added by hand) on rewrite.
	extra map[string]any
}

// Mutable reports whether the task may still be changed.
func (t *Task) Mutable() bool {
	return t.Status != StatusDeleted && t.Status != StatusAborted && t.DeletedAt == nil
}

// Store reads and writes a vault rooted at Root.
type Store struct {
	Root string
}

// Open returns a Store for root, creating the folder if needed.
func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating vault %s: %w", root, err)
	}
	return &Store{Root: root}, nil
}

// now is overridable in tests.
var now = func() time.Time { return time.Now().Truncate(time.Second) }

// -----------------------------------------------------------------------------
// Projects
// -----------------------------------------------------------------------------

// projectMeta is the content of <project>/.project.yaml. Unknown keys are ignored.
type projectMeta struct {
	Description string `yaml:"description"`
}

// isProjectDir reports whether a vault entry name is a project folder
// (hidden folders such as .trash and .git are not).
func isProjectDir(name string) bool {
	return !strings.HasPrefix(name, ".")
}

// validateName checks that name can be used as a project folder name.
func validateName(name string) error {
	invalid := name == "" || name == "." || name == ".." || !isProjectDir(name) ||
		strings.ContainsAny(name, `/\`) ||
		strings.IndexFunc(name, unicode.IsControl) >= 0
	if invalid {
		return fmt.Errorf("%w: %q", ErrInvalidProjectName, name)
	}
	return nil
}

// AddProject creates a project folder and its .project.yaml.
func (s *Store) AddProject(name, description string) (*Project, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	if _, err := s.GetProject(name); err == nil {
		return nil, fmt.Errorf("%w: %q", ErrProjectExists, name)
	} else if !errors.Is(err, ErrProjectNotFound) {
		return nil, err
	}

	dir := filepath.Join(s.Root, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrProjectExists, name)
		}
		return nil, fmt.Errorf("creating project folder: %w", err)
	}
	p := &Project{Name: name, Path: dir}
	if err := s.setProjectDescription(p, description); err != nil {
		return nil, err
	}
	return p, nil
}

// ListProjects returns every project folder, sorted by name (case-insensitive).
func (s *Store) ListProjects() ([]Project, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, fmt.Errorf("reading vault: %w", err)
	}
	var projects []Project
	for _, e := range entries {
		if !e.IsDir() || !isProjectDir(e.Name()) {
			continue
		}
		p, err := s.loadProject(e.Name())
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
	return projects, nil
}

// GetProject finds a project by name (case-insensitive).
func (s *Store) GetProject(name string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" || !isProjectDir(name) {
		return nil, ErrProjectNotFound
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, fmt.Errorf("reading vault: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && isProjectDir(e.Name()) && strings.EqualFold(e.Name(), name) {
			return s.loadProject(e.Name())
		}
	}
	return nil, ErrProjectNotFound
}

// RenameProject renames the project folder (tasks move with it).
// A case-only change of the name is allowed.
func (s *Store) RenameProject(name, newName string) (*Project, error) {
	p, err := s.GetProject(name)
	if err != nil {
		return nil, err
	}
	newName = strings.TrimSpace(newName)
	if err := validateName(newName); err != nil {
		return nil, err
	}
	if newName == p.Name {
		return p, nil
	}
	if other, err := s.GetProject(newName); err == nil && other.Path != p.Path {
		return nil, fmt.Errorf("%w: %q", ErrProjectExists, other.Name)
	} else if err != nil && !errors.Is(err, ErrProjectNotFound) {
		return nil, err
	}
	target := filepath.Join(s.Root, newName)
	if err := os.Rename(p.Path, target); err != nil {
		return nil, fmt.Errorf("renaming project folder: %w", err)
	}
	return s.loadProject(newName)
}

// SetProjectDescription updates (or clears) the project description.
func (s *Store) SetProjectDescription(name, description string) (*Project, error) {
	p, err := s.GetProject(name)
	if err != nil {
		return nil, err
	}
	if err := s.setProjectDescription(p, description); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) setProjectDescription(p *Project, description string) error {
	description = strings.TrimSpace(description)
	err := editYAMLFile(filepath.Join(p.Path, ProjectFile), "task-cli project settings", func(m *yaml.Node) {
		if description == "" {
			deleteMappingKey(m, "description")
		} else {
			setMappingValue(m, "description", description, "!!str")
		}
	})
	if err != nil {
		return err
	}
	p.Description = description
	return nil
}

func (s *Store) loadProject(dirName string) (*Project, error) {
	dir := filepath.Join(s.Root, dirName)
	var meta projectMeta
	if err := decodeYAMLFile(filepath.Join(dir, ProjectFile), &meta); err != nil {
		return nil, err
	}
	return &Project{Name: dirName, Description: strings.TrimSpace(meta.Description), Path: dir}, nil
}

// -----------------------------------------------------------------------------
// Tasks
// -----------------------------------------------------------------------------

type taskMeta struct {
	Status      Status         `yaml:"status"`
	Priority    Priority       `yaml:"priority"`
	Created     time.Time      `yaml:"created"`
	Updated     time.Time      `yaml:"updated"`
	AbortReason string         `yaml:"abort_reason,omitempty"`
	Deleted     *time.Time     `yaml:"deleted,omitempty"`
	Extra       map[string]any `yaml:",inline"`
}

var taskFileRe = regexp.MustCompile(`^(\d+)(?:-.*)?\.md$`)

// taskFile is a task location discovered while scanning the vault.
type taskFile struct {
	id      int64
	project string
	path    string
}

// scanTaskFiles lists active task files: vault root and each project folder.
func (s *Store) scanTaskFiles() ([]taskFile, error) {
	return scanTaskFilesIn(s.Root)
}

// scanTrashFiles lists soft-deleted task files (none if there is no trash yet).
func (s *Store) scanTrashFiles() ([]taskFile, error) {
	trash := filepath.Join(s.Root, TrashDir)
	if _, err := os.Stat(trash); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return scanTaskFilesIn(trash)
}

// scanTaskFilesIn lists task files in base and in its project subfolders.
func scanTaskFilesIn(base string) ([]taskFile, error) {
	var files []taskFile
	collect := func(dir, project string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("reading %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			m := taskFileRe.FindStringSubmatch(e.Name())
			if m == nil {
				continue
			}
			id, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil {
				continue
			}
			files = append(files, taskFile{id: id, project: project, path: filepath.Join(dir, e.Name())})
		}
		return nil
	}

	if err := collect(base, ""); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", base, err)
	}
	for _, e := range entries {
		if e.IsDir() && isProjectDir(e.Name()) {
			if err := collect(filepath.Join(base, e.Name()), e.Name()); err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

func (s *Store) findTaskFile(id int64) (*taskFile, error) {
	files, err := s.scanTaskFiles()
	if err != nil {
		return nil, err
	}
	var found *taskFile
	for i := range files {
		if files[i].id != id {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("duplicate task ID %d: %s and %s", id, found.path, files[i].path)
		}
		found = &files[i]
	}
	if found == nil {
		return nil, ErrTaskNotFound
	}
	return found, nil
}

// nextID returns max(next_id from the vault settings, highest ID on disk + 1),
// counting trashed tasks too.
// The stored counter keeps IDs from being reused after files are removed by
// hand; the scan keeps it correct if the counter is stale or missing.
func (s *Store) nextID() (int64, error) {
	files, err := s.scanTaskFiles()
	if err != nil {
		return 0, err
	}
	trashed, err := s.scanTrashFiles()
	if err != nil {
		return 0, err
	}
	files = append(files, trashed...)
	var max int64
	for _, f := range files {
		if f.id > max {
			max = f.id
		}
	}
	settings, err := s.Settings()
	if err != nil {
		return 0, err
	}
	return maxInt64(max+1, settings.NextID), nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func loadTask(f taskFile) (*Task, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("reading task %d: %w", f.id, err)
	}
	var meta taskMeta
	body, err := splitFrontmatter(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", f.path, err)
	}
	title, desc := splitTitle(body)

	t := &Task{
		ID:          f.id,
		ProjectName: f.project,
		Title:       title,
		Description: desc,
		Status:      StatusPending,
		Priority:    PriorityMedium,
		AbortReason: meta.AbortReason,
		CreatedAt:   meta.Created,
		UpdatedAt:   meta.Updated,
		DeletedAt:   meta.Deleted,
		Path:        f.path,
		extra:       meta.Extra,
	}
	if st, ok := ParseStatus(string(meta.Status)); ok {
		t.Status = st
	}
	if pr, ok := ParsePriority(string(meta.Priority)); ok {
		t.Priority = pr
	}
	return t, nil
}

// splitTitle takes the first "# Heading" as the title; the rest is the description.
func splitTitle(body string) (string, string) {
	body = strings.TrimLeft(body, "\r\n")
	first, rest, _ := strings.Cut(body, "\n")
	if strings.HasPrefix(first, "# ") {
		return strings.TrimSpace(first[2:]), strings.TrimSpace(rest)
	}
	return "", strings.TrimSpace(body)
}

func renderTask(t *Task) ([]byte, error) {
	meta := taskMeta{
		Status:      t.Status,
		Priority:    t.Priority,
		Created:     t.CreatedAt,
		Updated:     t.UpdatedAt,
		AbortReason: t.AbortReason,
		Deleted:     t.DeletedAt,
		Extra:       t.extra,
	}
	body := "# " + t.Title + "\n"
	if t.Description != "" {
		body += "\n" + t.Description + "\n"
	}
	return joinFrontmatter(meta, body)
}

// taskPath builds <vault>/[<project>/]<id>-<slug>.md.
func (s *Store) taskPath(id int64, project, title string) string {
	name := fmt.Sprintf("%04d", id)
	if slug := Slugify(title); slug != "" {
		name += "-" + slug
	}
	return filepath.Join(s.Root, project, name+".md")
}

// saveTask writes the task to its canonical path, moving/renaming the old file if needed.
func (s *Store) saveTask(t *Task) error {
	return s.saveTaskTo(t, s.taskPath(t.ID, t.ProjectName, t.Title))
}

// saveTaskTo writes the task to target and removes its previous file.
func (s *Store) saveTaskTo(t *Task, target string) error {
	data, err := renderTask(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(target), err)
	}
	if err := writeFileAtomic(target, data); err != nil {
		return err
	}
	if t.Path != "" && t.Path != target {
		if err := os.Remove(t.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing old task file: %w", err)
		}
	}
	t.Path = target
	return nil
}

// Add creates a new pending task. projectName may be empty (task in vault root).
func (s *Store) Add(title, description, projectName string, priority Priority) (*Task, error) {
	if priority == "" {
		priority = PriorityMedium
	}
	if _, ok := ParsePriority(string(priority)); !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPriority, priority)
	}
	if projectName != "" {
		p, err := s.GetProject(projectName)
		if err != nil {
			return nil, err
		}
		projectName = p.Name
	}
	id, err := s.nextID()
	if err != nil {
		return nil, err
	}
	// Reserve the ID before writing the task: a failed write only burns an ID.
	if err := s.setNextID(id + 1); err != nil {
		return nil, err
	}
	ts := now()
	t := &Task{
		ID:          id,
		ProjectName: projectName,
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Status:      StatusPending,
		Priority:    priority,
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}
	if err := s.saveTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTask returns a non-deleted task by ID.
func (s *Store) GetTask(id int64) (*Task, error) {
	t, err := s.getTaskAny(id)
	if err != nil {
		return nil, err
	}
	if t.Status == StatusDeleted || t.DeletedAt != nil {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

func (s *Store) getTaskAny(id int64) (*Task, error) {
	f, err := s.findTaskFile(id)
	if err != nil {
		return nil, err
	}
	return loadTask(*f)
}

// GetMutableTask returns a task that may be modified, or ErrTaskImmutable.
func (s *Store) GetMutableTask(id int64) (*Task, error) {
	t, err := s.getTaskAny(id)
	if err != nil {
		return nil, err
	}
	if !t.Mutable() {
		return nil, ErrTaskImmutable
	}
	return t, nil
}

// List returns non-deleted tasks filtered by status and project code (both optional),
// ordered by project code then ID.
func (s *Store) List(statusFilter, projectFilter string) ([]Task, error) {
	var status Status
	if statusFilter != "" {
		var ok bool
		if status, ok = ParseStatus(statusFilter); !ok {
			return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, statusFilter)
		}
	}
	files, err := s.scanTaskFiles()
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for _, f := range files {
		if projectFilter != "" && !strings.EqualFold(f.project, strings.TrimSpace(projectFilter)) {
			continue
		}
		t, err := loadTask(f)
		if err != nil {
			return nil, err
		}
		if t.Status == StatusDeleted || t.DeletedAt != nil {
			continue
		}
		if status != "" && t.Status != status {
			continue
		}
		tasks = append(tasks, *t)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		pi, pj := strings.ToLower(tasks[i].ProjectName), strings.ToLower(tasks[j].ProjectName)
		if pi != pj {
			return pi < pj
		}
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}

// UpdateTask applies fn to a mutable task and saves it.
func (s *Store) UpdateTask(id int64, fn func(t *Task) error) (*Task, error) {
	t, err := s.GetMutableTask(id)
	if err != nil {
		return nil, err
	}
	if err := fn(t); err != nil {
		return nil, err
	}
	if _, ok := ParsePriority(string(t.Priority)); !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPriority, t.Priority)
	}
	if t.ProjectName != "" {
		p, err := s.GetProject(t.ProjectName)
		if err != nil {
			return nil, err
		}
		t.ProjectName = p.Name
	}
	t.UpdatedAt = now()
	if err := s.saveTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

// SetStatus changes the status of a mutable task. reason is kept only for aborted.
func (s *Store) SetStatus(id int64, status Status, reason string) (*Task, error) {
	parsed, ok := ParseStatus(string(status))
	if !ok || parsed == StatusDeleted {
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, status)
	}
	return s.UpdateTask(id, func(t *Task) error {
		t.Status = parsed
		t.AbortReason = ""
		if parsed == StatusAborted {
			t.AbortReason = strings.TrimSpace(reason)
		}
		return nil
	})
}

// Delete soft-deletes a task: it is marked with status deleted and moved to
// <vault>/.trash/[<CODE>/], keeping its file name.
func (s *Store) Delete(id int64) error {
	t, err := s.GetTask(id)
	if err != nil {
		return err
	}
	ts := now()
	t.Status = StatusDeleted
	t.DeletedAt = &ts
	t.UpdatedAt = ts
	target := filepath.Join(s.Root, TrashDir, t.ProjectName, filepath.Base(s.taskPath(t.ID, t.ProjectName, t.Title)))
	return s.saveTaskTo(t, target)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

var frontmatterDelim = []byte("---")

// splitFrontmatter decodes a leading YAML frontmatter block into v and returns the body.
// Files without frontmatter return the whole content as body.
func splitFrontmatter(data []byte, v any) (string, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return string(normalized), nil
	}
	rest := normalized[4:]
	var yamlPart, body []byte
	if bytes.HasPrefix(rest, []byte("---\n")) || bytes.Equal(rest, frontmatterDelim) {
		yamlPart, body = nil, bytes.TrimPrefix(rest, frontmatterDelim)
	} else {
		end := bytes.Index(rest, []byte("\n---"))
		if end < 0 {
			return "", errors.New("unterminated frontmatter")
		}
		yamlPart = rest[:end+1]
		body = rest[end+4:]
	}
	if len(bytes.TrimSpace(yamlPart)) > 0 {
		if err := yaml.Unmarshal(yamlPart, v); err != nil {
			return "", fmt.Errorf("invalid frontmatter: %w", err)
		}
	}
	return strings.TrimPrefix(string(body), "\n"), nil
}

func joinFrontmatter(meta any, body string) ([]byte, error) {
	y, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("encoding frontmatter: %w", err)
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(y)
	b.WriteString("---\n")
	b.WriteString(body)
	return b.Bytes(), nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

const maxSlugLen = 50

// Slugify turns a title into a lowercase ASCII file-name fragment.
func Slugify(title string) string {
	s := accentReplacer.Replace(strings.ToLower(title))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugLen {
		// Cut at a word boundary when possible.
		cut := slug[:maxSlugLen+1]
		if i := strings.LastIndexByte(cut, '-'); i > 0 {
			slug = cut[:i]
		} else {
			slug = slug[:maxSlugLen]
		}
	}
	return slug
}
