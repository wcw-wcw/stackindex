package analyzers

import (
	"strings"

	"github.com/will/stackmap/internal/models"
)

func DetectStack(files []models.FileInfo, pkg *models.PackageInfo) models.StackInfo {
	var stack models.StackInfo
	for _, file := range files {
		switch file.Language {
		case "Go", "JavaScript", "TypeScript", "TSX", "JSX":
			stack.Languages = appendUnique(stack.Languages, normalizeLanguage(file.Language))
		}
		switch strings.ToLower(file.Path) {
		case "vercel.json":
			stack.Deployment = appendUnique(stack.Deployment, "Vercel")
		case "dockerfile":
			stack.Deployment = appendUnique(stack.Deployment, "Docker")
		case "tsconfig.json":
			stack.Languages = appendUnique(stack.Languages, "TypeScript")
		}
		if strings.HasPrefix(strings.ToLower(file.Path), "tailwind.config.") {
			stack.Frameworks = appendUnique(stack.Frameworks, "Tailwind CSS")
		}
		if strings.HasPrefix(strings.ToLower(file.Path), "vite.config.") {
			stack.Frameworks = appendUnique(stack.Frameworks, "Vite")
		}
		if strings.HasPrefix(strings.ToLower(file.Path), "next.config.") {
			stack.Frameworks = appendUnique(stack.Frameworks, "Next.js")
			stack.Deployment = appendUnique(stack.Deployment, "Vercel")
		}
	}
	if pkg != nil {
		addDepStack(pkg, &stack)
	}
	return stack
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
