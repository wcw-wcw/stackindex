package scanner

import (
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/will/stackmap/internal/models"
)

const maxHashBytes = 2 * 1024 * 1024

func Walk(root string) ([]models.FileInfo, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var files []models.FileInfo
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != absRoot && ShouldIgnoreDir(d.Name()) {
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
			return addFile(&files, rel, info.Size(), "")
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if IsLikelyBinary(data) {
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
	return files, nil
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
