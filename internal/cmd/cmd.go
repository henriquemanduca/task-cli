package cmd

import (
	"io"
	"os"

	"task-cli/internal/vault"
)

// Context armazena dependências compartilhadas entre os comandos.
type Context struct {
	Store     *vault.Store
	VaultPath string
	Format    string
	Out       io.Writer
	Stderr    io.Writer
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

// Execute routes a top-level command to its registered handler.
func Execute(root *Command, ctx *Context, command string, args []string) (bool, error) {
	if root == nil || root.FindSubcommand(command) == nil {
		return false, nil
	}
	return true, root.Execute(ctx, append([]string{command}, args...))
}
