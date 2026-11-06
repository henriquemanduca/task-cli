package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"task-cli/internal/db"
)

// DocAddArgs represents the argument schema for linking a document to a task.
type DocAddArgs struct {
	TaskID   int64
	FilePath string
	Name     string
	DocType  string
}

// DocAddResult holds the created document link and task ID.
type DocAddResult struct {
	Document *db.Document
	TaskID   int64
}

// DocAddAction encapsulates schema parsing, execution, and output formatting for document attachment.
type DocAddAction struct{}

// Parse parses flags and arguments for linking a document.
func (a *DocAddAction) Parse(ctx *Context, args []string) (DocAddArgs, error) {
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
		return DocAddArgs{}, err
	}

	if len(posArgs) < 2 {
		return DocAddArgs{}, fmt.Errorf("task ID and file path are required.\nUsage: task-cli task doc add <task_id> <file_path> [name] [--type <type>]")
	}

	taskID, err := strconv.ParseInt(posArgs[0], 10, 64)
	if err != nil {
		return DocAddArgs{}, fmt.Errorf("invalid task ID: %s", posArgs[0])
	}

	rawPath := posArgs[1]
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return DocAddArgs{}, fmt.Errorf("resolving absolute file path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return DocAddArgs{}, fmt.Errorf("file does not exist: %s", absPath)
		}
		return DocAddArgs{}, fmt.Errorf("cannot access file: %w", err)
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

	return DocAddArgs{
		TaskID:   taskID,
		FilePath: absPath,
		Name:     name,
		DocType:  docType,
	}, nil
}

// Execute attaches the document to the task in the database.
func (a *DocAddAction) Execute(ctx *Context, args DocAddArgs) (*DocAddResult, error) {
	doc, err := ctx.Store.AddDocument(args.TaskID, args.Name, args.DocType, args.FilePath)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.TaskID)
		}
		return nil, fmt.Errorf("failed to link document: %w", err)
	}

	return &DocAddResult{
		Document: doc,
		TaskID:   args.TaskID,
	}, nil
}

// Format renders the document attachment confirmation.
func (a *DocAddAction) Format(ctx *Context, w io.Writer, res *DocAddResult) error {
	_, err := fmt.Fprintf(w, "Document linked successfully (ID: %d) to task #%04d\n", res.Document.ID, res.TaskID)
	return err
}

// DocAddHandler is the standardized handler instance for DocAddAction.
var DocAddHandler = NewActionHandler(&DocAddAction{})

// DocAdd links a document to a task.
func DocAdd(ctx *Context, args []string) error {
	return DocAddHandler.Run(ctx, args)
}

// DocListArgs represents the argument schema for listing linked documents.
type DocListArgs struct {
	TaskID int64
}

// DocListResult holds the task ID and retrieved documents.
type DocListResult struct {
	TaskID    int64
	Documents []db.Document
}

// DocListAction encapsulates schema parsing, execution, and output formatting for document listing.
type DocListAction struct{}

// Parse validates the task ID positional argument.
func (a *DocListAction) Parse(ctx *Context, args []string) (DocListArgs, error) {
	if len(args) < 1 {
		return DocListArgs{}, fmt.Errorf("task ID is required.\nUsage: task-cli task doc list <task_id>")
	}

	taskID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return DocListArgs{}, fmt.Errorf("invalid task ID: %s", args[0])
	}

	return DocListArgs{TaskID: taskID}, nil
}

// Execute retrieves documents linked to the task.
func (a *DocListAction) Execute(ctx *Context, args DocListArgs) (*DocListResult, error) {
	if _, err := ctx.Store.GetTaskByID(args.TaskID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.TaskID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	docs, err := ctx.Store.ListDocuments(args.TaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	return &DocListResult{
		TaskID:    args.TaskID,
		Documents: docs,
	}, nil
}

// Format formats and outputs the document list as a table or empty notice.
func (a *DocListAction) Format(ctx *Context, w io.Writer, res *DocListResult) error {
	if len(res.Documents) == 0 {
		fmt.Fprintf(w, "No documents linked to task #%04d\n", res.TaskID)
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tTYPE\tPATH\tCREATED AT")

	for _, d := range res.Documents {
		created := d.CreatedAt.Local().Format("2006-01-02 15:04:05")
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", d.ID, d.Name, d.Type, d.FilePath, created)
	}

	return tw.Flush()
}

// DocListHandler is the standardized handler instance for DocListAction.
var DocListHandler = NewActionHandler(&DocListAction{})

// DocList lists all documents linked to a task.
func DocList(ctx *Context, args []string) error {
	return DocListHandler.Run(ctx, args)
}

// DocDeleteArgs represents the argument schema for deleting a linked document.
type DocDeleteArgs struct {
	DocID int64
}

// DocDeleteResult holds the deleted document ID.
type DocDeleteResult struct {
	DocID int64
}

// DocDeleteAction encapsulates schema parsing, execution, and output formatting for document deletion.
type DocDeleteAction struct{}

// Parse validates the document ID argument.
func (a *DocDeleteAction) Parse(ctx *Context, args []string) (DocDeleteArgs, error) {
	if len(args) < 1 {
		return DocDeleteArgs{}, fmt.Errorf("document ID is required.\nUsage: task-cli task doc remove <id>")
	}

	docID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return DocDeleteArgs{}, fmt.Errorf("invalid document ID: %s", args[0])
	}

	return DocDeleteArgs{DocID: docID}, nil
}

// Execute removes the document from the database.
func (a *DocDeleteAction) Execute(ctx *Context, args DocDeleteArgs) (*DocDeleteResult, error) {
	if err := ctx.Store.DeleteDocument(args.DocID); err != nil {
		if errors.Is(err, db.ErrDocumentNotFound) {
			return nil, fmt.Errorf("document with ID %d not found", args.DocID)
		}
		return nil, fmt.Errorf("failed to delete document: %w", err)
	}

	return &DocDeleteResult{DocID: args.DocID}, nil
}

// Format renders the deletion confirmation message.
func (a *DocDeleteAction) Format(ctx *Context, w io.Writer, res *DocDeleteResult) error {
	_, err := fmt.Fprintf(w, "Document #%d removed successfully\n", res.DocID)
	return err
}

// DocDeleteHandler is the standardized handler instance for DocDeleteAction.
var DocDeleteHandler = NewActionHandler(&DocDeleteAction{})

// DocDelete removes a document link from a task.
func DocDelete(ctx *Context, args []string) error {
	return DocDeleteHandler.Run(ctx, args)
}
