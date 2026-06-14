package scanner

import (
	"path/filepath"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func Classify(path string) (string, models.FileKind) {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasSuffix(base, "_test.go") {
		return languageForExt(ext), models.FileKindTest
	}

	switch base {
	case "package.json", "tsconfig.json", "vercel.json", "dockerfile", ".gitignore", ".env.example", "requirements.txt", "pyproject.toml":
		return configLanguage(base, ext), models.FileKindConfig
	case "readme.md":
		return "Markdown", models.FileKindDoc
	}

	if strings.Contains(path, "migration") || strings.Contains(path, "schema") {
		if ext == ".sql" || ext == ".prisma" || ext == ".ts" || ext == ".js" {
			return languageForExt(ext), models.FileKindConfig
		}
	}

	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".py", ".rs", ".java", ".rb", ".php":
		return languageForExt(ext), models.FileKindSource
	case ".json", ".yaml", ".yml", ".toml", ".env", ".sql", ".prisma":
		return languageForExt(ext), models.FileKindConfig
	case ".md", ".mdx", ".txt":
		return languageForExt(ext), models.FileKindDoc
	default:
		return languageForExt(ext), models.FileKindOther
	}
}

func languageForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js", ".mjs", ".cjs":
		return "JavaScript"
	case ".jsx":
		return "JSX"
	case ".ts":
		return "TypeScript"
	case ".tsx":
		return "TSX"
	case ".json":
		return "JSON"
	case ".md", ".mdx":
		return "Markdown"
	case ".sql":
		return "SQL"
	case ".prisma":
		return "Prisma"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	default:
		return ""
	}
}

func configLanguage(base, ext string) string {
	if base == "dockerfile" {
		return "Dockerfile"
	}
	return languageForExt(ext)
}
