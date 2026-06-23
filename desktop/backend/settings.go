package backend

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type DesktopSettings struct {
	DefaultRunAudit bool   `json:"defaultRunAudit"`
	DefaultUseAI    bool   `json:"defaultUseAI"`
	DefaultModel    string `json:"defaultModel"`
}

type DesktopPaths struct {
	RecentProjectsPath string `json:"recentProjectsPath"`
	GitHubCacheRoot    string `json:"githubCacheRoot"`
	SettingsPath       string `json:"settingsPath"`
}

func defaultDesktopSettings() DesktopSettings {
	return DesktopSettings{
		DefaultRunAudit: true,
		DefaultUseAI:    false,
		DefaultModel:    "",
	}
}

func (s *Session) GetDesktopSettings() (*DesktopSettings, error) {
	settings, err := readDesktopSettings(s.settingsFilePath())
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (s *Session) SaveDesktopSettings(settings DesktopSettings) (*DesktopSettings, error) {
	settings.DefaultModel = strings.TrimSpace(settings.DefaultModel)
	if err := writeDesktopSettings(s.settingsFilePath(), settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (s *Session) GetDesktopPaths() (*DesktopPaths, error) {
	githubRoot, err := s.githubCacheRootPath()
	if err != nil {
		return nil, err
	}
	return &DesktopPaths{
		RecentProjectsPath: s.recentPath(),
		GitHubCacheRoot:    githubRoot,
		SettingsPath:       s.settingsFilePath(),
	}, nil
}

func (s *Session) ClearGitHubCache() error {
	root, err := s.githubCacheRootPath()
	if err != nil {
		return err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !isSafeGitHubCacheRoot(cleanRoot) {
		return errors.New("refusing to clear a path outside the StackIndex GitHub cache root")
	}
	if err := os.RemoveAll(cleanRoot); err != nil {
		return err
	}
	return os.MkdirAll(cleanRoot, 0755)
}

func readDesktopSettings(path string) (DesktopSettings, error) {
	settings := defaultDesktopSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultDesktopSettings(), nil
	}
	settings.DefaultModel = strings.TrimSpace(settings.DefaultModel)
	return settings, nil
}

func writeDesktopSettings(path string, settings DesktopSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func (s *Session) settingsFilePath() string {
	if strings.TrimSpace(s.settingsPath) != "" {
		return s.settingsPath
	}
	return defaultDesktopSettingsPath()
}

func defaultDesktopSettingsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "StackIndex", "settings.json")
}

func (s *Session) githubCacheRootPath() (string, error) {
	if strings.TrimSpace(s.githubCacheRoot) != "" {
		return s.githubCacheRoot, nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		return "", errors.New("could not determine local StackIndex cache directory")
	}
	return githubCacheRootFromBase(cacheDir)
}

func isSafeGitHubCacheRoot(path string) bool {
	clean := filepath.Clean(path)
	parts := splitPath(clean)
	if len(parts) < 3 {
		return false
	}
	return parts[len(parts)-3] == "StackIndex" && parts[len(parts)-2] == "repos" && parts[len(parts)-1] == "github.com"
}

func splitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		file = strings.Trim(file, string(filepath.Separator))
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		next := strings.TrimSuffix(dir, string(filepath.Separator))
		if next == "" || next == path {
			break
		}
		path = next
	}
	return parts
}
