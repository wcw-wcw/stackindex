package analyzers

import "github.com/wcw-wcw/stackindex/internal/models"

func hasFile(files []models.FileInfo, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func hasSourceFiles(files []models.FileInfo) bool {
	for _, file := range files {
		if file.Kind == models.FileKindSource {
			return true
		}
	}
	return false
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}
