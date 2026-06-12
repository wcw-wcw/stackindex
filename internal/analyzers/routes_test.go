package analyzers

import "testing"

func TestExtractExpressRoutes(t *testing.T) {
	got := ExtractExpressRoutes(`app.get("/api/health", handler); router.post("/:id/reply", h)`, "src/server.ts")
	if len(got) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(got))
	}
	if got[0].Method != "GET" || got[0].Path != "/api/health" {
		t.Fatalf("unexpected first route: %#v", got[0])
	}
}
