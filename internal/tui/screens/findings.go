package screens

import (
	"fmt"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func Findings(a *models.Analysis, cursor int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Findings: %s\n\n", FindingCounts(a.Findings))
	if len(a.Findings) == 0 {
		fmt.Fprintln(&b, "No findings.")
		return b.String()
	}
	for i, f := range a.Findings {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		file := ""
		if f.File != "" {
			file = " - " + f.File
		}
		fmt.Fprintf(&b, "%s[%s] %s%s\n", prefix, f.Severity, f.Message, file)
	}
	selected := a.Findings[cursor]
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Category: %s\n", selected.Category)
	if selected.Recommendation != "" {
		fmt.Fprintf(&b, "Recommendation: %s\n", selected.Recommendation)
	}
	return b.String()
}
