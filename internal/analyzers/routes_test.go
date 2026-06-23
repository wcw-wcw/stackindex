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
