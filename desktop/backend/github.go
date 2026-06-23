package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	sourceTypeLocal  = "local"
	sourceTypeGitHub = "github"
)

var githubSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type GitHubAnalyzeRequest struct {
	URL      string `json:"url"`
	RunAudit bool   `json:"runAudit"`
	UseAI    bool   `json:"useAI"`
	Model    string `json:"model"`
	Refresh  bool   `json:"refresh"`
}

type githubRepo struct {
	Owner             string
	Repo              string
	CanonicalCloneURL string
	LocalCachePath    string
}

type sourceMetadata struct {
	SourceType     string
	GitHubURL      string
	LocalCachePath string
}

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return strings.TrimSpace(output.String()), err
}

func (s *Session) AnalyzeGitHubRepo(ctx context.Context, request GitHubAnalyzeRequest) (*AnalyzeResponse, error) {
	repo, err := parseGitHubRepoURL(request.URL)
	if err != nil {
		return nil, err
	}
	if cachePath, err := s.githubRepoCachePath(repo.Owner, repo.Repo); err != nil {
		return nil, err
	} else {
		repo.LocalCachePath = cachePath
	}
	if err := s.prepareGitHubClone(ctx, repo, request.Refresh); err != nil {
		return nil, err
	}
	response, err := s.analyzeProject(ctx, AnalyzeRequest{
		Path:     repo.LocalCachePath,
		RunAudit: request.RunAudit,
		UseAI:    request.UseAI,
		Model:    request.Model,
	}, sourceMetadata{
		SourceType:     sourceTypeGitHub,
		GitHubURL:      repo.CanonicalCloneURL,
		LocalCachePath: repo.LocalCachePath,
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *Session) prepareGitHubClone(ctx context.Context, repo githubRepo, refresh bool) error {
	if !refresh {
		return s.ensureGitHubClone(ctx, repo)
	}
	info, err := os.Stat(repo.LocalCachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.ensureGitHubClone(ctx, repo)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("cached GitHub path is not a directory: %s", repo.LocalCachePath)
	}
	return s.refreshGitHubClone(ctx, repo)
}

func (s *Session) ensureGitHubClone(ctx context.Context, repo githubRepo) error {
	info, err := os.Stat(repo.LocalCachePath)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("cached GitHub path is not a directory: %s", repo.LocalCachePath)
		}
		if _, statErr := os.Stat(filepath.Join(repo.LocalCachePath, ".git")); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("cached GitHub path exists but is not a git repository: %s", repo.LocalCachePath)
			}
			return statErr
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(repo.LocalCachePath), 0755); err != nil {
		return fmt.Errorf("could not prepare GitHub cache directory: %w", err)
	}
	args := gitCloneArgs(repo.CanonicalCloneURL, repo.LocalCachePath)
	output, runErr := s.gitCommandRunner().Run(ctx, "git", args...)
	if runErr != nil {
		if errors.Is(runErr, exec.ErrNotFound) {
			return errors.New("git is not installed or not available on PATH")
		}
		return fmt.Errorf("clone failed; repository may be unavailable or private: %s", conciseCommandOutput(output))
	}
	return nil
}

func (s *Session) refreshGitHubClone(ctx context.Context, repo githubRepo) error {
	if err := s.validateRefreshTarget(repo); err != nil {
		return err
	}
	runner := s.gitCommandRunner()
	if output, err := runner.Run(ctx, "git", "-C", repo.LocalCachePath, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(output) != "true" {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("git is not installed or not available on PATH")
		}
		return fmt.Errorf("cached GitHub path is not a usable git repository: %s", repo.LocalCachePath)
	}
	origin, err := runner.Run(ctx, "git", "-C", repo.LocalCachePath, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("could not read cached GitHub origin: %s", conciseCommandOutput(origin))
	}
	if strings.TrimSpace(origin) != repo.CanonicalCloneURL {
		return fmt.Errorf("cached GitHub origin does not match requested repository: got %s", strings.TrimSpace(origin))
	}
	branch, err := runner.Run(ctx, "git", "-C", repo.LocalCachePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("could not determine cached GitHub branch: %s", conciseCommandOutput(branch))
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		return errors.New("cached GitHub clone is not on a branch; refresh requires a checked-out branch")
	}
	if output, err := runner.Run(ctx, "git", "-C", repo.LocalCachePath, "fetch", "--prune", "origin"); err != nil {
		return fmt.Errorf("could not refresh cached GitHub clone: %s", conciseCommandOutput(output))
	}
	if output, err := runner.Run(ctx, "git", "-C", repo.LocalCachePath, "pull", "--ff-only", "origin", branch); err != nil {
		return fmt.Errorf("could not fast-forward cached GitHub clone: %s", conciseCommandOutput(output))
	}
	return nil
}

func (s *Session) validateRefreshTarget(repo githubRepo) error {
	root, err := s.githubCacheRootPath()
	if err != nil {
		return err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	cleanTarget, err := filepath.Abs(repo.LocalCachePath)
	if err != nil {
		return err
	}
	if !pathInside(cleanTarget, cleanRoot) {
		return errors.New("refusing to refresh a cached GitHub path outside the StackIndex GitHub cache root")
	}
	if _, err := os.Stat(filepath.Join(cleanTarget, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cached GitHub path exists but is not a git repository: %s", cleanTarget)
		}
		return err
	}
	return nil
}

func (s *Session) gitCommandRunner() commandRunner {
	if s.gitRunner != nil {
		return s.gitRunner
	}
	return execCommandRunner{}
}

func parseGitHubRepoURL(raw string) (githubRepo, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return githubRepo{}, errors.New("GitHub URL is required")
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return githubRepo{}, fmt.Errorf("invalid GitHub URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return githubRepo{}, errors.New("only public HTTPS GitHub URLs are supported")
	}
	if parsed.User != nil {
		return githubRepo{}, errors.New("GitHub URLs with credentials are not supported")
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Port() != "" {
		return githubRepo{}, errors.New("only github.com repository URLs are supported")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return githubRepo{}, errors.New("GitHub URL must not include query parameters or fragments")
	}
	if parsed.RawPath != "" && parsed.RawPath != parsed.EscapedPath() {
		return githubRepo{}, errors.New("GitHub URL path is not supported")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 {
		return githubRepo{}, errors.New("GitHub URL must be in the form https://github.com/owner/repo")
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if strings.HasSuffix(repo, ".git") {
		repo = strings.TrimSuffix(repo, ".git")
	}
	if !validGitHubSegment(owner) || !validGitHubSegment(repo) {
		return githubRepo{}, errors.New("GitHub URL contains an invalid owner or repo name")
	}
	cachePath, err := githubCachePath(owner, repo)
	if err != nil {
		return githubRepo{}, err
	}
	return githubRepo{
		Owner:             owner,
		Repo:              repo,
		CanonicalCloneURL: fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		LocalCachePath:    cachePath,
	}, nil
}

func validGitHubSegment(value string) bool {
	if value == "" || value == "." || value == ".." || strings.Contains(value, "..") {
		return false
	}
	return githubSegmentPattern.MatchString(value)
}

func githubCachePath(owner, repo string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		return "", errors.New("could not determine local StackIndex cache directory")
	}
	return githubCachePathFromBase(cacheDir, owner, repo)
}

func githubCachePathFromBase(base, owner, repo string) (string, error) {
	root, err := githubCacheRootFromBase(base)
	if err != nil {
		return "", err
	}
	return githubCachePathFromRoot(root, owner, repo)
}

func githubCacheRootFromBase(base string) (string, error) {
	target := strings.TrimSpace(base)
	if target == "" {
		return "", errors.New("could not determine local StackIndex cache directory")
	}
	return filepath.Join(target, "StackIndex", "repos", "github.com"), nil
}

func githubCachePathFromRoot(root, owner, repo string) (string, error) {
	if !validGitHubSegment(owner) || !validGitHubSegment(repo) {
		return "", errors.New("GitHub URL contains an invalid owner or repo name")
	}
	target := strings.TrimSpace(root)
	if target == "" {
		return "", errors.New("could not determine local StackIndex GitHub cache directory")
	}
	return filepath.Join(target, owner, repo), nil
}

func (s *Session) githubRepoCachePath(owner, repo string) (string, error) {
	root, err := s.githubCacheRootPath()
	if err != nil {
		return "", err
	}
	return githubCachePathFromRoot(root, owner, repo)
}

func gitCloneArgs(cloneURL, targetPath string) []string {
	return []string{"clone", "--depth", "1", cloneURL, targetPath}
}

func gitRefreshArgs(cachePath, branch string) [][]string {
	return [][]string{
		{"-C", cachePath, "rev-parse", "--is-inside-work-tree"},
		{"-C", cachePath, "remote", "get-url", "origin"},
		{"-C", cachePath, "rev-parse", "--abbrev-ref", "HEAD"},
		{"-C", cachePath, "fetch", "--prune", "origin"},
		{"-C", cachePath, "pull", "--ff-only", "origin", branch},
	}
}

func pathInside(target, root string) bool {
	target = filepath.Clean(target)
	root = filepath.Clean(root)
	if target == root {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func applySourceMetadata(response *AnalyzeResponse, source sourceMetadata) {
	if response == nil {
		return
	}
	response.SourceType = strings.TrimSpace(source.SourceType)
	if response.SourceType == "" {
		response.SourceType = sourceTypeLocal
	}
	response.GitHubURL = strings.TrimSpace(source.GitHubURL)
	response.LocalCachePath = strings.TrimSpace(source.LocalCachePath)
}

func conciseCommandOutput(output string) string {
	value := strings.TrimSpace(output)
	if value == "" {
		return "git returned an error without details"
	}
	if len(value) > 700 {
		return value[:700] + "..."
	}
	return value
}
