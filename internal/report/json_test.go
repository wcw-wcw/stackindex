package report

import (
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestMarshalJSONIncludesProjectContextAndStructureMap(t *testing.T) {
	data, err := MarshalJSON(&models.Analysis{
		RepoName: "demo",
		Context: models.ProjectContext{
			Purpose:    "Go CLI/TUI repository analysis tool",
			Confidence: "high",
		},
		Structure: models.StructureMap{
			Directories: []models.DirectoryRole{{Path: "cmd/", Role: "CLI entrypoints", FileCount: 1}},
			KeyFiles:    []models.FileRole{{Path: "go.mod", Role: "Go module definition", Importance: "high"}},
		},
		Dependencies: models.DependencyGraph{
			Entrypoints:       []string{"cmd/stackmap/main.go"},
			ArchitectureHints: []string{"CLI entrypoints connect to internal analyzer packages."},
			TopConnectedFiles: []models.ConnectedFileSummary{{Path: "cmd/stackmap/main.go", Role: "Main CLI entrypoint", ImportsCount: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{`"projectContext"`, `"structureMap"`, `"dependencyGraph"`, `"purpose"`, `"directories"`, `"keyFiles"`, `"entrypoints"`, `"topConnectedFiles"`, `"architectureHints"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON did not contain %q:\n%s", want, out)
		}
	}
}
