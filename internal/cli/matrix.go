package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	consumer "github.com/gay00ung/aargrade/internal/matrix"
)

type keyValueMap map[string]string

func (values *keyValueMap) String() string {
	return fmt.Sprintf("%v", map[string]string(*values))
}

func (values *keyValueMap) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("expected KEY=VALUE, got %q", value)
	}
	if *values == nil {
		*values = keyValueMap{}
	}
	(*values)[parts[0]] = parts[1]
	return nil
}

func runMatrix(args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("matrix", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "aargrade.yml", "matrix YAML configuration")
	candidateAAR := flags.String("candidate-aar", "", "candidate AAR override")
	baselineAAR := flags.String("baseline-aar", "", "baseline AAR override")
	workDirectory := flags.String("work-dir", "", "owned evidence directory (default: .aargrade/matrix beside config)")
	format := flags.String("format", "text", "output format: text or json")
	allowDownloads := flags.Bool("allow-downloads", false, "download checksum-verified Gradle distributions when absent")
	failFast := flags.Bool("fail-fast", false, "stop after the first failed or regressed cell")
	timeout := flags.Duration("timeout", 15*time.Minute, "timeout for each consumer build")
	var cells repeatedStrings
	var javaHomes keyValueMap
	var gradleBins keyValueMap
	flags.Var(&cells, "cell", "run one named cell; may be repeated")
	flags.Var(&javaHomes, "java-home", "JDK_MAJOR=JAVA_HOME override; may be repeated")
	flags.Var(&gradleBins, "gradle-bin", "GRADLE_VERSION=EXECUTABLE override; may be repeated")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: aargrade matrix [options]")
		_, _ = fmt.Fprintln(flags.Output(), "\nBuild a candidate and optional baseline AAR in isolated Java/Kotlin consumer projects.")
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
		_, _ = fmt.Fprintf(stderr, "aargrade matrix: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "aargrade matrix: unsupported format %q (use text or json)\n", *format)
		return 2
	}
	if *timeout <= 0 {
		_, _ = fmt.Fprintln(stderr, "aargrade matrix: --timeout must be positive")
		return 2
	}
	parsedJavaHomes := map[int]string{}
	for major, path := range javaHomes {
		parsed, err := strconv.Atoi(major)
		if err != nil || parsed <= 0 {
			_, _ = fmt.Fprintf(stderr, "aargrade matrix: invalid JDK major %q in --java-home\n", major)
			return 2
		}
		parsedJavaHomes[parsed] = path
	}
	report, err := consumer.Run(consumer.Options{
		ConfigPath: *configPath, CandidateAAR: *candidateAAR, BaselineAAR: *baselineAAR,
		WorkDirectory: *workDirectory, SelectedCells: cells,
		AllowDownloads: *allowDownloads, KeepGoing: !*failFast, Timeout: *timeout,
		ToolVersion: version, JavaHomes: parsedJavaHomes,
		GradleExecutables: map[string]string(gradleBins),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade matrix: %s\n", err)
		return 2
	}
	if *format == "json" {
		err = consumer.RenderJSON(stdout, report)
	} else {
		err = consumer.RenderText(stdout, report)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade matrix: write report: %s\n", err)
		return 2
	}
	switch report.Verdict {
	case "pass":
		return 0
	case "fail":
		return 1
	default:
		return 2
	}
}
