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

func TestMCPHelpAndUnknownOperation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"mcp"}, &stdout, &stderr, "test")
	if exitCode != 0 || !bytes.Contains(stdout.Bytes(), []byte("aargrade mcp serve")) || stderr.Len() != 0 {
		t.Fatalf("exit code=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"mcp", "unknown"}, &stdout, &stderr, "test")
	if exitCode != 2 || !bytes.Contains(stderr.Bytes(), []byte("unknown operation")) {
		t.Fatalf("exit code=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestPlanJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"plan", "--project", fixturePath(t, "kotlin-library"), "--target-agp", "9.0.1", "--format", "json"}, &stdout, &stderr, "test")
	if exitCode != 0 {
		t.Fatalf("exit code=%d stderr=%q", exitCode, stderr.String())
	}
	var result struct {
		SchemaVersion int   `json:"schemaVersion"`
		Ready         bool  `json:"ready"`
		Steps         []any `json:"steps"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("plan JSON: %v\n%s", err, stdout.String())
	}
	if result.SchemaVersion != 1 || !result.Ready || len(result.Steps) == 0 {
		t.Fatalf("plan = %#v", result)
	}
}

func TestMigrateJSONPreviewAndBlockedExitCodes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{
		"migrate",
		"--project", fixturePath(t, "migrate-kotlin-catalog"),
		"--target-agp", "9.2.0",
		"--format", "json",
	}, &stdout, &stderr, "test")
	if exitCode != 0 {
		t.Fatalf("preview exit code=%d stderr=%q", exitCode, stderr.String())
	}
	var result struct {
		Ready   bool  `json:"ready"`
		Applied bool  `json:"applied"`
		Changes []any `json:"changes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("migrate JSON: %v\n%s", err, stdout.String())
	}
	if !result.Ready || result.Applied || len(result.Changes) == 0 {
		t.Fatalf("migrate preview = %#v", result)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{
		"migrate",
		"--project", fixturePath(t, "groovy-mixed"),
		"--target-agp", "9.2.0",
	}, &stdout, &stderr, "test")
	if exitCode != 1 || !bytes.Contains(stdout.Bytes(), []byte("BLOCKED")) {
		t.Fatalf("blocked exit code=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestMigrateRequiresTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"migrate", "--project", fixturePath(t, "migrate-kotlin-catalog")}, &stdout, &stderr, "test")
	if exitCode != 2 || !bytes.Contains(stderr.Bytes(), []byte("--target-agp is required")) {
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
