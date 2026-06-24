package analyzers

import (
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestAnalyzeBuildsFeatureMapAndRouteChains(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":           `{"name":"stocks","dependencies":{"next":"latest","react":"latest"}}`,
		"src/app/rules/page.tsx": `export default function RulesPage() { return null }`,
		"src/app/api/rules/route.ts": `import { RuleSchema } from "../../../lib/rules/schema";
import { saveRule } from "../../../lib/db/repositories";
export async function POST() { return Response.json({ ok: RuleSchema && saveRule }) }`,
		"src/lib/rules/schema.ts":             `export const RuleSchema = {}`,
		"src/lib/db/repositories.ts":          `export function saveRule() { return true }`,
		"src/lib/rules/evaluate.test.ts":      `import { RuleSchema } from "./schema"; test("rules", () => RuleSchema)`,
		".stackmap/reports/repo-report.md":    `generated`,
		".stackmap/history/old/analysis.json": `{}`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasAnalyzedPath(analysis.Files, ".stackmap/reports/repo-report.md") {
		t.Fatalf("legacy .stackmap artifact should be ignored: %#v", analysis.Files)
	}
	if analysis.Quality.IgnoredDirCounts[".stackmap"] == 0 {
		t.Fatalf("expected .stackmap ignore count in quality: %#v", analysis.Quality.IgnoredDirCounts)
	}
	if len(analysis.Features.Features) == 0 {
		t.Fatal("expected feature clusters")
	}
	if !hasFeatureStart(analysis.Features.Features, "Rules / alerts", "src/app/api/rules/route.ts") {
		t.Fatalf("expected rules feature with API start file: %#v", analysis.Features.Features)
	}
	if !hasRouteChainFile(analysis.Features.RouteChains, "POST /api/rules", "src/lib/rules/schema.ts") {
		t.Fatalf("expected route chain to include schema source: %#v", analysis.Features.RouteChains)
	}
	if !hasExportedSymbol(analysis.Symbols.Files, "src/lib/rules/schema.ts", "RuleSchema") {
		t.Fatalf("expected symbol index to include RuleSchema: %#v", analysis.Symbols.Files)
	}
}

func TestAnalyzeResolvesTSConfigAliasesInRouteChains(t *testing.T) {
	root := tempProject(t, map[string]string{
		"tsconfig.json": `{"compilerOptions":{"baseUrl":".","paths":{"@/*":["src/*"]}}}`,
		"package.json":  `{"name":"stocks","dependencies":{"next":"latest","react":"latest"}}`,
		"src/app/api/rules/route.ts": `import { RuleSchema } from "@/lib/rules/schema";
import { saveRule } from "@/lib/db/repositories";
export async function POST() { return Response.json({ ok: RuleSchema && saveRule }) }`,
		"src/lib/rules/schema.ts":    `export const RuleSchema = {}`,
		"src/lib/db/repositories.ts": `export function saveRule() { return true }`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Quality.InternalAliasImportsResolved != 2 {
		t.Fatalf("alias resolved count = %d, want 2; graph=%#v", analysis.Quality.InternalAliasImportsResolved, analysis.Dependencies)
	}
	if analysis.Quality.UnresolvedAliasImports != 0 {
		t.Fatalf("unexpected unresolved alias imports: %#v", analysis.Dependencies.UnresolvedImports)
	}
	if !hasDependencyEdge(analysis.Dependencies.Edges, "src/app/api/rules/route.ts", "src/lib/rules/schema.ts", "internal") {
		t.Fatalf("missing internal alias edge to schema: %#v", analysis.Dependencies.Edges)
	}
	if !hasRouteChainFile(analysis.Features.RouteChains, "POST /api/rules", "src/lib/rules/schema.ts") ||
		!hasRouteChainFile(analysis.Features.RouteChains, "POST /api/rules", "src/lib/db/repositories.ts") {
		t.Fatalf("expected route chain to include alias-resolved lib files: %#v", analysis.Features.RouteChains)
	}
}

func TestAnalyzeWarnsForUnresolvedAliasLookingImports(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":                   `{"name":"stocks","dependencies":{"next":"latest"}}`,
		"src/app/api/rules/route.ts":     `import { RuleSchema } from "@/missing/rules"; export async function POST() { return Response.json(RuleSchema) }`,
		"src/lib/rules/evaluate.ts":      `export const ok = true`,
		"src/lib/rules/evaluate.test.ts": `import { ok } from "./evaluate"; test("ok", () => ok)`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Quality.UnresolvedAliasImports != 1 {
		t.Fatalf("unresolved alias count = %d, want 1", analysis.Quality.UnresolvedAliasImports)
	}
	if !contains(analysis.Quality.Warnings, "Alias-looking imports were detected but could not be resolved from tsconfig/jsconfig paths or baseUrl.") {
		t.Fatalf("missing alias warning: %#v", analysis.Quality.Warnings)
	}
}

func TestAnalyzeFeatureMapKeepsWatchlistSeparateFromGenericSymbol(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":                         `{"name":"stocks","dependencies":{"next":"latest","react":"latest"}}`,
		"src/app/watchlist/page.tsx":           `export default function WatchlistPage() { return null }`,
		"src/app/api/watchlist/route.ts":       `export async function GET() { return Response.json([]) }`,
		"src/app/api/market/[symbol]/route.ts": `export async function GET() { return Response.json({}) }`,
		"src/lib/market/symbol-levels.ts":      `export const symbolLevels = []`,
		"src/lib/watchlist/repository.ts":      `export const watchlistRepository = {}`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFeatureNamed(analysis.Features.Features, "Watchlist") {
		t.Fatalf("expected watchlist feature: %#v", analysis.Features.Features)
	}
	if hasFeatureNamed(analysis.Features.Features, "Symbol") {
		t.Fatalf("generic Symbol feature should not dominate: %#v", analysis.Features.Features)
	}
}

func TestAnalyzeFeatureMapSuppressesGeneratedTauriSchemasAndGenericTerms(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":                             `{"name":"devflow","scripts":{"dev":"vite --host 127.0.0.1","dev:api":"node server.js","tauri":"tauri dev"},"dependencies":{"@tauri-apps/api":"latest","vite":"latest","react":"latest"}}`,
		"README.md":                                "# DevFlow\n\nDevFlow is a local-first developer dashboard and project tracker for coding sessions.",
		"src/App.tsx":                              `export default function App() { return null }`,
		"src/main.tsx":                             `import App from "./App";`,
		"server.js":                                `import express from "express"; const app = express(); app.get("/api/projects", (_req, res) => res.json([]));`,
		"src/lib/projects/store.ts":                `export const projects = []`,
		"src/lib/dashboard/widgets.ts":             `export const widgets = []`,
		"src-tauri/tauri.conf.json":                `{}`,
		"src-tauri/Cargo.toml":                     `[package]\nname = "devflow"`,
		"src-tauri/src/main.rs":                    `fn main() {}`,
		"src-tauri/capabilities/default.json":      `{}`,
		"src-tauri/gen/schemas/desktop.json":       `{}`,
		"src-tauri/gen/schemas/window-schema.json": `{}`,
		"scripts/check-env.mjs":                    `console.log("check")`,
		"generated/schema/tooling.json":            `{}`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Gen", "Generated", "Schema", "Script", "Mjs", "Json"} {
		if hasFeatureNamed(analysis.Features.Features, name) {
			t.Fatalf("generated/generic feature %q should not be compact primary feature: %#v", name, analysis.Features.Features)
		}
	}
	if !hasFeatureNamed(analysis.Features.Features, "Project") && !hasFeatureNamed(analysis.Features.Features, "Dashboard") {
		t.Fatalf("expected a useful domain feature, got %#v", analysis.Features.Features)
	}
	if analysis.Features.Quality.SuppressedCount == 0 {
		t.Fatalf("expected suppressed feature candidates, quality=%+v", analysis.Features.Quality)
	}
}

func TestAnalyzeFeatureMapWarnsWhenOnlyGeneratedGenericCandidatesRemain(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":                      `{"name":"tiny","scripts":{"dev":"vite"}}`,
		"src-tauri/gen/schemas/config.json": `{}`,
		"generated/schema/tooling.json":     `{}`,
		"scripts/start.mjs":                 `console.log("start")`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Features.Quality.Confidence != "low" {
		t.Fatalf("Feature quality = %+v, want low", analysis.Features.Quality)
	}
}

func hasAnalyzedPath(files []models.FileInfo, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func hasFeatureStart(features []models.FeatureCluster, name, path string) bool {
	for _, feature := range features {
		if feature.Name != name {
			continue
		}
		for _, item := range feature.StartHere {
			if item == path {
				return true
			}
		}
	}
	return false
}

func hasFeatureNamed(features []models.FeatureCluster, name string) bool {
	for _, feature := range features {
		if feature.Name == name {
			return true
		}
	}
	return false
}

func hasRouteChainFile(chains []models.RouteChain, route, path string) bool {
	for _, chain := range chains {
		if chain.Route != route {
			continue
		}
		for _, item := range chain.Files {
			if item == path {
				return true
			}
		}
	}
	return false
}

func hasExportedSymbol(files []models.FileSymbols, path, name string) bool {
	for _, file := range files {
		if file.Path != path {
			continue
		}
		for _, symbol := range file.Symbols {
			if symbol.Name == name {
				return true
			}
		}
	}
	return false
}
