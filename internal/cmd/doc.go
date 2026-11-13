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
	"task-cli/internal/filestore"
)

// DocAddArgs represents the argument schema for linking a document to a task.
type DocAddArgs struct {
	TaskID    int64
	FilePath  string
	Name      string
	DocType   string
	ForceCopy bool
	ForceLink bool
}

// DocAddResult holds the created document link, task ID, copy status, and warnings.
type DocAddResult struct {
	Document *db.Document
	TaskID   int64
	Copied   bool
	Warning  string
}

// DocAddAction encapsulates schema parsing, execution, and output formatting for document attachment.
type DocAddAction struct{}

// Parse parses flags and arguments for linking a document.
func (a *DocAddAction) Parse(ctx *Context, args []string) (DocAddArgs, error) {
	docCmd := flag.NewFlagSet("doc-add", flag.ContinueOnError)
	var nameFlag string
	var typeFlag string
	var copyFlag bool
	var linkFlag bool

	docCmd.StringVar(&nameFlag, "name", "", "Document name (optional, defaults to filename)")
	docCmd.StringVar(&nameFlag, "n", "", "Document name (shorthand)")
	docCmd.StringVar(&typeFlag, "type", "", "Document type/extension (optional, e.g. pdf, txt)")
	docCmd.StringVar(&typeFlag, "t", "", "Document type (shorthand)")
	docCmd.BoolVar(&copyFlag, "copy", false, "Force duplication of attachment file into central storage")
	docCmd.BoolVar(&linkFlag, "link", false, "Link attachment by reference without copying")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"-n":     true,
		"--name": true,
		"-t":     true,
		"--type": true,
	})

	if err := docCmd.Parse(flagArgs); err != nil {
		return DocAddArgs{}, err
	}

	if copyFlag && linkFlag {
		return DocAddArgs{}, fmt.Errorf("cannot specify both --copy and --link")
	}

	if len(posArgs) < 2 {
		return DocAddArgs{}, fmt.Errorf("task ID and file path are required.\nUsage: task-cli task doc add <task_id> <file_path> [name] [--type <type>] [--copy] [--link]")
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
		TaskID:    taskID,
		FilePath:  absPath,
		Name:      name,
		DocType:   docType,
		ForceCopy: copyFlag,
		ForceLink: linkFlag,
	}, nil
}

// Execute attaches the document to the task in the database.
func (a *DocAddAction) Execute(ctx *Context, args DocAddArgs) (*DocAddResult, error) {
	if ctx == nil || ctx.Store == nil {
		return nil, errors.New("database store is not initialized")
	}

	// 1. Validate if task exists and is not deleted
	if _, err := ctx.Store.GetTaskByID(ctx.Context(), args.TaskID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.TaskID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	// 2. Verify source file exists and can be accessed
	st, err := os.Stat(args.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", args.FilePath)
		}
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("source path %q is not a regular file", args.FilePath)
	}

	// 3. Save attachment via filestore
	savedPath, copied, err := filestore.SaveAttachment(args.FilePath, args.TaskID, args.ForceCopy, args.ForceLink)
	if err != nil {
		return nil, fmt.Errorf("failed to save attachment: %w", err)
	}

	// 4. Insert document record into SQLite
	doc, err := ctx.Store.AddDocument(ctx.Context(), args.TaskID, args.Name, args.DocType, savedPath)
	if err != nil {
		// 5. Protected rollback: remove newly created copy if DB insertion failed
		if copied {
			_ = filestore.RemoveFile(savedPath)
		}
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.TaskID)
		}
		return nil, fmt.Errorf("failed to link document: %w", err)
	}

	result := &DocAddResult{
		Document: doc,
		TaskID:   args.TaskID,
		Copied:   copied,
	}
	if !copied && !args.ForceLink {
		result.Warning = "file exceeds 100MB; linking by reference without copying. Use --copy to force duplication."
	}
	return result, nil
}

// Format renders the document attachment confirmation.
func (a *DocAddAction) Format(ctx *Context, w io.Writer, res *DocAddResult) error {
	if res.Warning != "" {
		fmt.Fprintf(ctx.ErrWriter(), "Warning: %s\n", res.Warning)
	}
	_, err := fmt.Fprintf(w, "Document linked successfully (ID: %d) to task #%04d\n", res.Document.ID, res.TaskID)
	return err
}

// DocAddHandler is the standardized handler instance for DocAddAction.
var DocAddHandler = NewActionHandler(&DocAddAction{})

// DocAdd links a document to a task.
func DocAdd(ctx *Context, args []string) error {
	return DocAddHandler.Run(ctx, args)
}

// handleDocAdd is an alias for DocAdd for backward compatibility with design documentation.
var handleDocAdd = DocAdd

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
	if _, err := ctx.Store.GetTaskByID(ctx.Context(), args.TaskID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.TaskID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	docs, err := ctx.Store.ListDocuments(ctx.Context(), args.TaskID)
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
	if err := ctx.Store.DeleteDocument(ctx.Context(), args.DocID); err != nil {
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

// DocOpenArgs represents argument schema for opening a task's attachment directory or revealing a document.
type DocOpenArgs struct {
	TaskID int64
	DocID  int64
}

// DocOpenResult holds output message from opening the file or folder.
type DocOpenResult struct {
	Output string
}

// DocOpenAction encapsulates schema parsing, execution, and output formatting for opening attachments.
type DocOpenAction struct{}

// Parse parses flags and arguments for opening task attachments or revealing documents.
func (a *DocOpenAction) Parse(ctx *Context, args []string) (DocOpenArgs, error) {
	openCmd := flag.NewFlagSet("doc-open", flag.ContinueOnError)
	var docID int64
	openCmd.Int64Var(&docID, "doc", 0, "Document ID to reveal")

	flagArgs, posArgs := splitFlagsAndPosArgs(args, map[string]bool{
		"doc":   true,
		"-doc":  true,
		"--doc": true,
	})

	if err := openCmd.Parse(flagArgs); err != nil {
		return DocOpenArgs{}, err
	}

	if docID > 0 {
		if len(posArgs) > 0 {
			return DocOpenArgs{}, fmt.Errorf("cannot specify both task ID and --doc flag")
		}
		return DocOpenArgs{DocID: docID}, nil
	}

	if len(posArgs) < 1 {
		return DocOpenArgs{}, fmt.Errorf("task ID or --doc <id> is required.\nUsage: task-cli task doc open <task_id> | task-cli task doc open --doc <doc_id>")
	}

	taskID, err := strconv.ParseInt(posArgs[0], 10, 64)
	if err != nil {
		return DocOpenArgs{}, fmt.Errorf("invalid task ID: %s", posArgs[0])
	}

	return DocOpenArgs{TaskID: taskID}, nil
}

// Execute opens the task attachment folder or reveals the document.
func (a *DocOpenAction) Execute(ctx *Context, args DocOpenArgs) (*DocOpenResult, error) {
	if ctx == nil || ctx.Store == nil {
		return nil, errors.New("database store is not initialized")
	}

	if args.DocID > 0 {
		doc, err := ctx.Store.GetDocumentByID(ctx.Context(), args.DocID)
		if err != nil {
			if errors.Is(err, db.ErrDocumentNotFound) {
				return nil, fmt.Errorf("document with ID %d not found", args.DocID)
			}
			return nil, fmt.Errorf("database error: %w", err)
		}

		task, err := ctx.Store.GetTaskByID(ctx.Context(), doc.TaskID)
		if err != nil {
			if errors.Is(err, db.ErrTaskNotFound) {
				return nil, fmt.Errorf("task with ID %d not found or deleted", doc.TaskID)
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
		if task.Status == db.StatusDeleted {
			return nil, fmt.Errorf("task with ID %d is deleted", doc.TaskID)
		}

		out, err := filestore.OpenFile(doc.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open document: %w", err)
		}
		return &DocOpenResult{Output: out}, nil
	}

	task, err := ctx.Store.GetTaskByID(ctx.Context(), args.TaskID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			return nil, fmt.Errorf("task with ID %d not found", args.TaskID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	if task.Status == db.StatusDeleted {
		return nil, fmt.Errorf("task with ID %d is deleted", args.TaskID)
	}

	taskDir, err := filestore.TaskAttachmentDir(args.TaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task attachment directory: %w", err)
	}

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create task attachment directory: %w", err)
	}

	out, err := filestore.OpenFolder(taskDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open task attachment directory: %w", err)
	}

	return &DocOpenResult{Output: out}, nil
}

// Format writes the output message (if any) to the output writer.
func (a *DocOpenAction) Format(ctx *Context, w io.Writer, res *DocOpenResult) error {
	if res != nil && res.Output != "" {
		_, err := fmt.Fprint(w, res.Output)
		return err
	}
	return nil
}

// DocOpenHandler is the standardized handler instance for DocOpenAction.
var DocOpenHandler = NewActionHandler(&DocOpenAction{})

// DocOpen opens a task attachment directory or reveals a document.
func DocOpen(ctx *Context, args []string) error {
	return DocOpenHandler.Run(ctx, args)
}

// handleDocOpen is an alias for DocOpen for backward compatibility with design documentation.
var handleDocOpen = DocOpen

// DocPruneArgs represents argument schema for auditing and cleaning orphaned attachments.
type DocPruneArgs struct {
	DryRun bool
}

// DocPruneResult holds the pruned file paths and dry run flag.
type DocPruneResult struct {
	Pruned []string
	DryRun bool
}

// DocPruneAction encapsulates schema parsing, execution, and output formatting for pruning orphan attachments.
type DocPruneAction struct{}

// Parse parses flags for pruning orphaned attachments.
func (a *DocPruneAction) Parse(ctx *Context, args []string) (DocPruneArgs, error) {
	pruneCmd := flag.NewFlagSet("doc-prune", flag.ContinueOnError)
	var dryRun bool
	pruneCmd.BoolVar(&dryRun, "dry-run", false, "Report orphaned files without deleting them")
	pruneCmd.BoolVar(&dryRun, "n", false, "Dry run (shorthand)")

	flagArgs, _ := splitFlagsAndPosArgs(args, nil)
	if err := pruneCmd.Parse(flagArgs); err != nil {
		return DocPruneArgs{}, err
	}

	return DocPruneArgs{DryRun: dryRun}, nil
}

// Execute queries active document paths and removes orphaned files.
func (a *DocPruneAction) Execute(ctx *Context, args DocPruneArgs) (*DocPruneResult, error) {
	if ctx == nil || ctx.Store == nil {
		return nil, errors.New("database store is not initialized")
	}

	activePaths, err := ctx.Store.ListAllDocumentPaths(ctx.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to query active document paths: %w", err)
	}

	pruned, err := filestore.PruneAttachments(activePaths, args.DryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to prune attachments: %w", err)
	}

	return &DocPruneResult{
		Pruned: pruned,
		DryRun: args.DryRun,
	}, nil
}

// Format renders the prune operation results.
func (a *DocPruneAction) Format(ctx *Context, w io.Writer, res *DocPruneResult) error {
	if len(res.Pruned) == 0 {
		_, err := fmt.Fprintln(w, "No orphaned attachments found.")
		return err
	}

	for _, path := range res.Pruned {
		if res.DryRun {
			fmt.Fprintf(w, "Would prune: %s\n", path)
		} else {
			fmt.Fprintf(w, "Pruned: %s\n", path)
		}
	}

	noun := "attachment file"
	if len(res.Pruned) != 1 {
		noun = "attachment files"
	}

	if res.DryRun {
		_, err := fmt.Fprintf(w, "\nDry run complete: %d orphaned %s would be pruned.\n", len(res.Pruned), noun)
		return err
	}

	_, err := fmt.Fprintf(w, "\nPrune complete: %d orphaned %s removed.\n", len(res.Pruned), noun)
	return err
}

// DocPruneHandler is the standardized handler instance for DocPruneAction.
var DocPruneHandler = NewActionHandler(&DocPruneAction{})

// DocPrune audits and cleans orphaned attachment files.
func DocPrune(ctx *Context, args []string) error {
	return DocPruneHandler.Run(ctx, args)
}

// handleDocPrune is an alias for DocPrune for backward compatibility with design documentation.
var handleDocPrune = DocPrune
