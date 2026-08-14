package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/gay00ung/aargrade/internal/migration"
)

func runMigrate(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) > 0 {
		switch args[0] {
		case "rollback":
			return runMigrateRollback(args[1:], stdout, stderr)
		case "help", "-h", "--help":
			printMigrateUsage(stdout)
			return 0
		}
	}
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	targetAGP := flags.String("target-agp", "", "target Android Gradle Plugin version")
	currentAGP := flags.String("current-agp", "", "verified current AGP override")
	apply := flags.Bool("apply", false, "apply the previewed migration")
	format := flags.String("format", "text", "output format: text or json")
	flags.Usage = func() { printMigrateUsage(flags.Output()) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *targetAGP == "" {
		_, _ = fmt.Fprintln(stderr, "aargrade migrate: --target-agp is required")
		return 2
	}
	if !validMutationFormat(*format) {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	result, err := migration.Migrate(migration.MutationOptions{
		ProjectPath:        *projectPath,
		TargetAGP:          *targetAGP,
		CurrentAGPOverride: *currentAGP,
		ToolVersion:        version,
		Apply:              *apply,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate: %s\n", err)
		return 2
	}
	if err := renderMutation(stdout, result, *format); err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate: write report: %s\n", err)
		return 2
	}
	if !result.Ready {
		return 1
	}
	return 0
}

func runMigrateRollback(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("migrate rollback", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	apply := flags.Bool("apply", false, "apply the previewed rollback")
	format := flags.String("format", "text", "output format: text or json")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade migrate rollback [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nRestore unchanged AARGrade-migrated files from local ownership state. Preview-only by default.")
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
		_, _ = fmt.Fprintf(stderr, "aargrade migrate rollback: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if !validMutationFormat(*format) {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate rollback: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	result, err := migration.Rollback(migration.RollbackOptions{ProjectPath: *projectPath, Apply: *apply})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate rollback: %s\n", err)
		return 2
	}
	if err := renderMutation(stdout, result, *format); err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade migrate rollback: write report: %s\n", err)
		return 2
	}
	if !result.Ready {
		return 1
	}
	return 0
}

func validMutationFormat(format string) bool {
	return format == "text" || format == "json"
}

func renderMutation(writer io.Writer, result migration.MutationResult, format string) error {
	if format == "json" {
		return migration.RenderMutationJSON(writer, result)
	}
	return migration.RenderMutationText(writer, result)
}

func printMigrateUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, `Usage:
  aargrade migrate --target-agp VERSION [options]
  aargrade migrate rollback [options]

The default is a no-write preview. --apply updates only statically proven AGP,
Wrapper, and bounded AGP 9 Built-in Kotlin declarations and records exact local
rollback state. Unsupported Gradle logic is reported as a blocker.

Migration options:
  --project PATH       Gradle project root (default ".")
  --target-agp VERSION Required target AGP version
  --current-agp VALUE  Verified current AGP override
  --apply              Apply the reviewed preview
  --format VALUE       text or json (default "text")`)
}
