package upgrade

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestUpgradePreviewDoesNotWrite(t *testing.T) {
	root := copyUpgradeFixture(t)
	before := upgradeSnapshot(t, root)
	report, err := Run(Options{ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "preview" || report.Applied || !report.Migration.Ready || len(report.Migration.Changes) == 0 {
		t.Fatalf("preview = %#v", report)
	}
	if after := upgradeSnapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("preview changed project\nbefore=%v\nafter=%v", before, after)
	}
}

func TestUpgradeGradleFailureIsAnalyzedAndRolledBack(t *testing.T) {
	root := copyUpgradeFixture(t)
	before := upgradeSnapshot(t, root)
	writeFailingWrapper(t, root)
	report, err := Run(Options{
		ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test",
		Apply: true, RollbackOnFailure: true, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || !report.Applied || !report.RolledBack || report.Failure == nil || report.Failure.Category != "namespace" {
		t.Fatalf("failed upgrade report = %#v", report)
	}
	if _, err := os.Stat(filepath.Join(root, ".aargrade", "state", "migration.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback state remains: %v", err)
	}
	after := upgradeSnapshot(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed upgrade did not restore configuration\nbefore=%v\nafter=%v", before, after)
	}
}

func TestUpgradePartialMigrationWriteIsRolledBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix directory write bits")
	}
	root := copyNamedUpgradeFixture(t, "upgrade-agent")
	before := upgradeSnapshot(t, root)
	writePassingWrapper(t, root)
	manifestDirectory := filepath.Join(root, "sdk", "src", "main")
	if err := os.Chmod(manifestDirectory, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(manifestDirectory, 0o755) }()

	report, err := Run(Options{
		ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test",
		Apply: true, RollbackOnFailure: true, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "incomplete" || !report.Applied || !report.RolledBack ||
		report.Failure == nil || report.Failure.Category != "migration-apply" || !report.Migration.TransactionStarted {
		t.Fatalf("partial apply report = %#v", report)
	}
	if after := upgradeSnapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("partial migration write was not restored\nbefore=%v\nafter=%v", before, after)
	}
}

func TestUpgradeAcceptsEquivalentPreExistingRootDryRunFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scenario wrapper is covered by platform-independent comparison tests")
	}
	root := copyUpgradeFixture(t)
	writeCandidateAAR(t, root)
	writeDryRunScenarioWrapper(t, root, "pre-existing")

	report, err := Run(Options{
		ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test",
		Apply: true, RollbackOnFailure: true, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || !report.Applied || report.RolledBack || report.BeforeUpgradeDryRun == nil || report.BeforeUpgradeDryRun.Status != "fail" {
		t.Fatalf("upgrade report = %#v", report)
	}
	if report.Verification == nil || report.Verification.RootDryRun == nil || report.Verification.RootDryRun.Verdict != "pre-existing-failure" {
		t.Fatalf("root dry-run comparison = %#v", report.Verification)
	}
	if statusForUpgradeCheck(report, "gradle.root-dry-run") != "warning" || statusForUpgradeCommand(report, "aar-dry-run") != "pass" {
		t.Fatalf("verification evidence = %#v", report.Verification)
	}
}

func TestUpgradeRollsBackNewOrDifferentRootDryRunFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scenario wrapper is covered by platform-independent comparison tests")
	}
	for _, scenario := range []string{"new-failure", "different-failure"} {
		t.Run(scenario, func(t *testing.T) {
			root := copyUpgradeFixture(t)
			writeCandidateAAR(t, root)
			writeDryRunScenarioWrapper(t, root, scenario)
			before := upgradeSnapshot(t, root)

			report, err := Run(Options{
				ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test",
				Apply: true, RollbackOnFailure: true, Timeout: 10 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.Verdict != "fail" || !report.RolledBack || report.Verification == nil || report.Verification.RootDryRun == nil || report.Verification.RootDryRun.Verdict != "regression" {
				t.Fatalf("upgrade report = %#v", report)
			}
			if after := upgradeSnapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Fatalf("regression did not restore configuration\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

func TestUpgradeRecordsImprovedRootDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scenario wrapper is covered by platform-independent comparison tests")
	}
	root := copyUpgradeFixture(t)
	writeCandidateAAR(t, root)
	writeDryRunScenarioWrapper(t, root, "improved")

	report, err := Run(Options{
		ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test",
		Apply: true, RollbackOnFailure: true, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || report.Verification == nil || report.Verification.RootDryRun == nil || report.Verification.RootDryRun.Verdict != "improved" {
		t.Fatalf("upgrade report = %#v", report)
	}
}

func TestUpgradeRollsBackSelectedLibraryFailureAfterPreExistingRootFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scenario wrapper is covered by platform-independent comparison tests")
	}
	root := copyUpgradeFixture(t)
	writeCandidateAAR(t, root)
	writeDryRunScenarioWrapper(t, root, "pre-existing-module-failure")
	before := upgradeSnapshot(t, root)

	report, err := Run(Options{
		ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test",
		Apply: true, RollbackOnFailure: true, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || !report.RolledBack || report.Verification == nil || report.Verification.RootDryRun == nil || report.Verification.RootDryRun.Verdict != "pre-existing-failure" {
		t.Fatalf("upgrade report = %#v", report)
	}
	if statusForUpgradeCommand(report, "gradle-dry-run") != "warning" || statusForUpgradeCommand(report, "aar-dry-run") != "fail" {
		t.Fatalf("command evidence = %#v", report.Verification.Commands)
	}
	if after := upgradeSnapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("selected-library failure did not restore configuration\nbefore=%v\nafter=%v", before, after)
	}
}

func TestAnalyzeFailureClassifiesKnownAGPProblems(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"Namespace not specified. Specify a namespace", "namespace"},
		{"BuildConfig contains custom fields, but the feature is disabled", "buildconfig"},
		{"Cannot add extension after org.jetbrains.kotlin.android", "built-in-kotlin"},
		{"Inconsistent JVM targets between Java and Kotlin compile tasks: 11 and 17.", "jvm-target"},
		{"Could not cast to BaseExtension", "agp9-dsl"},
		{"SDK location not found", "android-sdk"},
	}
	for _, test := range tests {
		if got := analyzeFailure("gradle-help", errors.New(test.message)); got.Category != test.want {
			t.Fatalf("%q category = %q, want %q", test.message, got.Category, test.want)
		}
	}
}

func copyUpgradeFixture(t *testing.T) string {
	t.Helper()
	return copyNamedUpgradeFixture(t, "migrate-kotlin-catalog")
}

func copyNamedUpgradeFixture(t *testing.T, name string) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", name))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "project with space")
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func writeFailingWrapper(t *testing.T, root string) {
	t.Helper()
	unix := "#!/bin/sh\necho 'Namespace not specified. Specify a namespace' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(root, "gradlew"), []byte(unix), 0o755); err != nil {
		t.Fatal(err)
	}
	windows := "@echo off\r\necho Namespace not specified. Specify a namespace 1>&2\r\nexit /b 1\r\n"
	if err := os.WriteFile(filepath.Join(root, "gradlew.bat"), []byte(windows), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Join(root, "gradlew"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writePassingWrapper(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "gradlew"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gradlew.bat"), []byte("@echo off\r\nexit /b 0\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDryRunScenarioWrapper(t *testing.T, root, scenario string) {
	t.Helper()
	failExisting := `printf '%s\n' 'FAILURE: Build failed with an exception.' '* What went wrong:' "Could not determine the dependencies of task ':plugins:sdk-plugin-google-lvl:adjustLvlPluginJar'." "> Task with name 'packageReleaseAssets' not found in project ':plugins:sdk-plugin-google-lvl'." '* Try:' 'Run with --stacktrace option.' >&2
exit 1
`
	failExistingAfter := `printf '%s\n' 'Starting a Gradle Daemon' 'FAILURE: Build failed with an exception.' '* What went wrong:' "  Could not   determine the dependencies of task ':plugins:sdk-plugin-google-lvl:adjustLvlPluginJar'." "> Task with name 'packageReleaseAssets' not found in project ':plugins:sdk-plugin-google-lvl'." '* Try:' 'Run with --info option.' >&2
exit 1
`
	failDifferent := `printf '%s\n' 'FAILURE: Build failed with an exception.' '* What went wrong:' 'Namespace not specified. Specify a namespace.' '* Try:' 'Run with --stacktrace option.' >&2
exit 1
`
	pass := "exit 0\n"
	before, after, moduleDryRun := pass, pass, pass
	switch scenario {
	case "pre-existing":
		before, after = failExisting, failExistingAfter
	case "pre-existing-module-failure":
		before, after, moduleDryRun = failExisting, failExistingAfter, failDifferent
	case "new-failure":
		after = failExisting
	case "different-failure":
		before, after = failExisting, failDifferent
	case "improved":
		before = failExisting
	default:
		t.Fatalf("unknown dry-run scenario %q", scenario)
	}
	script := `#!/bin/sh
args="$*"
case "$args" in
  help*)
    exit 0
    ;;
  "build --dry-run --no-daemon"*)
    if grep -q 'agp = "9.2.0"' gradle/libs.versions.toml; then
` + after + `    else
` + before + `    fi
    ;;
  ":sdk:assembleRelease --dry-run --no-daemon"*)
` + moduleDryRun + `
    ;;
  ":sdk:assembleRelease --no-daemon"*)
    exit 0
    ;;
esac
printf '%s\n' "unexpected arguments: $args" >&2
exit 9
`
	if err := os.WriteFile(filepath.Join(root, "gradlew"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeCandidateAAR(t *testing.T, root string) {
	t.Helper()
	var classes bytes.Buffer
	classesWriter := zip.NewWriter(&classes)
	if err := classesWriter.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sdk", "build", "outputs", "aar", "sdk-release.aar")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range map[string][]byte{
		"AndroidManifest.xml": []byte("<manifest />"),
		"classes.jar":         classes.Bytes(),
		"META-INF/com/android/build/gradle/aar-metadata.properties": []byte("aarFormatVersion=1.0\n"),
	} {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write(content); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func statusForUpgradeCheck(report Report, id string) string {
	if report.Verification == nil {
		return ""
	}
	for _, check := range report.Verification.Checks {
		if check.ID == id {
			return string(check.Status)
		}
	}
	return ""
}

func statusForUpgradeCommand(report Report, name string) string {
	if report.Verification == nil {
		return ""
	}
	for _, command := range report.Verification.Commands {
		if command.Name == name {
			return string(command.Status)
		}
	}
	return ""
}

func upgradeSnapshot(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == ".aargrade" || strings.HasPrefix(relative, ".aargrade"+string(filepath.Separator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || relative == "gradlew" || relative == "gradlew.bat" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result = append(result, filepath.ToSlash(relative)+"\x00"+string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}
