package analyzers

import "testing"

func TestExtractEnvVars(t *testing.T) {
	content := `
const a = process.env.DATABASE_URL
const b = import.meta.env.VITE_API_URL
const c = Deno.env.get("DENO_TOKEN")
secret := os.Getenv("GO_SECRET")
`
	got := ExtractEnvVars(content)
	want := map[string]bool{"DATABASE_URL": true, "VITE_API_URL": true, "DENO_TOKEN": true, "GO_SECRET": true}
	if len(got) != len(want) {
		t.Fatalf("expected %d vars, got %d: %#v", len(want), len(got), got)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unexpected env var %s", name)
		}
	}
}
