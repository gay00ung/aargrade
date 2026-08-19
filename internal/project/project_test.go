package project

import (
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestDiscoverKotlinLibraryWithDefaultCatalog(t *testing.T) {
	project, err := Discover(fixturePath(t, "kotlin-library"))
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if project.WrapperVersion != "8.10.2" {
		t.Fatalf("WrapperVersion = %q, want 8.10.2", project.WrapperVersion)
	}
	if len(project.Modules) != 1 {
		t.Fatalf("len(Modules) = %d, want 1", len(project.Modules))
	}
	module := project.Modules[0]
	if module.GradlePath != ":sdk" || module.Kind() != "library" {
		t.Fatalf("module = %#v, want :sdk library", module)
	}
	wantPlugins := []string{"com.android.library", "com.google.devtools.ksp", "org.jetbrains.kotlin.android"}
	var gotPlugins []string
	for _, plugin := range module.Plugins {
		gotPlugins = append(gotPlugins, plugin.ID)
	}
	if !reflect.DeepEqual(gotPlugins, wantPlugins) {
		t.Fatalf("plugins = %v, want %v", gotPlugins, wantPlugins)
	}
	versions := map[string]bool{}
	for _, version := range project.AGPVersions {
		versions[version.Value] = true
	}
	if len(versions) != 1 || !versions["8.8.0"] {
		t.Fatalf("AGP versions = %#v, want only 8.8.0", project.AGPVersions)
	}
	if !project.HasBuildSrc {
		t.Fatal("HasBuildSrc = false, want true")
	}
	catalog, err := readDefaultCatalog(project.Root)
	if err != nil {
		t.Fatalf("readDefaultCatalog() error = %v", err)
	}
	entry, ok := catalog.Libraries["gradlePlugin.android"]
	if !ok || entry.Module != "com.android.tools.build:gradle" || entry.Version != "8.8.0" {
		t.Fatalf("AGP catalog library = %#v, present=%v", entry, ok)
	}
}

func TestDiscoverGroovyApplicationAndLibrary(t *testing.T) {
	project, err := Discover(fixturePath(t, "groovy-mixed"))
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	gotKinds := map[string]string{}
	for _, module := range project.Modules {
		gotKinds[module.GradlePath] = module.Kind()
	}
	wantKinds := map[string]string{":app": "application", ":sdk": "library"}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("module kinds = %v, want %v", gotKinds, wantKinds)
	}
	if len(project.AGPVersions) != 1 || project.AGPVersions[0].Value != "7.4.2" {
		t.Fatalf("AGPVersions = %#v, want 7.4.2", project.AGPVersions)
	}
}

func TestStripCommentsPreservesStringsAndLines(t *testing.T) {
	input := "id(\"com.android.library\") // id(\"fake\")\n/* block\ncomment */\nvalue = \"https://example.test/path\"\n"
	got := StripComments(input)
	if !regexpContains(got, `id\("com.android.library"\)`) {
		t.Fatalf("real plugin was removed: %q", got)
	}
	if regexpContains(got, `fake`) {
		t.Fatalf("comment content remains: %q", got)
	}
	if !regexpContains(got, `https://example.test/path`) {
		t.Fatalf("URL in string was treated as comment: %q", got)
	}
	if countLines(got) != countLines(input) {
		t.Fatalf("line count = %d, want %d", countLines(got), countLines(input))
	}
}

func TestParseAppliedPluginsIgnoresKotlinApplyFalse(t *testing.T) {
	plugins := parseAppliedPlugins(`
plugins {
    id("com.android.application") version "9.2.0" apply(false)
    id("com.android.library")
}
`, "build.gradle.kts", Catalog{Plugins: map[string]CatalogPlugin{}, Libraries: map[string]CatalogLibrary{}})
	if len(plugins) != 1 || plugins[0].ID != "com.android.library" {
		t.Fatalf("plugins = %#v, want only com.android.library", plugins)
	}
}

func TestFindAGPBuildscriptVariables(t *testing.T) {
	variables := FindAGPBuildscriptVariables(`
buildscript {
    ext {
        agp_version = '9.2.1'
        kotlin_version = '2.0.21'
    }
    dependencies {
        classpath "com.android.tools.build:gradle:$agp_version"
        classpath "org.jetbrains.kotlin:kotlin-gradle-plugin:$kotlin_version"
    }
}
`)
	if len(variables) != 1 || variables[0].Name != "agp_version" || variables[0].Value != "9.2.1" || variables[0].Line != 4 {
		t.Fatalf("variables = %#v", variables)
	}
}

func TestFindAGPBuildscriptVariablesRejectsSharedOrReassignedValues(t *testing.T) {
	for name, content := range map[string]string{
		"shared": `
agp_version = '9.2.1'
classpath "com.android.tools.build:gradle:${agp_version}"
implementation "example:unrelated:$agp_version"
`,
		"reassigned": `
agp_version = '9.2.1'
agp_version = providers.gradleProperty('agp')
classpath "com.android.tools.build:gradle:$agp_version"
`,
		"coordinate suffix": `
agp_version = '9.2.1'
classpath "com.android.tools.build:gradle:$agp_version-custom"
`,
	} {
		t.Run(name, func(t *testing.T) {
			if variables := FindAGPBuildscriptVariables(content); len(variables) != 0 {
				t.Fatalf("variables = %#v, want none", variables)
			}
		})
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

func regexpContains(value, pattern string) bool {
	return mustRegex(pattern).MatchString(value)
}

func mustRegex(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

func countLines(value string) int {
	return strings.Count(value, "\n")
}
