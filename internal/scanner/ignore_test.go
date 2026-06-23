package scanner

import "testing"

func TestShouldIgnoreDir(t *testing.T) {
	for _, dir := range []string{".git", "node_modules", ".stackindex", ".stackmap", ".codex", ".claude", ".gocache", ".turbo", ".next", "dist", "build", "coverage"} {
		if !ShouldIgnoreDir(dir) {
			t.Fatalf("expected %s to be ignored", dir)
		}
	}
	if ShouldIgnoreDir("src") {
		t.Fatal("src should not be ignored")
	}
}

func TestClassifySchemaSourceAsSource(t *testing.T) {
	language, kind := Classify("src/lib/rules/schema.ts")
	if language != "TypeScript" || kind != "source" {
		t.Fatalf("schema.ts classified as %s/%s, want TypeScript/source", language, kind)
	}
}

func TestShouldIgnoreFileEnvSafety(t *testing.T) {
	if !ShouldIgnoreFile(".env") {
		t.Fatal(".env should be ignored")
	}
	if ShouldIgnoreFile(".env.example") {
		t.Fatal(".env.example should be scannable")
	}
}
