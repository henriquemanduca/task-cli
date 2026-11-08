package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantProject string
		wantErr     string
	}{
		{
			name:        "simple key",
			content:     "PROJECT_CODE=BACKLOG\n",
			wantProject: "BACKLOG",
		},
		{
			name:        "comments, blanks, export and quotes",
			content:     "# workspace defaults\n\n  export PROJECT_CODE=\"BACKLOG\"  \n",
			wantProject: "BACKLOG",
		},
		{
			name:        "single quotes and inline comment",
			content:     "PROJECT_CODE='BACKLOG' # main backlog\n",
			wantProject: "BACKLOG",
		},
		{
			name:        "unquoted inline comment",
			content:     "PROJECT_CODE=BACKLOG # main backlog\n",
			wantProject: "BACKLOG",
		},
		{
			name:        "hash inside value is preserved",
			content:     "PROJECT_CODE=TEAM#1\n",
			wantProject: "TEAM#1",
		},
		{
			name:        "last duplicate wins",
			content:     "PROJECT_CODE=FIRST\nPROJECT_CODE=SECOND\n",
			wantProject: "SECOND",
		},
		{
			name:        "empty value clears default",
			content:     "PROJECT_CODE=\n",
			wantProject: "",
		},
		{
			name:    "invalid line",
			content: "PROJECT_CODE\n",
			wantErr: "expected KEY=VALUE",
		},
		{
			name:    "unknown key",
			content: "TASK_CLI_PROJET=BACKLOG\n",
			wantErr: "unknown key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parse(strings.NewReader(tt.content))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Project != tt.wantProject {
				t.Errorf("Project: expected %q, got %q", tt.wantProject, cfg.Project)
			}
		})
	}
}

func TestLoadFromDirNearestAndHomeBoundary(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(home, "project")
	nested := filepath.Join(project, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	rootConfig := filepath.Join(root, FileName)
	homeConfig := filepath.Join(home, FileName)
	projectConfig := filepath.Join(project, FileName)

	writeConfig(t, rootConfig, "PROJECT_CODE=ABOVE_HOME\n")
	writeConfig(t, homeConfig, "PROJECT_CODE=HOME\n")
	writeConfig(t, projectConfig, "PROJECT_CODE=PROJECT\n")

	// Nearest file wins.
	cfg, path, err := loadFromDir(nested, home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "PROJECT" {
		t.Errorf("expected PROJECT, got %q", cfg.Project)
	}
	if path != projectConfig {
		t.Errorf("expected path %q, got %q", projectConfig, path)
	}

	// Without the project file, the home file is used.
	if err := os.Remove(projectConfig); err != nil {
		t.Fatalf("failed to remove project config: %v", err)
	}
	cfg, path, err = loadFromDir(nested, home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "HOME" || path != homeConfig {
		t.Errorf("expected HOME at %q, got %q at %q", homeConfig, cfg.Project, path)
	}

	// The search stops at home: files above home are ignored.
	if err := os.Remove(homeConfig); err != nil {
		t.Fatalf("failed to remove home config: %v", err)
	}
	cfg, path, err = loadFromDir(nested, home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "" || path != "" {
		t.Errorf("expected empty config above home boundary, got %q at %q", cfg.Project, path)
	}

	// Outside home, the search walks up to the filesystem root.
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	cfg, path, err = loadFromDir(outside, home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "ABOVE_HOME" || path != rootConfig {
		t.Errorf("expected ABOVE_HOME at %q, got %q at %q", rootConfig, cfg.Project, path)
	}
}

func TestLoadFromDirMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, path, err := loadFromDir(dir, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Project != "" {
		t.Errorf("expected empty project, got %q", cfg.Project)
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}
