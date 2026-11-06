package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"task-cli/internal/db"
)

// ProjectAddArgs represents the argument schema for adding a project.
type ProjectAddArgs struct {
	Code        string
	Name        string
	Description string
}

// ProjectAddAction encapsulates schema parsing, execution, and output formatting for project creation.
type ProjectAddAction struct{}

// Parse validates and extracts code, name, and optional description from arguments.
func (a *ProjectAddAction) Parse(ctx *Context, args []string) (ProjectAddArgs, error) {
	if len(args) < 2 {
		return ProjectAddArgs{}, fmt.Errorf("'code' and 'name' are required.\nUsage: task-cli project add <code> <name> [description]")
	}

	code := strings.TrimSpace(args[0])
	name := strings.TrimSpace(args[1])
	var description string
	if len(args) > 2 {
		description = strings.TrimSpace(strings.Join(args[2:], " "))
	}

	return ProjectAddArgs{
		Code:        code,
		Name:        name,
		Description: description,
	}, nil
}

// Execute persists the project into the database.
func (a *ProjectAddAction) Execute(ctx *Context, args ProjectAddArgs) (*db.Project, error) {
	project, err := ctx.Store.AddProject(args.Code, args.Name, args.Description)
	if err != nil {
		if errors.Is(err, db.ErrProjectCodeExists) {
			return nil, fmt.Errorf("project with code '%s' already exists", args.Code)
		}
		return nil, err
	}
	return project, nil
}

// Format renders the project creation confirmation message.
func (a *ProjectAddAction) Format(ctx *Context, w io.Writer, project *db.Project) error {
	_, err := fmt.Fprintf(w, "Project '%s' created successfully (ID: %d)\n", project.Code, project.ID)
	return err
}

// ProjectAddHandler is the standardized handler instance for ProjectAddAction.
var ProjectAddHandler = NewActionHandler(&ProjectAddAction{})

// ProjectAdd executes project creation.
func ProjectAdd(ctx *Context, args []string) error {
	return ProjectAddHandler.Run(ctx, args)
}

// ProjectListArgs represents the argument schema for listing projects.
type ProjectListArgs struct {
	ShowTime bool
	ShowDesc bool
	NoHeader bool
	Format   string
}

// ProjectListResult holds the query results and display options for project listing.
type ProjectListResult struct {
	Projects []db.Project
	ShowTime bool
	ShowDesc bool
	NoHeader bool
	Format   string
}

// ProjectListAction encapsulates schema parsing, execution, and output formatting for listing projects.
type ProjectListAction struct{}

// Parse parses flags for listing projects.
func (a *ProjectListAction) Parse(ctx *Context, args []string) (ProjectListArgs, error) {
	pCmd := flag.NewFlagSet("project-list", flag.ContinueOnError)
	var showTime bool
	var showDesc bool
	var noHeader bool
	var format string
	defaultFormat := FormatTSV
	if ctx != nil && ctx.Format != "" {
		defaultFormat = ctx.Format
	}

	pCmd.BoolVar(&showTime, "t", false, "Include creation date column")
	pCmd.BoolVar(&showTime, "time", false, "Include creation date column")
	pCmd.BoolVar(&showDesc, "d", false, "Include description column")
	pCmd.BoolVar(&showDesc, "desc", false, "Include description column")
	pCmd.BoolVar(&showDesc, "description", false, "Include description column")
	pCmd.BoolVar(&noHeader, "n", false, "Suppress header row")
	pCmd.BoolVar(&noHeader, "no-header", false, "Suppress header row")
	pCmd.StringVar(&format, "format", defaultFormat, "Output format (tsv, table)")
	pCmd.StringVar(&format, "F", defaultFormat, "Output format (shorthand)")
	pCmd.Bool("p", false, "List projects (shorthand)")
	pCmd.Bool("projects", false, "List projects")

	flagArgs, _ := splitFlagsAndPosArgs(args, map[string]bool{
		"-F":       true,
		"--format": true,
	})
	if err := pCmd.Parse(flagArgs); err != nil {
		return ProjectListArgs{}, err
	}

	resolvedFmt, err := validateFormat(format)
	if err != nil {
		return ProjectListArgs{}, err
	}

	return ProjectListArgs{
		ShowTime: showTime,
		ShowDesc: showDesc,
		NoHeader: noHeader,
		Format:   resolvedFmt,
	}, nil
}

// Execute queries the database for all projects.
func (a *ProjectListAction) Execute(ctx *Context, args ProjectListArgs) (*ProjectListResult, error) {
	projects, err := ctx.Store.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}

	return &ProjectListResult{
		Projects: projects,
		ShowTime: args.ShowTime,
		ShowDesc: args.ShowDesc,
		NoHeader: args.NoHeader,
		Format:   args.Format,
	}, nil
}

// Format delegates project output rendering to the tabular output formatter.
func (a *ProjectListAction) Format(ctx *Context, w io.Writer, res *ProjectListResult) error {
	if len(res.Projects) == 0 {
		return nil
	}

	cols := []string{"id", "code", "name"}
	if res.ShowDesc {
		cols = append(cols, "description")
	}
	if res.ShowTime {
		cols = append(cols, "created_at")
	}

	var rows [][]string
	for _, p := range res.Projects {
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		row := []string{
			fmt.Sprintf("%d", p.ID),
			p.Code,
			p.Name,
		}
		if res.ShowDesc {
			row = append(row, cleanOneLine(desc))
		}
		if res.ShowTime {
			row = append(row, p.CreatedAt.Local().Format("2006-01-02T15:04:05"))
		}
		rows = append(rows, row)
	}

	return printOutput(w, res.Format, res.NoHeader, cols, rows)
}

// ProjectListHandler is the standardized handler instance for ProjectListAction.
var ProjectListHandler = NewActionHandler(&ProjectListAction{})

// ProjectList lists all registered projects in TSV or table format.
func ProjectList(ctx *Context, args []string) error {
	return ProjectListHandler.Run(ctx, args)
}

// ProjectViewArgs represents the argument schema for viewing a project.
type ProjectViewArgs struct {
	Target   string
	ShowTime bool
}

// ProjectViewResult holds the retrieved project and display options.
type ProjectViewResult struct {
	Project  *db.Project
	ShowTime bool
}

// ProjectViewAction encapsulates schema parsing, execution, and output formatting for project details.
type ProjectViewAction struct{}

// Parse validates arguments and parses flags for project viewing.
func (a *ProjectViewAction) Parse(ctx *Context, args []string) (ProjectViewArgs, error) {
	viewCmd := flag.NewFlagSet("project view", flag.ContinueOnError)
	var showTime bool
	viewCmd.BoolVar(&showTime, "t", false, "Show timestamps (created_at, updated_at)")
	viewCmd.BoolVar(&showTime, "time", false, "Show timestamps (created_at, updated_at)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	if err := viewCmd.Parse(flagArgs); err != nil {
		return ProjectViewArgs{}, err
	}

	if len(posArgs) < 1 {
		return ProjectViewArgs{}, fmt.Errorf("project code or ID is required.\nUsage: task-cli project view <code|id> [-t]")
	}

	return ProjectViewArgs{
		Target:   strings.TrimSpace(posArgs[0]),
		ShowTime: showTime,
	}, nil
}

// Execute resolves the project by code or ID from the database.
func (a *ProjectViewAction) Execute(ctx *Context, args ProjectViewArgs) (*ProjectViewResult, error) {
	project, err := ctx.Store.GetProjectByCode(args.Target)
	if err != nil && errors.Is(err, db.ErrProjectNotFound) {
		if id, parseErr := strconv.ParseInt(args.Target, 10, 64); parseErr == nil {
			project, err = ctx.Store.GetProjectByID(id)
		}
	}

	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return nil, fmt.Errorf("project '%s' not found", args.Target)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &ProjectViewResult{
		Project:  project,
		ShowTime: args.ShowTime,
	}, nil
}

// Format renders the full project details.
func (a *ProjectViewAction) Format(ctx *Context, w io.Writer, res *ProjectViewResult) error {
	project := res.Project
	fmt.Fprintf(w, "Project: %s (ID: %d)\n", project.Code, project.ID)
	fmt.Fprintf(w, "Name: %s\n", project.Name)
	if res.ShowTime {
		fmt.Fprintf(w, "Created: %s\n", project.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(w, "Updated: %s\n", project.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if project.Description != "" {
		fmt.Fprintf(w, "Description:\n%s\n", project.Description)
	}
	return nil
}

// ProjectViewHandler is the standardized handler instance for ProjectViewAction.
var ProjectViewHandler = NewActionHandler(&ProjectViewAction{})

// ProjectView displays project details by code or ID.
func ProjectView(ctx *Context, args []string) error {
	return ProjectViewHandler.Run(ctx, args)
}
