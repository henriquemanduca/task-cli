package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"task-cli/internal/db"
)

// DocAdd links a document to a task.
func DocAdd(ctx *Context, args []string) error {
	docCmd := flag.NewFlagSet("doc-add", flag.ContinueOnError)
	var nameFlag string
	var typeFlag string

	docCmd.StringVar(&nameFlag, "name", "", "Document name (optional, defaults to filename)")
	docCmd.StringVar(&nameFlag, "n", "", "Document name (shorthand)")
	docCmd.StringVar(&typeFlag, "type", "", "Document type/extension (optional, e.g. pdf, txt)")
	docCmd.StringVar(&typeFlag, "t", "", "Document type (shorthand)")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-n":     true,
		"--name": true,
		"-t":     true,
		"--type": true,
	})

	if err := docCmd.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 2 {
		return fmt.Errorf("task ID and file path are required.\nUsage: task-cli task doc add <task_id> <file_path> [name] [--type <type>]")
	}

	taskID, err := strconv.ParseInt(posArgs[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", posArgs[0])
	}

	rawPath := posArgs[1]
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return fmt.Errorf("resolving absolute file path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", absPath)
		}
		return fmt.Errorf("cannot access file: %w", err)
	}

	name := nameFlag
	if name == "" && len(posArgs) > 2 {
		name = strings.TrimSpace(strings.Join(posArgs[2:], " "))
	}
	if name == "" {
		name = filepath.Base(absPath)
	}

	docType := typeFlag
	if docType == "" {
		ext := strings.TrimPrefix(filepath.Ext(absPath), ".")
		if ext == "" {
			docType = "unknown"
		} else {
			docType = strings.ToLower(ext)
		}
	}

	doc, err := ctx.Store.AddDocument(taskID, name, docType, absPath)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", taskID)
		}
		return fmt.Errorf("failed to link document: %w", err)
	}

	fmt.Printf("Document linked successfully (ID: %d) to task #%04d\n", doc.ID, taskID)
	return nil
}

// DocList lists all documents linked to a task.
func DocList(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("task ID is required.\nUsage: task-cli task doc list <task_id>")
	}

	taskID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	if _, err := ctx.Store.GetTaskByID(taskID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return fmt.Errorf("task with ID %d not found", taskID)
		}
		return fmt.Errorf("database error: %w", err)
	}

	docs, err := ctx.Store.ListDocuments(taskID)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	if len(docs) == 0 {
		fmt.Printf("No documents linked to task #%04d\n", taskID)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tPATH\tCREATED AT")

	for _, d := range docs {
		created := d.CreatedAt.Local().Format("2006-01-02 15:04:05")
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", d.ID, d.Name, d.Type, d.FilePath, created)
	}

	return w.Flush()
}

// DocDelete removes a document link from a task.
func DocDelete(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("document ID is required.\nUsage: task-cli task doc remove <id>")
	}

	docID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid document ID: %s", args[0])
	}

	if err := ctx.Store.DeleteDocument(docID); err != nil {
		if errors.Is(err, db.ErrDocumentNotFound) {
			return fmt.Errorf("document with ID %d not found", docID)
		}
		return fmt.Errorf("failed to delete document: %w", err)
	}

	fmt.Printf("Document #%d removed successfully\n", docID)
	return nil
}

