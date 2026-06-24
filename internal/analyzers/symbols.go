package analyzers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

const (
	symbolFileLimit     = 30
	symbolsPerFileLimit = 10
)

var (
	jsExportFunctionPattern   = regexp.MustCompile(`(?m)^\s*export\s+(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	jsExportClassPattern      = regexp.MustCompile(`(?m)^\s*export\s+class\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	jsExportTypePattern       = regexp.MustCompile(`(?m)^\s*export\s+(?:type|interface|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	jsExportConstPattern      = regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	jsNamedExportBlockPattern = regexp.MustCompile(`(?m)^\s*export\s*\{([^}]+)\}`)
	pythonDefPattern          = regexp.MustCompile(`(?m)^def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonClassPattern        = regexp.MustCompile(`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
)

func AnalyzeSymbols(root string, files []models.FileInfo, features models.FeatureMap, graph models.DependencyGraph) models.SymbolIndex {
	priority := symbolPriority(files, features, graph)
	var candidates []models.FileInfo
	for _, file := range files {
		if priority[file.Path] == 0 || !supportsSymbols(file) {
			continue
		}
		candidates = append(candidates, file)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if priority[candidates[i].Path] == priority[candidates[j].Path] {
			return candidates[i].Path < candidates[j].Path
		}
		return priority[candidates[i].Path] > priority[candidates[j].Path]
	})
	var out []models.FileSymbols
	for _, file := range candidates {
		symbols := extractFileSymbols(root, file)
		if len(symbols) == 0 {
			continue
		}
		out = append(out, models.FileSymbols{Path: file.Path, Symbols: capSymbols(symbols, symbolsPerFileLimit)})
		if len(out) == symbolFileLimit {
			break
		}
	}
	return models.SymbolIndex{Files: out}
}

func symbolPriority(files []models.FileInfo, features models.FeatureMap, graph models.DependencyGraph) map[string]int {
	priority := map[string]int{}
	for _, feature := range features.Features {
		for _, path := range feature.StartHere {
			priority[path] += 5
		}
		for _, path := range feature.RelatedTests {
			priority[path] += 1
		}
	}
	for _, chain := range features.RouteChains {
		for i, path := range chain.Files {
			priority[path] += 4 - minInt(i, 3)
		}
		for _, path := range chain.Tests {
			priority[path] += 2
		}
	}
	for _, file := range graph.TopConnectedFiles {
		priority[file.Path] += 4
	}
	for _, file := range files {
		lower := strings.ToLower(file.Path)
		switch {
		case strings.Contains(lower, "schema") || strings.Contains(lower, "validat"):
			priority[file.Path] += 9
		case strings.Contains(lower, "repositor") || strings.Contains(lower, "/db/"):
			priority[file.Path] += 8
		case strings.Contains(lower, "service") || strings.Contains(lower, "provider"):
			priority[file.Path] += 7
		case strings.Contains(lower, "worker"):
			priority[file.Path] += 6
		case strings.Contains(lower, "config") || strings.Contains(lower, "/env"):
			priority[file.Path] += 5
		case strings.Contains(lower, "/api/") && strings.HasSuffix(lower, "/route.ts"):
			priority[file.Path] -= 2
		}
	}
	return priority
}

func supportsSymbols(file models.FileInfo) bool {
	if file.Kind != models.FileKindSource && file.Kind != models.FileKindTest {
		return false
	}
	switch strings.ToLower(filepath.Ext(file.Path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".go", ".py":
		return true
	default:
		return false
	}
}

func extractFileSymbols(root string, file models.FileInfo) []models.ExportedSymbol {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
	if err != nil {
		return nil
	}
	switch strings.ToLower(filepath.Ext(file.Path)) {
	case ".go":
		return extractGoSymbols(string(data))
	case ".py":
		return extractPythonSymbols(string(data))
	default:
		return extractJSSymbols(string(data))
	}
}

func extractJSSymbols(content string) []models.ExportedSymbol {
	var symbols []models.ExportedSymbol
	for _, match := range jsExportFunctionPattern.FindAllStringSubmatch(content, -1) {
		symbols = append(symbols, models.ExportedSymbol{Name: match[1], Kind: "function"})
	}
	for _, match := range jsExportClassPattern.FindAllStringSubmatch(content, -1) {
		symbols = append(symbols, models.ExportedSymbol{Name: match[1], Kind: "class"})
	}
	for _, match := range jsExportTypePattern.FindAllStringSubmatch(content, -1) {
		symbols = append(symbols, models.ExportedSymbol{Name: match[1], Kind: "type"})
	}
	for _, match := range jsExportConstPattern.FindAllStringSubmatch(content, -1) {
		symbols = append(symbols, models.ExportedSymbol{Name: match[1], Kind: "value"})
	}
	for _, match := range jsNamedExportBlockPattern.FindAllStringSubmatch(content, -1) {
		for _, part := range strings.Split(match[1], ",") {
			name := strings.TrimSpace(strings.Split(strings.TrimSpace(part), " as ")[0])
			if name != "" && name != "default" {
				symbols = append(symbols, models.ExportedSymbol{Name: name, Kind: "export"})
			}
		}
	}
	return dedupeSymbols(symbols)
}

func extractGoSymbols(content string) []models.ExportedSymbol {
	file, err := parser.ParseFile(token.NewFileSet(), "", content, parser.ParseComments)
	if err != nil {
		return nil
	}
	var symbols []models.ExportedSymbol
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Name != nil && decl.Name.IsExported() {
				kind := "function"
				if decl.Recv != nil {
					kind = "method"
				}
				symbols = append(symbols, models.ExportedSymbol{Name: decl.Name.Name, Kind: kind})
			}
		case *ast.GenDecl:
			kind := strings.ToLower(decl.Tok.String())
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						symbols = append(symbols, models.ExportedSymbol{Name: spec.Name.Name, Kind: "type"})
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.IsExported() {
							symbols = append(symbols, models.ExportedSymbol{Name: name.Name, Kind: kind})
						}
					}
				}
			}
		}
	}
	return dedupeSymbols(symbols)
}

func extractPythonSymbols(content string) []models.ExportedSymbol {
	var symbols []models.ExportedSymbol
	for _, match := range pythonDefPattern.FindAllStringSubmatch(content, -1) {
		if !strings.HasPrefix(match[1], "_") {
			symbols = append(symbols, models.ExportedSymbol{Name: match[1], Kind: "function"})
		}
	}
	for _, match := range pythonClassPattern.FindAllStringSubmatch(content, -1) {
		if !strings.HasPrefix(match[1], "_") {
			symbols = append(symbols, models.ExportedSymbol{Name: match[1], Kind: "class"})
		}
	}
	return dedupeSymbols(symbols)
}

func dedupeSymbols(in []models.ExportedSymbol) []models.ExportedSymbol {
	seen := map[string]bool{}
	var out []models.ExportedSymbol
	for _, symbol := range in {
		key := symbol.Kind + ":" + symbol.Name
		if symbol.Name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, symbol)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Name < out[j].Name
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func capSymbols(symbols []models.ExportedSymbol, limit int) []models.ExportedSymbol {
	if len(symbols) <= limit {
		return symbols
	}
	return symbols[:limit]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
