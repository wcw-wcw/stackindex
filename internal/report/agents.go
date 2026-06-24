package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

type AgentsOptions struct {
	RootTarget bool
	Force      bool
}

func WriteAgentsMD(root string, analysis *models.Analysis, opts AgentsOptions) (string, error) {
	target := filepath.Join(root, ".stackindex", "AGENTS.md")
	if opts.RootTarget {
		target = filepath.Join(root, "AGENTS.md")
		if !opts.Force {
			if _, err := os.Stat(target); err == nil {
				return "", fmt.Errorf("refusing to overwrite existing %s without --agents-md-force", target)
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	return target, os.WriteFile(target, []byte(AgentsMarkdown(analysis)), 0644)
}

func AgentsMarkdown(a *models.Analysis) string {
	repo := "this repository"
	if a != nil && strings.TrimSpace(a.RepoName) != "" {
		repo = a.RepoName
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# AGENTS.md\n\n")
	fmt.Fprintf(&b, "Use StackIndex before broad exploration in `%s`.\n\n", repo)
	fmt.Fprintln(&b, "- Read `.stackindex/reports/repo-index.md` first.")
	fmt.Fprintln(&b, "- Use Feature Map for feature work.")
	fmt.Fprintln(&b, "- Use Route Implementation Chains for API work.")
	fmt.Fprintln(&b, "- Use Task Search Recipes before whole-repo search.")
	fmt.Fprintln(&b, "- Open `.stackindex/reports/repo-index.full.md` only when the compact index is insufficient.")
	fmt.Fprintln(&b, "- Avoid generated/cache folders such as `.stackindex/`, `node_modules/`, `dist/`, `build/`, and framework caches.")
	fmt.Fprintln(&b, "- If the index says it is stale, rerun `stackindex analyze <repo> --no-tui`.")
	return b.String()
}
