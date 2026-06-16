package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type PathActionRequest struct {
	Path string `json:"path"`
}

type CLICommandRequest struct {
	RepoPath       string `json:"repoPath"`
	SourceType     string `json:"sourceType,omitempty"`
	LocalCachePath string `json:"localCachePath,omitempty"`
	AuditStatus    string `json:"auditStatus,omitempty"`
	AIStatus       string `json:"aiStatus,omitempty"`
	AIModel        string `json:"aiModel,omitempty"`
}

func (s *Session) RevealProjectFolder(ctx context.Context, request PathActionRequest) error {
	path, err := validateExistingDir(request.Path, "project folder")
	if err != nil {
		return err
	}
	return revealPath(ctx, path)
}

func (s *Session) RevealStackMapFolder(ctx context.Context, request PathActionRequest) error {
	projectPath, err := validateExistingDir(request.Path, "project folder")
	if err != nil {
		return err
	}
	stackmapPath, err := validateExistingDir(filepath.Join(projectPath, ".stackmap"), ".stackmap folder")
	if err != nil {
		return err
	}
	return revealPath(ctx, stackmapPath)
}

func (s *Session) RevealSnapshotFolder(ctx context.Context, request PathActionRequest) error {
	path, err := validateExistingDir(request.Path, "snapshot folder")
	if err != nil {
		return err
	}
	return revealPath(ctx, path)
}

func (s *Session) OpenMarkdownReport(ctx context.Context, request PathActionRequest) error {
	path, err := validateReportFile(request.Path, ".md", "Markdown report")
	if err != nil {
		return err
	}
	return openPath(ctx, path)
}

func (s *Session) OpenJSONReport(ctx context.Context, request PathActionRequest) error {
	path, err := validateReportFile(request.Path, ".json", "JSON report")
	if err != nil {
		return err
	}
	return openPath(ctx, path)
}

func (s *Session) GenerateCLICommand(request CLICommandRequest) (string, error) {
	return buildCLICommand(request)
}

func validateExistingDir(path, label string) (string, error) {
	target, err := cleanActionPath(path, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%s does not exist: %s", label, target)
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory: %s", label, target)
	}
	return target, nil
}

func validateReportFile(path, extension, label string) (string, error) {
	target, err := cleanActionPath(path, label)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(target), extension) {
		return "", fmt.Errorf("%s must be a %s file: %s", label, extension, target)
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%s does not exist: %s", label, target)
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file: %s", label, target)
	}
	return target, nil
}

func cleanActionPath(path, label string) (string, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return "", fmt.Errorf("%s path is required", label)
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absPath), nil
}

func revealPath(ctx context.Context, path string) error {
	if runtime.GOOS == "darwin" {
		return runOpenCommand(ctx, "-R", path)
	}
	return fmt.Errorf("revealing files is not supported on %s yet", runtime.GOOS)
}

func openPath(ctx context.Context, path string) error {
	if runtime.GOOS == "darwin" {
		return runOpenCommand(ctx, path)
	}
	return fmt.Errorf("opening files is not supported on %s yet", runtime.GOOS)
}

func runOpenCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "open", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("could not open path: %s", message)
	}
	return nil
}

func buildCLICommand(request CLICommandRequest) (string, error) {
	target := strings.TrimSpace(request.RepoPath)
	if strings.EqualFold(strings.TrimSpace(request.SourceType), sourceTypeGitHub) && strings.TrimSpace(request.LocalCachePath) != "" {
		target = strings.TrimSpace(request.LocalCachePath)
	}
	if target == "" {
		return "", errors.New("project path is required")
	}

	args := []string{"stackmap", "analyze", quoteCLIArg(target)}
	if auditWasRun(request.AuditStatus) {
		args = append(args, "--audit")
	}
	if aiWasRequested(request.AIStatus) {
		args = append(args, "--ai")
		if strings.TrimSpace(request.AIModel) != "" {
			args = append(args, "--model", quoteCLIArg(strings.TrimSpace(request.AIModel)))
		}
	}
	return strings.Join(args, " "), nil
}

func auditWasRun(status string) bool {
	value := strings.TrimSpace(strings.ToLower(status))
	return value != "" && value != "not run"
}

func aiWasRequested(status string) bool {
	value := strings.TrimSpace(strings.ToLower(status))
	return value != "" && value != "not requested"
}

func quoteCLIArg(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
