package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"task-cli/internal/vault"
)

// setupTestContext returns a Context backed by an empty vault in a temp dir.
func setupTestContext(t *testing.T) *Context {
	t.Helper()
	root := t.TempDir()
	store, err := vault.Open(root)
	if err != nil {
		t.Fatalf("failed to open test vault: %v", err)
	}
	return &Context{Store: store, VaultPath: root}
}

func captureStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	cmdErr := f()
	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	return out, cmdErr
}
