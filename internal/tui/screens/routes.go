package screens

import (
	"fmt"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func Routes(a *models.Analysis, cursor int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "API Routes: %d\n\n", len(a.Routes))
	if len(a.Routes) == 0 {
		fmt.Fprintln(&b, "No routes detected.")
		return b.String()
	}
	for i, route := range a.Routes {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		fmt.Fprintf(&b, "%s%s %s\n", prefix, route.Method, route.Path)
	}
	selected := a.Routes[cursor]
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Source: %s\nConfidence: %s\n", selected.SourceFile, selected.Confidence)
	return b.String()
}
