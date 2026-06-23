package scanner

import (
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

const maxHashBytes = 2 * 1024 * 1024

type WalkResult struct {
	Files   []models.FileInfo
	Quality models.IndexQuality
}

func Walk(root string) ([]models.FileInfo, error) {
	result, err := WalkDetailed(root)
	if err != nil {
		return nil, err
	}
	return result.Files, nil
}

func WalkDetailed(root string) (*WalkResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var files []models.FileInfo
	quality := models.IndexQuality{
		GeneratedOrCacheDirsIgnored: true,
		IgnoredDirCounts:            map[string]int{},
	}
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != absRoot && ShouldIgnoreDir(d.Name()) {
				quality.IgnoredDirCounts[d.Name()]++
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ShouldIgnoreFile(rel) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxHashBytes {
			quality.LargeFilesSkipped++
			quality.SkippedLargeFiles = appendSkippedFile(quality.SkippedLargeFiles, rel, info.Size(), "larger than scanner limit")
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if IsLikelyBinary(data) {
			quality.BinaryFilesSkipped++
			quality.SkippedBinaryFiles = appendSkippedFile(quality.SkippedBinaryFiles, rel, info.Size(), "binary content")
			return nil
		}

		sum := sha1.Sum(data)
		return addFile(&files, rel, info.Size(), hex.EncodeToString(sum[:]))
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.Compare(files[i].Path, files[j].Path) < 0
	})
	if len(quality.IgnoredDirCounts) == 0 {
		quality.IgnoredDirCounts = nil
	}
	return &WalkResult{Files: files, Quality: quality}, nil
}

func addFile(files *[]models.FileInfo, rel string, size int64, hash string) error {
	lang, kind := Classify(rel)
	*files = append(*files, models.FileInfo{
		Path:      rel,
		Ext:       strings.TrimPrefix(strings.ToLower(filepath.Ext(rel)), "."),
		Language:  lang,
		SizeBytes: size,
		Kind:      kind,
		Hash:      hash,
	})
	return nil
}

func appendSkippedFile(files []models.SkippedFileInfo, path string, size int64, reason string) []models.SkippedFileInfo {
	if len(files) >= 20 {
		return files
	}
	return append(files, models.SkippedFileInfo{Path: path, SizeBytes: size, Reason: reason})
}
