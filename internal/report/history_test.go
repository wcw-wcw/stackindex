package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/will/stackmap/internal/models"
)

func TestExportAllCreatesSnapshotAndKeepsLatestReports(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 6, 16, 12, 34, 56, 0, time.UTC)
	withSnapshotNow(t, fixed)
	analysis := &models.Analysis{RepoName: "demo", RepoPath: root, GeneratedAt: fixed}

	if err := ExportAll(root, analysis); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(root, ".stackmap", "analysis.json"),
		filepath.Join(root, ".stackmap", "reports", "repo-report.md"),
		filepath.Join(root, ".stackmap", "history", "20260616-123456", "analysis.json"),
		filepath.Join(root, ".stackmap", "history", "20260616-123456", "repo-report.md"),
	} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("expected written report at %s: info=%#v err=%v", path, info, err)
		}
	}
}

func TestWriteSnapshotDoesNotOverwriteTimestampCollision(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 6, 16, 12, 34, 56, 0, time.UTC)
	withSnapshotNow(t, fixed)
	analysis := &models.Analysis{RepoName: "demo", RepoPath: root, GeneratedAt: fixed}

	if err := ExportAll(root, analysis); err != nil {
		t.Fatal(err)
	}
	firstMarker := filepath.Join(root, ".stackmap", "history", "20260616-123456", "marker.txt")
	if err := os.WriteFile(firstMarker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ExportAll(root, analysis); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(firstMarker); err != nil {
		t.Fatalf("first snapshot was overwritten: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".stackmap", "history", "20260616-123456-1", "analysis.json")); err != nil {
		t.Fatalf("expected collision snapshot: %v", err)
	}
}

func TestListSnapshotsFromExistingFolders(t *testing.T) {
	root := t.TempDir()
	generated := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName:    "demo",
		GeneratedAt: generated,
		Audit:       &models.AuditResult{Passed: true},
		AI:          &models.AISummary{Enabled: true, Status: "generated_structured"},
	})
	writeSnapshotFixture(t, root, "20260616-130000", &models.Analysis{RepoName: "demo"})

	snapshots, err := ListSnapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snapshots))
	}
	if snapshots[0].Timestamp != "20260616-130000" || snapshots[1].Timestamp != "20260616-120000" {
		t.Fatalf("snapshots not sorted newest first: %#v", snapshots)
	}
	if snapshots[1].AuditStatus != "passed" || snapshots[1].AIStatus != "generated_structured" || !snapshots[1].GeneratedAt.Equal(generated) {
		t.Fatalf("snapshot metadata not read: %#v", snapshots[1])
	}
}

func TestListSnapshotsMissingHistoryIsEmpty(t *testing.T) {
	snapshots, err := ListSnapshots(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("snapshot count = %d, want 0", len(snapshots))
	}
}

func withSnapshotNow(t *testing.T, value time.Time) {
	t.Helper()
	original := snapshotNow
	snapshotNow = func() time.Time { return value }
	t.Cleanup(func() { snapshotNow = original })
}

func writeSnapshotFixture(t *testing.T, root, timestamp string, analysis *models.Analysis) {
	t.Helper()
	dir := filepath.Join(root, ".stackmap", "history", timestamp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "analysis.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "repo-report.md"), []byte("# report\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
