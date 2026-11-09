package cmd

import (
	"io"
	"os"

	"task-cli/internal/config"
	"task-cli/internal/db"
)

// Context armazena dependências compartilhadas entre os comandos.
type Context struct {
	Store  *db.Store
	DBPath string
	Format string
	Out    io.Writer
	Stderr io.Writer
	Config *config.Config
}

// DefaultProject returns the project code configured for the workspace, if any.
func (c *Context) DefaultProject() string {
	if c == nil || c.Config == nil {
		return ""
	}
	return c.Config.Project
}

// OutWriter returns the configured output writer for the context,
// or defaults to os.Stdout if unset.
func (c *Context) OutWriter() io.Writer {
	if c != nil && c.Out != nil {
		return c.Out
	}
	return os.Stdout
}

// ErrWriter returns the configured error writer for the context,
// or defaults to os.Stderr if unset.
func (c *Context) ErrWriter() io.Writer {
	if c != nil && c.Stderr != nil {
		return c.Stderr
	}
	return os.Stderr
}

// CommandFunc defines the function signature for subcommands.
type CommandFunc func(ctx *Context, args []string) error

// Execute routes a command to its registered handler via flat alias or hierarchical Root router.
func Execute(ctx *Context, command string, args []string) (bool, error) {
	if Root != nil {
		if targetPath, ok := ResolveFlatAlias(command); ok {
			return true, Root.Execute(ctx, append(targetPath, args...))
		}

		target := Root.FindSubcommand(command)
		if target != nil {
			return true, Root.Execute(ctx, append([]string{command}, args...))
		}
	}

	return false, nil
}
