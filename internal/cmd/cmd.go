package cmd

import (
	"task-cli/internal/db"
)

// Context armazena dependências compartilhadas entre os comandos.
type Context struct {
	Store  *db.Store
	DBPath string
	Format string
}

// CommandFunc defines the function signature for subcommands.
type CommandFunc func(ctx *Context, args []string) error

// Registry maps command names and aliases to their corresponding handlers.
var Registry = map[string]CommandFunc{
	"doc-add":    DocAdd,
	"attach":     DocAdd,
	"doc-list":   DocList,
	"docs":       DocList,
	"doc-del":    DocDelete,
	"doc-rm":     DocDelete,
	"doc-remove": DocDelete,
	"export":     Export,
	"exp":        Export,
	"import":     Import,
	"imp":        Import,
}

// Execute routes a command to its registered handler.
func Execute(ctx *Context, command string, args []string) (bool, error) {
	if Root != nil {
		target := Root.FindSubcommand(command)
		if target != nil {
			return true, Root.Execute(ctx, append([]string{command}, args...))
		}
	}

	handler, ok := Registry[command]
	if !ok {
		return false, nil
	}
	return true, handler(ctx, args)
}
