package main

import (
	"context"
	"errors"
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
  stackmap audit [path] [--json] [--allow-medium] [--allow-missing-tests] [--fail-on-low] [--ai]
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
findings and deployment-readiness blockers. Optional AI report content never
affects the audit exit code.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		var failure auditFailure
		if errors.As(err, &failure) {
			os.Exit(failure.exitCode)
		}
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
	allowMedium := fs.Bool("allow-medium", false, "treat medium findings as audit warnings instead of blockers")
	allowMissingTests := fs.Bool("allow-missing-tests", false, "treat missing tests as an audit warning instead of a blocker")
	failOnLow := fs.Bool("fail-on-low", false, "treat low findings as audit blockers")
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
	if auditMode {
		analysis.Audit = EvaluateAudit(analysis, AuditOptions{
			AllowMedium:       *allowMedium,
			AllowMissingTests: *allowMissingTests,
			FailOnLow:         *failOnLow,
		})
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
			return auditError(analysis.Audit)
		}
		return nil
	}

	if err := report.ExportAll(root, analysis); err != nil {
		return err
	}
	if auditMode {
		printAuditSummary(analysis)
		return auditError(analysis.Audit)
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

type AuditOptions struct {
	AllowMedium       bool
	AllowMissingTests bool
	FailOnLow         bool
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

func EvaluateAudit(analysis *models.Analysis, opts AuditOptions) *models.AuditResult {
	result := &models.AuditResult{
		Mode:              "deployment-readiness",
		AllowMedium:       opts.AllowMedium,
		AllowMissingTests: opts.AllowMissingTests,
		FailOnLow:         opts.FailOnLow,
	}

	high, medium, low := auditSeverityCounts(analysis.Findings)
	if high > 0 {
		result.Reasons = append(result.Reasons, pluralizeCount(high, "high finding")+" detected.")
	}
	if medium > 0 {
		message := pluralizeCount(medium, "medium finding") + " detected."
		if opts.AllowMedium {
			result.Warnings = append(result.Warnings, message)
		} else {
			result.Reasons = append(result.Reasons, message)
		}
	}
	if low > 0 {
		message := pluralizeCount(low, "low finding") + " detected."
		if opts.FailOnLow {
			result.Reasons = append(result.Reasons, message)
		} else {
			result.Warnings = append(result.Warnings, message)
		}
	}
	if !auditStackDetected(analysis.Stack) {
		result.Reasons = append(result.Reasons, "No stack was detected.")
	}
	if analysis.Env.UsesEnvVars && analysis.Env.ExampleFile == "" {
		result.Reasons = append(result.Reasons, "Environment variables were detected but no `.env.example` file was found.")
	}
	if len(analysis.Stack.Deployment) > 0 && !analysis.Deployment.HasHealthEndpoint {
		result.Reasons = append(result.Reasons, "Deployment target detected but no health endpoint was found.")
	}
	if !auditTestsDetected(analysis.Tests) {
		message := "Tests were not detected."
		if opts.AllowMissingTests {
			result.Warnings = append(result.Warnings, message)
		} else {
			result.Reasons = append(result.Reasons, message)
		}
	}

	result.Passed = len(result.Reasons) == 0
	if result.Passed {
		result.ExitCode = 0
	} else {
		result.ExitCode = 1
	}
	return result
}

func auditSeverityCounts(findings []models.Finding) (int, int, int) {
	var high, medium, low int
	for _, finding := range findings {
		switch finding.Severity {
		case models.SeverityHigh:
			high++
		case models.SeverityMedium:
			medium++
		case models.SeverityLow:
			low++
		}
	}
	return high, medium, low
}

func auditStackDetected(stack models.StackInfo) bool {
	return len(stack.Languages)+len(stack.Frameworks)+len(stack.Libraries)+len(stack.Databases)+len(stack.Testing)+len(stack.Deployment) > 0
}

func auditTestsDetected(tests models.TestAnalysis) bool {
	return tests.HasTestFiles || tests.HasTestScript
}

func pluralizeCount(count int, label string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", label)
	}
	return fmt.Sprintf("%d %ss", count, label)
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
	fmt.Printf("StackMap audit: %s\n", status)
	if !result.Passed {
		fmt.Println()
		for _, reason := range result.Reasons {
			fmt.Printf("* %s\n", reason)
		}
	}
	fmt.Printf("Report: %s\n", filepath.Join(".stackmap", "reports", "repo-report.md"))
	fmt.Printf("JSON: %s\n", filepath.Join(".stackmap", "analysis.json"))
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

func isBoolFlagAssignment(arg string) bool {
	for _, prefix := range []string{"--json=", "--no-tui=", "--audit=", "--allow-medium=", "--allow-missing-tests=", "--fail-on-low=", "--ai=", "--ai-debug=", "-json=", "-no-tui=", "-audit=", "-allow-medium=", "-allow-missing-tests=", "-fail-on-low=", "-ai=", "-ai-debug="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
