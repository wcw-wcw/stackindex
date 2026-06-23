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
	if !strings.Contains(repo.LocalCachePath, filepath.Join("StackIndex", "repos", "github.com", "owner", "repo")) {
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
	got, err := githubCachePathFromBase(base, "will", "stackindex")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "StackIndex", "repos", "github.com", "will", "stackindex")
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

func TestGitRefreshArgs(t *testing.T) {
	got := gitRefreshArgs("/tmp/repo", "main")
	want := [][]string{
		{"-C", "/tmp/repo", "rev-parse", "--is-inside-work-tree"},
		{"-C", "/tmp/repo", "remote", "get-url", "origin"},
		{"-C", "/tmp/repo", "rev-parse", "--abbrev-ref", "HEAD"},
		{"-C", "/tmp/repo", "fetch", "--prune", "origin"},
		{"-C", "/tmp/repo", "pull", "--ff-only", "origin", "main"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("refresh args = %#v, want %#v", got, want)
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

func TestPrepareGitHubCloneRefreshFalseReusesExistingCache(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	target := filepath.Join(cacheRoot, "owner", "repo")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandRunner{}
	session := &Session{githubCacheRoot: cacheRoot, gitRunner: runner}
	err := session.prepareGitHubClone(context.Background(), githubRepo{
		Owner:             "owner",
		Repo:              "repo",
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected refresh=false to skip git commands for cached clone, got %#v", runner.calls)
	}
}

func TestPrepareGitHubCloneRefreshTrueRunsRefreshCommands(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	target := filepath.Join(cacheRoot, "owner", "repo")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandRunner{outputs: []string{
		"true",
		"https://github.com/owner/repo.git",
		"main",
		"",
		"Already up to date.",
	}}
	session := &Session{githubCacheRoot: cacheRoot, gitRunner: runner}
	err := session.prepareGitHubClone(context.Background(), githubRepo{
		Owner:             "owner",
		Repo:              "repo",
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := gitRefreshArgs(target, "main")
	if len(runner.calls) != len(want) {
		t.Fatalf("command count = %d, want %d: %#v", len(runner.calls), len(want), runner.calls)
	}
	for i, args := range want {
		if runner.calls[i].name != "git" || !reflect.DeepEqual(runner.calls[i].args, args) {
			t.Fatalf("command %d = %#v, want git %#v", i, runner.calls[i], args)
		}
	}
}

func TestPrepareGitHubCloneRefreshRejectsOutsideCacheRoot(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	target := filepath.Join(t.TempDir(), "owner", "repo")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	session := &Session{githubCacheRoot: cacheRoot, gitRunner: &fakeCommandRunner{}}
	err := session.prepareGitHubClone(context.Background(), githubRepo{
		Owner:             "owner",
		Repo:              "repo",
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	}, true)
	if err == nil || !strings.Contains(err.Error(), "outside the StackIndex GitHub cache root") {
		t.Fatalf("expected outside cache error, got %v", err)
	}
}

func TestPrepareGitHubCloneRefreshRejectsMismatchedOrigin(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	target := filepath.Join(cacheRoot, "owner", "repo")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandRunner{outputs: []string{
		"true",
		"https://github.com/other/repo.git",
	}}
	session := &Session{githubCacheRoot: cacheRoot, gitRunner: runner}
	err := session.prepareGitHubClone(context.Background(), githubRepo{
		Owner:             "owner",
		Repo:              "repo",
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	}, true)
	if err == nil || !strings.Contains(err.Error(), "origin does not match") {
		t.Fatalf("expected origin mismatch error, got %v", err)
	}
}

func TestPrepareGitHubCloneRefreshRejectsNonGitCachedDirectory(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	target := filepath.Join(cacheRoot, "owner", "repo")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	session := &Session{githubCacheRoot: cacheRoot, gitRunner: &fakeCommandRunner{}}
	err := session.prepareGitHubClone(context.Background(), githubRepo{
		Owner:             "owner",
		Repo:              "repo",
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	}, true)
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected non-git cached directory error, got %v", err)
	}
}

func TestPrepareGitHubCloneRefreshClonesWhenCacheMissing(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "StackIndex", "repos", "github.com")
	target := filepath.Join(cacheRoot, "owner", "repo")
	runner := &fakeCommandRunner{}
	session := &Session{githubCacheRoot: cacheRoot, gitRunner: runner}
	err := session.prepareGitHubClone(context.Background(), githubRepo{
		Owner:             "owner",
		Repo:              "repo",
		CanonicalCloneURL: "https://github.com/owner/repo.git",
		LocalCachePath:    target,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"clone", "--depth", "1", "https://github.com/owner/repo.git", target}
	if len(runner.calls) != 1 || runner.calls[0].name != "git" || !reflect.DeepEqual(runner.calls[0].args, wantArgs) {
		t.Fatalf("unexpected clone command: %#v", runner.calls)
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
	calls   []fakeCommandCall
	outputs []string
	errs    []error
	output  string
	err     error
}

type fakeCommandCall struct {
	name string
	args []string
}

func (r *fakeCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCommandCall{name: name, args: append([]string{}, args...)})
	if len(r.outputs) > 0 || len(r.errs) > 0 {
		var output string
		var err error
		if len(r.outputs) > 0 {
			output = r.outputs[0]
			r.outputs = r.outputs[1:]
		}
		if len(r.errs) > 0 {
			err = r.errs[0]
			r.errs = r.errs[1:]
		}
		return output, err
	}
	return r.output, r.err
}
