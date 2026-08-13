package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/gay00ung/aargrade/internal/migration"
)

func runMigrationPlan(args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	targetAGP := flags.String("target-agp", "", "target Android Gradle Plugin version")
	currentAGP := flags.String("current-agp", "", "verified current AGP override")
	format := flags.String("format", "text", "output format: text or json")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade plan --target-agp VERSION [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nCreate a read-only, ordered AGP migration plan.")
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
		_, _ = fmt.Fprintf(stderr, "aargrade plan: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *targetAGP == "" {
		_, _ = fmt.Fprintln(stderr, "aargrade plan: --target-agp is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade plan: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	plan, err := migration.Create(migration.Options{
		ProjectPath:        *projectPath,
		TargetAGP:          *targetAGP,
		CurrentAGPOverride: *currentAGP,
		ToolVersion:        version,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade plan: %s\n", err)
		return 2
	}
	if *format == "json" {
		err = migration.RenderJSON(stdout, plan)
	} else {
		err = migration.RenderText(stdout, plan)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade plan: write report: %s\n", err)
		return 2
	}
	if !plan.Ready {
		return 1
	}
	return 0
}
