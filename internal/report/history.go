package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/will/stackmap/internal/models"
)

type Snapshot struct {
	Timestamp    string
	Directory    string
	JSONPath     string
	MarkdownPath string
	AuditStatus  string
	AIStatus     string
	GeneratedAt  time.Time
}

var snapshotNow = time.Now

func WriteSnapshot(root string) (*Snapshot, error) {
	historyDir := filepath.Join(root, ".stackmap", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, err
	}
	timestamp := snapshotNow().Format("20060102-150405")
	snapshotDir, snapshotName, err := createSnapshotDir(historyDir, timestamp)
	if err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(snapshotDir, "analysis.json")
	markdownPath := filepath.Join(snapshotDir, "repo-report.md")
	if err := copyFile(filepath.Join(root, ".stackmap", "analysis.json"), jsonPath); err != nil {
		return nil, err
	}
	if err := copyFile(filepath.Join(root, ".stackmap", "reports", "repo-report.md"), markdownPath); err != nil {
		return nil, err
	}
	return &Snapshot{
		Timestamp:    snapshotName,
		Directory:    snapshotDir,
		JSONPath:     jsonPath,
		MarkdownPath: markdownPath,
	}, nil
}

func ListSnapshots(root string) ([]Snapshot, error) {
	historyDir := filepath.Join(root, ".stackmap", "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Snapshot{}, nil
		}
		return nil, err
	}
	snapshots := make([]Snapshot, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		dir := filepath.Join(historyDir, name)
		snapshot := Snapshot{
			Timestamp:    name,
			Directory:    dir,
			JSONPath:     filepath.Join(dir, "analysis.json"),
			MarkdownPath: filepath.Join(dir, "repo-report.md"),
			AuditStatus:  "unknown",
			AIStatus:     "unknown",
		}
		enrichSnapshotFromAnalysis(&snapshot)
		snapshots = append(snapshots, snapshot)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp > snapshots[j].Timestamp
	})
	return snapshots, nil
}

func createSnapshotDir(historyDir, timestamp string) (string, string, error) {
	for i := 0; ; i++ {
		name := timestamp
		if i > 0 {
			name = fmt.Sprintf("%s-%d", timestamp, i)
		}
		dir := filepath.Join(historyDir, name)
		err := os.Mkdir(dir, 0755)
		if err == nil {
			return dir, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", err
		}
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func enrichSnapshotFromAnalysis(snapshot *Snapshot) {
	data, err := os.ReadFile(snapshot.JSONPath)
	if err != nil {
		return
	}
	var analysis models.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return
	}
	snapshot.GeneratedAt = analysis.GeneratedAt
	snapshot.AuditStatus = auditStatus(analysis.Audit)
	snapshot.AIStatus = aiSnapshotStatus(analysis.AI)
}

func auditStatus(result *models.AuditResult) string {
	if result == nil {
		return "not run"
	}
	if result.Passed {
		return "passed"
	}
	return "failed"
}

func aiSnapshotStatus(summary *models.AISummary) string {
	if summary == nil {
		return "not requested"
	}
	if summary.Status != "" {
		return summary.Status
	}
	if summary.Enabled {
		return "requested"
	}
	return "unavailable"
}
