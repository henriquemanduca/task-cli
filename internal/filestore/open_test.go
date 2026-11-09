package filestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_EmptyAndNonExistent(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("empty paths", func(t *testing.T) {
		out, err := OpenFolder("")
		if err == nil {
			t.Fatal("expected error for empty folder path, got nil")
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}

		out, err = OpenFile("")
		if err == nil {
			t.Fatal("expected error for empty file path, got nil")
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
	})

	t.Run("non-existent paths", func(t *testing.T) {
		nonExistentDir := filepath.Join(tempDir, "does_not_exist_dir")
		out, err := OpenFolder(nonExistentDir)
		if err == nil {
			t.Fatal("expected error for non-existent folder, got nil")
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}

		nonExistentFile := filepath.Join(tempDir, "does_not_exist_file.txt")
		out, err = OpenFile(nonExistentFile)
		if err == nil {
			t.Fatal("expected error for non-existent file, got nil")
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
	})
}

func TestOpen_Linux_Headless(t *testing.T) {
	origGOOS := currentGOOS
	origOpener := runOpener
	defer func() {
		currentGOOS = origGOOS
		runOpener = origOpener
	}()

	currentGOOS = "linux"
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	runOpener = func(name string, args ...string) error {
		t.Fatalf("runOpener should not be called in headless mode (invoked with %s %v)", name, args)
		return nil
	}

	tempDir := t.TempDir()
	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		t.Fatalf("failed to resolve absolute path: %v", err)
	}

	testFile := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	absTestFile, err := filepath.Abs(testFile)
	if err != nil {
		t.Fatalf("failed to resolve absolute path: %v", err)
	}

	t.Run("OpenFolder returns exact path in headless mode", func(t *testing.T) {
		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := fmt.Sprintf("%s\n", absTempDir)
		if out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})

	t.Run("OpenFile returns exact path in headless mode", func(t *testing.T) {
		out, err := OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := fmt.Sprintf("%s\n", absTestFile)
		if out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})
}

func TestOpen_Linux_GraphicDisplay(t *testing.T) {
	origGOOS := currentGOOS
	origOpener := runOpener
	defer func() {
		currentGOOS = origGOOS
		runOpener = origOpener
	}()

	currentGOOS = "linux"
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")

	tempDir := t.TempDir()
	absTempDir, _ := filepath.Abs(tempDir)
	testFile := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	absTestFile, _ := filepath.Abs(testFile)

	t.Run("opener success returns empty output", func(t *testing.T) {
		var recordedName string
		var recordedArgs []string

		runOpener = func(name string, args ...string) error {
			recordedName = name
			recordedArgs = args
			return nil
		}

		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if recordedName != "xdg-open" || len(recordedArgs) != 1 || recordedArgs[0] != absTempDir {
			t.Errorf("unexpected command: %s %v", recordedName, recordedArgs)
		}

		out, err = OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if recordedName != "xdg-open" || len(recordedArgs) != 1 || recordedArgs[0] != absTestFile {
			t.Errorf("unexpected command: %s %v", recordedName, recordedArgs)
		}
	})

	t.Run("opener failure falls back to path output", func(t *testing.T) {
		runOpener = func(name string, args ...string) error {
			return errors.New("xdg-open failed")
		}

		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := fmt.Sprintf("%s\n", absTempDir)
		if out != expected {
			t.Errorf("expected fallback %q, got %q", expected, out)
		}

		out, err = OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected = fmt.Sprintf("%s\n", absTestFile)
		if out != expected {
			t.Errorf("expected fallback %q, got %q", expected, out)
		}
	})
}

func TestOpen_Darwin(t *testing.T) {
	origGOOS := currentGOOS
	origOpener := runOpener
	defer func() {
		currentGOOS = origGOOS
		runOpener = origOpener
	}()

	currentGOOS = "darwin"
	tempDir := t.TempDir()
	absTempDir, _ := filepath.Abs(tempDir)
	testFile := filepath.Join(tempDir, "doc.pdf")
	if err := os.WriteFile(testFile, []byte("pdf data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	absTestFile, _ := filepath.Abs(testFile)

	t.Run("open folder dispatches open <path>", func(t *testing.T) {
		var recordedName string
		var recordedArgs []string

		runOpener = func(name string, args ...string) error {
			recordedName = name
			recordedArgs = args
			return nil
		}

		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if recordedName != "open" || len(recordedArgs) != 1 || recordedArgs[0] != absTempDir {
			t.Errorf("unexpected command: %s %v", recordedName, recordedArgs)
		}
	})

	t.Run("open file dispatches open -R <path>", func(t *testing.T) {
		var recordedName string
		var recordedArgs []string

		runOpener = func(name string, args ...string) error {
			recordedName = name
			recordedArgs = args
			return nil
		}

		out, err := OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if recordedName != "open" || len(recordedArgs) != 2 || recordedArgs[0] != "-R" || recordedArgs[1] != absTestFile {
			t.Errorf("unexpected command: %s %v", recordedName, recordedArgs)
		}
	})

	t.Run("opener failure falls back to path output", func(t *testing.T) {
		runOpener = func(name string, args ...string) error {
			return errors.New("open failed")
		}

		out, err := OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := fmt.Sprintf("%s\n", absTestFile)
		if out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})
}

func TestOpen_Windows(t *testing.T) {
	origGOOS := currentGOOS
	origOpener := runOpener
	defer func() {
		currentGOOS = origGOOS
		runOpener = origOpener
	}()

	currentGOOS = "windows"
	tempDir := t.TempDir()
	absTempDir, _ := filepath.Abs(tempDir)
	testFile := filepath.Join(tempDir, "sheet.xlsx")
	if err := os.WriteFile(testFile, []byte("sheet data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	absTestFile, _ := filepath.Abs(testFile)

	t.Run("open folder dispatches explorer.exe <path>", func(t *testing.T) {
		var recordedName string
		var recordedArgs []string

		runOpener = func(name string, args ...string) error {
			recordedName = name
			recordedArgs = args
			return nil
		}

		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if recordedName != "explorer.exe" || len(recordedArgs) != 1 || recordedArgs[0] != absTempDir {
			t.Errorf("unexpected command: %s %v", recordedName, recordedArgs)
		}
	})

	t.Run("open file dispatches explorer.exe /select,<path>", func(t *testing.T) {
		var recordedName string
		var recordedArgs []string

		runOpener = func(name string, args ...string) error {
			recordedName = name
			recordedArgs = args
			return nil
		}

		out, err := OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		expectedTarget := "/select," + absTestFile
		if recordedName != "explorer.exe" || len(recordedArgs) != 1 || recordedArgs[0] != expectedTarget {
			t.Errorf("unexpected command: %s %v", recordedName, recordedArgs)
		}
	})

	t.Run("opener failure falls back to path output", func(t *testing.T) {
		runOpener = func(name string, args ...string) error {
			return errors.New("explorer failed")
		}

		out, err := OpenFile(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := fmt.Sprintf("%s\n", absTestFile)
		if out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})
}

func TestOpen_OtherOS(t *testing.T) {
	origGOOS := currentGOOS
	origOpener := runOpener
	defer func() {
		currentGOOS = origGOOS
		runOpener = origOpener
	}()

	currentGOOS = "solaris"
	tempDir := t.TempDir()
	absTempDir, _ := filepath.Abs(tempDir)

	t.Run("headless fallback", func(t *testing.T) {
		t.Setenv("DISPLAY", "")
		t.Setenv("WAYLAND_DISPLAY", "")

		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := fmt.Sprintf("%s\n", absTempDir)
		if out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})

	t.Run("display present with opener success", func(t *testing.T) {
		t.Setenv("DISPLAY", ":0")
		runOpener = func(name string, args ...string) error {
			return nil
		}

		out, err := OpenFolder(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
	})
}

func TestDefaultRunOpener_CommandNotFound(t *testing.T) {
	err := defaultRunOpener("command_that_definitely_does_not_exist_xyz123")
	if err == nil {
		t.Fatal("expected error for non-existent command, got nil")
	}
}
