package analyzers

import (
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
	"github.com/wcw-wcw/stackindex/internal/scanner"
)

func TestDetectStackIncludesNeonPostgres(t *testing.T) {
	stack := DetectStack("", nil, &models.PackageInfo{
		Dependencies: map[string]string{
			"@neondatabase/serverless": "^1.0.0",
		},
	})
	if len(stack.Databases) != 1 || stack.Databases[0] != "Neon Postgres" {
		t.Fatalf("Databases = %#v, want Neon Postgres", stack.Databases)
	}
}

func TestDetectStackUsesNestedFrontendPackageAndConfig(t *testing.T) {
	root := tempProject(t, map[string]string{
		"frontend/package.json":   `{"dependencies":{"react":"latest"},"devDependencies":{"vite":"latest","typescript":"latest"}}`,
		"frontend/vite.config.ts": "export default {}",
		"frontend/src/main.tsx":   "import React from 'react'",
	})
	files, err := scanner.Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := AnalyzePackage(root, files)
	if err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(root, files, pkg)
	for _, want := range []string{"React", "Vite"} {
		if !contains(stack.Frameworks, want) {
			t.Fatalf("Frameworks = %#v, want %s", stack.Frameworks, want)
		}
	}
	if !contains(stack.Languages, "TypeScript") {
		t.Fatalf("Languages = %#v, want TypeScript", stack.Languages)
	}
}

func TestDetectStackDetectsPythonFastAPI(t *testing.T) {
	root := tempProject(t, map[string]string{
		"backend/requirements.txt": "fastapi\nuvicorn\n",
		"backend/app/main.py":      "from fastapi import FastAPI\napp = FastAPI()\n",
	})
	files, err := scanner.Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(root, files, nil)
	if !contains(stack.Languages, "Python") {
		t.Fatalf("Languages = %#v, want Python", stack.Languages)
	}
	if !contains(stack.Frameworks, "FastAPI") {
		t.Fatalf("Frameworks = %#v, want FastAPI", stack.Frameworks)
	}
}

func TestDetectStackDetectsTauriFromFilesAndScripts(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":              `{"scripts":{"tauri":"tauri dev"},"dependencies":{"@tauri-apps/api":"latest","vite":"latest","react":"latest"}}`,
		"src-tauri/tauri.conf.json": `{}`,
		"src-tauri/Cargo.toml":      `[package]\nname = "desktop"`,
		"src-tauri/src/main.rs":     "fn main() {}",
		"src/App.tsx":               "export default function App() { return null }",
	})
	files, err := scanner.Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := AnalyzePackage(root, files)
	if err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(root, files, pkg)
	if !contains(stack.Frameworks, "Tauri") {
		t.Fatalf("Frameworks = %#v, want Tauri", stack.Frameworks)
	}
	if !contains(stack.Languages, "Rust") {
		t.Fatalf("Languages = %#v, want Rust", stack.Languages)
	}
}
