package report

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/wcw-wcw/stackindex/internal/models"
	"github.com/wcw-wcw/stackindex/internal/scanner"
)

const rerunRecommendation = "Rerun `stackindex analyze <repo> --no-tui` before relying on this index."

func RefreshStaleness(root string, analysis *models.Analysis) error {
	if analysis == nil {
		return nil
	}
	if root == "" {
		root = analysis.RepoPath
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	walk, err := scanner.WalkDetailed(absRoot)
	if err != nil {
		return err
	}
	current := map[string]models.FileFingerprint{}
	for _, file := range walk.Files {
		current[file.Path] = models.FileFingerprint{Path: file.Path, SizeBytes: file.SizeBytes, Hash: file.Hash}
	}
	indexed := analysis.Index.IndexedFiles
	if len(indexed) == 0 {
		indexed = fingerprintsFromFiles(analysis.Files)
	}
	previous := map[string]models.FileFingerprint{}
	var changed []string
	for _, file := range indexed {
		previous[file.Path] = file
		now, ok := current[file.Path]
		if !ok || now.Hash != file.Hash || now.SizeBytes != file.SizeBytes {
			changed = append(changed, file.Path)
		}
	}
	for path := range current {
		if _, ok := previous[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	analysis.Index.HashAlgorithm = "sha1"
	analysis.Index.IndexedFiles = indexed
	analysis.Index.Staleness = models.IndexStaleness{
		Stale:            len(changed) > 0,
		ChangedFileCount: len(changed),
		ChangedFiles:     capChangedFiles(changed, 20),
		CheckedAt:        time.Now().UTC().Format(time.RFC3339),
		Recommendation:   "Index is current.",
	}
	if len(changed) > 0 {
		analysis.Index.Staleness.Recommendation = rerunRecommendation
	}
	return nil
}

func fingerprintsFromFiles(files []models.FileInfo) []models.FileFingerprint {
	out := make([]models.FileFingerprint, 0, len(files))
	for _, file := range files {
		out = append(out, models.FileFingerprint{Path: file.Path, SizeBytes: file.SizeBytes, Hash: file.Hash})
	}
	return out
}

func capChangedFiles(paths []string, limit int) []string {
	if limit <= 0 || len(paths) <= limit {
		return append([]string{}, paths...)
	}
	return append([]string{}, paths[:limit]...)
}
