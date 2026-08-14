package upgrade

import (
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
