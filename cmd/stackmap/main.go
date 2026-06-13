package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/will/stackmap/internal/ai"
	"github.com/will/stackmap/internal/analyzers"
	"github.com/will/stackmap/internal/models"
	"github.com/will/stackmap/internal/report"
	"github.com/will/stackmap/internal/tui"
)

const usage = `StackMap - local-first codebase analyzer

Usage:
  stackmap
  stackmap analyze [path] [--json] [--no-tui] [--ai] [--model llama3.2:3b] [--ai-debug]
  stackmap audit [path] [--json] [--ai] [--model llama3.2:3b] [--ai-debug]
  stackmap --help

Examples:
  stackmap analyze .
  stackmap analyze ./path-to-project --no-tui
  stackmap analyze . --json
  stackmap analyze . --ai --model llama3.2:3b
  stackmap audit .

Local Ollama model behavior varies. By default StackMap tries llama3.2:3b,
then qwen:7b, then the deterministic StackMap summary.

Audit mode is deterministic for CI: it fails only on static high or medium
findings. Optional AI report content never affects the audit exit code.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "stackmap:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Print(usage)
		return nil
	}
	if len(args) == 0 {
		return analyze([]string{"."}, false)
	}
	switch args[0] {
	case "analyze":
		return analyze(args[1:], false)
	case "audit":
		return analyze(args[1:], true)
	default:
		return analyze(args, false)
	}
}

func analyze(args []string, auditMode bool) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON to stdout without launching TUI")
	noTUI := fs.Bool("no-tui", false, "run analysis and export reports without launching TUI")
	auditFlag := fs.Bool("audit", false, "run deterministic CI audit and fail on high or medium static findings")
	enableAI := fs.Bool("ai", false, "enable optional local Ollama analysis")
	aiDebug := fs.Bool("ai-debug", false, "write local AI prompt/response diagnostics under .stackmap/ai-debug")
	model := fs.String("model", "", "Ollama model to use when --ai is enabled; default tries llama3.2:3b then qwen:7b")
	if err := fs.Parse(normalizeAnalyzeArgs(args)); err != nil {
		return err
	}
	auditMode = auditMode || *auditFlag

	target := "."
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	root, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	analysis, err := analyzers.Analyze(root)
	if err != nil {
		return err
	}
	if *enableAI {
		opts := ai.SummaryOptions{}
		if *aiDebug {
			opts.DebugDir = filepath.Join(root, ".stackmap", "ai-debug")
		}
		analysis.AI = ai.SummarizeWithOptions(context.Background(), analysis, *model, opts)
		if analysis.AI.Warning != "" {
			fmt.Fprintf(os.Stderr, "stackmap: %s\n", analysis.AI.Warning)
		}
	}

	if *jsonOut {
		if auditMode {
			if err := report.ExportAll(root, analysis); err != nil {
				return err
			}
		}
		data, err := report.MarshalJSON(analysis)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		if auditMode {
			return auditError(analysis)
		}
		return nil
	}

	if err := report.ExportAll(root, analysis); err != nil {
		return err
	}
	if auditMode {
		printAuditSummary(root, analysis)
		return auditError(analysis)
	}
	if *noTUI {
		printExportSummary(root)
		return nil
	}
	return tui.Run(analysis, root)
}

func printExportSummary(root string) {
	outDir := filepath.Join(root, ".stackmap")
	fmt.Printf("StackMap analysis exported to %s\n", outDir)
	fmt.Printf("JSON: %s\n", filepath.Join(outDir, "analysis.json"))
	fmt.Printf("Markdown: %s\n", filepath.Join(outDir, "reports", "repo-report.md"))
	fmt.Println("Note: .stackmap is a hidden folder on macOS Finder. Press Cmd+Shift+. in Finder to show hidden files.")
}

type auditFailure struct {
	high   int
	medium int
}

func (e auditFailure) Error() string {
	return fmt.Sprintf("audit failed: %d high and %d medium findings", e.high, e.medium)
}

func auditError(analysis *models.Analysis) error {
	high, medium := auditBlockingCounts(analysis.Findings)
	if high == 0 && medium == 0 {
		return nil
	}
	return auditFailure{high: high, medium: medium}
}

func auditBlockingCounts(findings []models.Finding) (int, int) {
	var high, medium int
	for _, finding := range findings {
		switch finding.Severity {
		case models.SeverityHigh:
			high++
		case models.SeverityMedium:
			medium++
		}
	}
	return high, medium
}

func printAuditSummary(root string, analysis *models.Analysis) {
	outDir := filepath.Join(root, ".stackmap")
	high, medium := auditBlockingCounts(analysis.Findings)
	fmt.Printf("StackMap audit exported to %s\n", outDir)
	fmt.Printf("JSON: %s\n", filepath.Join(outDir, "analysis.json"))
	fmt.Printf("Markdown: %s\n", filepath.Join(outDir, "reports", "repo-report.md"))
	if high == 0 && medium == 0 {
		fmt.Println("Audit passed: no high or medium static findings.")
		return
	}
	fmt.Printf("Audit failed: %d high and %d medium static findings.\n", high, medium)
}

func normalizeAnalyzeArgs(args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json", "--no-tui", "--audit", "--ai", "--ai-debug", "-json", "-no-tui", "-audit", "-ai", "-ai-debug":
			flags = append(flags, arg)
		case "--model", "-model":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		default:
			if strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-model=") {
				flags = append(flags, arg)
			} else {
				positionals = append(positionals, arg)
			}
		}
	}
	return append(flags, positionals...)
}
