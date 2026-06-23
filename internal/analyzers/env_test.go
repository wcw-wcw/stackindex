package analyzers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
)

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

func TestAnalyzeEnvDoesNotWarnForPlatformBuildVars(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "app.ts")
	if err := os.WriteFile(sourcePath, []byte(`console.log(process.env.NODE_ENV, process.env.VERCEL_GIT_COMMIT_SHA, process.env.BUILD_TIME)`), 0644); err != nil {
		t.Fatal(err)
	}
	env, findings := AnalyzeEnv(root, []models.FileInfo{{Path: "app.ts", Kind: models.FileKindSource}})
	if len(env.MissingFromExample) != 3 {
		t.Fatalf("expected detected missing vars to stay visible, got %#v", env.MissingFromExample)
	}
	if len(env.MissingRequiredFromExample) != 0 {
		t.Fatalf("expected no required missing vars, got %#v", env.MissingRequiredFromExample)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no env findings for platform/build vars, got %#v", findings)
	}
}
