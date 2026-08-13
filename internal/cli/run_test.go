package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gay00ung/aargrade/internal/model"
)

func TestDoctorJSONAndThresholdExitCode(t *testing.T) {
	root := fixturePath(t, "kotlin-library")
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"doctor", "--project", root, "--format", "json", "--fail-on", "warn"}, &stdout, &stderr, "test")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", exitCode, stderr.String())
	}
	var report model.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("JSON output: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.ReportSchemaVersion || len(report.Findings) == 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestDoctorNeverThresholdReturnsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"doctor", "--project", fixturePath(t, "kotlin-library"), "--fail-on", "never"}, &stdout, &stderr, "test")
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("android.library-only")) {
		t.Fatalf("text output missing finding:\n%s", stdout.String())
	}
}

func TestDoctorInvalidProjectReturnsTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"doctor", "--project", t.TempDir()}, &stdout, &stderr, "test")
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("no settings.gradle")) {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRoadmapCommandDoesNotPretendSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"matrix"}, &stdout, &stderr, "test")
	if exitCode != 2 || !bytes.Contains(stderr.Bytes(), []byte("not implemented")) {
		t.Fatalf("exit code=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", name))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	return path
}
