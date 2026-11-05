package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"task-cli/internal/db"
)

// Export serializes projects, tasks, and documents to JSON.
func Export(ctx *Context, args []string) error {
	exportCmd := flag.NewFlagSet("export", flag.ContinueOnError)
	var outFlag string
	var prettyFlag bool
	var projectFlag string
	var statusFlag string
	var includeDeletedFlag bool
	var dryRunFlag bool
	var quietFlag bool

	exportCmd.StringVar(&outFlag, "out", "", "Output file path (default: stdout)")
	exportCmd.StringVar(&outFlag, "o", "", "Output file path (shorthand)")
	exportCmd.BoolVar(&prettyFlag, "pretty", false, "Pretty print JSON output")
	exportCmd.StringVar(&projectFlag, "f", "", "Filter tasks by project code (shorthand)")
	exportCmd.StringVar(&projectFlag, "filter", "", "Filter tasks by project code")
	exportCmd.StringVar(&projectFlag, "proj", "", "Filter tasks by project code")
	exportCmd.StringVar(&projectFlag, "project", "", "Filter tasks by project code")
	exportCmd.StringVar(&statusFlag, "status", "", "Filter tasks by status")
	exportCmd.StringVar(&statusFlag, "s", "", "Filter tasks by status (shorthand)")
	exportCmd.BoolVar(&includeDeletedFlag, "include-deleted", false, "Include soft-deleted tasks in export")
	exportCmd.BoolVar(&dryRunFlag, "dry-run", false, "Simulate export without writing output")
	exportCmd.BoolVar(&quietFlag, "q", false, "Suppress non-essential messages (shorthand)")
	exportCmd.BoolVar(&quietFlag, "quiet", false, "Suppress non-essential messages")

	flagArgs, _ := splitFlagsAndPosArgs(args, map[string]bool{
		"-o":        true,
		"--out":     true,
		"-f":        true,
		"--filter":  true,
		"-proj":     true,
		"--project": true,
		"-s":        true,
		"--status":  true,
	})

	if err := exportCmd.Parse(flagArgs); err != nil {
		return err
	}

	data, err := ctx.Store.ExportData(db.ExportOptions{
		ProjectCode:    projectFlag,
		Status:         statusFlag,
		IncludeDeleted: includeDeletedFlag,
	})
	if err != nil {
		return err
	}

	numProjects := len(data.Projects)
	numTasks := len(data.StandaloneTasks)
	numDocs := 0
	for _, t := range data.StandaloneTasks {
		numDocs += len(t.Documents)
	}
	for _, p := range data.Projects {
		numTasks += len(p.Tasks)
		for _, t := range p.Tasks {
			numDocs += len(t.Documents)
		}
	}

	if dryRunFlag {
		fmt.Printf("[dry-run] Export: %d project(s), %d task(s), %d document(s)\n", numProjects, numTasks, numDocs)
		return nil
	}

	var raw []byte
	if prettyFlag {
		raw, err = json.MarshalIndent(data, "", "  ")
	} else {
		raw, err = json.Marshal(data)
	}
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	if outFlag != "" {
		if err := os.WriteFile(outFlag, append(raw, '\n'), 0644); err != nil {
			return fmt.Errorf("failed to write export file: %w", err)
		}
		if !quietFlag {
			fmt.Printf("Data exported successfully to %s (%d projects, %d tasks, %d documents)\n", outFlag, numProjects, numTasks, numDocs)
		}
	} else {
		fmt.Println(string(raw))
	}

	return nil
}

// Import reads JSON data and merges projects, tasks, and documents into the database.
func Import(ctx *Context, args []string) error {
	importCmd := flag.NewFlagSet("import", flag.ContinueOnError)
	var dryRunFlag bool
	var overwriteFlag bool
	var strictFlag bool
	var forceDocsFlag bool
	var quietFlag bool

	importCmd.BoolVar(&dryRunFlag, "dry-run", false, "Simulate import without committing changes")
	importCmd.BoolVar(&overwriteFlag, "overwrite", false, "Overwrite existing project metadata")
	importCmd.BoolVar(&strictFlag, "strict", false, "Abort on first task validation error")
	importCmd.BoolVar(&forceDocsFlag, "force-docs", false, "Allow duplicate document attachments")
	importCmd.BoolVar(&quietFlag, "q", false, "Suppress non-essential messages (shorthand)")
	importCmd.BoolVar(&quietFlag, "quiet", false, "Suppress non-essential messages")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, nil)
	if err := importCmd.Parse(flagArgs); err != nil {
		return err
	}

	var raw []byte
	if len(posArgs) > 0 && posArgs[0] != "-" {
		var err error
		raw, err = os.ReadFile(posArgs[0])
		if err != nil {
			return fmt.Errorf("failed to read import file: %w", err)
		}
	} else {
		stat, err := os.Stdin.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) != 0 && len(posArgs) == 0 {
			return fmt.Errorf("input file is required or pipe JSON through stdin.\nUsage: task-cli import <file.json> [flags]")
		}
		raw, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return errors.New("empty import data")
	}

	var data db.TransferData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	res, err := ctx.Store.ImportData(&data, db.ImportOptions{
		Overwrite: overwriteFlag,
		Strict:    strictFlag,
		ForceDocs: forceDocsFlag,
		DryRun:    dryRunFlag,
	})
	if err != nil {
		return err
	}

	if !quietFlag {
		if len(res.Warnings) > 0 {
			for _, w := range res.Warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
			}
		}
		totalImportedProjs := res.ProjectsImported + res.ProjectsUpdated
		totalIgnored := res.ProjectsSkipped + res.TasksSkipped + res.DocumentsSkipped

		prefix := ""
		if dryRunFlag {
			prefix = "[dry-run] "
		}

		if res.ProjectsUpdated > 0 {
			fmt.Printf("%sImportados: %d projetos (%d novos, %d atualizados), %d tarefas, %d documentos. Ignorados: %d.\n",
				prefix, totalImportedProjs, res.ProjectsImported, res.ProjectsUpdated, res.TasksImported, res.DocumentsImported, totalIgnored)
		} else {
			fmt.Printf("%sImportados: %d projetos, %d tarefas, %d documentos. Ignorados: %d.\n",
				prefix, totalImportedProjs, res.TasksImported, res.DocumentsImported, totalIgnored)
		}
	}

	return nil
}
