package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type mockArgs struct {
	Value string
}

type mockResult struct {
	Output string
}

type mockAction struct {
	parseErr   error
	executeErr error
	formatErr  error
	parsedArgs mockArgs
}

func (m *mockAction) Parse(ctx *Context, args []string) (mockArgs, error) {
	if m.parseErr != nil {
		return mockArgs{}, m.parseErr
	}
	if len(args) == 0 {
		return mockArgs{}, errors.New("no args provided")
	}
	m.parsedArgs = mockArgs{Value: args[0]}
	return m.parsedArgs, nil
}

func (m *mockAction) Execute(ctx *Context, args mockArgs) (mockResult, error) {
	if m.executeErr != nil {
		return mockResult{}, m.executeErr
	}
	return mockResult{Output: "Processed: " + args.Value}, nil
}

func (m *mockAction) Format(ctx *Context, w io.Writer, res mockResult) error {
	if m.formatErr != nil {
		return m.formatErr
	}
	_, err := fmt.Fprint(w, res.Output)
	return err
}

func TestHandlerFunc(t *testing.T) {
	var executed bool
	h := HandlerFunc(func(ctx *Context, args []string) error {
		executed = true
		if len(args) != 1 || args[0] != "test" {
			t.Errorf("unexpected args: %v", args)
		}
		return nil
	})

	err := h.Run(nil, []string{"test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("expected handler func to have been executed")
	}
}

func TestActionHandler_Success(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Out: &buf}

	action := &mockAction{}
	handler := NewActionHandler(action)

	err := handler.Run(ctx, []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "Processed: hello" {
		t.Errorf("expected 'Processed: hello', got %q", buf.String())
	}
}

func TestActionHandler_ParseError(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Out: &buf}

	action := &mockAction{parseErr: errors.New("invalid arguments")}
	handler := NewActionHandler(action)

	err := handler.Run(ctx, []string{"anything"})
	if err == nil || !strings.Contains(err.Error(), "invalid arguments") {
		t.Fatalf("expected parse error, got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no output on parse error, got %q", buf.String())
	}
}

func TestActionHandler_ExecuteError(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Out: &buf}

	action := &mockAction{executeErr: errors.New("execution failed")}
	handler := NewActionHandler(action)

	err := handler.Run(ctx, []string{"valid"})
	if err == nil || !strings.Contains(err.Error(), "execution failed") {
		t.Fatalf("expected execute error, got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no output on execute error, got %q", buf.String())
	}
}

func TestActionHandler_FormatError(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Out: &buf}

	action := &mockAction{formatErr: errors.New("formatting error")}
	handler := NewActionHandler(action)

	err := handler.Run(ctx, []string{"valid"})
	if err == nil || !strings.Contains(err.Error(), "formatting error") {
		t.Fatalf("expected format error, got: %v", err)
	}
}

func TestActionHandler_CommandIntegration(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Out: &buf}

	action := &mockAction{}
	handler := NewActionHandler(action)

	cmd := &Command{
		Name: "test-cmd",
	}
	cmd.SetHandler(handler)

	if cmd.Run == nil {
		t.Fatal("expected SetHandler to configure cmd.Run")
	}

	err := cmd.Run(ctx, []string{"integration"})
	if err != nil {
		t.Fatalf("unexpected error running command: %v", err)
	}

	if buf.String() != "Processed: integration" {
		t.Errorf("expected 'Processed: integration', got %q", buf.String())
	}
}

func TestDecoupledActions_IsolatedPipeline(t *testing.T) {
	ctx, cleanup := setupTestContext(t)
	defer cleanup()

	// 1. Test ProjectAddAction decoupled
	projectAction := &ProjectAddAction{}

	// Parse failure
	_, err := projectAction.Parse(ctx, []string{})
	if err == nil {
		t.Fatal("expected error parsing empty project add args")
	}

	// Parse success
	pArgs, err := projectAction.Parse(ctx, []string{"TDEC", "Decoupled Project", "Decoupled Description"})
	if err != nil {
		t.Fatalf("unexpected error parsing project add args: %v", err)
	}
	if pArgs.Code != "TDEC" || pArgs.Name != "Decoupled Project" {
		t.Errorf("unexpected parsed project args: %+v", pArgs)
	}

	// Execute
	project, err := projectAction.Execute(ctx, pArgs)
	if err != nil {
		t.Fatalf("unexpected error executing project add: %v", err)
	}
	if project.Code != "TDEC" {
		t.Errorf("expected project code 'TDEC', got %s", project.Code)
	}

	// Format
	var pBuf bytes.Buffer
	err = projectAction.Format(ctx, &pBuf, project)
	if err != nil {
		t.Fatalf("unexpected error formatting project add: %v", err)
	}
	if !strings.Contains(pBuf.String(), "Project 'TDEC' created successfully") {
		t.Errorf("unexpected formatted output: %q", pBuf.String())
	}

	// 2. Test TaskAddAction decoupled
	taskAction := &TaskAddAction{}

	// Parse failure
	_, err = taskAction.Parse(ctx, []string{"-p", "TDEC"})
	if err == nil {
		t.Fatal("expected error parsing task add without title")
	}

	// Parse success
	tArgs, err := taskAction.Parse(ctx, []string{"-p", "TDEC", "--priority", "high", "Isolated Task", "Task details"})
	if err != nil {
		t.Fatalf("unexpected error parsing task add args: %v", err)
	}
	if tArgs.Title != "Isolated Task" || tArgs.Priority != "high" || tArgs.ProjectCode != "TDEC" {
		t.Errorf("unexpected parsed task args: %+v", tArgs)
	}

	// Execute
	tResult, err := taskAction.Execute(ctx, tArgs)
	if err != nil {
		t.Fatalf("unexpected error executing task add: %v", err)
	}
	if tResult.Task.Title != "Isolated Task" || tResult.ProjectRef != "TDEC" {
		t.Errorf("unexpected task result: %+v", tResult)
	}

	// Format
	var tBuf bytes.Buffer
	err = taskAction.Format(ctx, &tBuf, tResult)
	if err != nil {
		t.Fatalf("unexpected error formatting task add: %v", err)
	}
	if !strings.Contains(tBuf.String(), "Task created successfully (ID: 1) in project 'TDEC'") {
		t.Errorf("unexpected formatted output: %q", tBuf.String())
	}

	// 3. Test TaskListAction decoupled
	listAction := &TaskListAction{}

	// Parse success
	lArgs, err := listAction.Parse(ctx, []string{"-p", "TDEC", "-a"})
	if err != nil {
		t.Fatalf("unexpected error parsing task list args: %v", err)
	}
	if lArgs.StatusFilter != "" || lArgs.ProjectCode != "TDEC" {
		t.Errorf("unexpected parsed list args: %+v", lArgs)
	}

	// Execute
	lResult, err := listAction.Execute(ctx, lArgs)
	if err != nil {
		t.Fatalf("unexpected error executing task list: %v", err)
	}
	if len(lResult.Tasks) != 1 || lResult.Tasks[0].Title != "Isolated Task" {
		t.Errorf("unexpected task list result: %+v", lResult)
	}

	// Format
	var lBuf bytes.Buffer
	err = listAction.Format(ctx, &lBuf, lResult)
	if err != nil {
		t.Fatalf("unexpected error formatting task list: %v", err)
	}
	if !strings.Contains(lBuf.String(), "Isolated Task") || !strings.Contains(lBuf.String(), "TDEC") {
		t.Errorf("unexpected formatted task list output: %q", lBuf.String())
	}
}
