package analyzers

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func DetectStack(root string, files []models.FileInfo, pkg *models.PackageInfo) models.StackInfo {
	var stack models.StackInfo
	for _, file := range files {
		switch file.Language {
		case "Go", "JavaScript", "TypeScript", "TSX", "JSX", "Python":
			stack.Languages = appendUnique(stack.Languages, normalizeLanguage(file.Language))
		}
		lowerPath := strings.ToLower(filepath.ToSlash(file.Path))
		base := strings.ToLower(filepath.Base(lowerPath))
		switch lowerPath {
		case "vercel.json":
			stack.Deployment = appendUnique(stack.Deployment, "Vercel")
		case "dockerfile":
			stack.Deployment = appendUnique(stack.Deployment, "Docker")
		}
		switch base {
		case "tsconfig.json":
			stack.Languages = appendUnique(stack.Languages, "TypeScript")
		case "requirements.txt", "pyproject.toml":
			if fileMentions(root, file.Path, "fastapi") {
				stack.Frameworks = appendUnique(stack.Frameworks, "FastAPI")
			}
		}
		if strings.HasPrefix(base, "tailwind.config.") {
			stack.Frameworks = appendUnique(stack.Frameworks, "Tailwind CSS")
		}
		if strings.HasPrefix(base, "vite.config.") {
			stack.Frameworks = appendUnique(stack.Frameworks, "Vite")
		}
		if strings.HasPrefix(base, "next.config.") {
			stack.Frameworks = appendUnique(stack.Frameworks, "Next.js")
			stack.Deployment = appendUnique(stack.Deployment, "Vercel")
		}
		if file.Language == "Python" && fileMentionsAny(root, file.Path, "from fastapi import", "import fastapi", "FastAPI(") {
			stack.Frameworks = appendUnique(stack.Frameworks, "FastAPI")
		}
	}
	if pkg != nil {
		addDepStack(pkg, &stack)
	}
	return stack
}

func fileMentions(root, path string, terms ...string) bool {
	return fileMentionsAny(root, path, terms...)
}

func fileMentionsAny(root, path string, terms ...string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	for _, term := range terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func addDepStack(pkg *models.PackageInfo, stack *models.StackInfo) {
	deps := allDeps(pkg)
	for dep := range deps {
		switch dep {
		case "react":
			stack.Frameworks = appendUnique(stack.Frameworks, "React")
		case "vite":
			stack.Frameworks = appendUnique(stack.Frameworks, "Vite")
		case "next":
			stack.Frameworks = appendUnique(stack.Frameworks, "Next.js")
			stack.Deployment = appendUnique(stack.Deployment, "Vercel")
		case "express":
			stack.Frameworks = appendUnique(stack.Frameworks, "Express")
		case "typescript":
			stack.Languages = appendUnique(stack.Languages, "TypeScript")
		case "tailwindcss":
			stack.Frameworks = appendUnique(stack.Frameworks, "Tailwind CSS")
		case "pg":
			stack.Databases = appendUnique(stack.Databases, "PostgreSQL")
		case "@neondatabase/serverless":
			stack.Databases = appendUnique(stack.Databases, "Neon Postgres")
		case "prisma":
			stack.Databases = appendUnique(stack.Databases, "Prisma")
		case "drizzle-orm":
			stack.Databases = appendUnique(stack.Databases, "Drizzle")
		case "sqlite3", "better-sqlite3":
			stack.Databases = appendUnique(stack.Databases, "SQLite")
		case "vitest":
			stack.Testing = appendUnique(stack.Testing, "Vitest")
		case "jest":
			stack.Testing = appendUnique(stack.Testing, "Jest")
		case "@playwright/test":
			stack.Testing = appendUnique(stack.Testing, "Playwright")
		case "vercel":
			stack.Deployment = appendUnique(stack.Deployment, "Vercel")
		}
	}
	if len(pkg.Dependencies) > 0 || len(pkg.DevDependencies) > 0 {
		stack.Frameworks = appendUnique(stack.Frameworks, "Node.js")
	}
}

func normalizeLanguage(lang string) string {
	if lang == "TSX" {
		return "TypeScript"
	}
	if lang == "JSX" {
		return "JavaScript"
	}
	return lang
}
