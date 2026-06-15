package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeProjectIntegration(t *testing.T) {
	target := os.Getenv("STACKMAP_DESKTOP_ANALYZE_PATH")
	if target == "" {
		t.Skip("set STACKMAP_DESKTOP_ANALYZE_PATH to run desktop backend integration analysis")
	}

	response, err := AnalyzeProject(context.Background(), AnalyzeRequest{
		Path:     target,
		RunAudit: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{response.JSONReportPath, response.MDReportPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected report at %s: %v", path, err)
		}
	}
	if response.RepoPath != filepath.Clean(target) {
		t.Fatalf("expected repo path %q, got %q", filepath.Clean(target), response.RepoPath)
	}
}
