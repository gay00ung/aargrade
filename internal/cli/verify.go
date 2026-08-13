package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	verification "github.com/gay00ung/aargrade/internal/verify"
)

type repeatedStrings []string

func (values *repeatedStrings) String() string {
	return fmt.Sprintf("%v", []string(*values))
}

func (values *repeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runVerify(args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root used when building a candidate")
	libraryPath := flags.String("library", "", "Android library Gradle path")
	variant := flags.String("variant", "release", "Android library build variant")
	candidateAAR := flags.String("candidate-aar", "", "existing candidate AAR; skips project build")
	baselineAAR := flags.String("baseline-aar", "", "released baseline AAR for compatibility comparison")
	format := flags.String("format", "text", "output format: text or json")
	timeout := flags.Duration("timeout", 10*time.Minute, "timeout for each Gradle command")
	var gradleArgs repeatedStrings
	flags.Var(&gradleArgs, "gradle-arg", "extra Gradle argument; may be repeated")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade verify [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nBuild or inspect a candidate AAR and optionally compare it with a released baseline.")
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
		_, _ = fmt.Fprintf(stderr, "aargrade verify: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade verify: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	if *timeout <= 0 {
		_, _ = fmt.Fprintln(stderr, "aargrade verify: --timeout must be positive")
		return 2
	}
	report, err := verification.Run(verification.Options{
		ProjectPath: *projectPath, LibraryPath: *libraryPath, Variant: *variant,
		CandidateAAR: *candidateAAR, BaselineAAR: *baselineAAR,
		GradleArgs: gradleArgs, Timeout: *timeout, ToolVersion: version,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade verify: %s\n", err)
		return 2
	}
	if *format == "json" {
		err = verification.RenderJSON(stdout, report)
	} else {
		err = verification.RenderText(stdout, report)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade verify: write report: %s\n", err)
		return 2
	}
	if report.Verdict == "fail" {
		return 1
	}
	return 0
}
