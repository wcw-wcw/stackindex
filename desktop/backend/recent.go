package backend

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const maxRecentProjects = 20

type RecentProject struct {
	RepoName       string         `json:"repoName"`
	RepoPath       string         `json:"repoPath"`
	LastAnalyzed   string         `json:"lastAnalyzed"`
	Files          int            `json:"files"`
	Routes         int            `json:"routes"`
	Tests          int            `json:"tests"`
	Findings       map[string]int `json:"findings"`
	AuditStatus    string         `json:"auditStatus,omitempty"`
	AIStatus       string         `json:"aiStatus,omitempty"`
	AIModel        string         `json:"aiModel,omitempty"`
	JSONReportPath string         `json:"jsonReportPath"`
	MDReportPath   string         `json:"mdReportPath"`
}

func (s *Session) GetRecentProjects() ([]RecentProject, error) {
	return readRecentProjects(s.recentPath())
}

func (s *Session) RemoveRecentProject(path string) error {
	target, err := normalizeProjectPath(path)
	if err != nil {
		return err
	}
	projects, err := readRecentProjects(s.recentPath())
	if err != nil {
		return err
	}
	next := make([]RecentProject, 0, len(projects))
	for _, project := range projects {
		projectPath, err := normalizeProjectPath(project.RepoPath)
		if err != nil || projectPath == target {
			continue
		}
		next = append(next, project)
	}
	return writeRecentProjects(s.recentPath(), next)
}

func (s *Session) ClearRecentProjects() error {
	return writeRecentProjects(s.recentPath(), []RecentProject{})
}

func (s *Session) upsertRecentProject(response *AnalyzeResponse) error {
	if response == nil {
		return nil
	}
	entry := recentProjectFromResponse(response)
	if strings.TrimSpace(entry.RepoPath) == "" {
		return nil
	}
	projects, err := readRecentProjects(s.recentPath())
	if err != nil {
		return err
	}
	next := []RecentProject{entry}
	target, err := normalizeProjectPath(entry.RepoPath)
	if err != nil {
		return err
	}
	for _, project := range projects {
		projectPath, err := normalizeProjectPath(project.RepoPath)
		if err != nil || projectPath == target {
			continue
		}
		next = append(next, project)
		if len(next) >= maxRecentProjects {
			break
		}
	}
	return writeRecentProjects(s.recentPath(), next)
}

func recentProjectFromResponse(response *AnalyzeResponse) RecentProject {
	return RecentProject{
		RepoName:       response.RepoName,
		RepoPath:       response.RepoPath,
		LastAnalyzed:   response.GeneratedAt,
		Files:          response.Files,
		Routes:         response.Routes,
		Tests:          response.Tests,
		Findings:       copyFindingCounts(response.Findings),
		AuditStatus:    response.AuditStatus,
		AIStatus:       response.AIStatus,
		AIModel:        response.AIModel,
		JSONReportPath: response.JSONReportPath,
		MDReportPath:   response.MDReportPath,
	}
}

func readRecentProjects(path string) ([]RecentProject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []RecentProject{}, nil
		}
		return nil, err
	}
	var projects []RecentProject
	if err := json.Unmarshal(data, &projects); err != nil {
		return []RecentProject{}, nil
	}
	cleaned := make([]RecentProject, 0, len(projects))
	for _, project := range projects {
		if strings.TrimSpace(project.RepoPath) == "" {
			continue
		}
		project.Findings = copyFindingCounts(project.Findings)
		cleaned = append(cleaned, project)
		if len(cleaned) >= maxRecentProjects {
			break
		}
	}
	return cleaned, nil
}

func writeRecentProjects(path string, projects []RecentProject) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if len(projects) > maxRecentProjects {
		projects = projects[:maxRecentProjects]
	}
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func (s *Session) recentPath() string {
	if strings.TrimSpace(s.recentProjectsPath) != "" {
		return s.recentProjectsPath
	}
	return defaultRecentProjectsPath()
}

func defaultRecentProjectsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "StackMap", "recent-projects.json")
}

func normalizeProjectPath(path string) (string, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return "", errors.New("project path is required")
	}
	return filepath.Abs(target)
}

func copyFindingCounts(values map[string]int) map[string]int {
	counts := map[string]int{
		"high":   0,
		"medium": 0,
		"low":    0,
		"info":   0,
	}
	for key, value := range values {
		counts[key] = value
	}
	return counts
}
