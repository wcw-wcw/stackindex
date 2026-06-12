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
