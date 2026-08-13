package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/gay00ung/aargrade/internal/doctor"
	"github.com/gay00ung/aargrade/internal/host"
	"github.com/gay00ung/aargrade/internal/model"
	"github.com/gay00ung/aargrade/internal/report"
)

func Run(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 {
		printRootUsage(stdout)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return 0
	case "version", "--version":
		_, _ = fmt.Fprintf(stdout, "aargrade %s\n", version)
		return 0
	case "doctor":
		return runDoctor(args[1:], stdout, stderr, version)
	case "host":
		return runHost(args[1:], stdout, stderr)
	case "plan":
		return runMigrationPlan(args[1:], stdout, stderr, version)
	case "verify":
		return runVerify(args[1:], stdout, stderr, version)
	case "matrix":
		return runMatrix(args[1:], stdout, stderr, version)
	case "mcp":
		return runMCP(args[1:], stdout, stderr, version)
	default:
		_, _ = fmt.Fprintf(stderr, "aargrade: unknown command %q\n\n", args[0])
		printRootUsage(stderr)
		return 2
	}
}

func runHost(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHostUsage(stdout)
		return 0
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printHostUsage(stdout)
		return 0
	}
	switch args[0] {
	case "add":
		return runHostAdd(args[1:], stdout, stderr)
	case "remove":
		return runHostRemove(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "aargrade host: unknown operation %q\n\n", args[0])
		printHostUsage(stderr)
		return 2
	}
}

func runHostAdd(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("host add", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	libraryPath := flags.String("library", "", "Android library Gradle path (required when ambiguous)")
	modulePath := flags.String("module", ":aargrade-upgrade-host", "temporary host Gradle path")
	agpVersion := flags.String("agp-version", "", "literal current AGP version override")
	compileSDK := flags.Int("compile-sdk", 0, "literal compile SDK override")
	minSDK := flags.Int("min-sdk", 0, "minimum SDK override (defaults to library value or 21)")
	apply := flags.Bool("apply", false, "apply the previewed changes")
	format := flags.String("format", "text", "output format: text or json")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade host add [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nPreview an owned temporary Android application host. Use --apply to write it.")
		_, _ = fmt.Fprintln(flags.Output(), "\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "aargrade host add: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade host add: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	plan, err := host.Add(host.Options{
		ProjectPath: *projectPath, LibraryPath: *libraryPath, ModulePath: *modulePath,
		AGPVersion: *agpVersion, CompileSDK: *compileSDK, MinSDK: *minSDK, Apply: *apply,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade host add: %s\n", err)
		return 2
	}
	if err := renderHostPlan(stdout, plan, *format); err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade host add: write report: %s\n", err)
		return 2
	}
	return 0
}

func runHostRemove(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("host remove", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	apply := flags.Bool("apply", false, "apply the previewed removal")
	format := flags.String("format", "text", "output format: text or json")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade host remove [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nPreview removal of an unchanged AARGrade-owned host. Use --apply to remove it.")
		_, _ = fmt.Fprintln(flags.Output(), "\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "aargrade host remove: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade host remove: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	plan, err := host.Remove(host.Options{ProjectPath: *projectPath, Apply: *apply})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade host remove: %s\n", err)
		return 2
	}
	if err := renderHostPlan(stdout, plan, *format); err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade host remove: write report: %s\n", err)
		return 2
	}
	return 0
}

func renderHostPlan(writer io.Writer, plan host.Plan, format string) error {
	if format == "json" {
		return host.RenderJSON(writer, plan)
	}
	return host.RenderText(writer, plan)
}

func runDoctor(args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	format := flags.String("format", "text", "output format: text or json")
	failOn := flags.String("fail-on", "error", "exit 1 threshold: info, warn, error, or never")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade doctor [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nRead-only static diagnosis of an Android Gradle project.")
		_, _ = fmt.Fprintln(flags.Output(), "\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "aargrade doctor: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade doctor: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	threshold, thresholdEnabled, ok := parseThreshold(*failOn)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "aargrade doctor: unsupported --fail-on value %q (use info, warn, error, or never)\n", *failOn)
		return 2
	}

	result, err := doctor.Analyze(*projectPath, version)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade doctor: %s\n", err)
		return 2
	}
	if *format == "json" {
		err = report.JSON(stdout, result)
	} else {
		err = report.Text(stdout, result)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade doctor: write report: %s\n", err)
		return 2
	}
	if thresholdEnabled && meetsThreshold(result.Findings, threshold) {
		return 1
	}
	return 0
}

func parseThreshold(value string) (model.Severity, bool, bool) {
	if strings.EqualFold(value, "never") {
		return "", false, true
	}
	severity, ok := model.ParseSeverity(value)
	return severity, true, ok
}

func meetsThreshold(findings []model.Finding, threshold model.Severity) bool {
	for _, finding := range findings {
		if model.SeverityRank(finding.Severity) >= model.SeverityRank(threshold) {
			return true
		}
	}
	return false
}

func printRootUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, `AARGrade — Upgrade your AAR without breaking consumers.

Usage:
  aargrade <command> [options]

Commands:
  doctor   Diagnose an Android Gradle project without writing to it
  host     Preview or manage an owned temporary application host
  plan     Create a read-only, version-aware AGP migration plan
  verify   Build or inspect an AAR and compare it with a baseline
  matrix   Build an AAR in isolated Java and Kotlin consumer cells
  mcp      Serve the CLI capabilities to MCP-compatible agents
  version  Print the CLI version

Run "aargrade <command> --help" for command help.`)
}

func printHostUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, `Usage:
  aargrade host add [options]
  aargrade host remove [options]

Operations:
  add      Preview or create an owned temporary application host
  remove   Preview or remove an unchanged owned host

Both operations are preview-only unless --apply is supplied.`)
}
