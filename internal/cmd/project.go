package cmd

import (
	"errors"
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
func ProjectList(ctx *Context, _ []string) error {
	projects, err := ctx.Store.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tCODE\tNAME\tDESCRIPTION\tCREATED AT")
	fmt.Fprintln(w, "--\t----\t----\t-----------\t----------")
	for _, p := range projects {
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			p.ID,
			p.Code,
			p.Name,
			desc,
			p.CreatedAt.Local().Format("2006-01-02 15:04"),
		)
	}
	return w.Flush()
}

