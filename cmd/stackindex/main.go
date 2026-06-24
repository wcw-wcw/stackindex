package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/app"
	"github.com/wcw-wcw/stackindex/internal/audit"
	"github.com/wcw-wcw/stackindex/internal/models"
	"github.com/wcw-wcw/stackindex/internal/planner"
	"github.com/wcw-wcw/stackindex/internal/qa"
	"github.com/wcw-wcw/stackindex/internal/report"
	"github.com/wcw-wcw/stackindex/internal/tui"
)

const usage = `StackIndex - local-first codebase analyzer

Usage:
  stackindex
  stackindex analyze [path] [--json] [--no-tui] [--ai] [--model llama3.2:3b] [--ai-debug]
  stackindex audit [path] [--json] [--allow-medium] [--allow-missing-tests] [--fail-on-low] [--ai]
  stackindex ask [path] "question" [--json] [--ai] [--model llama3.2:3b] [--ai-debug]
  stackindex plan <repo> "task" [--json]
  stackindex eval <repo> [--json]
  stackindex --help

Examples:
  stackindex analyze .
  stackindex analyze ./path-to-project --no-tui
  stackindex analyze . --json
  stackindex analyze . --ai --model llama3.2:3b
  stackindex audit .
  stackindex ask . "What is this project for?"
  stackindex ask . "Where are the API routes?"
  stackindex plan . "fix rule validation bug"
  stackindex eval .

Local Ollama model behavior varies. By default StackIndex tries llama3.2:3b,
then qwen:7b, then the deterministic StackIndex summary.

Audit mode is deterministic for CI: it fails only on static high or medium
findings and deployment-readiness blockers. Optional AI report content never
affects the audit exit code.

Ask mode answers from StackIndex's deterministic local analysis first. With
--ai, local Ollama may polish the bounded evidence, but AI is never required.
`

var launchTUI = tui.Run

func main() {
	if err := run(os.Args[1:]); err != nil {
		var failure auditFailure
		if errors.As(err, &failure) {
			os.Exit(failure.exitCode)
		}
		fmt.Fprintln(os.Stderr, "stackindex:", err)
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
	case "ask":
		return ask(args[1:])
	case "plan":
		return plan(args[1:])
	case "eval":
		return evalIndex(args[1:])
	default:
		return analyze(args, false)
	}
}

func plan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print search plan JSON to stdout")
	if err := fs.Parse(normalizePlanArgs(args)); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return errors.New(`plan requires a repo and task, for example: stackindex plan . "fix rule validation bug"`)
	}
	target := fs.Arg(0)
	task := strings.Join(fs.Args()[1:], " ")
	analysis, _, err := planner.LoadAnalysis(target)
	if err != nil {
		return err
	}
	result := planner.Plan(task, analysis)
	if *jsonOut {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(planner.FormatPlan(result))
	return nil
}

func evalIndex(args []string) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print eval result JSON to stdout")
	if err := fs.Parse(normalizePlanArgs(args)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("eval requires a repo, for example: stackindex eval .")
	}
	target := fs.Arg(0)
	analysis, _, err := planner.LoadAnalysis(target)
	if err != nil {
		return err
	}
	fixtures := planner.LoadFixtures(target)
	scores := make([]planner.Score, 0, len(fixtures))
	for _, fixture := range fixtures {
		scores = append(scores, planner.ScorePlan(fixture, planner.Plan(fixture.Task, analysis)))
	}
	if *jsonOut {
		data, err := json.MarshalIndent(scores, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(planner.FormatEval(scores))
	return nil
}

func ask(args []string) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print Q&A result JSON to stdout")
	enableAI := fs.Bool("ai", false, "enable optional local Ollama synthesis from bounded Q&A evidence")
	aiDebug := fs.Bool("ai-debug", false, "write local AI Q&A diagnostics under .stackindex/ai-debug/ask")
	model := fs.String("model", "", "Ollama model to use when --ai is enabled; default tries llama3.2:3b then qwen:7b")
	_ = fs.Bool("no-tui", false, "accepted for compatibility; ask mode never launches the TUI")
	if err := fs.Parse(normalizeAskArgs(args)); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New(`ask requires a question, for example: stackindex ask . "What is this project for?"`)
	}

	target := "."
	question := ""
	switch fs.NArg() {
	case 1:
		question = fs.Arg(0)
	default:
		target = fs.Arg(0)
		question = strings.Join(fs.Args()[1:], " ")
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return errors.New("ask question cannot be empty")
	}
	analyzeResult, err := app.Analyze(context.Background(), app.AnalyzeOptions{Path: target})
	if err != nil {
		return err
	}
	result, err := app.Ask(context.Background(), analyzeResult.Analysis, app.AskOptions{
		Root:     analyzeResult.Root,
		Question: question,
		UseAI:    *enableAI,
		Model:    *model,
		AIDebug:  *aiDebug,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		data, err := qa.MarshalJSON(result)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}
	fmt.Print(qa.FormatText(result))
	return nil
}

func analyze(args []string, auditMode bool) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON to stdout without launching TUI")
	noTUI := fs.Bool("no-tui", false, "run analysis and export reports without launching TUI")
	auditFlag := fs.Bool("audit", false, "run deterministic CI audit and fail on high or medium static findings")
	allowMedium := fs.Bool("allow-medium", false, "treat medium findings as audit warnings instead of blockers")
	allowMissingTests := fs.Bool("allow-missing-tests", false, "treat missing tests as an audit warning instead of a blocker")
	failOnLow := fs.Bool("fail-on-low", false, "treat low findings as audit blockers")
	enableAI := fs.Bool("ai", false, "enable optional local Ollama analysis")
	aiDebug := fs.Bool("ai-debug", false, "write local AI prompt/response diagnostics under .stackindex/ai-debug")
	model := fs.String("model", "", "Ollama model to use when --ai is enabled; default tries llama3.2:3b then qwen:7b")
	if err := fs.Parse(normalizeAnalyzeArgs(args)); err != nil {
		return err
	}
	auditCommand := auditMode
	auditMode = auditCommand || *auditFlag
	nonInteractiveAudit := auditCommand || (auditMode && (*noTUI || *jsonOut))

	target := "."
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	result, err := app.Analyze(context.Background(), app.AnalyzeOptions{
		Path:     target,
		RunAudit: auditMode,
		AuditOptions: audit.Options{
			AllowMedium:       *allowMedium,
			AllowMissingTests: *allowMissingTests,
			FailOnLow:         *failOnLow,
		},
		UseAI:   *enableAI,
		Model:   *model,
		AIDebug: *aiDebug,
	})
	if err != nil {
		return err
	}
	root := result.Root
	analysis := result.Analysis
	if analysis.AI != nil && analysis.AI.Warning != "" {
		fmt.Fprintf(os.Stderr, "stackindex: %s\n", analysis.AI.Warning)
	}

	if *jsonOut {
		if auditMode {
			if err := app.ExportReports(root, analysis); err != nil {
				return err
			}
		}
		data, err := report.MarshalJSON(analysis)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		if auditMode {
			return auditError(analysis.Audit)
		}
		return nil
	}

	if err := app.ExportReports(root, analysis); err != nil {
		return err
	}
	if nonInteractiveAudit {
		printAuditSummary(analysis)
		return auditError(analysis.Audit)
	}
	if *noTUI {
		printExportSummary(root)
		return nil
	}
	return launchTUI(analysis, root)
}

func printExportSummary(root string) {
	outDir := filepath.Join(root, ".stackindex")
	fmt.Printf("StackIndex analysis exported to %s\n", outDir)
	fmt.Printf("JSON: %s\n", filepath.Join(outDir, "analysis.json"))
	fmt.Printf("Markdown: %s\n", filepath.Join(outDir, "reports", "repo-index.md"))
	fmt.Printf("Full Markdown: %s\n", filepath.Join(outDir, "reports", "repo-index.full.md"))
	fmt.Println("Note: .stackindex is a hidden folder on macOS Finder. Press Cmd+Shift+. in Finder to show hidden files.")
}

type auditFailure struct {
	exitCode int
}

func (e auditFailure) Error() string {
	return fmt.Sprintf("audit failed with exit code %d", e.exitCode)
}

func auditError(result *models.AuditResult) error {
	if result == nil || result.ExitCode == 0 {
		return nil
	}
	return auditFailure{exitCode: result.ExitCode}
}

func printAuditSummary(analysis *models.Analysis) {
	result := analysis.Audit
	if result == nil {
		return
	}
	status := "failed"
	if result.Passed {
		status = "passed"
	}
	fmt.Printf("StackIndex audit: %s\n", status)
	if !result.Passed {
		fmt.Println()
		for _, reason := range result.Reasons {
			fmt.Printf("* %s\n", reason)
		}
	}
	fmt.Printf("Report: %s\n", filepath.Join(".stackindex", "reports", "repo-index.md"))
	fmt.Printf("JSON: %s\n", filepath.Join(".stackindex", "analysis.json"))
}

func normalizeAnalyzeArgs(args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json", "--no-tui", "--audit", "--allow-medium", "--allow-missing-tests", "--fail-on-low", "--ai", "--ai-debug", "-json", "-no-tui", "-audit", "-allow-medium", "-allow-missing-tests", "-fail-on-low", "-ai", "-ai-debug":
			flags = append(flags, arg)
		case "--model", "-model":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		default:
			if strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-model=") || isBoolFlagAssignment(arg) {
				flags = append(flags, arg)
			} else {
				positionals = append(positionals, arg)
			}
		}
	}
	return append(flags, positionals...)
}

func normalizeAskArgs(args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json", "--ai", "--ai-debug", "--no-tui", "-json", "-ai", "-ai-debug", "-no-tui":
			flags = append(flags, arg)
		case "--model", "-model":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		default:
			if strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-model=") || isAskBoolFlagAssignment(arg) {
				flags = append(flags, arg)
			} else {
				positionals = append(positionals, arg)
			}
		}
	}
	return append(flags, positionals...)
}

func normalizePlanArgs(args []string) []string {
	var flags []string
	var positionals []string
	for _, arg := range args {
		if arg == "--json" || arg == "-json" || strings.HasPrefix(arg, "--json=") || strings.HasPrefix(arg, "-json=") {
			flags = append(flags, arg)
		} else {
			positionals = append(positionals, arg)
		}
	}
	return append(flags, positionals...)
}

func isBoolFlagAssignment(arg string) bool {
	for _, prefix := range []string{"--json=", "--no-tui=", "--audit=", "--allow-medium=", "--allow-missing-tests=", "--fail-on-low=", "--ai=", "--ai-debug=", "-json=", "-no-tui=", "-audit=", "-allow-medium=", "-allow-missing-tests=", "-fail-on-low=", "-ai=", "-ai-debug="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func isAskBoolFlagAssignment(arg string) bool {
	for _, prefix := range []string{"--json=", "--ai=", "--ai-debug=", "--no-tui=", "-json=", "-ai=", "-ai-debug=", "-no-tui="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
