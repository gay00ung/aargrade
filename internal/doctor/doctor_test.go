package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gay00ung/aargrade/internal/model"
)

func TestAnalyzeKotlinLibraryProducesMigrationEvidence(t *testing.T) {
	root := fixturePath(t, "kotlin-library")
	first, err := Analyze(root, "test")
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	second, err := Analyze(root, "test")
	if err != nil {
		t.Fatalf("Analyze() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Analyze() is not deterministic")
	}
	if first.SchemaVersion != model.ReportSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", first.SchemaVersion, model.ReportSchemaVersion)
	}
	if first.Inventory.AGP.Value != "8.8.0" || first.Inventory.Gradle.Value != "8.10.2" {
		t.Fatalf("versions = AGP %q Gradle %q", first.Inventory.AGP.Value, first.Inventory.Gradle.Value)
	}

	wantIDs := []string{
		"agp9.compatibility-opt-out.android.newDsl",
		"agp9.kotlin-android-plugin",
		"agp9.legacy-api",
		"android.buildconfig.feature-implicit",
		"android.library-only",
		"r8.consumer-global-option",
		"upgrade-assistant.buildsrc",
		"agp9.ksp-plugin",
		"android.native-build.present",
	}
	for _, id := range wantIDs {
		if !hasFinding(first, id) {
			t.Errorf("missing finding %q; got %v", id, findingIDs(first))
		}
	}
}

func TestAnalyzeMixedProjectDoesNotSuggestLibraryOnlyHost(t *testing.T) {
	report, err := Analyze(fixturePath(t, "groovy-mixed"), "test")
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if hasFinding(report, "android.library-only") {
		t.Fatalf("unexpected library-only finding: %v", findingIDs(report))
	}
	if !hasFinding(report, "android.application.present") {
		t.Fatalf("missing application finding: %v", findingIDs(report))
	}
}

func TestAnalyzeConventionPluginFailsOpenWithUncertainty(t *testing.T) {
	report, err := Analyze(fixturePath(t, "convention-unknown"), "test")
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !hasFinding(report, "android.model.unresolved") {
		t.Fatalf("missing model uncertainty: %v", findingIDs(report))
	}
	if !hasFinding(report, "upgrade-assistant.settings-version-catalog") {
		t.Fatalf("missing catalog finding: %v", findingIDs(report))
	}
}

func TestAnalyzeDoesNotWriteToProject(t *testing.T) {
	root := fixturePath(t, "kotlin-library")
	before := treeDigest(t, root)
	if _, err := Analyze(root, "test"); err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	after := treeDigest(t, root)
	if before != after {
		t.Fatalf("project tree changed: before=%s after=%s", before, after)
	}
}

func hasFinding(report model.Report, id string) bool {
	for _, finding := range report.Findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func findingIDs(report model.Report) []string {
	result := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		result = append(result, finding.ID)
	}
	return result
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", name))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	return path
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	var records []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		record := filepath.ToSlash(relative) + "\x00" + info.Mode().String()
		if !entry.IsDir() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(content)
			record += "\x00" + hex.EncodeToString(sum[:])
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree: %v", err)
	}
	sort.Strings(records)
	sum := sha256.Sum256([]byte(strings.Join(records, "\n")))
	return hex.EncodeToString(sum[:])
}
