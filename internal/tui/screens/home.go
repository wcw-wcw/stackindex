package screens

import (
	"fmt"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func Home(a *models.Analysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Repo: %s\n", a.RepoPath)
	fmt.Fprintf(&b, "Files scanned: %d\n", len(a.Files))
	fmt.Fprintf(&b, "Detected stack: %s\n", stackLine(a.Stack))
	fmt.Fprintf(&b, "Findings: %s\n", FindingCounts(a.Findings))
	if a.AI != nil && a.AI.Warning != "" {
		fmt.Fprintf(&b, "AI: %s\n", a.AI.Warning)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Screens: 1 Summary  2 Findings  3 Routes  4 Reports")
	return b.String()
}

func stackLine(stack models.StackInfo) string {
	var parts []string
	parts = append(parts, stack.Languages...)
	parts = append(parts, stack.Frameworks...)
	parts = append(parts, stack.Databases...)
	parts = append(parts, stack.Deployment...)
	if len(parts) == 0 {
		return "none detected"
	}
	return strings.Join(parts, " · ")
}

func FindingCounts(findings []models.Finding) string {
	counts := map[models.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	return fmt.Sprintf("%d high · %d medium · %d low · %d info", counts[models.SeverityHigh], counts[models.SeverityMedium], counts[models.SeverityLow], counts[models.SeverityInfo])
}
