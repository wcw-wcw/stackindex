package report

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/will/stackmap/internal/models"
)

func WriteJSON(root string, analysis *models.Analysis) error {
	outDir := filepath.Join(root, ".stackmap")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(outDir, "analysis.json"), data, 0644)
}

func MarshalJSON(analysis *models.Analysis) ([]byte, error) {
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
