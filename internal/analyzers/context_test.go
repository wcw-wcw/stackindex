package analyzers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestParseReadmeContextExtractsTitleSummaryAndSignals(t *testing.T) {
	ctx := parseReadmeContext(`# AnimeRec

[![build](badge.svg)](x)

AnimeRec helps people discover anime recommendations from MAL catalog data.

` + "```" + `
npm install
` + "```" + `

## Features

Anime catalog search and anime similarity recommendations.
`)
	if ctx.Title != "AnimeRec" {
		t.Fatalf("Title = %q, want AnimeRec", ctx.Title)
	}
	if !strings.Contains(ctx.Summary, "discover anime recommendations") {
		t.Fatalf("Summary did not include useful first paragraph: %#v", ctx)
	}
	if !contains(ctx.Headings, "Features") {
		t.Fatalf("Headings missing Features: %#v", ctx.Headings)
	}
	if !contains(ctx.Terms, "anime") {
		t.Fatalf("Terms missing repeated anime signal: %#v", ctx.Terms)
	}
}

func TestAnalyzeProjectContextInfersStockMonitoringPurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# Stock Watcher

Stock monitoring and alerting for SPY watchlists and market rules.`,
		"package.json":                       `{"name":"stkapp","description":"Stock monitoring alerts","scripts":{"worker":"node scripts/worker.mjs","health:check":"node scripts/health.mjs"},"dependencies":{"next":"latest","pg":"latest"}}`,
		"src/app/api/market/route.ts":        `export function GET() {}`,
		"src/app/api/rules/route.ts":         `export function POST() {}`,
		"src/app/api/watchlist/route.ts":     `export function GET() {}`,
		"src/app/api/notifications/route.ts": `export function GET() {}`,
		".env.example":                       "ALPACA_API_KEY=\nDATABASE_URL=\n",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Stock monitoring and alerting application" || analysis.Context.Confidence != "high" {
		t.Fatalf("Context = %+v, want high-confidence stock purpose", analysis.Context)
	}
	if len(analysis.Context.Evidence) == 0 {
		t.Fatalf("expected purpose evidence")
	}
}

func TestAnalyzeProjectContextInfersAnimeRecommendationPurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md":               "# AnimeRec\n\nAnime recommendation and discovery app using MAL catalog data.",
		"package.json":            `{"name":"animerec","description":"Anime discovery recommendations","scripts":{"sync:mal":"node scripts/sync-mal.mjs"},"dependencies":{"vite":"latest","react":"latest"}}`,
		"src/lib/mal.ts":          "export const MAL = true",
		"scripts/sync-mal.mjs":    "console.log('mal')",
		"src/components/Card.tsx": "export function Card() { return null }",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Anime recommendation/discovery application" {
		t.Fatalf("Purpose = %q", analysis.Context.Purpose)
	}
}

func TestAnalyzeProjectContextInfersAssessmentEducationPurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# AssessmentHub

AssessmentHub is an assessment-taking app for students to complete multiple-choice assessments, submit answers, and view scores while teachers review submissions and grading outcomes.`,
		"package.json":                     `{"name":"assessmenthub","description":"Assessment-taking app for students and teachers","dependencies":{"next":"latest","react":"latest"}}`,
		"src/app/api/submissions/route.ts": `export async function POST() {}`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Assessment/education application" {
		t.Fatalf("Purpose = %q", analysis.Context.Purpose)
	}
	if analysis.Context.Confidence != "high" {
		t.Fatalf("Confidence = %q, want high", analysis.Context.Confidence)
	}
}

func TestAnalyzeProjectContextInfersBoardGamePurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# BoardArena

BoardArena is a local-first AI board-game arena built with React, Vite, TypeScript, and FastAPI. It supports Connect Four game sessions, legal move validation, and AI move generation.`,
		"frontend/package.json":    `{"dependencies":{"react":"latest"},"devDependencies":{"vite":"latest","typescript":"latest"}}`,
		"backend/requirements.txt": "fastapi\n",
		"backend/app/main.py":      "from fastapi import FastAPI\napp = FastAPI()\n",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Board-game/game arena application" {
		t.Fatalf("Purpose = %q", analysis.Context.Purpose)
	}
	for _, want := range []string{"React", "Vite", "FastAPI"} {
		if !contains(analysis.Stack.Frameworks, want) {
			t.Fatalf("Frameworks = %#v, want %s", analysis.Stack.Frameworks, want)
		}
	}
}

func TestAnalyzeProjectContextInfersPortfolioPurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# Will West Portfolio

A personal portfolio built with Next.js, TypeScript, Tailwind CSS, and Vercel to showcase projects, case studies, resume details, and contact links for recruiters.`,
		"package.json": `{"name":"folio","description":"Personal portfolio site","dependencies":{"next":"latest","react":"latest"},"devDependencies":{"typescript":"latest","tailwindcss":"latest"}}`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Portfolio/personal site" {
		t.Fatalf("Purpose = %q", analysis.Context.Purpose)
	}
}

func TestAnalyzeProjectContextPrefersDevFlowDashboardOverPortfolio(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# DevFlow

DevFlow is a local-first developer dashboard and project tracker for coding sessions, tasks, notes, and local workflow state. It is a desktop productivity app, not a portfolio or resume site.`,
		"package.json":              `{"name":"devflow","description":"Local-first developer dashboard","scripts":{"dev":"vite","dev:api":"node server.js","tauri":"tauri dev"},"dependencies":{"@tauri-apps/api":"latest","vite":"latest","react":"latest","express":"latest"}}`,
		"src/App.tsx":               "export default function App() { return null }",
		"server.js":                 `const express = require("express"); const app = express(); app.get("/api/projects", (_req, res) => res.json([]));`,
		"src-tauri/tauri.conf.json": `{}`,
		"src-tauri/Cargo.toml":      `[package]\nname = "devflow"`,
		"src-tauri/src/main.rs":     "fn main() {}",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Local-first developer dashboard" {
		t.Fatalf("Purpose = %q, want local-first developer dashboard; context=%+v", analysis.Context.Purpose, analysis.Context)
	}
	if strings.Contains(strings.ToLower(analysis.Context.Purpose), "portfolio") {
		t.Fatalf("DevFlow-style README was classified as portfolio: %+v", analysis.Context)
	}
}

func TestAnalyzeProjectContextInfersStackIndexPurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md":                     "# StackIndex\n\nStackIndex is a Go CLI/TUI repository analysis tool that scans codebases, runs audit checks, and writes Markdown/JSON reports.",
		"go.mod":                        "module github.com/wcw-wcw/stackindex\n\ngo 1.22\n",
		"cmd/stackindex/main.go":        "package main\nfunc main() {}",
		"internal/analyzers/analyze.go": "package analyzers\n",
		"internal/tui/app.go":           "package tui\n",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Go CLI/TUI repository analysis tool" {
		t.Fatalf("Purpose = %q", analysis.Context.Purpose)
	}
	if analysis.PackageInfo == nil || analysis.PackageInfo.ModuleName != "github.com/wcw-wcw/stackindex" {
		t.Fatalf("Go module metadata missing: %+v", analysis.PackageInfo)
	}
}

func TestAnalyzeProjectContextWeakSignalsDoNotInferSpecificWrongPurpose(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md":              "# Utilities\n\nA small web application for internal helper workflows.",
		"package.json":           `{"name":"helpers","scripts":{"seed":"node scripts/seed.js"},"dependencies":{"react":"latest"}}`,
		"scripts/market-sync.js": "console.log('market')",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose == "Stock monitoring and alerting application" {
		t.Fatalf("weak support-only stock signal produced specific purpose: %+v", analysis.Context)
	}
	if analysis.Context.Purpose != "General web application" && analysis.Context.Purpose != "Frontend web application" && analysis.Context.Purpose != "Unknown project purpose" {
		t.Fatalf("Purpose = %q, want generic or unknown", analysis.Context.Purpose)
	}
}

func TestAnalyzeProjectContextReadmeDominatesWeakUnrelatedSignals(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# Portfolio

Personal portfolio and project case studies for recruiters, with resume and contact links.`,
		"package.json":      `{"name":"folio","scripts":{"sync:market":"node scripts/market.js"},"dependencies":{"next":"latest"}}`,
		"scripts/market.js": "console.log('stock market')",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Portfolio/personal site" {
		t.Fatalf("Purpose = %q, want portfolio despite unrelated script signal", analysis.Context.Purpose)
	}
}

func TestAnalyzeProjectContextSocialPurposeStillWorks(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": `# Twt

A Twitter-style social app with posts, reposts, follows, followers, timeline views, hashtags, mentions, and user profiles.`,
		"package.json": `{"name":"twt","description":"Social posting app","dependencies":{"express":"latest","react":"latest"}}`,
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Twitter-style social application" {
		t.Fatalf("Purpose = %q", analysis.Context.Purpose)
	}
}

func TestAnalyzeProjectContextInfersSpecificLocalProjectTaxonomy(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		purpose string
	}{
		{
			name: "reddit scraper",
			files: map[string]string{
				"README.md":    "# RSlashScrape\n\nA Reddit/subreddit scraper for collecting posts from r/all and target subreddits.",
				"package.json": `{"name":"rslashscrape","description":"Subreddit scraper"}`,
			},
			purpose: "Reddit/subreddit scraper",
		},
		{
			name: "riot tracker",
			files: map[string]string{
				"README.md":    "# RGTracker\n\nRiot League of Legends and Valorant stats tracker for summoner match history.",
				"package.json": `{"name":"rgtracker","description":"Riot stats tracker"}`,
			},
			purpose: "Riot/League/Valorant stats tracker",
		},
		{
			name: "opgg dashboard",
			files: map[string]string{
				"README.md":    "# OPGG\n\nAn OP.GG-style stats dashboard for League ranked stats and match history.",
				"package.json": `{"name":"opgg","description":"OP.GG-style stats dashboard"}`,
			},
			purpose: "OP.GG-style stats dashboard",
		},
		{
			name: "sorting visualizer",
			files: map[string]string{
				"README.md": "# Algo\n\nSorting visualizer for bubble sort, merge sort, and quick sort algorithms.",
			},
			purpose: "Sorting visualizer",
		},
		{
			name: "face desktop",
			files: map[string]string{
				"README.md": "# DFGen\n\nFace-swap / face detection desktop app using OpenCV face landmarks.",
			},
			purpose: "Face-swap / face detection desktop app",
		},
		{
			name: "card game",
			files: map[string]string{
				"README.md": "# Slay The Spire MVP\n\nSlay-the-Spire-like card game with deckbuilder combat, relics, and encounters.",
			},
			purpose: "Slay-the-Spire-like card game",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tempProject(t, tc.files)
			analysis, err := Analyze(root)
			if err != nil {
				t.Fatal(err)
			}
			if analysis.Context.Purpose != tc.purpose {
				t.Fatalf("Purpose = %q, want %q; context=%+v", analysis.Context.Purpose, tc.purpose, analysis.Context)
			}
		})
	}
}

func TestAnalyzeProjectContextDoesNotMisclassifyRedditOrRiotAsTwitterSocial(t *testing.T) {
	for _, files := range []map[string]string{
		{
			"README.md":    "# Reddit Posts Exporter\n\nScrape subreddit posts and profiles for local analysis.",
			"package.json": `{"name":"reddit-posts","description":"Reddit post scraper"}`,
		},
		{
			"README.md":    "# Riot Profiles\n\nLeague profiles, posts, and match history for Riot players.",
			"package.json": `{"name":"riot-profiles","description":"League player stats"}`,
		},
	} {
		root := tempProject(t, files)
		analysis, err := Analyze(root)
		if err != nil {
			t.Fatal(err)
		}
		if analysis.Context.Purpose == "Twitter-style social application" {
			t.Fatalf("misclassified as Twitter-style social: %+v", analysis.Context)
		}
	}
}

func TestAnalyzeProjectContextUnknownFallback(t *testing.T) {
	root := tempProject(t, map[string]string{
		"README.md": "# Utilities\n\nA collection of small helpers.",
		"main.go":   "package main\n",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Context.Purpose != "Unknown project purpose" || analysis.Context.Confidence != "low" {
		t.Fatalf("Context = %+v, want unknown low fallback", analysis.Context)
	}
}

func TestAnalyzeStructureMapDetectsDirectoryAndKeyFileRoles(t *testing.T) {
	files := []models.FileInfo{
		{Path: "package.json", Kind: models.FileKindConfig},
		{Path: "go.mod", Kind: models.FileKindConfig},
		{Path: "README.md", Kind: models.FileKindDoc},
		{Path: ".env.example", Kind: models.FileKindConfig},
		{Path: "src/app/api/health/route.ts", Kind: models.FileKindSource},
		{Path: "src/components/Button.tsx", Kind: models.FileKindSource},
		{Path: "src/lib/db.ts", Kind: models.FileKindSource},
		{Path: "scripts/worker.mjs", Kind: models.FileKindSource},
		{Path: "db/migrations/001_init.sql", Kind: models.FileKindConfig},
		{Path: "cmd/stackindex/main.go", Kind: models.FileKindSource},
	}
	structure := AnalyzeStructureMap(files, []models.RouteInfo{{Method: "GET", Path: "/api/health", SourceFile: "src/app/api/health/route.ts"}})
	if !hasDirectoryRole(structure.Directories, "src/app/api/", "Next.js API route handlers") {
		t.Fatalf("missing src/app/api role: %+v", structure.Directories)
	}
	if !hasDirectoryRole(structure.Directories, "cmd/", "CLI entrypoints") {
		t.Fatalf("missing cmd role: %+v", structure.Directories)
	}
	if !hasFileRole(structure.KeyFiles, "src/app/api/health/route.ts", "Health endpoint implementation") {
		t.Fatalf("missing health key file: %+v", structure.KeyFiles)
	}
	if !hasFileRole(structure.KeyFiles, "cmd/stackindex/main.go", "Main CLI entrypoint") {
		t.Fatalf("missing CLI key file: %+v", structure.KeyFiles)
	}
}

func TestAnalyzeStructureMapDetectsSplitFrontendBackendRoles(t *testing.T) {
	files := []models.FileInfo{
		{Path: "frontend/package.json", Kind: models.FileKindConfig},
		{Path: "frontend/src/main.tsx", Kind: models.FileKindSource},
		{Path: "backend/requirements.txt", Kind: models.FileKindConfig},
		{Path: "backend/app/main.py", Kind: models.FileKindSource},
		{Path: "backend/app/api/games.py", Kind: models.FileKindSource},
		{Path: "backend/routes/users.js", Kind: models.FileKindSource},
	}
	structure := AnalyzeStructureMap(files, nil)
	for _, want := range []struct {
		path string
		role string
	}{
		{"frontend/", "Frontend app"},
		{"frontend/src/", "Frontend source code"},
		{"backend/", "Backend service"},
		{"backend/app/", "Backend application code"},
		{"backend/app/api/", "Backend API routes"},
		{"backend/routes/", "Backend API routes"},
	} {
		if !hasDirectoryRole(structure.Directories, want.path, want.role) {
			t.Fatalf("missing split app directory role %+v from %+v", want, structure.Directories)
		}
	}
}

func TestAnalyzeStructureMapIncludesAnimerecStyleDirectories(t *testing.T) {
	files := []models.FileInfo{
		{Path: "src/App.tsx", Kind: models.FileKindSource},
		{Path: "src/main.tsx", Kind: models.FileKindSource},
		{Path: "api/anime/lookup.js", Kind: models.FileKindSource},
		{Path: "database/migrations/001_init.sql", Kind: models.FileKindConfig},
		{Path: "scripts/catalog-api.mjs", Kind: models.FileKindSource},
		{Path: "docs/deployment-checklist.md", Kind: models.FileKindDoc},
	}
	structure := AnalyzeStructureMap(files, []models.RouteInfo{
		{Method: "ANY", Path: "/api/anime/lookup", SourceFile: "api/anime/lookup.js", Confidence: "medium"},
		{Method: "LOCAL", Path: "/scripts/catalog-api", SourceFile: "scripts/catalog-api.mjs", Confidence: "low"},
	})
	for _, want := range []struct {
		path string
		role string
	}{
		{"src/", "Frontend/source application code"},
		{"api/", "Serverless/API functions"},
		{"database/migrations/", "Database schema migration files"},
		{"scripts/", "Operational scripts/tooling"},
		{"docs/", "Documentation"},
	} {
		if !hasDirectoryRole(structure.Directories, want.path, want.role) {
			t.Fatalf("missing directory role %+v from %+v", want, structure.Directories)
		}
	}
	if !hasFileRole(structure.KeyFiles, "api/anime/lookup.js", "Serverless/API function") {
		t.Fatalf("missing API function key file: %+v", structure.KeyFiles)
	}
	if !hasFileRole(structure.KeyFiles, "docs/deployment-checklist.md", "Deployment documentation") {
		t.Fatalf("missing deployment doc key file: %+v", structure.KeyFiles)
	}
}

func TestAnalyzeStructureMapDetectsTauriAndRootServerRoles(t *testing.T) {
	files := []models.FileInfo{
		{Path: "package.json", Kind: models.FileKindConfig},
		{Path: "README.md", Kind: models.FileKindDoc},
		{Path: "src/App.tsx", Kind: models.FileKindSource},
		{Path: "src/main.tsx", Kind: models.FileKindSource},
		{Path: "server.js", Kind: models.FileKindSource},
		{Path: "src-tauri/tauri.conf.json", Kind: models.FileKindConfig},
		{Path: "src-tauri/Cargo.toml", Kind: models.FileKindConfig},
		{Path: "src-tauri/src/main.rs", Kind: models.FileKindSource},
		{Path: "src-tauri/capabilities/default.json", Kind: models.FileKindConfig},
		{Path: "src-tauri/gen/schemas/window.json", Kind: models.FileKindConfig},
	}
	structure := AnalyzeStructureMap(files, nil)
	for _, want := range []struct {
		path string
		role string
	}{
		{"./", "Root local Node backend/server"},
		{"src-tauri/src/", "Tauri/Rust backend code"},
		{"src-tauri/capabilities/", "Tauri permissions/config"},
	} {
		if !hasDirectoryRole(structure.Directories, want.path, want.role) {
			t.Fatalf("missing directory role %+v from %+v", want, structure.Directories)
		}
	}
	for _, want := range []struct {
		path string
		role string
	}{
		{"server.js", "Local Node backend/server entrypoint"},
		{"src/App.tsx", "Frontend application entrypoint"},
		{"src-tauri/src/main.rs", "Tauri/Rust backend entrypoint"},
		{"src-tauri/tauri.conf.json", "Tauri application config"},
	} {
		if !hasFileRole(structure.KeyFiles, want.path, want.role) {
			t.Fatalf("missing key file role %+v from %+v", want, structure.KeyFiles)
		}
	}
}

func TestAnalyzeStructureMapDetectsRojoRobloxRoles(t *testing.T) {
	files := []models.FileInfo{
		{Path: "default.project.json", Kind: models.FileKindConfig},
		{Path: "src/client/controllers/CombatController.luau", Kind: models.FileKindSource, Language: "Luau"},
		{Path: "src/server/services/MatchService.luau", Kind: models.FileKindSource, Language: "Luau"},
		{Path: "src/shared/remotes/CombatRemotes.luau", Kind: models.FileKindSource, Language: "Luau"},
	}
	structure := AnalyzeStructureMap(files, nil)
	for _, want := range []struct {
		path string
		role string
	}{
		{"src/client/", "Roblox client code"},
		{"src/client/controllers/", "Roblox client controllers"},
		{"src/server/", "Roblox server code"},
		{"src/server/services/", "Roblox server services"},
		{"src/shared/", "Roblox shared definitions/remotes/config"},
	} {
		if !hasDirectoryRole(structure.Directories, want.path, want.role) {
			t.Fatalf("missing Roblox directory role %+v from %+v", want, structure.Directories)
		}
	}
	if !hasFileRole(structure.KeyFiles, "default.project.json", "Rojo project config") {
		t.Fatalf("missing Rojo key file role: %+v", structure.KeyFiles)
	}
}

func TestAnalyzeDetectsRootServerFromPackageScriptsAndFileConnections(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json": `{"name":"local-api","scripts":{"dev:api":"node server.js","start":"node server.js"},"dependencies":{"express":"latest","vite":"latest","react":"latest"}}`,
		"README.md":    "# Local API\n\nA local-first developer dashboard.",
		"server.js": `const express = require("express");
const app = express();
const store = require("./src/lib/store.js");
app.get("/api/projects", (_req, res) => res.json(store.projects));`,
		"src/lib/store.js": `exports.projects = [];`,
		"src/App.tsx":      "export default function App() { return null }",
	})
	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFileRole(analysis.Structure.KeyFiles, "server.js", "Local Node backend/server entrypoint") {
		t.Fatalf("missing server.js key role: %+v", analysis.Structure.KeyFiles)
	}
	if !hasConnectedFile(analysis.Dependencies.TopConnectedFiles, "server.js") {
		t.Fatalf("server.js missing from file connections: %+v", analysis.Dependencies.TopConnectedFiles)
	}
	if !contains(analysis.Context.ScriptSignals, "dev:api") && !contains(analysis.Context.ScriptSignals, "start") {
		t.Fatalf("package scripts did not produce backend signal: %+v", analysis.Context.ScriptSignals)
	}
}

func TestAnalyzePackageExtractsPackageDescription(t *testing.T) {
	info, err := ParsePackageJSON([]byte(`{"name":"demo","description":"Demo app","scripts":{"build":"vite build"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Demo app" {
		t.Fatalf("Description = %q", info.Description)
	}
}

func TestAnalyzePackageIsMonorepoAndWailsRuntimeAware(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":                                  `{"name":"root","scripts":{"frontend:build":"npm --prefix frontend run build","backend:start":"node backend/server.js"},"dependencies":{"express":"latest","vite":"latest"}}`,
		"desktop/frontend/package.json":                 `{"scripts":{"build":"vite build","test":"vitest"},"dependencies":{"vite":"latest"},"devDependencies":{"vitest":"latest"}}`,
		"desktop/frontend/wailsjs/runtime/package.json": `{"name":"generated-runtime","scripts":{},"dependencies":{"left-pad":"latest"}}`,
		"go.mod": `module example.com/app`,
	})
	files := []models.FileInfo{
		{Path: "package.json", Kind: models.FileKindConfig},
		{Path: "desktop/frontend/package.json", Kind: models.FileKindConfig},
		{Path: "desktop/frontend/wailsjs/runtime/package.json", Kind: models.FileKindConfig},
		{Path: "go.mod", Kind: models.FileKindConfig},
	}
	info, findings, err := AnalyzePackage(root, files)
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Name != "root" || info.ModuleName != "example.com/app" {
		t.Fatalf("unexpected package info: %+v", info)
	}
	if _, ok := info.Dependencies["left-pad"]; ok {
		t.Fatalf("generated Wails runtime package metadata should be ignored: %+v", info.Dependencies)
	}
	for _, finding := range findings {
		if strings.Contains(finding.Message, "No build script") || strings.Contains(finding.Message, "no start script") {
			t.Fatalf("monorepo-prefixed scripts should avoid warning, findings=%+v", findings)
		}
	}
}

func tempProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func hasDirectoryRole(roles []models.DirectoryRole, path, role string) bool {
	for _, item := range roles {
		if item.Path == path && item.Role == role {
			return true
		}
	}
	return false
}

func hasFileRole(roles []models.FileRole, path, role string) bool {
	for _, item := range roles {
		if item.Path == path && item.Role == role {
			return true
		}
	}
	return false
}

func hasConnectedFile(files []models.ConnectedFileSummary, path string) bool {
	for _, item := range files {
		if item.Path == path {
			return true
		}
	}
	return false
}
