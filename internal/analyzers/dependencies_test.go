package analyzers

import (
	"strings"
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestAnalyzeDependencyGraphExtractsAndResolvesJSTSImports(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":              `{"name":"demo","scripts":{"dev":"vite"},"dependencies":{"@vitejs/plugin-react":"latest","vite":"latest","react":"latest","lodash":"latest"}}`,
		"src/main.tsx":              `import React from "react"; import { App } from "./App"; import "./setup";`,
		"src/App.tsx":               `import Button from "./components/Button"; export { helper } from "./lib"; const api = require("./api/client"); const dyn = import("./dynamic");`,
		"src/components/Button.tsx": `export function Button() { return null }`,
		"src/lib/index.ts":          `export const helper = true`,
		"src/api/client.ts":         `import missing from "../missing"; import lodash from "lodash";`,
		"src/setup.ts":              `export const setup = true`,
		"src/dynamic.ts":            `export const dynamic = true`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := analysis.Dependencies
	for _, want := range []struct {
		from string
		to   string
		kind string
	}{
		{"src/main.tsx", "src/App.tsx", "relative"},
		{"src/main.tsx", "src/setup.ts", "relative"},
		{"src/App.tsx", "src/components/Button.tsx", "relative"},
		{"src/App.tsx", "src/lib/index.ts", "relative"},
		{"src/App.tsx", "src/api/client.ts", "relative"},
		{"src/App.tsx", "src/dynamic.ts", "relative"},
	} {
		if !hasDependencyEdge(graph.Edges, want.from, want.to, want.kind) {
			t.Fatalf("missing edge %+v in %#v", want, graph.Edges)
		}
	}
	if !hasPackageEdge(graph.Edges, "src/main.tsx", "react", "package") || !hasPackageEdge(graph.Edges, "src/api/client.ts", "lodash", "package") {
		t.Fatalf("missing external package edges: %#v", graph.Edges)
	}
	if !hasUnresolvedImport(graph.UnresolvedImports, "src/api/client.ts", "../missing") {
		t.Fatalf("missing unresolved relative import: %#v", graph.UnresolvedImports)
	}
	if !contains(graph.Entrypoints, "src/main.tsx") {
		t.Fatalf("missing Vite/React entrypoint: %#v", graph.Entrypoints)
	}
	if !topFileHasCounts(graph.TopConnectedFiles, "src/App.tsx", 4, 1) {
		t.Fatalf("top connected files missing App fan-out/fan-in: %#v", graph.TopConnectedFiles)
	}
	if !contains(graph.ArchitectureHints, "Frontend entrypoints connect to top-level UI or component modules.") {
		t.Fatalf("missing frontend architecture hint: %#v", graph.ArchitectureHints)
	}
}

func TestAnalyzeDependencyGraphParsesGoImportsAndModuleInternalEdges(t *testing.T) {
	root := tempProject(t, map[string]string{
		"go.mod":                "module example.com/demo\n\ngo 1.22\n",
		"cmd/demo/main.go":      "package main\n\nimport (\n  \"fmt\"\n  run \"example.com/demo/internal/run\"\n  \"github.com/pkg/errors\"\n)\n\nfunc main() { fmt.Println(run.Name, errors.New(\"x\")) }\n",
		"internal/run/run.go":   "package run\n\nimport \"strings\"\n\nconst Name = strings.TrimSpace(\"demo\")\n",
		"internal/other/doc.go": "package other\n",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := analysis.Dependencies
	if !hasDependencyEdge(graph.Edges, "cmd/demo/main.go", "internal/run/run.go", "internal") {
		t.Fatalf("missing Go internal module edge: %#v", graph.Edges)
	}
	if !hasPackageEdge(graph.Edges, "cmd/demo/main.go", "fmt", "package") {
		t.Fatalf("missing Go stdlib package edge: %#v", graph.Edges)
	}
	if !hasPackageEdge(graph.Edges, "cmd/demo/main.go", "github.com/pkg/errors", "external") {
		t.Fatalf("missing Go external edge: %#v", graph.Edges)
	}
	if !contains(graph.Entrypoints, "cmd/demo/main.go") {
		t.Fatalf("missing Go CLI entrypoint: %#v", graph.Entrypoints)
	}
	if !topFileHasCounts(graph.TopConnectedFiles, "cmd/demo/main.go", 1, 0) {
		t.Fatalf("top connected files missing Go main import count: %#v", graph.TopConnectedFiles)
	}
}

func TestAnalyzeDependencyGraphDetectsAPIRouteHintsAndDatabaseScripts(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":               `{"name":"api-demo","scripts":{"migrate":"node scripts/migrate.mjs"},"dependencies":{"next":"latest","pg":"latest"}}`,
		"src/app/api/rules/route.ts": `import { db } from "../../../lib/db"; export async function GET() { return Response.json(db) }`,
		"src/lib/db.ts":              `export const db = {}`,
		"scripts/migrate.mjs":        `import "../src/lib/db";`,
		"db/migrations/001_init.sql": `create table rules(id text);`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"API route files import shared library or database-related code.",
		"Worker or operational scripts connect to shared application modules.",
		"Database migration files are present alongside database-related scripts.",
	} {
		if !contains(analysis.Dependencies.ArchitectureHints, want) {
			t.Fatalf("missing architecture hint %q from %#v", want, analysis.Dependencies.ArchitectureHints)
		}
	}
	if !contains(analysis.Dependencies.Entrypoints, "src/app/api/rules/route.ts") || !contains(analysis.Dependencies.Entrypoints, "scripts/migrate.mjs") {
		t.Fatalf("missing route/script entrypoints: %#v", analysis.Dependencies.Entrypoints)
	}
}

func TestAnalyzeDependencyGraphDoesNotCallViteProjectStaticOnlyWhenAPIFilesExist(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":                        `{"name":"animerec","scripts":{"sync:mal":"node scripts/sync-mal-catalog.mjs"},"dependencies":{"vite":"latest","react":"latest","@vercel/node":"latest"}}`,
		"src/main.tsx":                        `import "./App";`,
		"src/App.tsx":                         `import type { Anime } from "./types"; export function App() { return null }`,
		"src/types.ts":                        `export type Anime = { id: string }`,
		"api/anime/lookup.js":                 `import "../../scripts/catalog-storage.mjs"; export default function handler(req, res) { res.json({}) }`,
		"scripts/catalog-api.mjs":             `import "./catalog-storage.mjs";`,
		"scripts/catalog-storage.mjs":         `import "./neon-db.mjs";`,
		"scripts/neon-db.mjs":                 `export const db = true`,
		"scripts/sync-mal-catalog.mjs":        `import "./catalog-storage.mjs";`,
		"database/migrations/001_catalog.sql": `create table anime(id text);`,
		"docs/deployment-checklist.md":        `# Deploy`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Routes) == 0 || !hasRoute(analysis.Routes, "/api/anime/lookup") || !hasRoute(analysis.Routes, "/scripts/catalog-api") {
		t.Fatalf("missing API route/script detection: %#v", analysis.Routes)
	}
	for _, hint := range analysis.Dependencies.ArchitectureHints {
		if strings.Contains(hint, "mostly frontend/static") {
			t.Fatalf("static-only hint should not appear with API files: %#v", analysis.Dependencies.ArchitectureHints)
		}
	}
	for _, want := range []string{
		"This appears to be a Vite/React frontend with supporting API or data tooling.",
		"Serverless/API files or local API scripts are present, but no health endpoint was detected.",
	} {
		if !contains(analysis.Dependencies.ArchitectureHints, want) {
			t.Fatalf("missing architecture hint %q from %#v", want, analysis.Dependencies.ArchitectureHints)
		}
	}
}

func TestAnalyzeDependencyGraphFallsBackToRootAliasForNextAppDirs(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":          `{"name":"folio","dependencies":{"next":"latest","react":"latest"}}`,
		"app/page.tsx":          `import Card from "@/components/Card"; import { projects } from "@/data/projects"; export default function Page() { return <Card items={projects} /> }`,
		"components/Card.tsx":   `export default function Card() { return null }`,
		"data/projects.ts":      `export const projects = []`,
		"lib/format.ts":         `export const format = (value: string) => value`,
		"app/projects/page.tsx": `import { format } from "@/lib/format"; export default function Projects() { return format("x") }`,
		"tsconfig.json":         `{"compilerOptions":{"baseUrl":"."}}`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		from string
		to   string
	}{
		{"app/page.tsx", "components/Card.tsx"},
		{"app/page.tsx", "data/projects.ts"},
		{"app/projects/page.tsx", "lib/format.ts"},
	} {
		if !hasDependencyEdge(analysis.Dependencies.Edges, want.from, want.to, "internal") {
			t.Fatalf("missing root alias edge %+v from %#v", want, analysis.Dependencies.Edges)
		}
	}
	if analysis.Quality.UnresolvedAliasImports != 0 {
		t.Fatalf("unexpected unresolved alias imports: %#v", analysis.Dependencies.UnresolvedImports)
	}
}

func hasDependencyEdge(edges []models.DependencyEdge, from, to, kind string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return true
		}
	}
	return false
}

func hasRoute(routes []models.RouteInfo, path string) bool {
	for _, route := range routes {
		if route.Path == path {
			return true
		}
	}
	return false
}

func hasPackageEdge(edges []models.DependencyEdge, from, importPath, kind string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.ImportPath == importPath && edge.Kind == kind && edge.To == "" {
			return true
		}
	}
	return false
}

func hasUnresolvedImport(imports []models.UnresolvedImport, from, importPath string) bool {
	for _, item := range imports {
		if item.From == from && item.ImportPath == importPath {
			return true
		}
	}
	return false
}

func topFileHasCounts(files []models.ConnectedFileSummary, path string, imports, importedBy int) bool {
	for _, file := range files {
		if file.Path == path && file.ImportsCount == imports && file.ImportedByCount == importedBy {
			return true
		}
	}
	return false
}
