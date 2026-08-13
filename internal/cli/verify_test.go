package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyCandidateJSON(t *testing.T) {
	aarPath := filepath.Join(t.TempDir(), "candidate.aar")
	var jar bytes.Buffer
	jarWriter := zip.NewWriter(&jar)
	if err := jarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	var aar bytes.Buffer
	writer := zip.NewWriter(&aar)
	for name, content := range map[string][]byte{
		"AndroidManifest.xml": []byte("<manifest />"),
		"classes.jar":         jar.Bytes(),
	} {
		entry, err := writer.Create(name)
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
	if err := os.WriteFile(aarPath, aar.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "--candidate-aar", aarPath, "--format", "json"}, &stdout, &stderr, "test")
	if exitCode != 0 {
		t.Fatalf("exit code=%d stderr=%q", exitCode, stderr.String())
	}
	var report struct {
		SchemaVersion int    `json:"schemaVersion"`
		Verdict       string `json:"verdict"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("JSON output: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Verdict != "evidence" {
		t.Fatalf("report = %#v", report)
	}
}

func TestVerifyRejectsUnsafeBuildVariant(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "--project", ".", "--variant", "release:tasks"}, &stdout, &stderr, "test")
	if exitCode != 2 || !bytes.Contains(stderr.Bytes(), []byte("must match")) {
		t.Fatalf("exit code=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}
