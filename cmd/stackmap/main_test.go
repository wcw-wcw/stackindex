package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestAnalyzeAuditLaunchesTUIWithAuditResult(t *testing.T) {
	root := healthyProject(t)
	called := false
	var tuiAudit *models.AuditResult
	restore := withLaunchTUI(func(analysis *models.Analysis, root string) error {
		called = true
		tuiAudit = analysis.Audit
		return nil
	})
	defer restore()

	err := analyze([]string{root, "--audit"}, false)
	if err != nil {
		t.Fatalf("analyze audit returned error: %v", err)
	}
	if !called {
		t.Fatal("analyze --audit did not launch TUI")
	}
	if tuiAudit == nil {
		t.Fatal("TUI analysis did not include audit result")
	}
	if !tuiAudit.Passed || tuiAudit.ExitCode != 0 {
		t.Fatalf("TUI audit result = %+v, want passed with exit code 0", tuiAudit)
	}

	analysis := readAnalysisJSON(t, root)
	if analysis.Audit == nil {
		t.Fatal("analysis.json did not include audit result")
	}
	if !analysis.Audit.Passed || analysis.Audit.ExitCode != 0 {
		t.Fatalf("audit result = %+v, want passed with exit code 0", analysis.Audit)
	}
}

func TestAnalyzeAuditNoTUIStaysNonInteractive(t *testing.T) {
	root := healthyProject(t)
	called := false
	restore := withLaunchTUI(func(analysis *models.Analysis, root string) error {
		called = true
		return errors.New("TUI should not launch")
	})
	defer restore()

	err := analyze([]string{root, "--audit", "--no-tui"}, false)
	if err != nil {
		t.Fatalf("analyze --audit --no-tui returned error: %v", err)
	}
	if called {
		t.Fatal("analyze --audit --no-tui launched TUI")
	}

	analysis := readAnalysisJSON(t, root)
	if analysis.Audit == nil {
		t.Fatal("analysis.json did not include audit result")
	}
}

func TestAnalyzeAuditNoTUIExitsByAuditResult(t *testing.T) {
	root := projectWithoutTests(t)
	called := false
	restore := withLaunchTUI(func(analysis *models.Analysis, root string) error {
		called = true
		return errors.New("TUI should not launch")
	})
	defer restore()

	err := analyze([]string{root, "--audit", "--no-tui"}, false)
	var failure auditFailure
	if !errors.As(err, &failure) {
		t.Fatalf("analyze --audit --no-tui error = %v, want auditFailure", err)
	}
	if failure.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", failure.exitCode)
	}
	if called {
		t.Fatal("analyze --audit --no-tui launched TUI")
	}
}

func TestAuditCommandStaysNonInteractive(t *testing.T) {
	root := healthyProject(t)
	called := false
	restore := withLaunchTUI(func(analysis *models.Analysis, root string) error {
		called = true
		return errors.New("TUI should not launch")
	})
	defer restore()

	err := run([]string{"audit", root})
	if err != nil {
		t.Fatalf("audit command returned error: %v", err)
	}
	if called {
		t.Fatal("audit command launched TUI")
	}
}

func TestAuditResultAbsentOutsideAuditMode(t *testing.T) {
	root := healthyProject(t)

	err := analyze([]string{root, "--no-tui"}, false)
	if err != nil {
		t.Fatalf("analyze returned error: %v", err)
	}

	analysis := readAnalysisJSON(t, root)
	if analysis.Audit != nil {
		t.Fatalf("analysis.json included audit result outside audit mode: %+v", analysis.Audit)
	}
}

func TestAnalyzeAuditJSONIncludesAuditResult(t *testing.T) {
	root := healthyProject(t)
	called := false
	restoreTUI := withLaunchTUI(func(analysis *models.Analysis, root string) error {
		called = true
		return errors.New("TUI should not launch")
	})
	defer restoreTUI()
	stdout, restoreStdout := captureStdout(t)

	err := analyze([]string{root, "--audit", "--json"}, false)
	restoreStdout()
	if err != nil {
		t.Fatalf("analyze --audit --json returned error: %v", err)
	}
	if called {
		t.Fatal("analyze --audit --json launched TUI")
	}

	var analysis models.Analysis
	if err := json.Unmarshal([]byte(stdout.String()), &analysis); err != nil {
		t.Fatalf("unmarshal JSON stdout: %v\n%s", err, stdout.String())
	}
	if analysis.Audit == nil {
		t.Fatal("JSON stdout did not include audit result")
	}
	if !analysis.Audit.Passed || analysis.Audit.ExitCode != 0 {
		t.Fatalf("audit result = %+v, want passed with exit code 0", analysis.Audit)
	}
}

func TestAuditErrorUsesAuditResultExitCode(t *testing.T) {
	err := auditError(&models.AuditResult{Passed: false, ExitCode: 1, Reasons: []string{"Tests were not detected."}})
	var failure auditFailure
	if !errors.As(err, &failure) {
		t.Fatalf("auditError() = %v, want auditFailure", err)
	}
	if failure.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", failure.exitCode)
	}
}

func TestNormalizeAnalyzeArgsKeepsAuditFlagsBeforePositionals(t *testing.T) {
	got := normalizeAnalyzeArgs([]string{".", "--audit", "--allow-medium", "--allow-missing-tests", "--fail-on-low", "--ai"})
	want := []string{"--audit", "--allow-medium", "--allow-missing-tests", "--fail-on-low", "--ai", "."}
	if len(got) != len(want) {
		t.Fatalf("normalizeAnalyzeArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAnalyzeArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeAnalyzeArgsKeepsAssignedAuditFlagsBeforePositionals(t *testing.T) {
	got := normalizeAnalyzeArgs([]string{".", "--audit=true", "--allow-medium=true", "--allow-missing-tests=true", "--fail-on-low=true"})
	want := []string{"--audit=true", "--allow-medium=true", "--allow-missing-tests=true", "--fail-on-low=true", "."}
	if len(got) != len(want) {
		t.Fatalf("normalizeAnalyzeArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAnalyzeArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeAskArgsKeepsFlagsBeforePathAndQuestion(t *testing.T) {
	got := normalizeAskArgs([]string{".", "Where are the API routes?", "--json", "--ai", "--model", "llama3.2:3b", "--no-tui"})
	want := []string{"--json", "--ai", "--model", "llama3.2:3b", "--no-tui", ".", "Where are the API routes?"}
	if len(got) != len(want) {
		t.Fatalf("normalizeAskArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAskArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeAskArgsKeepsAssignedFlagsBeforePositionals(t *testing.T) {
	got := normalizeAskArgs([]string{".", "What is this project for?", "--json=true", "--ai=false", "--model=qwen:7b", "--ai-debug=true"})
	want := []string{"--json=true", "--ai=false", "--model=qwen:7b", "--ai-debug=true", ".", "What is this project for?"}
	if len(got) != len(want) {
		t.Fatalf("normalizeAskArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAskArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func healthyProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "main_test.go"), "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n")
	return root
}

func projectWithoutTests(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	return root
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readAnalysisJSON(t *testing.T, root string) *models.Analysis {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".stackmap", "analysis.json"))
	if err != nil {
		t.Fatalf("read analysis.json: %v", err)
	}
	var analysis models.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		t.Fatalf("unmarshal analysis.json: %v", err)
	}
	return &analysis
}

func contains(items []string, want string) bool {
	return strings.Contains("\x00"+strings.Join(items, "\x00")+"\x00", "\x00"+want+"\x00")
}

func captureStdout(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer
	buffer := &bytes.Buffer{}
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(buffer, reader)
		done <- err
	}()
	return buffer, func() {
		_ = writer.Close()
		os.Stdout = previous
		if err := <-done; err != nil {
			t.Fatalf("copy stdout: %v", err)
		}
		_ = reader.Close()
	}
}

func withLaunchTUI(fn func(*models.Analysis, string) error) func() {
	previous := launchTUI
	launchTUI = fn
	return func() {
		launchTUI = previous
	}
}
