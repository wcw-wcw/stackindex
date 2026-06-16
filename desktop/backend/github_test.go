package backend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseGitHubRepoURLValidWithoutGit(t *testing.T) {
	repo, err := parseGitHubRepoURL("https://github.com/owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.Owner != "owner" || repo.Repo != "repo" {
		t.Fatalf("unexpected owner/repo: %#v", repo)
	}
	if repo.CanonicalCloneURL != "https://github.com/owner/repo.git" {
		t.Fatalf("unexpected clone URL: %s", repo.CanonicalCloneURL)
	}
	if !strings.Contains(repo.LocalCachePath, filepath.Join("StackMap", "repos", "github.com", "owner", "repo")) {
		t.Fatalf("unexpected cache path: %s", repo.LocalCachePath)
	}
}

func TestParseGitHubRepoURLValidWithGit(t *testing.T) {
	repo, err := parseGitHubRepoURL("https://github.com/owner/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if repo.Repo != "repo" || repo.CanonicalCloneURL != "https://github.com/owner/repo.git" {
		t.Fatalf("unexpected repo: %#v", repo)
	}
}

func TestParseGitHubRepoURLTrailingSlash(t *testing.T) {
	repo, err := parseGitHubRepoURL("https://github.com/owner/repo/")
	if err != nil {
		t.Fatal(err)
	}
	if repo.Owner != "owner" || repo.Repo != "repo" {
		t.Fatalf("unexpected repo: %#v", repo)
	}
}

func TestParseGitHubRepoURLRejectsInvalidHost(t *testing.T) {
	_, err := parseGitHubRepoURL("https://example.com/owner/repo")
	if err == nil {
		t.Fatal("expected invalid host error")
	}
}

func TestParseGitHubRepoURLRejectsSSH(t *testing.T) {
	_, err := parseGitHubRepoURL("git@github.com:owner/repo.git")
	if err == nil {
		t.Fatal("expected SSH URL error")
	}
}

func TestParseGitHubRepoURLRejectsCredentials(t *testing.T) {
	_, err := parseGitHubRepoURL("https://token@github.com/owner/repo")
	if err == nil {
		t.Fatal("expected credential URL error")
	}
}

func TestParseGitHubRepoURLRejectsMissingRepo(t *testing.T) {
	_, err := parseGitHubRepoURL("https://github.com/owner")
	if err == nil {
		t.Fatal("expected missing repo error")
	}
}

func TestParseGitHubRepoURLRejectsPathTraversal(t *testing.T) {
	for _, value := range []string{
		"https://github.com/owner/..",
		"https://github.com/../repo",
		"https://github.com/owner/repo/../../other",
		"https://github.com/owner/repo/tree/main",
	} {
		t.Run(value, func(t *testing.T) {
			_, err := parseGitHubRepoURL(value)
			if err == nil {
				t.Fatal("expected path traversal or extra segment error")
			}
		})
	}
}

func TestGitHubCachePathFromBase(t *testing.T) {
	base := t.TempDir()
	got, err := githubCachePathFromBase(base, "will", "stackmap")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "StackMap", "repos", "github.com", "will", "stackmap")
	if got != want {
		t.Fatalf("cache path = %q, want %q", got, want)
	}
}

func TestGitCloneArgs(t *testing.T) {
	got := gitCloneArgs("https://github.com/owner/repo.git", "/tmp/repo")
	want := []string{"clone", "--depth", "1", "https://github.com/owner/repo.git", "/tmp/repo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clone args = %#v, want %#v", got, want)
	}
}

func TestEnsureGitHubCloneUsesCachedClone(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandRunner{}
	session := &Session{gitRunner: runner}
	err := session.ensureGitHubClone(context.Background(), githubRepo{
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected cached clone to skip git command, got %#v", runner.calls)
	}
}

func TestEnsureGitHubCloneRunsCloneCommand(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "owner", "repo")
	runner := &fakeCommandRunner{}
	session := &Session{gitRunner: runner}
	err := session.ensureGitHubClone(context.Background(), githubRepo{
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one git command, got %#v", runner.calls)
	}
	wantArgs := []string{"clone", "--depth", "1", "https://github.com/owner/repo.git", target}
	if runner.calls[0].name != "git" || !reflect.DeepEqual(runner.calls[0].args, wantArgs) {
		t.Fatalf("unexpected command: %#v", runner.calls[0])
	}
}

func TestEnsureGitHubCloneReturnsCleanCloneError(t *testing.T) {
	base := t.TempDir()
	runner := &fakeCommandRunner{output: "fatal: repository not found", err: errors.New("exit status 128")}
	session := &Session{gitRunner: runner}
	err := session.ensureGitHubClone(context.Background(), githubRepo{
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    filepath.Join(base, "repo"),
	})
	if err == nil {
		t.Fatal("expected clone error")
	}
	if !strings.Contains(err.Error(), "repository may be unavailable or private") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fakeCommandRunner struct {
	calls  []fakeCommandCall
	output string
	err    error
}

type fakeCommandCall struct {
	name string
	args []string
}

func (r *fakeCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCommandCall{name: name, args: append([]string{}, args...)})
	return r.output, r.err
}
