package verify

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gay00ung/aargrade/internal/project"
)

func TestRunEvidenceWithoutBaseline(t *testing.T) {
	candidate := writeTestAAR(t, "candidate.aar", "minCompileSdk=35\n", "")
	report, err := Run(Options{CandidateAAR: candidate, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "evidence" || report.Candidate.Path == "" {
		t.Fatalf("report = %#v", report)
	}
	if statusFor(report, "abi.binary") != StatusSkipped {
		t.Fatalf("ABI status = %s", statusFor(report, "abi.binary"))
	}
}

func TestRunFailsRaisedCompileSDK(t *testing.T) {
	baseline := writeTestAAR(t, "baseline.aar", "minCompileSdk=21\nminAndroidGradlePluginVersion=1.0.0\n", "")
	candidate := writeTestAAR(t, "candidate.aar", "minCompileSdk=35\nminAndroidGradlePluginVersion=1.0.0\n", "")
	report, err := Run(Options{CandidateAAR: candidate, BaselineAAR: baseline, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || statusFor(report, "metadata.requirements") != StatusFail {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunFailsNewConsumerRequirement(t *testing.T) {
	baseline := writeTestAAR(t, "baseline.aar", "aarFormatVersion=1.0\n", "")
	candidate := writeTestAAR(t, "candidate.aar", "minCompileSdk=35\nminAndroidGradlePluginVersion=8.0.0\n", "")
	report, err := Run(Options{CandidateAAR: candidate, BaselineAAR: baseline, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || statusFor(report, "metadata.requirements") != StatusFail {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunFailsInvalidMetadata(t *testing.T) {
	candidate := writeTestAAR(t, "candidate.aar", "minCompileSdk=not-a-number\n", "")
	report, err := Run(Options{CandidateAAR: candidate, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || statusFor(report, "aar.metadata") != StatusFail {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunFailsInvalidBaselineMetadata(t *testing.T) {
	baseline := writeTestAAR(t, "baseline.aar", "minCompileSdk=bad\n", "")
	candidate := writeTestAAR(t, "candidate.aar", "minCompileSdk=35\n", "")
	report, err := Run(Options{CandidateAAR: candidate, BaselineAAR: baseline, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || statusFor(report, "metadata.requirements") != StatusFail {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunFailsUnsafeConsumerRule(t *testing.T) {
	candidate := writeTestAAR(t, "candidate.aar", "minCompileSdk=35\n", "-dontoptimize\n")
	report, err := Run(Options{CandidateAAR: candidate, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "fail" || statusFor(report, "r8.consumer-rules") != StatusFail {
		t.Fatalf("report = %#v", report)
	}
}

func TestKMPAndroidLibraryUsesBundleTask(t *testing.T) {
	library := project.Module{
		GradlePath: ":sdk",
		Plugins:    []project.Plugin{{ID: "com.android.kotlin.multiplatform.library"}},
	}
	task, kmp, err := libraryAssembleTask(library, "release")
	if err != nil || !kmp || task != ":sdk:bundleAndroidMainAar" {
		t.Fatalf("task=%q kmp=%v err=%v", task, kmp, err)
	}
	if _, _, err := libraryAssembleTask(library, "debug"); err == nil || !strings.Contains(err.Error(), "one Android AAR") {
		t.Fatalf("KMP debug variant should be rejected: %v", err)
	}
}

func TestLocateKMPAndroidAARWithoutVariantSuffix(t *testing.T) {
	module := t.TempDir()
	output := filepath.Join(module, "build", "outputs", "aar")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(output, "sdk.aar")
	if err := os.WriteFile(want, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := locateAAR(module, "release", true)
	if err != nil || got != want {
		t.Fatalf("got=%q want=%q err=%v", got, want, err)
	}
}

func statusFor(report Report, id string) Status {
	for _, check := range report.Checks {
		if check.ID == id {
			return check.Status
		}
	}
	return ""
}

func writeTestAAR(t *testing.T, name, metadata, rules string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	var jar bytes.Buffer
	jarWriter := zip.NewWriter(&jar)
	if err := jarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"AndroidManifest.xml": []byte("<manifest />"),
		"classes.jar":         jar.Bytes(),
		"META-INF/com/android/build/gradle/aar-metadata.properties": []byte(metadata),
	}
	if rules != "" {
		files["proguard.txt"] = []byte(rules)
	}
	var aar bytes.Buffer
	writer := zip.NewWriter(&aar)
	for fileName, content := range files {
		entry, err := writer.Create(fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, aar.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
