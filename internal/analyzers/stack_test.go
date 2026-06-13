package analyzers

import (
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestDetectStackIncludesNeonPostgres(t *testing.T) {
	stack := DetectStack(nil, &models.PackageInfo{
		Dependencies: map[string]string{
			"@neondatabase/serverless": "^1.0.0",
		},
	})
	if len(stack.Databases) != 1 || stack.Databases[0] != "Neon Postgres" {
		t.Fatalf("Databases = %#v, want Neon Postgres", stack.Databases)
	}
}
