package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"task-cli/internal/db"
)

// ProjectAdd executa a criação de um novo projeto.
func ProjectAdd(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("'code' and 'name' are required.\nUsage: task-cli project add <code> <name> [description]")
	}

	code := strings.TrimSpace(args[0])
	name := strings.TrimSpace(args[1])
	var description string
	if len(args) > 2 {
		description = strings.TrimSpace(strings.Join(args[2:], " "))
	}

	project, err := ctx.Store.AddProject(code, name, description)
	if err != nil {
		if errors.Is(err, db.ErrProjectCodeExists) {
			return fmt.Errorf("project with code '%s' already exists", code)
		}
		return err
	}

	fmt.Printf("Project '%s' created successfully (ID: %d)\n", project.Code, project.ID)
	return nil
}

// ProjectList lista todos os projetos cadastrados em formato TSV ou tabela.
func ProjectList(ctx *Context, args []string) error {
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
		return err
	}

	if _, err := validateFormat(format); err != nil {
		return err
	}

	projects, err := ctx.Store.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	if len(projects) == 0 {
		return nil
	}

	cols := []string{"id", "code", "name"}
	if showDesc {
		cols = append(cols, "description")
	}
	if showTime {
		cols = append(cols, "created_at")
	}

	var rows [][]string
	for _, p := range projects {
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		row := []string{
			fmt.Sprintf("%d", p.ID),
			p.Code,
			p.Name,
		}
		if showDesc {
			row = append(row, cleanOneLine(desc))
		}
		if showTime {
			row = append(row, p.CreatedAt.Local().Format("2006-01-02T15:04:05"))
		}
		rows = append(rows, row)
	}

	return printOutput(os.Stdout, format, noHeader, cols, rows)
}

// ProjectView exibe os detalhes de um projeto específico por código ou ID.
func ProjectView(ctx *Context, args []string) error {
	viewCmd := flag.NewFlagSet("project view", flag.ContinueOnError)
	var showTime bool
	viewCmd.BoolVar(&showTime, "t", false, "Show timestamps (created_at, updated_at)")
	viewCmd.BoolVar(&showTime, "time", false, "Show timestamps (created_at, updated_at)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	if err := viewCmd.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 1 {
		return fmt.Errorf("project code or ID is required.\nUsage: task-cli project view <code|id> [-t]")
	}

	target := strings.TrimSpace(posArgs[0])
	project, err := ctx.Store.GetProjectByCode(target)
	if err != nil && errors.Is(err, db.ErrProjectNotFound) {
		if id, parseErr := strconv.ParseInt(target, 10, 64); parseErr == nil {
			project, err = ctx.Store.GetProjectByID(id)
		}
	}

	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return fmt.Errorf("project '%s' not found", target)
		}
		return fmt.Errorf("database error: %w", err)
	}

	fmt.Printf("Project: %s (ID: %d)\n", project.Code, project.ID)
	fmt.Printf("Name: %s\n", project.Name)
	if showTime {
		fmt.Printf("Created: %s\n", project.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", project.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if project.Description != "" {
		fmt.Printf("Description:\n%s\n", project.Description)
	}
	return nil
}

