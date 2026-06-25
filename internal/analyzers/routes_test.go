package analyzers

import (
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestExtractExpressRoutes(t *testing.T) {
	got := ExtractExpressRoutes(`app.get("/api/health", handler); router.post("/:id/reply", h)`, "src/server.ts")
	if len(got) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(got))
	}
	if got[0].Method != "GET" || got[0].Path != "/api/health" {
		t.Fatalf("unexpected first route: %#v", got[0])
	}
}

func TestExtractNextRouteMethods(t *testing.T) {
	got := ExtractNextRouteMethods(`
export async function GET() {}
export const POST = async () => {}
const remove = async () => {}
export { remove as DELETE }
`)
	want := []string{"GET", "POST", "DELETE"}
	if len(got) != len(want) {
		t.Fatalf("expected %d methods, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("method %d: expected %s, got %s", i, want[i], got[i])
		}
	}
}

func TestAnalyzeRoutesDetectsVercelAPIFunction(t *testing.T) {
	root := tempProject(t, map[string]string{
		"api/anime/lookup.js": "export default function handler(req, res) { res.json({ ok: true }) }",
	})
	files := []models.FileInfo{{Path: "api/anime/lookup.js", Kind: models.FileKindSource}}

	got := AnalyzeRoutes(root, files)
	if len(got) != 1 {
		t.Fatalf("expected 1 route, got %d: %#v", len(got), got)
	}
	if got[0].Method != "ANY" || got[0].Path != "/api/anime/lookup" || got[0].Confidence != "medium" {
		t.Fatalf("unexpected Vercel API route: %#v", got[0])
	}
}

func TestAnalyzeRoutesDetectsLocalAPIScriptWithLowConfidence(t *testing.T) {
	root := tempProject(t, map[string]string{
		"scripts/catalog-api.mjs": "import http from 'node:http'",
	})
	files := []models.FileInfo{{Path: "scripts/catalog-api.mjs", Kind: models.FileKindSource}}

	got := AnalyzeRoutes(root, files)
	if len(got) != 1 {
		t.Fatalf("expected 1 route, got %d: %#v", len(got), got)
	}
	if got[0].Method != "LOCAL" || got[0].Path != "/scripts/catalog-api" || got[0].Confidence != "low" {
		t.Fatalf("unexpected local API script route: %#v", got[0])
	}
}

func TestAnalyzeRoutesReconstructsMountedExpressRouters(t *testing.T) {
	root := tempProject(t, map[string]string{
		"backend/server.js": `
const express = require("express");
const auth = require("./routes/auth");
const posts = require("./routes/posts");
const users = require("./routes/users");
const notifications = require("./routes/notifications");
const app = express();
app.use("/api/auth", auth);
app.use("/api/posts", posts);
app.use("/api/users", users);
app.use("/api/notifications", notifications);
`,
		"backend/routes/auth.js":          `const router = require("express").Router(); router.post("/login", login); module.exports = router;`,
		"backend/routes/posts.js":         `const router = require("express").Router(); router.get("/", list); router.post("/:id/read", read); module.exports = router;`,
		"backend/routes/users.js":         `const router = require("express").Router(); router.get("/:id/profile", show); module.exports = router;`,
		"backend/routes/notifications.js": `const router = require("express").Router(); router.get("/", list); module.exports = router;`,
	})
	files := []models.FileInfo{
		{Path: "backend/server.js", Kind: models.FileKindSource},
		{Path: "backend/routes/auth.js", Kind: models.FileKindSource},
		{Path: "backend/routes/posts.js", Kind: models.FileKindSource},
		{Path: "backend/routes/users.js", Kind: models.FileKindSource},
		{Path: "backend/routes/notifications.js", Kind: models.FileKindSource},
	}

	got := AnalyzeRoutes(root, files)
	for _, want := range []string{
		"POST /api/auth/login",
		"GET /api/posts",
		"POST /api/posts/:id/read",
		"GET /api/users/:id/profile",
		"GET /api/notifications",
	} {
		if !hasRouteLabel(got, want) {
			t.Fatalf("missing mounted route %q from %#v", want, got)
		}
	}
	if hasRouteLabel(got, "GET /") {
		t.Fatalf("unmounted root route leaked into results: %#v", got)
	}
}

func TestAnalyzeRoutesReconstructsFastAPIRouterPrefixes(t *testing.T) {
	root := tempProject(t, map[string]string{
		"backend/app/main.py": `
from fastapi import FastAPI
from routes.games import router
app = FastAPI()
app.include_router(router, prefix="/api")
`,
		"backend/app/routes/games.py": `
from fastapi import APIRouter
router = APIRouter(prefix="/games")
@router.post("/{game_id}/moves/human")
def human_move(game_id: str):
    return {}
`,
	})
	files := []models.FileInfo{
		{Path: "backend/app/main.py", Kind: models.FileKindSource, Language: "Python"},
		{Path: "backend/app/routes/games.py", Kind: models.FileKindSource, Language: "Python"},
	}

	got := AnalyzeRoutes(root, files)
	if !hasRouteLabel(got, "POST /api/games/{game_id}/moves/human") {
		t.Fatalf("missing prefixed FastAPI route: %#v", got)
	}
}

func hasRouteLabel(routes []models.RouteInfo, label string) bool {
	for _, route := range routes {
		if route.Method+" "+route.Path == label {
			return true
		}
	}
	return false
}
