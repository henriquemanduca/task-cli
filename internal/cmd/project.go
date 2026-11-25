package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"task-cli/internal/vault"
)

// projectError turns vault sentinel errors into user-facing messages.
func projectError(name string, err error) error {
	if errors.Is(err, vault.ErrProjectNotFound) {
		return fmt.Errorf("project '%s' not found", name)
	}
	return err
}

// -----------------------------------------------------------------------------
// ProjectAdd
// -----------------------------------------------------------------------------

// ProjectAddArgs represents the argument schema for adding a project.
type ProjectAddArgs struct {
	Name        string
	Description string
}

// ProjectAddAction encapsulates schema parsing, execution, and output formatting for project creation.
type ProjectAddAction struct{}

// Parse extracts the project name and optional description.
func (a *ProjectAddAction) Parse(ctx *Context, args []string) (ProjectAddArgs, error) {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return ProjectAddArgs{}, fmt.Errorf("'name' is required.\nUsage: task-cli project add <name> [description]")
	}
	return ProjectAddArgs{
		Name:        strings.TrimSpace(args[0]),
		Description: strings.TrimSpace(strings.Join(args[1:], " ")),
	}, nil
}

// Execute creates the project folder in the vault.
func (a *ProjectAddAction) Execute(ctx *Context, args ProjectAddArgs) (*vault.Project, error) {
	project, err := ctx.Store.AddProject(args.Name, args.Description)
	if err != nil {
		if errors.Is(err, vault.ErrProjectExists) {
			return nil, fmt.Errorf("project '%s' already exists", args.Name)
		}
		return nil, err
	}
	return project, nil
}

// Format renders the project creation confirmation message.
func (a *ProjectAddAction) Format(ctx *Context, w io.Writer, project *vault.Project) error {
	_, err := fmt.Fprintf(w, "Project '%s' created successfully\n", project.Name)
	return err
}

// ProjectAddHandler is the standardized handler instance for ProjectAddAction.
var ProjectAddHandler = NewActionHandler(&ProjectAddAction{})

// ProjectAdd executes project creation.
func ProjectAdd(ctx *Context, args []string) error {
	return ProjectAddHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// ProjectList
// -----------------------------------------------------------------------------

// ProjectListArgs represents the argument schema for listing projects.
type ProjectListArgs struct {
	ShowDesc bool
	NoHeader bool
	Format   string
}

// ProjectListResult holds the query results and display options for project listing.
type ProjectListResult struct {
	Projects []vault.Project
	ProjectListArgs
}

// ProjectListAction encapsulates schema parsing, execution, and output formatting for listing projects.
type ProjectListAction struct{}

// Parse parses flags for listing projects.
func (a *ProjectListAction) Parse(ctx *Context, args []string) (ProjectListArgs, error) {
	pCmd := flag.NewFlagSet("project-list", flag.ContinueOnError)
	var showDesc, noHeader bool
	var format string
	defaultFormat := FormatTSV
	if ctx != nil && ctx.Format != "" {
		defaultFormat = ctx.Format
	}

	pCmd.BoolVar(&showDesc, "d", false, "Include description column")
	pCmd.BoolVar(&showDesc, "desc", false, "Include description column")
	pCmd.BoolVar(&showDesc, "description", false, "Include description column")
	pCmd.BoolVar(&noHeader, "n", false, "Suppress header row")
	pCmd.BoolVar(&noHeader, "no-header", false, "Suppress header row")
	pCmd.StringVar(&format, "format", defaultFormat, "Output format (tsv, table)")
	pCmd.StringVar(&format, "F", defaultFormat, "Output format (shorthand)")

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
	return ProjectListArgs{ShowDesc: showDesc, NoHeader: noHeader, Format: resolvedFmt}, nil
}

// Execute lists all project folders in the vault.
func (a *ProjectListAction) Execute(ctx *Context, args ProjectListArgs) (*ProjectListResult, error) {
	projects, err := ctx.Store.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	return &ProjectListResult{Projects: projects, ProjectListArgs: args}, nil
}

// Format delegates project output rendering to the tabular output formatter.
func (a *ProjectListAction) Format(ctx *Context, w io.Writer, res *ProjectListResult) error {
	cols := []string{"name"}
	if res.ShowDesc {
		cols = append(cols, "description")
	}

	var rows [][]string
	for _, p := range res.Projects {
		row := []string{cleanOneLine(p.Name)}
		if res.ShowDesc {
			row = append(row, cleanOneLine(p.Description))
		}
		rows = append(rows, row)
	}
	return printOutput(w, res.Format, res.NoHeader, cols, rows)
}

// ProjectListHandler is the standardized handler instance for ProjectListAction.
var ProjectListHandler = NewActionHandler(&ProjectListAction{})

// ProjectList lists all projects in TSV or table format.
func ProjectList(ctx *Context, args []string) error {
	return ProjectListHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// ProjectView
// -----------------------------------------------------------------------------

// ProjectViewArgs represents the argument schema for viewing a project.
type ProjectViewArgs struct {
	Name string
}

// ProjectViewAction encapsulates schema parsing, execution, and output formatting for project details.
type ProjectViewAction struct{}

// Parse validates the project name argument.
func (a *ProjectViewAction) Parse(ctx *Context, args []string) (ProjectViewArgs, error) {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return ProjectViewArgs{}, fmt.Errorf("project name is required.\nUsage: task-cli project view <name>")
	}
	return ProjectViewArgs{Name: strings.TrimSpace(args[0])}, nil
}

// Execute resolves the project by name.
func (a *ProjectViewAction) Execute(ctx *Context, args ProjectViewArgs) (*vault.Project, error) {
	project, err := ctx.Store.GetProject(args.Name)
	if err != nil {
		return nil, projectError(args.Name, err)
	}
	return project, nil
}

// Format renders the full project details.
func (a *ProjectViewAction) Format(ctx *Context, w io.Writer, project *vault.Project) error {
	fmt.Fprintf(w, "Project: %s\n", project.Name)
	fmt.Fprintf(w, "Folder: %s\n", project.Path)
	if project.Description != "" {
		fmt.Fprintf(w, "Description:\n%s\n", project.Description)
	}
	return nil
}

// ProjectViewHandler is the standardized handler instance for ProjectViewAction.
var ProjectViewHandler = NewActionHandler(&ProjectViewAction{})

// ProjectView displays project details by name.
func ProjectView(ctx *Context, args []string) error {
	return ProjectViewHandler.Run(ctx, args)
}

// -----------------------------------------------------------------------------
// ProjectUpdate
// -----------------------------------------------------------------------------

// ProjectUpdateArgs represents the argument schema for updating a project.
type ProjectUpdateArgs struct {
	Name           string
	NewName        string
	HasNewName     bool
	Description    string
	HasDescription bool
}

// ProjectUpdateAction encapsulates schema parsing, execution, and output formatting for project updates.
type ProjectUpdateAction struct{}

const projectUpdateUsage = "Usage: task-cli project update <name> [--name <new name>] [--desc <description>]"

// Parse parses flags and the project name for updating a project.
func (a *ProjectUpdateAction) Parse(ctx *Context, args []string) (ProjectUpdateArgs, error) {
	updateCmd := flag.NewFlagSet("project update", flag.ContinueOnError)
	var newName, desc string
	updateCmd.StringVar(&newName, "name", "", "Rename the project (renames its folder)")
	updateCmd.StringVar(&desc, "desc", "", "New project description (empty clears it)")
	updateCmd.StringVar(&desc, "description", "", "New project description (empty clears it)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"--name":        true,
		"-name":         true,
		"--desc":        true,
		"-desc":         true,
		"--description": true,
		"-description":  true,
	})
	if err := updateCmd.Parse(flagArgs); err != nil {
		return ProjectUpdateArgs{}, err
	}
	if len(posArgs) < 1 || strings.TrimSpace(posArgs[0]) == "" {
		return ProjectUpdateArgs{}, fmt.Errorf("project name is required.\n%s", projectUpdateUsage)
	}

	parsed := ProjectUpdateArgs{
		Name:           strings.TrimSpace(posArgs[0]),
		NewName:        strings.TrimSpace(newName),
		HasNewName:     flagWasSet(updateCmd, "name"),
		Description:    desc,
		HasDescription: flagWasSet(updateCmd, "desc", "description"),
	}
	if !parsed.HasNewName && !parsed.HasDescription {
		return ProjectUpdateArgs{}, fmt.Errorf("nothing to update.\n%s", projectUpdateUsage)
	}
	return parsed, nil
}

// Execute renames the project folder and/or updates its description.
func (a *ProjectUpdateAction) Execute(ctx *Context, args ProjectUpdateArgs) (*vault.Project, error) {
	project, err := ctx.Store.GetProject(args.Name)
	if err != nil {
		return nil, projectError(args.Name, err)
	}
	if args.HasNewName {
		project, err = ctx.Store.RenameProject(project.Name, args.NewName)
		if err != nil {
			if errors.Is(err, vault.ErrProjectExists) {
				return nil, fmt.Errorf("project '%s' already exists", args.NewName)
			}
			return nil, err
		}
	}
	if args.HasDescription {
		project, err = ctx.Store.SetProjectDescription(project.Name, args.Description)
		if err != nil {
			return nil, err
		}
	}
	return project, nil
}

// Format renders the update confirmation message.
func (a *ProjectUpdateAction) Format(ctx *Context, w io.Writer, project *vault.Project) error {
	_, err := fmt.Fprintf(w, "Project '%s' updated successfully\n", project.Name)
	return err
}

// ProjectUpdateHandler is the standardized handler instance for ProjectUpdateAction.
var ProjectUpdateHandler = NewActionHandler(&ProjectUpdateAction{})

// ProjectUpdate renames a project and/or updates its description.
func ProjectUpdate(ctx *Context, args []string) error {
	return ProjectUpdateHandler.Run(ctx, args)
}

// flagWasSet reports whether any of the named flags was explicitly provided.
func flagWasSet(fs *flag.FlagSet, names ...string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		for _, name := range names {
			if f.Name == name {
				found = true
			}
		}
	})
	return found
}
