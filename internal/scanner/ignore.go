package scanner

import (
	"path/filepath"
	"strings"
)

var ignoredDirs = map[string]bool{
	".cache":        true,
	".claude":       true,
	".codex":        true,
	".git":          true,
	".gocache":      true,
	".mypy_cache":   true,
	".next":         true,
	".nuxt":         true,
	".nyc_output":   true,
	".output":       true,
	".parcel-cache": true,
	".pytest_cache": true,
	".ruff_cache":   true,
	".stackindex":   true,
	".stackmap":     true,
	".svelte-kit":   true,
	".turbo":        true,
	".vite":         true,
	"build":         true,
	"coverage":      true,
	"dist":          true,
	"logs":          true,
	"node_modules":  true,
	"out":           true,
	"target":        true,
	"tmp":           true,
	"vendor":        true,
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
