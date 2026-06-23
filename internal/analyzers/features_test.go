package analyzers

import (
	"testing"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestAnalyzeBuildsFeatureMapAndRouteChains(t *testing.T) {
	root := tempProject(t, map[string]string{
		"package.json":           `{"name":"stocks","dependencies":{"next":"latest","react":"latest"}}`,
		"src/app/rules/page.tsx": `export default function RulesPage() { return null }`,
		"src/app/api/rules/route.ts": `import { RuleSchema } from "../../../lib/rules/schema";
import { saveRule } from "../../../lib/db/repositories";
export async function POST() { return Response.json({ ok: RuleSchema && saveRule }) }`,
		"src/lib/rules/schema.ts":             `export const RuleSchema = {}`,
		"src/lib/db/repositories.ts":          `export function saveRule() { return true }`,
		"src/lib/rules/evaluate.test.ts":      `import { RuleSchema } from "./schema"; test("rules", () => RuleSchema)`,
		".stackmap/reports/repo-report.md":    `generated`,
		".stackmap/history/old/analysis.json": `{}`,
	})

	analysis, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasAnalyzedPath(analysis.Files, ".stackmap/reports/repo-report.md") {
		t.Fatalf("legacy .stackmap artifact should be ignored: %#v", analysis.Files)
	}
	if analysis.Quality.IgnoredDirCounts[".stackmap"] == 0 {
		t.Fatalf("expected .stackmap ignore count in quality: %#v", analysis.Quality.IgnoredDirCounts)
	}
	if len(analysis.Features.Features) == 0 {
		t.Fatal("expected feature clusters")
	}
	if !hasFeatureStart(analysis.Features.Features, "Rules / alerts", "src/app/api/rules/route.ts") {
		t.Fatalf("expected rules feature with API start file: %#v", analysis.Features.Features)
	}
	if !hasRouteChainFile(analysis.Features.RouteChains, "POST /api/rules", "src/lib/rules/schema.ts") {
		t.Fatalf("expected route chain to include schema source: %#v", analysis.Features.RouteChains)
	}
}

func hasAnalyzedPath(files []models.FileInfo, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func hasFeatureStart(features []models.FeatureCluster, name, path string) bool {
	for _, feature := range features {
		if feature.Name != name {
			continue
		}
		for _, item := range feature.StartHere {
			if item == path {
				return true
			}
		}
	}
	return false
}

func hasRouteChainFile(chains []models.RouteChain, route, path string) bool {
	for _, chain := range chains {
		if chain.Route != route {
			continue
		}
		for _, item := range chain.Files {
			if item == path {
				return true
			}
		}
	}
	return false
}
