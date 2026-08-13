package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/model"
	"github.com/gay00ung/aargrade/internal/project"
)

var (
	legacyAPIPattern           = regexp.MustCompile(`\b(applicationVariants|libraryVariants|testVariants|unitTestVariants|BaseExtension|com\.android\.build\.gradle\.internal)\b`)
	buildConfigFieldPattern    = regexp.MustCompile(`\bbuildConfigField\b`)
	buildConfigEnabledPattern  = regexp.MustCompile(`(?m)\bbuildConfig\s*(?:=\s*|\s+)true\b`)
	consumerRulesCallPattern   = regexp.MustCompile(`(?s)\bconsumerProguardFiles\s*\(([^)]*)\)`)
	consumerRulesGroovyPattern = regexp.MustCompile(`(?m)\bconsumerProguardFiles\s+([^\n]+)`)
	quotedFilePattern          = regexp.MustCompile(`["']([^"']+)["']`)
	globalR8Pattern            = regexp.MustCompile(`(?m)^\s*(-(?:dontoptimize|dontshrink|dontobfuscate|repackageclasses|allowaccessmodification|processkotlinnullchecks))\b`)
	nativeBuildPattern         = regexp.MustCompile(`\b(externalNativeBuild|cmake|ndkVersion|abiFilters)\b|\bndk\s*\{`)
)

func Analyze(root, toolVersion string) (model.Report, error) {
	discovered, err := project.Discover(root)
	if err != nil {
		return model.Report{}, err
	}

	report := model.Report{
		SchemaVersion: model.ReportSchemaVersion,
		ToolVersion:   toolVersion,
		ProjectRoot:   discovered.Root,
		Inventory: model.Inventory{
			SettingsFile: relative(discovered.Root, discovered.SettingsFile),
			Gradle: model.Version{
				Value:  discovered.WrapperVersion,
				Source: relative(discovered.Root, discovered.WrapperFile),
			},
			Modules: make([]model.Module, 0, len(discovered.Modules)),
		},
	}

	for _, module := range discovered.Modules {
		plugins := make([]string, 0, len(module.Plugins))
		for _, plugin := range module.Plugins {
			plugins = append(plugins, plugin.ID)
		}
		report.Inventory.Modules = append(report.Inventory.Modules, model.Module{
			GradlePath: module.GradlePath,
			Directory:  relative(discovered.Root, module.Directory),
			BuildFile:  relative(discovered.Root, module.BuildFile),
			Kind:       module.Kind(),
			Plugins:    plugins,
		})
	}

	uniqueAGP := uniqueVersions(discovered.AGPVersions)
	if len(uniqueAGP) == 1 {
		report.Inventory.AGP = model.Version{
			Value:  uniqueAGP[0],
			Source: agpSource(discovered.Root, discovered.AGPVersions, uniqueAGP[0]),
		}
	}

	report.Findings = append(report.Findings, structuralFindings(discovered, uniqueAGP)...)
	report.Findings = append(report.Findings, migrationFindings(discovered)...)
	report.Findings = append(report.Findings, r8Findings(discovered)...)
	sortFindings(report.Findings)
	return report, nil
}

func structuralFindings(discovered *project.Project, uniqueAGP []string) []model.Finding {
	var findings []model.Finding
	if discovered.WrapperFile == "" {
		findings = append(findings, model.Finding{
			ID:             "gradle.wrapper.missing",
			Severity:       model.SeverityWarn,
			Title:          "Gradle Wrapper not found",
			Message:        "No gradle/wrapper/gradle-wrapper.properties file was found. Reproducible verification needs a project-owned Gradle version.",
			Recommendation: "Add or restore the Gradle Wrapper before relying on build verification.",
		})
	} else if discovered.WrapperVersion == "" {
		findings = append(findings, model.Finding{
			ID:             "gradle.wrapper.version-unresolved",
			Severity:       model.SeverityWarn,
			Title:          "Gradle Wrapper version is not literal",
			Message:        "The wrapper file exists, but AARGrade could not extract a Gradle distribution version.",
			Recommendation: "Inspect distributionUrl and record the effective Gradle version explicitly.",
			Evidence:       []model.Evidence{{Path: relative(discovered.Root, discovered.WrapperFile)}},
		})
	}

	var apps, libraries int
	var missingBuild []model.Evidence
	var unresolvedAndroid []model.Evidence
	for _, module := range discovered.Modules {
		switch module.Kind() {
		case "application":
			apps++
		case "library":
			libraries++
		}
		if module.BuildFile == "" {
			missingBuild = append(missingBuild, model.Evidence{Path: relative(discovered.Root, module.Directory)})
		}
		if module.Kind() == "unknown" {
			manifest := filepath.Join(module.Directory, "src", "main", "AndroidManifest.xml")
			if info, err := os.Stat(manifest); err == nil && !info.IsDir() {
				unresolvedAndroid = append(unresolvedAndroid, model.Evidence{Path: relative(discovered.Root, module.BuildFile)})
			}
		}
	}
	if len(missingBuild) > 0 {
		findings = append(findings, model.Finding{
			ID:             "gradle.module.build-file-missing",
			Severity:       model.SeverityWarn,
			Title:          "Included module has no build file",
			Message:        "One or more settings entries resolve to a directory without build.gradle.kts or build.gradle.",
			Recommendation: "Correct the include/projectDir mapping or remove stale module declarations.",
			Evidence:       missingBuild,
		})
	}
	if len(discovered.HasAmbiguousBuildFiles) > 0 {
		evidence := make([]model.Evidence, 0, len(discovered.HasAmbiguousBuildFiles))
		for _, path := range discovered.HasAmbiguousBuildFiles {
			evidence = append(evidence, model.Evidence{Path: path})
		}
		findings = append(findings, model.Finding{
			ID:             "gradle.module.build-file-ambiguous",
			Severity:       model.SeverityError,
			Title:          "Module has both Gradle DSL build files",
			Message:        "A module contains both build.gradle.kts and build.gradle, so its effective build logic is ambiguous.",
			Recommendation: "Keep one build file per module before running migration tooling.",
			Evidence:       evidence,
		})
	}
	if len(unresolvedAndroid) > 0 {
		findings = append(findings, model.Finding{
			ID:             "android.model.unresolved",
			Severity:       model.SeverityWarn,
			Title:          "Android-looking module could not be classified",
			Message:        "An AndroidManifest.xml exists, but no literal or default-catalog Android plugin was resolved. Convention or dynamic build logic may own the plugin.",
			Recommendation: "Inspect the convention plugin manually; AARGrade will not guess the module type.",
			Evidence:       unresolvedAndroid,
		})
	}

	switch {
	case apps > 0:
		findings = append(findings, model.Finding{
			ID:       "android.application.present",
			Severity: model.SeverityInfo,
			Title:    "Android application module found",
			Message:  fmt.Sprintf("Detected %d application and %d library module(s). A temporary upgrade host is not indicated by project structure alone.", apps, libraries),
		})
	case libraries > 0:
		findings = append(findings, model.Finding{
			ID:             "android.library-only",
			Severity:       model.SeverityWarn,
			Title:          "Library-only Android project",
			Message:        fmt.Sprintf("Detected %d Android library module(s) and no application module. This is an inventory fact, not proof that Upgrade Assistant will fail.", libraries),
			Recommendation: "Reproduce the Upgrade Assistant issue first; use `aargrade host add` only if an application model is demonstrably required.",
			Evidence:       []model.Evidence{{Path: relative(discovered.Root, discovered.SettingsFile)}},
		})
	default:
		findings = append(findings, model.Finding{
			ID:             "android.modules.none",
			Severity:       model.SeverityWarn,
			Title:          "No Android module resolved",
			Message:        "AARGrade found no application or library plugin in the statically discoverable modules.",
			Recommendation: "Check settings includes and convention plugins. Static analysis does not evaluate Gradle build logic.",
		})
	}

	if apps+libraries > 0 && len(uniqueAGP) == 0 {
		findings = append(findings, model.Finding{
			ID:             "agp.version.unresolved",
			Severity:       model.SeverityWarn,
			Title:          "AGP version could not be resolved",
			Message:        "No literal AGP version was found in plugin declarations, the default version catalog, or a buildscript classpath.",
			Recommendation: "Record the effective AGP version manually before planning a migration.",
		})
	}
	if len(uniqueAGP) > 1 {
		evidence := make([]model.Evidence, 0, len(discovered.AGPVersions))
		for _, item := range discovered.AGPVersions {
			evidence = append(evidence, model.Evidence{Path: item.Path, Line: item.Line, Snippet: item.Value})
		}
		findings = append(findings, model.Finding{
			ID:             "agp.version.conflict",
			Severity:       model.SeverityError,
			Title:          "Conflicting AGP versions found",
			Message:        fmt.Sprintf("Static declarations contain multiple AGP versions: %s.", strings.Join(uniqueAGP, ", ")),
			Recommendation: "Resolve the declarations to one effective AGP version before migration.",
			Evidence:       evidence,
		})
	}
	return findings
}

func migrationFindings(discovered *project.Project) []model.Finding {
	var findings []model.Finding
	if discovered.HasBuildSrc {
		findings = append(findings, model.Finding{
			ID:             "upgrade-assistant.buildsrc",
			Severity:       model.SeverityWarn,
			Title:          "buildSrc can hide migration inputs",
			Message:        "The official Upgrade Assistant documents buildSrc constants and variables as unsupported static-analysis inputs.",
			Recommendation: "Move migration-critical plugin versions to supported literal or default-catalog declarations, or plan a manual step.",
			Evidence:       []model.Evidence{{Path: "buildSrc"}},
		})
	}
	if discovered.HasSettingsVersionCatalogs {
		findings = append(findings, model.Finding{
			ID:             "upgrade-assistant.settings-version-catalog",
			Severity:       model.SeverityWarn,
			Title:          "Settings-defined version catalog detected",
			Message:        "Upgrade Assistant documents version catalogs defined in settings files as unsupported for its static analysis.",
			Recommendation: "Use the conventional gradle/libs.versions.toml catalog for migration-critical plugin versions or plan manual edits.",
			Evidence:       evidenceForText(discovered.Root, discovered.SettingsFile, discovered.SettingsContent, "versionCatalogs"),
		})
	}
	if discovered.HasRefreshVersions {
		findings = append(findings, model.Finding{
			ID:             "gradle.version-manager.refresh-versions",
			Severity:       model.SeverityWarn,
			Title:          "RefreshVersions-managed plugin versions detected",
			Message:        "AARGrade does not evaluate RefreshVersions' generated plugin version mapping, so an unresolved AGP version is expected from static analysis.",
			Recommendation: "Pass explicit versions to mutation commands and record the effective AGP version in migration evidence.",
			Evidence:       evidenceForText(discovered.Root, discovered.SettingsFile, discovered.SettingsContent, "de.fayard.refreshVersions"),
		})
	}

	pluginGroups := []struct {
		ids                                       []string
		findingID, title, message, recommendation string
		severity                                  model.Severity
		androidOnly                               bool
	}{
		{
			ids: []string{"org.jetbrains.kotlin.android", "kotlin-android"}, findingID: "agp9.kotlin-android-plugin", severity: model.SeverityWarn,
			title:          "Kotlin Android plugin requires an AGP 9 decision",
			message:        "AGP 9 enables built-in Kotlin by default; the Kotlin Android plugin cannot remain applied in that mode.",
			recommendation: "Plan built-in Kotlin migration or document a temporary android.builtInKotlin=false opt-out.",
		},
		{
			ids: []string{"org.jetbrains.kotlin.kapt", "kotlin-kapt"}, findingID: "agp9.kapt-plugin", severity: model.SeverityWarn, androidOnly: true,
			title:          "kapt migration signal detected",
			message:        "kapt needs explicit review when moving to AGP 9 built-in Kotlin.",
			recommendation: "Prefer KSP where supported and follow the AGP 9 kapt migration guidance for remaining processors.",
		},
		{
			ids: []string{"com.google.devtools.ksp"}, findingID: "agp9.ksp-plugin", severity: model.SeverityInfo, androidOnly: true,
			title:          "KSP plugin detected",
			message:        "KSP version compatibility is part of an AGP 9 migration and should be recorded in the plan.",
			recommendation: "Verify the KSP version against the selected AGP/Kotlin combination.",
		},
	}
	for _, group := range pluginGroups {
		var evidence []model.Evidence
		for _, module := range discovered.Modules {
			if group.androidOnly && !isAndroidKind(module.Kind()) {
				continue
			}
			for _, plugin := range module.Plugins {
				for _, id := range group.ids {
					if plugin.ID == id {
						evidence = append(evidence, model.Evidence{Path: plugin.Source, Line: plugin.Line, Snippet: plugin.ID})
					}
				}
			}
		}
		if len(evidence) > 0 {
			findings = append(findings, model.Finding{
				ID: group.findingID, Severity: group.severity, Title: group.title,
				Message: group.message, Recommendation: group.recommendation, Evidence: evidence,
			})
		}
	}

	var legacy, buildConfig, native []model.Evidence
	for _, module := range discovered.Modules {
		if module.BuildFile == "" {
			continue
		}
		clean := project.StripComments(module.Content)
		legacy = append(legacy, evidenceForPattern(discovered.Root, module.BuildFile, clean, legacyAPIPattern)...)
		if module.Kind() == "library" && buildConfigFieldPattern.MatchString(clean) && !buildConfigEnabledPattern.MatchString(clean) {
			buildConfig = append(buildConfig, evidenceForPattern(discovered.Root, module.BuildFile, clean, buildConfigFieldPattern)...)
		}
		native = append(native, evidenceForPattern(discovered.Root, module.BuildFile, clean, nativeBuildPattern)...)
	}
	if len(legacy) > 0 {
		findings = append(findings, model.Finding{
			ID: "agp9.legacy-api", Severity: model.SeverityWarn, Title: "Legacy Android Gradle API detected",
			Message:        "The build references a legacy Variant API, BaseExtension, or an internal AGP type that is removed or hidden by AGP 9's new DSL mode.",
			Recommendation: "Migrate to public DSL interfaces and androidComponents before removing compatibility opt-outs.", Evidence: legacy,
		})
	}
	if len(buildConfig) > 0 {
		findings = append(findings, model.Finding{
			ID: "android.buildconfig.feature-implicit", Severity: model.SeverityWarn, Title: "Custom BuildConfig field may rely on an old default",
			Message:        "A library declares buildConfigField but static analysis did not find buildFeatures.buildConfig enabled in the same build file.",
			Recommendation: "Enable buildFeatures.buildConfig explicitly and confirm the generated API before upgrading.", Evidence: buildConfig,
		})
	}
	if len(native) > 0 {
		findings = append(findings, model.Finding{
			ID: "android.native-build.present", Severity: model.SeverityInfo, Title: "Native build configuration detected",
			Message:        "NDK, CMake, or ABI packaging needs artifact-level comparison; static doctor checks are not a compatibility verdict.",
			Recommendation: "Include JNI ABI and packaged native-library comparison in `verify`.", Evidence: native,
		})
	}

	if discovered.GradleProperties != "" {
		for _, property := range []string{"android.newDsl=false", "android.builtInKotlin=false"} {
			if evidence := evidenceForText(discovered.Root, discovered.GradlePropertiesFile, discovered.GradleProperties, property); len(evidence) > 0 {
				findings = append(findings, model.Finding{
					ID: "agp9.compatibility-opt-out." + strings.Split(property, "=")[0], Severity: model.SeverityWarn,
					Title:          "Temporary AGP compatibility opt-out enabled",
					Message:        fmt.Sprintf("%s is an escape hatch, not a durable migration endpoint.", property),
					Recommendation: "Record an owner and removal milestone before AGP 10 makes the modern APIs mandatory.", Evidence: evidence,
				})
			}
		}
	}
	return findings
}

func isAndroidKind(kind string) bool {
	switch kind {
	case "application", "library", "test", "dynamic-feature":
		return true
	default:
		return false
	}
}

func r8Findings(discovered *project.Project) []model.Finding {
	var evidence []model.Evidence
	seen := map[string]bool{}
	for _, module := range discovered.Modules {
		if module.Kind() != "library" || module.BuildFile == "" {
			continue
		}
		for _, rulesPath := range consumerRuleFiles(module) {
			absolute := filepath.Clean(filepath.Join(module.Directory, filepath.FromSlash(rulesPath)))
			if !within(discovered.Root, absolute) || seen[absolute] {
				continue
			}
			seen[absolute] = true
			content, err := os.ReadFile(absolute)
			if err != nil {
				continue
			}
			evidence = append(evidence, evidenceForPattern(discovered.Root, absolute, string(content), globalR8Pattern)...)
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []model.Finding{{
		ID: "r8.consumer-global-option", Severity: model.SeverityWarn, Title: "Global R8 option in consumer rules",
		Message:        "A consumer keep-rule file contains a global option that affects the consuming app or is filtered by modern AGP versions.",
		Recommendation: "Remove global optimization toggles from consumer rules and keep only narrowly scoped library requirements.", Evidence: evidence,
	}}
}

func consumerRuleFiles(module project.Module) []string {
	clean := project.StripComments(module.Content)
	set := map[string]bool{}
	for _, pattern := range []*regexp.Regexp{consumerRulesCallPattern, consumerRulesGroovyPattern} {
		for _, match := range pattern.FindAllStringSubmatch(clean, -1) {
			for _, file := range quotedFilePattern.FindAllStringSubmatch(match[1], -1) {
				set[file[1]] = true
			}
		}
	}
	result := make([]string, 0, len(set))
	for file := range set {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

func evidenceForPattern(root, path, content string, pattern *regexp.Regexp) []model.Evidence {
	var result []model.Evidence
	for _, location := range pattern.FindAllStringIndex(content, -1) {
		result = append(result, model.Evidence{
			Path: relative(root, path), Line: lineAt(content, location[0]), Snippet: snippetAt(content, location[0]),
		})
	}
	return result
}

func evidenceForText(root, path, content, text string) []model.Evidence {
	var result []model.Evidence
	for offset := 0; ; {
		index := strings.Index(content[offset:], text)
		if index < 0 {
			break
		}
		absolute := offset + index
		result = append(result, model.Evidence{Path: relative(root, path), Line: lineAt(content, absolute), Snippet: snippetAt(content, absolute)})
		offset = absolute + len(text)
	}
	return result
}

func uniqueVersions(values []project.VersionEvidence) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value.Value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func agpSource(root string, values []project.VersionEvidence, version string) string {
	for _, value := range values {
		if value.Value == version {
			location := relative(root, value.Path)
			if value.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, value.Line)
			}
			return location
		}
	}
	return ""
}

func sortFindings(findings []model.Finding) {
	for index := range findings {
		sort.Slice(findings[index].Evidence, func(i, j int) bool {
			if findings[index].Evidence[i].Path != findings[index].Evidence[j].Path {
				return findings[index].Evidence[i].Path < findings[index].Evidence[j].Path
			}
			return findings[index].Evidence[i].Line < findings[index].Evidence[j].Line
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := model.SeverityRank(findings[i].Severity), model.SeverityRank(findings[j].Severity)
		if left != right {
			return left > right
		}
		return findings[i].ID < findings[j].ID
	})
}

func relative(root, path string) string {
	if path == "" {
		return ""
	}
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relativePath)
}

func within(root, path string) bool {
	relativePath, err := filepath.Rel(root, path)
	return err == nil && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func lineAt(content string, offset int) int {
	return strings.Count(content[:offset], "\n") + 1
}

func snippetAt(content string, offset int) string {
	start := strings.LastIndex(content[:offset], "\n") + 1
	endOffset := strings.Index(content[offset:], "\n")
	end := len(content)
	if endOffset >= 0 {
		end = offset + endOffset
	}
	snippet := strings.TrimSpace(content[start:end])
	if len(snippet) > 160 {
		snippet = snippet[:157] + "..."
	}
	return snippet
}
