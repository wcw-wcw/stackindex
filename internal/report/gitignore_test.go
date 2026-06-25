package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGeneratedArtifactsIgnoredCreatesGitignore(t *testing.T) {
	root := t.TempDir()
	if err := EnsureGeneratedArtifactsIgnored(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != ".stackindex/\n" {
		t.Fatalf(".gitignore = %q", string(data))
	}
}

func TestEnsureGeneratedArtifactsIgnoredAppendsOnce(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGeneratedArtifactsIgnored(root); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGeneratedArtifactsIgnored(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, ".stackindex/") != 1 {
		t.Fatalf(".gitignore did not contain exactly one .stackindex entry: %q", content)
	}
	if content != "node_modules/\n.stackindex/\n" {
		t.Fatalf(".gitignore = %q", content)
	}
}

func TestEnsureGeneratedArtifactsIgnoredAcceptsExistingVariant(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte("/.stackindex\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGeneratedArtifactsIgnored(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "/.stackindex\n" {
		t.Fatalf(".gitignore changed unexpectedly: %q", string(data))
	}
}
