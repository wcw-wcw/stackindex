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
	"github.com/will/stackmap/internal/report"
	"github.com/will/stackmap/internal/tui"
)

const usage = `StackMap - local-first codebase analyzer

Usage:
  stackmap
  stackmap analyze [path] [--json] [--no-tui] [--ai] [--model qwen2.5-coder:7b]
  stackmap --help

Examples:
  stackmap analyze .
  stackmap analyze ./path-to-project --no-tui
  stackmap analyze . --json
  stackmap analyze . --ai --model qwen2.5-coder:7b
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
		return analyze([]string{"."})
	}
	switch args[0] {
	case "analyze":
		return analyze(args[1:])
	default:
		return analyze(args)
	}
}

func analyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON to stdout without launching TUI")
	noTUI := fs.Bool("no-tui", false, "run analysis and export reports without launching TUI")
	enableAI := fs.Bool("ai", false, "enable optional local Ollama analysis")
	model := fs.String("model", ai.DefaultModel, "Ollama model to use when --ai is enabled")
	if err := fs.Parse(normalizeAnalyzeArgs(args)); err != nil {
		return err
	}

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
		analysis.AI = ai.Summarize(context.Background(), analysis, *model)
		if analysis.AI.Warning != "" {
			fmt.Fprintf(os.Stderr, "stackmap: %s\n", analysis.AI.Warning)
		}
	}

	if *jsonOut {
		data, err := report.MarshalJSON(analysis)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}

	if err := report.ExportAll(root, analysis); err != nil {
		return err
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

func normalizeAnalyzeArgs(args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json", "--no-tui", "--ai", "-json", "-no-tui", "-ai":
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
