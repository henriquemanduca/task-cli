package cmd

import (
	"io"
)

// Handler represents the standard interface for command execution.
// Individual command handlers implement this interface to run their action
// with an injected execution Context and raw command-line arguments.
type Handler interface {
	Run(ctx *Context, args []string) error
}

// HandlerFunc is an adapter to allow the use of ordinary functions as command Handlers.
type HandlerFunc func(ctx *Context, args []string) error

// Run calls f(ctx, args).
func (f HandlerFunc) Run(ctx *Context, args []string) error {
	return f(ctx, args)
}

// Action defines the decoupled contract for an individual command action adhering to
// the Single Responsibility Principle:
//  1. Parse parses and validates command line arguments into a typed argument schema (TArgs).
//  2. Execute runs domain logic using the argument schema and returns a typed result (TResult).
//  3. Format presents and writes the execution result to the provided io.Writer.
type Action[TArgs any, TResult any] interface {
	Parse(ctx *Context, args []string) (TArgs, error)
	Execute(ctx *Context, args TArgs) (TResult, error)
	Format(ctx *Context, w io.Writer, result TResult) error
}

// ActionHandler coordinates the execution of an Action by running its pipeline:
// Parse -> Execute -> Format. It implements the Handler interface.
type ActionHandler[TArgs any, TResult any] struct {
	Action Action[TArgs, TResult]
}

// NewActionHandler wraps an Action into an ActionHandler implementing Handler.
func NewActionHandler[TArgs any, TResult any](action Action[TArgs, TResult]) *ActionHandler[TArgs, TResult] {
	return &ActionHandler[TArgs, TResult]{Action: action}
}

// Run executes the full action pipeline (Parse, Execute, Format).
func (h *ActionHandler[TArgs, TResult]) Run(ctx *Context, args []string) error {
	parsedArgs, err := h.Action.Parse(ctx, args)
	if err != nil {
		return err
	}
	result, err := h.Action.Execute(ctx, parsedArgs)
	if err != nil {
		return err
	}
	var w io.Writer
	if ctx != nil {
		w = ctx.OutWriter()
	} else {
		w = (&Context{}).OutWriter()
	}
	return h.Action.Format(ctx, w, result)
}

// AsCommandFunc adapts the ActionHandler into a CommandFunc.
func (h *ActionHandler[TArgs, TResult]) AsCommandFunc() CommandFunc {
	return h.Run
}
