package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gay00ung/aargrade/internal/project"
)

func TestMigrateCatalogKotlinPreviewApplyAndRollback(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	original := migrationTreeSnapshot(t, root)

	preview, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Ready || preview.Applied || preview.CurrentAGP != "8.8.0" || preview.TargetAGP != "9.2.0" {
		t.Fatalf("preview = %#v", preview)
	}
	wantPaths := []string{
		"build.gradle.kts",
		"gradle.properties",
		"gradle/libs.versions.toml",
		"gradle/wrapper/gradle-wrapper.properties",
		"sdk/build.gradle.kts",
	}
	if got := mutationChangePaths(preview); !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("preview paths = %v, want %v", got, wantPaths)
	}
	if afterPreview := migrationTreeSnapshot(t, root); !reflect.DeepEqual(afterPreview, original) {
		t.Fatal("preview changed the fixture")
	}

	applied, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", ToolVersion: "test", Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Ready || !applied.Applied {
		t.Fatalf("apply result = %#v", applied)
	}
	assertFileContains(t, filepath.Join(root, "gradle", "libs.versions.toml"), `agp = "9.2.0"`)
	assertFileContains(t, filepath.Join(root, "gradle", "wrapper", "gradle-wrapper.properties"), "gradle-9.4.1-bin.zip")
	assertFileContains(t, filepath.Join(root, "gradle", "wrapper", "gradle-wrapper.properties"), "2ab2958f2a1e51120c326cad6f385153bb11ee93b3c216c5fccebfdfbb7ec6cb")
	for _, relative := range []string{"build.gradle.kts", "sdk/build.gradle.kts"} {
		content := readMigrationTestFile(t, filepath.Join(root, filepath.FromSlash(relative)))
		if strings.Contains(content, "libs.plugins.kotlin.android") {
			t.Fatalf("Kotlin Android plugin remains in %s:\n%s", relative, content)
		}
	}
	properties := readMigrationTestFile(t, filepath.Join(root, "gradle.properties"))
	if strings.Contains(properties, "android.") {
		t.Fatalf("obsolete AGP properties remain:\n%s", properties)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); err != nil {
		t.Fatalf("migration state was not created: %v", err)
	}
	discovered, err := project.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := uniqueProjectVersions(discovered.AGPVersions); !reflect.DeepEqual(got, []string{"9.2.0"}) || discovered.WrapperVersion != "9.4.1" {
		t.Fatalf("migrated discovery: AGP=%v Gradle=%s", got, discovered.WrapperVersion)
	}
	for _, module := range discovered.Modules {
		if module.HasPlugin("org.jetbrains.kotlin.android") || module.HasPlugin("kotlin-android") {
			t.Fatalf("Kotlin Android plugin remains applied in %s", module.GradlePath)
		}
	}

	rollbackPreview, err := Rollback(RollbackOptions{ProjectPath: root})
	if err != nil {
		t.Fatal(err)
	}
	if !rollbackPreview.Ready || rollbackPreview.Applied || len(rollbackPreview.Changes) != len(wantPaths) {
		t.Fatalf("rollback preview = %#v", rollbackPreview)
	}
	rolledBack, err := Rollback(RollbackOptions{ProjectPath: root, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if !rolledBack.Applied {
		t.Fatalf("rollback apply = %#v", rolledBack)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration state remains after rollback: %v", err)
	}
	if restored := migrationTreeSnapshot(t, root); !reflect.DeepEqual(restored, original) {
		t.Fatalf("rollback did not restore exact fixture\nbefore=%v\nafter=%v", original, restored)
	}
}

func TestRollbackRefusesUserModifiedFile(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	if _, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", Apply: true}); err != nil {
		t.Fatal(err)
	}
	buildFile := filepath.Join(root, "sdk", "build.gradle.kts")
	content := readMigrationTestFile(t, buildFile) + "\n// user edit after migration\n"
	if err := os.WriteFile(buildFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Rollback(RollbackOptions{ProjectPath: root, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Applied || len(result.Blockers) == 0 {
		t.Fatalf("rollback should be blocked: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); err != nil {
		t.Fatalf("blocked rollback removed state: %v", err)
	}
	assertFileContains(t, buildFile, "user edit after migration")
}

func TestAcceptKeepsMigrationAndRemovesRollbackState(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	applied, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatalf("migration apply = %#v", applied)
	}
	preview, err := Accept(AcceptOptions{ProjectPath: root})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Ready || preview.Applied || len(preview.Changes) != len(applied.Changes) {
		t.Fatalf("accept preview = %#v", preview)
	}
	accepted, err := Accept(AcceptOptions{ProjectPath: root, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if !accepted.Applied {
		t.Fatalf("accept apply = %#v", accepted)
	}
	assertFileContains(t, filepath.Join(root, "gradle", "libs.versions.toml"), `agp = "9.2.0"`)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("accepted migration state remains: %v", err)
	}
}

func TestAcceptRefusesChangedMigratedFile(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	if _, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", Apply: true}); err != nil {
		t.Fatal(err)
	}
	buildPath := filepath.Join(root, "sdk", "build.gradle.kts")
	if err := os.WriteFile(buildPath, []byte(readMigrationTestFile(t, buildPath)+"\n// reviewed edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Accept(AcceptOptions{ProjectPath: root, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Applied || !containsMutationText(result.Blockers, "refusing to discard rollback state") {
		t.Fatalf("changed migration should block accept: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(stateRelativePath))); err != nil {
		t.Fatalf("blocked accept removed state: %v", err)
	}
}

func TestRollbackRecoversPartiallyAppliedTransaction(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	original := migrationTreeSnapshot(t, root)
	if _, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", Apply: true}); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, filepath.FromSlash(stateRelativePath))
	state, _, err := readMigrationState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	first := state.Files[0]
	originalFirst, err := decodeMigrationOriginal(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(first.Path)), originalFirst, os.FileMode(first.Mode)); err != nil {
		t.Fatal(err)
	}

	preview, err := Rollback(RollbackOptions{ProjectPath: root})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Ready || !containsMutationText(preview.Warnings, "already matches") {
		t.Fatalf("partial rollback preview = %#v", preview)
	}
	if _, err := Rollback(RollbackOptions{ProjectPath: root, Apply: true}); err != nil {
		t.Fatal(err)
	}
	if restored := migrationTreeSnapshot(t, root); !reflect.DeepEqual(restored, original) {
		t.Fatalf("partial transaction was not restored\nbefore=%v\nafter=%v", original, restored)
	}
}

func TestMigrateBlocksCatalogVersionSharedWithNonAGPEntry(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	catalogPath := filepath.Join(root, "gradle", "libs.versions.toml")
	catalog := readMigrationTestFile(t, catalogPath) + `
[libraries]
unrelated = { module = "example:unrelated", version.ref = "agp" }
`
	if err := os.WriteFile(catalogPath, []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || !containsMutationText(result.Blockers, "shared with non-AGP") {
		t.Fatalf("shared catalog ref should block mutation: %#v", result)
	}
}

func TestMigrateBlocksKnownUnsafeFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		want    string
	}{
		{fixture: "kotlin-library", want: "KSP 2.1.0-1.0.29"},
		{fixture: "groovy-mixed", want: "requires an explicit Android namespace"},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", test.fixture))
			if err != nil {
				t.Fatal(err)
			}
			result, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready || !containsMutationText(result.Blockers, test.want) {
				t.Fatalf("result = %#v, want blocker containing %q", result, test.want)
			}
		})
	}
}

func TestMigrateBlocksBuildSrcForPreAGP9Target(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "kotlin-library"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "8.9.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || !containsMutationText(result.Blockers, "buildSrc") {
		t.Fatalf("buildSrc should block every automatic migration target: %#v", result)
	}
}

func TestMigrateAutoRepairHandlesCommonAGP9Recipes(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	buildPath := filepath.Join(root, "sdk", "build.gradle.kts")
	build := `plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
}

android {
    compileSdkVersion(35)

    defaultConfig {
        minSdkVersion 21
        buildConfigField("String", "SDK_NAME", "\"agent\"")
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}
`
	if err := os.WriteFile(buildPath, []byte(build), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "sdk", "src", "main", "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(`<manifest package="dev.aargrade.agentfixture" />`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	preview, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", AutoRepair: true})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Ready {
		t.Fatalf("auto-repair preview blocked: %#v", preview.Blockers)
	}
	wantRepairs := []string{"agp9.built-in-kotlin.catalog", "agp9.java-kotlin-target", "agp9.kotlin-options", "agp9.sdk-dsl", "android.buildconfig.enable", "android.manifest-package", "android.namespace"}
	var gotRepairs []string
	for _, repair := range preview.Repairs {
		gotRepairs = append(gotRepairs, repair.ID)
	}
	sort.Strings(gotRepairs)
	if !reflect.DeepEqual(gotRepairs, wantRepairs) {
		t.Fatalf("repairs = %v, want %v", gotRepairs, wantRepairs)
	}

	applied, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", AutoRepair: true, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatalf("apply = %#v", applied)
	}
	updated := readMigrationTestFile(t, buildPath)
	for _, want := range []string{
		`namespace = "dev.aargrade.agentfixture"`,
		"compileSdk = 35",
		"minSdk = 21",
		"buildConfig = true",
		`sourceCompatibility = JavaVersion.toVersion("17")`,
		`targetCompatibility = JavaVersion.toVersion("17")`,
		"kotlin {",
		`JvmTarget.fromTarget("17")`,
	} {
		if !strings.Contains(updated, want) {
			t.Fatalf("updated build file does not contain %q:\n%s", want, updated)
		}
	}
	for _, unwanted := range []string{"compileSdkVersion", "minSdkVersion", "kotlinOptions", "libs.plugins.kotlin.android"} {
		if strings.Contains(updated, unwanted) {
			t.Fatalf("updated build file still contains %q:\n%s", unwanted, updated)
		}
	}
	catalog := readMigrationTestFile(t, filepath.Join(root, "gradle", "libs.versions.toml"))
	if strings.Contains(catalog, "org.jetbrains.kotlin.android") {
		t.Fatalf("Kotlin Android catalog entry remains:\n%s", catalog)
	}
	manifest := readMigrationTestFile(t, manifestPath)
	if strings.Contains(manifest, "package=") || strings.TrimSpace(manifest) != "<manifest />" {
		t.Fatalf("legacy manifest package was not removed safely: %q", manifest)
	}
}

func TestManifestPackageAttributeRangePreservesOtherXML(t *testing.T) {
	before := []byte("<manifest\n    xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    package='dev.example.sdk'>")
	start, end, err := manifestPackageAttributeRange(before)
	if err != nil {
		t.Fatal(err)
	}
	after := string(before[:start]) + string(before[end:])
	want := "<manifest\n    xmlns:android=\"http://schemas.android.com/apk/res/android\">"
	if after != want {
		t.Fatalf("manifest transform = %q, want %q", after, want)
	}
}

func TestTransformLegacySDKSettersPreservesCRLF(t *testing.T) {
	before := "android {\r\n    compileSdkVersion(35)\r\n}\r\n"
	after, count := transformLegacySDKSetters(before)
	if count != 1 || after != "android {\r\n    compileSdk = 35\r\n}\r\n" {
		t.Fatalf("count=%d after=%q", count, after)
	}
}

func TestEnsureJavaCompileTargetRefusesExistingTarget(t *testing.T) {
	content := `android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
}
`
	if _, _, err := ensureJavaCompileTarget(content, "17"); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("existing Java targets should require review: %v", err)
	}
}

func TestEnsureJavaCompileTargetAcceptsMatchingExistingTargets(t *testing.T) {
	content := `android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility JavaVersion.toVersion("17")
    }
}
`
	updated, changed, err := ensureJavaCompileTarget(content, "17")
	if err != nil || changed || updated != content {
		t.Fatalf("matching targets should remain unchanged: changed=%v err=%v\n%s", changed, err, updated)
	}
}

func TestLegacySDKSetterOutsideAndroidIsNotTransformed(t *testing.T) {
	content := `fun configureSomething() {
    minSdkVersion 19
}

android {
    compileSdkVersion(35)
}
`
	updated, count := transformLegacySDKSetters(content)
	if count != 1 || !strings.Contains(updated, "minSdkVersion 19") || !strings.Contains(updated, "compileSdk = 35") {
		t.Fatalf("count=%d updated=\n%s", count, updated)
	}
}

func TestMigrateAutoRepairRefusesNamespaceWithoutLiteralManifestPackage(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	buildPath := filepath.Join(root, "sdk", "build.gradle.kts")
	build := strings.Replace(readMigrationTestFile(t, buildPath), `    namespace = "dev.aargrade.migratefixture"`+"\n", "", 1)
	if err := os.WriteFile(buildPath, []byte(build), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0", AutoRepair: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || !containsMutationText(result.Blockers, "no literal package") {
		t.Fatalf("missing manifest package should block: %#v", result)
	}
}

func TestKotlinPluginMultilineDeclarationIsNotRemoved(t *testing.T) {
	content := "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n        version \"2.1.0\"\n}\n"
	updated, removed, unsafe := removeKotlinAndroidPluginLines(content, nil)
	if updated != content || removed != 0 || !reflect.DeepEqual(unsafe, []int{2}) {
		t.Fatalf("updated=%q removed=%d unsafe=%v", updated, removed, unsafe)
	}
}

func TestKSPDependencyDoesNotBorrowUnusedCatalogPluginVersion(t *testing.T) {
	catalog := parseCatalog(`[versions]
ksp = "2.3.6"
[plugins]
ksp = { id = "com.google.devtools.ksp", version.ref = "ksp" }
`)
	files := []migrationFile{{relative: "sdk/build.gradle.kts", content: `dependencies { ksp("example:processor:1.0") }`}}
	detected, versions := scanKSPVersions(files, catalog)
	if !detected || len(versions) != 0 {
		t.Fatalf("detected=%v versions=%v", detected, versions)
	}
}

func TestLegacyKaptConfigurationIsAcceptedButOldPluginIsNot(t *testing.T) {
	legacyFile := migrationFile{
		relative: "sdk/build.gradle.kts",
		content: "plugins { id(\"com.android.legacy-kapt\") }\n" +
			"dependencies { kapt(\"example:processor:1.0\") }",
	}
	if !hasLegacyKapt(legacyFile, nil) || len(hasKapt([]migrationFile{legacyFile}, nil, nil)) != 0 {
		t.Fatalf("legacy kapt was not recognized")
	}
	oldFiles := []migrationFile{{
		relative: "sdk/build.gradle.kts",
		content:  `plugins { id("org.jetbrains.kotlin.kapt") }`,
	}}
	if evidence := hasKapt(oldFiles, nil, nil); !reflect.DeepEqual(evidence, []string{"sdk/build.gradle.kts"}) {
		t.Fatalf("old kapt evidence = %v", evidence)
	}
}

func TestLegacyKaptInOneModuleDoesNotAuthorizeAnotherModule(t *testing.T) {
	files := []migrationFile{
		{relative: "legacy/build.gradle.kts", content: `plugins { id("com.android.legacy-kapt") }`},
		{relative: "unsafe/build.gradle.kts", content: `dependencies { kapt("example:processor:1.0") }`},
	}
	if evidence := hasKapt(files, nil, nil); !reflect.DeepEqual(evidence, []string{"unsafe/build.gradle.kts"}) {
		t.Fatalf("cross-module kapt evidence = %v", evidence)
	}
}

func TestMigrateBlocksKotlinBlockSourceSets(t *testing.T) {
	root := copyMigrationFixture(t, "migrate-kotlin-catalog")
	buildPath := filepath.Join(root, "sdk", "build.gradle.kts")
	content := readMigrationTestFile(t, buildPath) + `
kotlin {
    sourceSets {
    }
}
`
	if err := os.WriteFile(buildPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Migrate(MutationOptions{ProjectPath: root, TargetAGP: "9.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || !containsMutationText(result.Blockers, "kotlin { sourceSets") {
		t.Fatalf("Kotlin source-set DSL should block: %#v", result)
	}
}

func TestTransformWrapperAddsPinnedChecksum(t *testing.T) {
	before := "distributionUrl=https\\://services.gradle.org/distributions/gradle-7.5-all.zip\r\n"
	after, checksum, err := transformWrapper(before, "7.5", "9.4.1")
	if err != nil {
		t.Fatal(err)
	}
	if checksum != "708d2c6ecc97ca9a11838ef64a6c2301151b8dd10387e22dc1a12c30557cab5b" ||
		!strings.Contains(after, "gradle-9.4.1-all.zip\r\ndistributionSha256Sum="+checksum+"\r\n") {
		t.Fatalf("wrapper transform = %q, checksum=%q", after, checksum)
	}
}

func TestTransformLiteralAGPGroovyPreservesCommentsAndUnknownPlugins(t *testing.T) {
	before := `plugins {
    id 'com.android.library' version '7.4.2' apply false
    id 'com.android.unrelated-tool' version '7.4.2' apply false
}
buildscript {
    dependencies {
        classpath 'com.android.tools.build:gradle:7.4.2'
        // classpath 'com.android.tools.build:gradle:7.4.2'
    }
}
`
	after, replacements := transformLiteralAGP(before, "7.4.2", "8.0.2")
	if replacements != 2 {
		t.Fatalf("replacements = %d\n%s", replacements, after)
	}
	if strings.Count(after, "com.android.tools.build:gradle:7.4.2") != 1 ||
		!strings.Contains(after, "com.android.unrelated-tool' version '7.4.2") ||
		!strings.Contains(after, "com.android.library' version '8.0.2") {
		t.Fatalf("unexpected Groovy transform:\n%s", after)
	}
}

func TestValidateMigrationStateRejectsNonGradleOwnedPath(t *testing.T) {
	root := t.TempDir()
	content := []byte("do not overwrite")
	state := migrationState{
		SchemaVersion: StateSchemaVersion,
		Status:        "applied",
		CurrentAGP:    "8.8.0",
		TargetAGP:     "9.2.0",
		Files: []migrationStateFile{{
			Path:           "src/main/Secret.kt",
			Mode:           0o644,
			BeforeSHA256:   migrationDigest(content),
			AfterSHA256:    strings.Repeat("a", 64),
			OriginalBase64: encodeMigrationOriginal(content),
		}},
	}
	if err := validateMigrationState(root, state); err == nil || !strings.Contains(err.Error(), "outside the supported Gradle configuration set") {
		t.Fatalf("unsafe state path error = %v", err)
	}
}

func mutationChangePaths(result MutationResult) []string {
	paths := make([]string, 0, len(result.Changes))
	for _, change := range result.Changes {
		paths = append(paths, change.Path)
	}
	return paths
}

func containsMutationText(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func copyMigrationFixture(t *testing.T, name string) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", name))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "project")
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

func migrationTreeSnapshot(t *testing.T, root string) []string {
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
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		result = append(result, filepath.ToSlash(relative)+"\x00"+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	content := readMigrationTestFile(t, path)
	if !strings.Contains(content, want) {
		t.Fatalf("%s does not contain %q:\n%s", path, want, content)
	}
}

func readMigrationTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
