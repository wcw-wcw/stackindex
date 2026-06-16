package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateExistingDir(t *testing.T) {
	root := t.TempDir()

	path, err := validateExistingDir(root, "project folder")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Clean(root) {
		t.Fatalf("validated path = %q, want %q", path, filepath.Clean(root))
	}
}

func TestValidateExistingDirRejectsFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "analysis.json")
	if err := os.WriteFile(file, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := validateExistingDir(file, "project folder")
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not a directory error, got %v", err)
	}
}

func TestValidateReportFileRejectsMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), ".stackmap", "analysis.json")

	_, err := validateReportFile(missing, ".json", "JSON report")
	if err == nil || !strings.Contains(err.Error(), "JSON report does not exist") {
		t.Fatalf("expected missing report error, got %v", err)
	}
}

func TestValidateReportFileRejectsWrongExtension(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "repo-report.txt")
	if err := os.WriteFile(file, []byte("report"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := validateReportFile(file, ".md", "Markdown report")
	if err == nil || !strings.Contains(err.Error(), "must be a .md file") {
		t.Fatalf("expected extension error, got %v", err)
	}
}

func TestBuildCLICommandLocalAuditAIModel(t *testing.T) {
	command, err := buildCLICommand(CLICommandRequest{
		RepoPath:    `/tmp/example repo`,
		AuditStatus: "passed",
		AIStatus:    "generated",
		AIModel:     `llama "small"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `stackmap analyze "/tmp/example repo" --audit --ai --model "llama \"small\""`
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuildCLICommandOmitsAuditAndAIWhenNotRun(t *testing.T) {
	command, err := buildCLICommand(CLICommandRequest{
		RepoPath:    "/tmp/example",
		AuditStatus: "not run",
		AIStatus:    "not requested",
		AIModel:     "llama3.2",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `stackmap analyze "/tmp/example"`
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuildCLICommandUsesGitHubLocalCachePath(t *testing.T) {
	command, err := buildCLICommand(CLICommandRequest{
		RepoPath:       "/tmp/other",
		SourceType:     sourceTypeGitHub,
		LocalCachePath: "/Users/will/Library/Caches/StackMap/repos/github.com/owner/repo",
		AuditStatus:    "failed",
		AIStatus:       "not requested",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `stackmap analyze "/Users/will/Library/Caches/StackMap/repos/github.com/owner/repo" --audit`
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuildCLICommandRequiresProjectPath(t *testing.T) {
	_, err := buildCLICommand(CLICommandRequest{})
	if err == nil || !strings.Contains(err.Error(), "project path is required") {
		t.Fatalf("expected project path error, got %v", err)
	}
}
