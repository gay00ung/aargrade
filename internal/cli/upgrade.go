package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gay00ung/aargrade/internal/upgrade"
)

func runUpgrade(args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectPath := flags.String("project", ".", "Gradle project root")
	targetAGP := flags.String("target-agp", "", "target Android Gradle Plugin version")
	currentAGP := flags.String("current-agp", "", "verified current AGP override")
	libraryPath := flags.String("library", "", "Android library Gradle path")
	variant := flags.String("variant", "release", "Android library build variant")
	baselineAAR := flags.String("baseline-aar", "", "released baseline AAR for compatibility comparison")
	matrixConfig := flags.String("matrix-config", "", "optional consumer matrix YAML configuration")
	matrixWorkDirectory := flags.String("matrix-work-dir", "", "owned matrix evidence directory")
	apply := flags.Bool("apply", false, "apply repairs and run Gradle/AAR evidence")
	keepFailedChanges := flags.Bool("keep-failed-changes", false, "preserve owned changes when verification fails")
	allowDownloads := flags.Bool("allow-downloads", false, "allow checksum-verified matrix Gradle downloads")
	failFast := flags.Bool("fail-fast", false, "stop consumer matrix after the first regression")
	format := flags.String("format", "text", "output format: text or json")
	timeout := flags.Duration("timeout", 15*time.Minute, "timeout for each Gradle command")
	var gradleArgs repeatedStrings
	var cells repeatedStrings
	var javaHomes keyValueMap
	var gradleBins keyValueMap
	flags.Var(&gradleArgs, "gradle-arg", "extra project Gradle argument; may be repeated")
	flags.Var(&cells, "cell", "run one named matrix cell; may be repeated")
	flags.Var(&javaHomes, "java-home", "JDK_MAJOR=JAVA_HOME matrix override; may be repeated")
	flags.Var(&gradleBins, "gradle-bin", "GRADLE_VERSION=EXECUTABLE matrix override; may be repeated")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade upgrade --target-agp VERSION [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nPreview or run diagnosis, deterministic repairs, AGP migration, Gradle verification, AAR comparison, and an optional consumer matrix.")
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
		_, _ = fmt.Fprintf(stderr, "aargrade upgrade: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *targetAGP == "" {
		_, _ = fmt.Fprintln(stderr, "aargrade upgrade: --target-agp is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade upgrade: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	if *timeout <= 0 {
		_, _ = fmt.Fprintln(stderr, "aargrade upgrade: --timeout must be positive")
		return 2
	}
	parsedJavaHomes := map[int]string{}
	for major, path := range javaHomes {
		parsed, err := strconv.Atoi(major)
		if err != nil || parsed <= 0 {
			_, _ = fmt.Fprintf(stderr, "aargrade upgrade: invalid JDK major %q in --java-home\n", major)
			return 2
		}
		parsedJavaHomes[parsed] = path
	}
	report, err := upgrade.Run(upgrade.Options{
		ProjectPath:         *projectPath,
		TargetAGP:           *targetAGP,
		CurrentAGPOverride:  *currentAGP,
		LibraryPath:         *libraryPath,
		Variant:             *variant,
		BaselineAAR:         *baselineAAR,
		MatrixConfig:        *matrixConfig,
		MatrixWorkDirectory: *matrixWorkDirectory,
		SelectedCells:       cells,
		Apply:               *apply,
		RollbackOnFailure:   !*keepFailedChanges,
		AllowDownloads:      *allowDownloads,
		KeepGoing:           !*failFast,
		GradleArgs:          gradleArgs,
		Timeout:             *timeout,
		ToolVersion:         version,
		JavaHomes:           parsedJavaHomes,
		GradleExecutables:   map[string]string(gradleBins),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade upgrade: %s\n", err)
		return 2
	}
	if *format == "json" {
		err = upgrade.RenderJSON(stdout, report)
	} else {
		err = upgrade.RenderText(stdout, report)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade upgrade: write report: %s\n", err)
		return 2
	}
	switch report.Verdict {
	case "preview", "pass":
		return 0
	case "blocked", "fail":
		return 1
	default:
		return 2
	}
}
