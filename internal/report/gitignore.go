package report

import (
	"os"
	"path/filepath"
	"strings"
)

const stackindexGitignoreEntry = ".stackindex/"

func EnsureGeneratedArtifactsIgnored(root string) error {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	if gitignoreHasStackIndex(content) {
		return nil
	}
	var b strings.Builder
	b.WriteString(content)
	if content != "" && !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(stackindexGitignoreEntry)
	b.WriteString("\n")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func gitignoreHasStackIndex(content string) bool {
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "/")
		if line == ".stackindex" || line == ".stackindex/" {
			return true
		}
	}
	return false
}
