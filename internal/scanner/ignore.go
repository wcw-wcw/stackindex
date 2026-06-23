package scanner

import (
	"path/filepath"
	"strings"
)

var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"coverage":     true,
	".stackindex":  true,
	".gocache":     true,
	"vendor":       true,
}

var ignoredFiles = map[string]bool{
	".DS_Store": true,
}

func ShouldIgnoreDir(name string) bool {
	return ignoredDirs[name]
}

func ShouldIgnoreFile(rel string) bool {
	base := filepath.Base(rel)
	if ignoredFiles[base] {
		return true
	}
	if base == ".env" || strings.HasPrefix(base, ".env.") && base != ".env.example" {
		return true
	}
	return false
}

func IsLikelyBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
