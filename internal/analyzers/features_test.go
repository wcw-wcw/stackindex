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

func TestAnalyzeFeatureMapPrefersMountedRouteDomainsOverActions(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json": `{"name":"social","dependencies":{"express":"latest"}}`,
		"backend/server.js": `
const posts = require("./routes/posts");
const users = require("./routes/users");
const notifications = require("./routes/notifications");
const search = require("./routes/search");
const hashtags = require("./routes/hashtags");
app.use("/api/posts", posts);
app.use("/api/users", users);
app.use("/api/notifications", notifications);
app.use("/api/search", search);
app.use("/api/hashtags", hashtags);
`,
		"backend/routes/posts.js":         `router.get("/all", h); router.post("/:id/read", h); router.post("/:id/repost", h);`,
		"backend/routes/users.js":         `router.get("/:id/followers", h); router.post("/:id/follow", h);`,
		"backend/routes/notifications.js": `router.get("/count", h);`,
		"backend/routes/search.js":        `router.get("/", h);`,
		"backend/routes/hashtags.js":      `router.get("/:tag", h);`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Posts", "Users", "Notifications", "Search", "Hashtags"} {
		if !hasFeatureNamed(analysis.Features.Features, want) {
			t.Fatalf("missing domain feature %q from %#v", want, analysis.Features.Features)
		}
	}
	for _, noisy := range []string{"All", "Count", "Read", "Follow", "Following", "Follower", "Id"} {
		if hasFeatureNamed(analysis.Features.Features, noisy) {
			t.Fatalf("action/generic feature %q should not dominate: %#v", noisy, analysis.Features.Features)
		}
	}
}

func TestAnalyzeFeatureMapSuppressesFastAPIPathParams(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md":           "# BoardArena\n\nConnect Four game sessions with AI move generation.",
		"backend/app/main.py": `from routes.games import router; app.include_router(router, prefix="/api")`,
		"backend/app/routes/games.py": `from fastapi import APIRouter
router = APIRouter(prefix="/games")
@router.post("/{game_id}/moves/human")
def human_move(game_id: str): return {}`,
		"frontend/src/App.tsx": `export default function GameBoard() { return null }`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFeatureNamed(analysis.Features.Features, "Games") || !hasFeatureNamed(analysis.Features.Features, "AI moves") {
		t.Fatalf("expected game route features: %#v", analysis.Features.Features)
	}
	for _, noisy := range []string{"Game", "Game_id", "Id", "Human"} {
		if hasFeatureNamed(analysis.Features.Features, noisy) {
			t.Fatalf("path-param/action feature %q should not appear: %#v", noisy, analysis.Features.Features)
		}
	}
}

func TestAnalyzeFeatureMapDetectsRobloxGameplaySystems(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md":            "# BladeRivals\n\nRoblox melee arena combat with abilities, movement, HUD, remotes, and weapon definitions.",
		"default.project.json": `{}`,
		"src/client/controllers/CombatController.luau":   `return {}`,
		"src/client/controllers/HUDController.luau":      `return {}`,
		"src/client/controllers/MovementController.luau": `return {}`,
		"src/client/Viewmodel.luau":                      `return {}`,
		"src/server/services/MatchFlowService.luau":      `return {}`,
		"src/server/services/TrainingDummyService.luau":  `return {}`,
		"src/shared/remotes/CombatRemotes.luau":          `return {}`,
		"src/shared/weapons/WeaponDefinitions.luau":      `return {}`,
		"src/shared/effects/HitEffects.luau":             `return {}`,
		"src/server/services/InventoryService.luau":      `return {}`,
		"src/server/services/LobbyService.luau":          `return {}`,
		"src/server/services/ArenaService.luau":          `return {}`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Combat", "Movement", "HUD", "Viewmodel", "Match flow", "Training dummy", "Remotes", "Weapon definitions"} {
		if !hasFeatureNamed(analysis.Features.Features, want) {
			t.Fatalf("missing Roblox feature %q from %#v", want, analysis.Features.Features)
		}
	}
	if !contains(analysis.Stack.Languages, "Luau") || !contains(analysis.Stack.Frameworks, "Rojo") || !contains(analysis.Stack.Frameworks, "Roblox") {
		t.Fatalf("missing Luau/Rojo stack detection: %+v", analysis.Stack)
	}
	if analysis.Context.Purpose != "Roblox melee arena" {
		t.Fatalf("Purpose = %q, want Roblox melee arena", analysis.Context.Purpose)
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
