package matrix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gay00ung/aargrade/internal/artifact"
)

func TestLoadConfigStrictAndResolvePaths(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "aargrade.yml")
	content := `schemaVersion: 1
candidateAar: candidate.aar
cells:
  - name: current-java
    agp: 9.2.0
    gradle: 9.4.1
    jdk: 17
    compileSdk: 35
    language: java
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	config, absolute, err := loadConfig(Options{ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if absolute != configPath || config.CandidateAAR != filepath.Join(root, "candidate.aar") {
		t.Fatalf("config=%#v path=%s", config, absolute)
	}
}

func TestLoadConfigRejectsUnknownField(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aargrade.yml")
	content := "schemaVersion: 1\ncandidateAar: candidate.aar\nunknown: true\ncells: []\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadConfig(Options{ConfigPath: configPath}); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigRejectsMultipleDocuments(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aargrade.yml")
	content := `schemaVersion: 1
candidateAar: candidate.aar
cells:
  - name: current
    agp: 9.2.0
    gradle: 9.4.1
    jdk: 17
    compileSdk: 35
    language: java
---
schemaVersion: 1
candidateAar: ignored.aar
cells: []
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadConfig(Options{ConfigPath: configPath}); err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateConfigRejectsIncompatibleToolchain(t *testing.T) {
	config := Config{SchemaVersion: 1, CandidateAAR: "candidate.aar", Cells: []CellConfig{{
		Name: "bad", AGP: "9.2.0", Gradle: "8.0", JDK: 17, CompileSDK: 35, Language: "java",
	}}}
	if err := validateConfig(config, nil); err == nil || !strings.Contains(err.Error(), "lower") {
		t.Fatalf("error = %v", err)
	}
}

func TestGenerateConsumerUsesCompileProbeAndMinification(t *testing.T) {
	root := t.TempDir()
	aar := filepath.Join(root, "candidate.aar")
	if err := os.WriteFile(aar, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	consumer := filepath.Join(root, "consumer")
	cell := CellConfig{Name: "java", AGP: "9.2.0", Gradle: "9.4.1", JDK: 17, CompileSDK: 35, Language: "java"}
	if err := generateConsumer(consumer, cell, aar, "dev/aargrade/example/Greeting"); err != nil {
		t.Fatal(err)
	}
	build, err := os.ReadFile(filepath.Join(consumer, "app", "build.gradle"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(build), "minifyEnabled true") || !strings.Contains(string(build), "proguard-android-optimize.txt") {
		t.Fatalf("build.gradle:\n%s", build)
	}
	probe, err := os.ReadFile(filepath.Join(consumer, "app", "src", "main", "java", "dev", "aargrade", "consumer", "MainActivity.java"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(probe), "dev.aargrade.example.Greeting.class") {
		t.Fatalf("probe:\n%s", probe)
	}
}

func TestGenerateConsumerAddsManifestPackageForAGP4(t *testing.T) {
	root := t.TempDir()
	aar := filepath.Join(root, "candidate.aar")
	if err := os.WriteFile(aar, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	consumer := filepath.Join(root, "consumer")
	cell := CellConfig{Name: "legacy", AGP: "4.2.2", Gradle: "6.7.1", JDK: 11, CompileSDK: 30, Language: "java"}
	if err := generateConsumer(consumer, cell, aar, ""); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(consumer, "app", "src", "main", "AndroidManifest.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `package="dev.aargrade.consumer"`) {
		t.Fatalf("manifest:\n%s", manifest)
	}
}

func TestChooseProbePrefersBaseline(t *testing.T) {
	baseline := artifact.Snapshot{Classes: []artifact.Class{{Name: "dev/Baseline"}}}
	candidate := artifact.Snapshot{Classes: []artifact.Class{{Name: "dev/Candidate"}}}
	if got := chooseProbe(baseline, candidate); got != "dev/Baseline" {
		t.Fatalf("probe = %q", got)
	}
}

func TestClassifyCell(t *testing.T) {
	pass := &Execution{Status: "pass"}
	fail := &Execution{Status: "fail"}
	if verdict, _ := classifyCell(pass, fail); verdict != "regression" {
		t.Fatalf("verdict = %s", verdict)
	}
	if verdict, _ := classifyCell(fail, fail); verdict != "unsupported" {
		t.Fatalf("verdict = %s", verdict)
	}
	if verdict, _ := classifyCell(nil, pass); verdict != "pass" {
		t.Fatalf("verdict = %s", verdict)
	}
}
