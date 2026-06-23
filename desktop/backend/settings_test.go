package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopSettingsDefaultsWhenMissing(t *testing.T) {
	session := &Session{settingsPath: filepath.Join(t.TempDir(), "StackIndex", "settings.json")}

	settings, err := session.GetDesktopSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.DefaultRunAudit || settings.DefaultUseAI || settings.DefaultModel != "" {
		t.Fatalf("unexpected default settings: %#v", settings)
	}
}

func TestDesktopSettingsSaveAndRead(t *testing.T) {
	session := &Session{settingsPath: filepath.Join(t.TempDir(), "StackIndex", "settings.json")}

	saved, err := session.SaveDesktopSettings(DesktopSettings{
		DefaultRunAudit: false,
		DefaultUseAI:    true,
		DefaultModel:    "  llama3.2  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.DefaultModel != "llama3.2" {
		t.Fatalf("expected trimmed model, got %#v", saved)
	}

	read, err := session.GetDesktopSettings()
	if err != nil {
		t.Fatal(err)
	}
	if read.DefaultRunAudit || !read.DefaultUseAI || read.DefaultModel != "llama3.2" {
		t.Fatalf("unexpected persisted settings: %#v", read)
	}
}

func TestDesktopPathsUseSessionOverrides(t *testing.T) {
	base := t.TempDir()
	session := &Session{
		recentProjectsPath: filepath.Join(base, "StackIndex", "recent-projects.json"),
		settingsPath:       filepath.Join(base, "StackIndex", "settings.json"),
		githubCacheRoot:    filepath.Join(base, "StackIndex", "repos", "github.com"),
	}

	paths, err := session.GetDesktopPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.RecentProjectsPath != session.recentProjectsPath || paths.SettingsPath != session.settingsPath || paths.GitHubCacheRoot != session.githubCacheRoot {
		t.Fatalf("unexpected desktop paths: %#v", paths)
	}
}

func TestClearGitHubCacheOnlyClearsSafeRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "StackIndex", "repos", "github.com")
	keep := filepath.Join(base, "StackIndex", "repos", "not-github", "keep.txt")
	remove := filepath.Join(root, "owner", "repo", "file.txt")
	if err := os.MkdirAll(filepath.Dir(remove), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remove, []byte("cached"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(keep), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	session := &Session{githubCacheRoot: root}
	if err := session.ClearGitHubCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(remove); !os.IsNotExist(err) {
		t.Fatalf("expected cached repo file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("expected cache root to be recreated: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("expected sibling path to remain: %v", err)
	}
}

func TestClearGitHubCacheRejectsUnsafeRoot(t *testing.T) {
	session := &Session{githubCacheRoot: filepath.Join(t.TempDir(), "github.com")}

	if err := session.ClearGitHubCache(); err == nil {
		t.Fatal("expected unsafe cache root error")
	}
}

func TestGitHubRepoCachePathUsesSessionRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	session := &Session{githubCacheRoot: root}

	got, err := session.githubRepoCachePath("owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "owner", "repo")
	if got != want {
		t.Fatalf("repo cache path = %q, want %q", got, want)
	}
}
