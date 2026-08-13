package host

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAddPreviewDoesNotMutateProject(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	settingsPath := filepath.Join(root, "settings.gradle.kts")
	before := readFile(t, settingsPath)

	plan, err := Add(Options{ProjectPath: root})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if plan.Applied {
		t.Fatal("preview plan is marked applied")
	}
	if got := readFile(t, settingsPath); got != before {
		t.Fatal("preview changed settings")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview created state: %v", err)
	}
	if plan.LibraryModule != ":sdk" || plan.HostModule != defaultModulePath || plan.AGPVersion != "8.8.0" {
		t.Fatalf("plan selection = %#v", plan)
	}
	if plan.CompileSDK != 35 || plan.MinSDK != 21 {
		t.Fatalf("SDK selection = compile %d min %d", plan.CompileSDK, plan.MinSDK)
	}
	if len(plan.Changes) != 4 {
		t.Fatalf("len(Changes) = %d, want 4", len(plan.Changes))
	}
	if !strings.Contains(plan.Changes[0].Preview, markerStart) {
		t.Fatalf("settings preview lacks marker: %s", plan.Changes[0].Preview)
	}
}

func TestAddAndRemovePreserveUnrelatedSettingsEdits(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	settingsPath := filepath.Join(root, "settings.gradle.kts")
	original := readFile(t, settingsPath)

	addPlan, err := Add(Options{ProjectPath: root, Apply: true})
	if err != nil {
		t.Fatalf("Add(apply) error = %v", err)
	}
	if !addPlan.Applied {
		t.Fatal("add plan is not marked applied")
	}
	for _, relative := range []string{hostBuildRelativePath(true), hostManifestRelativePath(), stateRelativePath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("generated path %s: %v", relative, err)
		}
	}
	stateInfo, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && stateInfo.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o, want 600", stateInfo.Mode().Perm())
	}

	const userEdit = "\n// user-owned edit after AARGrade apply\n"
	file, err := os.OpenFile(settingsPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(userEdit); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	preview, err := Remove(Options{ProjectPath: root})
	if err != nil {
		t.Fatalf("Remove(preview) error = %v", err)
	}
	if preview.Applied {
		t.Fatal("remove preview marked applied")
	}
	removePlan, err := Remove(Options{ProjectPath: root, Apply: true})
	if err != nil {
		t.Fatalf("Remove(apply) error = %v", err)
	}
	if !removePlan.Applied {
		t.Fatal("remove plan is not marked applied")
	}
	if got, want := readFile(t, settingsPath), original+userEdit; got != want {
		t.Fatalf("settings after remove:\n%s\nwant:\n%s", got, want)
	}
	for _, relative := range []string{hostBuildRelativePath(true), hostManifestRelativePath(), stateRelativePath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned path remains %s: %v", relative, err)
		}
	}
}

func TestRemoveRefusesModifiedGeneratedFile(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	if _, err := Add(Options{ProjectPath: root, Apply: true}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	settingsPath := filepath.Join(root, "settings.gradle.kts")
	settingsBefore := readFile(t, settingsPath)
	hostBuild := filepath.Join(root, filepath.FromSlash(hostBuildRelativePath(true)))
	if err := os.WriteFile(hostBuild, []byte(readFile(t, hostBuild)+"\n// user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Remove(Options{ProjectPath: root, Apply: true})
	if err == nil || !strings.Contains(err.Error(), "was modified") {
		t.Fatalf("Remove() error = %v, want modified-file refusal", err)
	}
	if got := readFile(t, settingsPath); got != settingsBefore {
		t.Fatal("failed remove changed settings")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); err != nil {
		t.Fatalf("failed remove deleted state: %v", err)
	}
}

func TestRemoveRefusesChangedOwnedSettingsBlock(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	if _, err := Add(Options{ProjectPath: root, Apply: true}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	settingsPath := filepath.Join(root, "settings.gradle.kts")
	changed := strings.Replace(readFile(t, settingsPath), "include(\":aargrade-upgrade-host\")", "include(\":changed-host\")", 1)
	if err := os.WriteFile(settingsPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Remove(Options{ProjectPath: root, Apply: true})
	if err == nil || !strings.Contains(err.Error(), "settings block is missing or changed") {
		t.Fatalf("Remove() error = %v, want settings refusal", err)
	}
	if got := readFile(t, settingsPath); got != changed {
		t.Fatal("failed remove rewrote changed settings")
	}
}

func TestAddRefusesExistingApplication(t *testing.T) {
	root := copyFixture(t, "groovy-mixed")
	_, err := Add(Options{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "already has") {
		t.Fatalf("Add() error = %v, want application refusal", err)
	}
}

func TestAddRejectsInvalidOverrides(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	for _, testCase := range []struct {
		name    string
		options Options
		want    string
	}{
		{name: "bad version", options: Options{ProjectPath: root, AGPVersion: "9.0oops"}, want: "not a supported literal"},
		{name: "negative min SDK", options: Options{ProjectPath: root, MinSDK: -1}, want: "must be positive"},
		{name: "min exceeds compile", options: Options{ProjectPath: root, MinSDK: 36}, want: "cannot exceed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Add(testCase.options)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Add() error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestRemoveRefusesSymlinkedOwnedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges on Windows")
	}
	root := copyFixture(t, "kotlin-library")
	if _, err := Add(Options{ProjectPath: root, Apply: true}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	victim := filepath.Join(root, "user-owned.txt")
	writeFile(t, victim, "keep me\n")
	hostBuild := filepath.Join(root, filepath.FromSlash(hostBuildRelativePath(true)))
	if err := os.Remove(hostBuild); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, hostBuild); err != nil {
		t.Fatal(err)
	}

	_, err := Remove(Options{ProjectPath: root, Apply: true})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Remove() error = %v, want symlink refusal", err)
	}
	if got := readFile(t, victim); got != "keep me\n" {
		t.Fatalf("victim content = %q", got)
	}
}

func TestAddRequiresLibrarySelectionWhenAmbiguous(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	settingsPath := filepath.Join(root, "settings.gradle.kts")
	settings := readFile(t, settingsPath) + "\ninclude(\":other\")\n"
	writeFile(t, settingsPath, settings)
	otherBuild := filepath.Join(root, "other", "build.gradle.kts")
	writeFile(t, otherBuild, "plugins { alias(libs.plugins.android.library) }\nandroid { compileSdk = 35 }\n")

	_, err := Add(Options{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "select one with --library") {
		t.Fatalf("Add() error = %v, want ambiguity refusal", err)
	}
	plan, err := Add(Options{ProjectPath: root, LibraryPath: ":other", MinSDK: 24})
	if err != nil {
		t.Fatalf("Add(selected) error = %v", err)
	}
	if plan.LibraryModule != ":other" || plan.MinSDK != 24 {
		t.Fatalf("selected plan = %#v", plan)
	}
}

func TestGeneratedDSLMatchesAGPGeneration(t *testing.T) {
	old, err := parseAGPVersion("4.2.2")
	if err != nil {
		t.Fatal(err)
	}
	oldBuild := hostBuildFile(false, old, ":sdk", 30, 21)
	for _, want := range []string{"apply plugin: 'com.android.application'", "compileSdkVersion 30", "minSdkVersion 21"} {
		if !strings.Contains(oldBuild, want) {
			t.Errorf("old build missing %q:\n%s", want, oldBuild)
		}
	}
	if strings.Contains(oldBuild, "namespace") {
		t.Errorf("AGP 4 build unexpectedly has namespace:\n%s", oldBuild)
	}

	modern, err := parseAGPVersion("9.3.0")
	if err != nil {
		t.Fatal(err)
	}
	modernBuild := hostBuildFile(true, modern, ":sdk", 37, 23)
	for _, want := range []string{"id(\"com.android.application\")", "namespace = \"dev.aargrade.upgradehost\"", "compileSdk = 37", "minSdk = 23"} {
		if !strings.Contains(modernBuild, want) {
			t.Errorf("modern build missing %q:\n%s", want, modernBuild)
		}
	}
}

func TestPlanJSONDoesNotLeakRollbackInternals(t *testing.T) {
	root := copyFixture(t, "kotlin-library")
	plan, err := Add(Options{ProjectPath: root})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RenderJSON(&output, plan); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["settingsPath"]; ok {
		t.Fatalf("internal field leaked: %s", output.String())
	}
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", name))
	if err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
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
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return destination
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
