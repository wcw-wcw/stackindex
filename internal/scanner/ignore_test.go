package scanner

import "testing"

func TestShouldIgnoreDir(t *testing.T) {
	for _, dir := range []string{".git", "node_modules", ".stackmap"} {
		if !ShouldIgnoreDir(dir) {
			t.Fatalf("expected %s to be ignored", dir)
		}
	}
	if ShouldIgnoreDir("src") {
		t.Fatal("src should not be ignored")
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
