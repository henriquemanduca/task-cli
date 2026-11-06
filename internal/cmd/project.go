package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"task-cli/internal/db"
)

// ProjectAdd executa a criação de um novo projeto.
func ProjectAdd(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("'code' and 'name' are required.\nUsage: task-cli project-add <code> <name> [description]")
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
	pCmd.BoolVar(&showTime, "t", false, "Include creation date column")
	pCmd.BoolVar(&showTime, "time", false, "Include creation date column")
	pCmd.BoolVar(&showDesc, "d", false, "Include description column")
	pCmd.BoolVar(&showDesc, "desc", false, "Include description column")
	pCmd.BoolVar(&showDesc, "description", false, "Include description column")
	pCmd.BoolVar(&noHeader, "n", false, "Suppress header row")
	pCmd.BoolVar(&noHeader, "no-header", false, "Suppress header row")
	pCmd.StringVar(&format, "format", "tsv", "Output format (tsv, table)")
	pCmd.StringVar(&format, "F", "tsv", "Output format (shorthand)")
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

