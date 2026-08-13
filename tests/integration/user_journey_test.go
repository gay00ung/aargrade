package integration_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/gay00ung/aargrade/internal/cli"
)

func TestPublicExampleUserJourney(t *testing.T) {
	root := copyPublicExample(t)
	settingsPath := filepath.Join(root, "settings.gradle.kts")
	originalSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exitCode := cli.Run([]string{"doctor", "--project", root, "--fail-on", "never"}, &stdout, &stderr, "test"); exitCode != 0 {
		t.Fatalf("doctor exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("android.library-only")) {
		t.Fatalf("doctor output does not classify the example:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := cli.Run([]string{"plan", "--project", root, "--target-agp", "9.3.0"}, &stdout, &stderr, "test"); exitCode != 0 {
		t.Fatalf("plan exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("migration plan — READY")) || !bytes.Contains(stdout.Bytes(), []byte("Gradle 9.5.0+")) {
		t.Fatalf("plan output does not contain the expected target toolchain:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := cli.Run([]string{"host", "add", "--project", root}, &stdout, &stderr, "test"); exitCode != 0 {
		t.Fatalf("host preview exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("PREVIEW — no files changed")) {
		t.Fatalf("host preview output missing preview status:\n%s", stdout.String())
	}
	assertFileContent(t, settingsPath, originalSettings)

	stdout.Reset()
	stderr.Reset()
	if exitCode := cli.Run([]string{"host", "add", "--project", root, "--apply"}, &stdout, &stderr, "test"); exitCode != 0 {
		t.Fatalf("host add exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".aargrade", "state", "upgrade-host.json")); err != nil {
		t.Fatalf("host state was not created: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := cli.Run([]string{"host", "remove", "--project", root}, &stdout, &stderr, "test"); exitCode != 0 {
		t.Fatalf("host removal preview exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("PREVIEW — no files changed")) {
		t.Fatalf("host removal preview output missing preview status:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := cli.Run([]string{"host", "remove", "--project", root, "--apply"}, &stdout, &stderr, "test"); exitCode != 0 {
		t.Fatalf("host remove exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	assertFileContent(t, settingsPath, originalSettings)
	if _, err := os.Stat(filepath.Join(root, ".aargrade", "state", "upgrade-host.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("host state remains after removal: %v", err)
	}
}

func TestVerifyJourneyWithExistingArtifacts(t *testing.T) {
	aarPath := filepath.Join(t.TempDir(), "sdk.aar")
	createMinimalAAR(t, aarPath)

	var stdout, stderr bytes.Buffer
	exitCode := cli.Run([]string{
		"verify",
		"--baseline-aar", aarPath,
		"--candidate-aar", aarPath,
		"--format", "json",
	}, &stdout, &stderr, "test")
	if exitCode != 0 {
		t.Fatalf("verify exit code = %d; stderr=%s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"verdict": "pass"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"abi.binary"`)) {
		t.Fatalf("verify output does not contain a compatibility pass:\n%s", stdout.String())
	}
}

func copyPublicExample(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "examples", "library-only"))
	if err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			if relative != "." && (entry.Name() == ".gradle" || entry.Name() == ".aargrade" || entry.Name() == "build") {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		if entry.Name() == "local.properties" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	}); err != nil {
		t.Fatalf("copy public example: %v", err)
	}
	return destination
}

func createMinimalAAR(t *testing.T, path string) {
	t.Helper()
	var classes bytes.Buffer
	classesArchive := zip.NewWriter(&classes)
	if err := classesArchive.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entries := map[string][]byte{
		"AndroidManifest.xml": []byte(`<manifest package="dev.aargrade.test" />`),
		"classes.jar":         classes.Bytes(),
		"META-INF/com/android/build/gradle/aar-metadata.properties": []byte("aarFormatVersion=1.0\naarMetadataVersion=1.0\nminCompileSdk=21\n"),
	}
	for name, content := range entries {
		writer, createErr := archive.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := writer.Write(content); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s was not restored exactly", path)
	}
}
