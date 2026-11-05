package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"task-cli/internal/db"
)

// ProjectAdd executa a criação de um novo projeto.
func ProjectAdd(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("'code' and 'name' are required.\nUsage: task-cli add -p <code> <name> [description]")
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

// ProjectList lista todos os projetos cadastrados.
func ProjectList(ctx *Context, args []string) error {
	pCmd := flag.NewFlagSet("project-list", flag.ContinueOnError)
	var showTime bool
	var showDesc bool
	pCmd.BoolVar(&showTime, "t", false, "Show creation dates")
	pCmd.BoolVar(&showTime, "time", false, "Show creation dates")
	pCmd.BoolVar(&showDesc, "d", false, "Show project description")
	pCmd.BoolVar(&showDesc, "desc", false, "Show project description")
	pCmd.BoolVar(&showDesc, "description", false, "Show project description")
	pCmd.Bool("p", false, "List projects (shorthand)")
	pCmd.Bool("projects", false, "List projects")

	flagArgs, _ := splitFlagsAndPosArgs(args, nil)
	if err := pCmd.Parse(flagArgs); err != nil {
		return err
	}

	projects, err := ctx.Store.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	headers := []string{"ID", "CODE", "NAME"}
	dividers := []string{"----", "----", "----"}
	if showDesc {
		headers = append(headers, "DESCRIPTION")
		dividers = append(dividers, "-----------")
	}
	if showTime {
		headers = append(headers, "CREATED AT")
		dividers = append(dividers, "----------")
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(dividers, "\t"))

	for _, p := range projects {
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		row := []string{
			fmt.Sprintf("%04d", p.ID),
			p.Code,
			p.Name,
		}
		if showDesc {
			row = append(row, desc)
		}
		if showTime {
			row = append(row, p.CreatedAt.Local().Format("2006-01-02 15:04"))
		}
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

