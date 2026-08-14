package migration

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/project"
	"github.com/gay00ung/aargrade/internal/toolchain"
)

const agpPluginIDExpression = `com\.android\.(?:application|library|test|dynamic-feature|asset-pack|asset-pack-bundle|privacy-sandbox-sdk|settings|lint|legacy-kapt|kotlin\.multiplatform\.library)`

var (
	literalAGPCallPattern             = regexp.MustCompile(`(?s)\bid\s*\(\s*["']` + agpPluginIDExpression + `["']\s*\)\s*version\s*["']([^"']+)["']`)
	literalAGPGroovyPattern           = regexp.MustCompile(`(?m)\bid\s+["']` + agpPluginIDExpression + `["']\s+version\s+["']([^"']+)["']`)
	literalAGPClasspathPattern        = regexp.MustCompile(`com\.android\.tools\.build:gradle:([0-9][0-9A-Za-z.+-]*)`)
	wrapperDistributionPattern        = regexp.MustCompile(`gradle-([0-9][0-9A-Za-z.+-]*)-(bin|all)\.zip`)
	wrapperChecksumPattern            = regexp.MustCompile(`(?m)^(\s*distributionSha256Sum\s*=\s*)([0-9A-Fa-f]{64})(\s*)$`)
	wrapperChecksumKeyPattern         = regexp.MustCompile(`(?m)^\s*distributionSha256Sum\s*=`)
	catalogVersionPattern             = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*=\s*["']([^"']+)["']\s*$`)
	catalogIDPattern                  = regexp.MustCompile(`\bid\s*=\s*["']([^"']+)["']`)
	catalogModulePattern              = regexp.MustCompile(`\bmodule\s*=\s*["']([^"']+)["']`)
	catalogInlineVersionPattern       = regexp.MustCompile(`\bversion\s*=\s*["']([^"']+)["']`)
	catalogVersionRefPattern          = regexp.MustCompile(`\bversion\.ref\s*=\s*["']([^"']+)["']`)
	catalogCoordinatePattern          = regexp.MustCompile(`^\s*[A-Za-z0-9_.-]+\s*=\s*["']([^"']+)["']\s*$`)
	kotlinAndroidIDPattern            = regexp.MustCompile(`(?:org\.jetbrains\.kotlin\.android|kotlin-android)`)
	kotlinAndroidCallPattern          = regexp.MustCompile(`\bkotlin\s*\(\s*["']android["']\s*\)`)
	kaptIDPattern                     = regexp.MustCompile(`(?:org\.jetbrains\.kotlin\.kapt|kotlin-kapt)`)
	kaptCallPattern                   = regexp.MustCompile(`\bkotlin\s*\(\s*["']kapt["']\s*\)`)
	kaptDependencyPattern             = regexp.MustCompile(`\bkapt(?:Test|AndroidTest)?\s*\(`)
	legacyKaptIDPattern               = regexp.MustCompile(`com\.android\.legacy-kapt`)
	kspLiteralCallPattern             = regexp.MustCompile(`(?s)\bid\s*\(\s*["']com\.google\.devtools\.ksp["']\s*\)\s*version\s*["']([^"']+)["']`)
	kspLiteralGroovyPattern           = regexp.MustCompile(`(?m)\bid\s+["']com\.google\.devtools\.ksp["']\s+version\s+["']([^"']+)["']`)
	kspIDPattern                      = regexp.MustCompile(`com\.google\.devtools\.ksp`)
	kspDependencyPattern              = regexp.MustCompile(`\bksp(?:Test|AndroidTest)?\s*\(`)
	kotlinOptionsPattern              = regexp.MustCompile(`\b(?:android\s*\.\s*)?kotlinOptions\s*\{|\bkotlin\s*\.\s*sourceSets\b`)
	kotlinBlockPattern                = regexp.MustCompile(`\bkotlin\s*\{`)
	sourceSetsPattern                 = regexp.MustCompile(`\bsourceSets\b`)
	javaKotlinSourceDirPattern        = regexp.MustCompile(`(?mi)\bjava\s*\.?\s*srcDirs?\b[^\n]*kotlin`)
	kmpPattern                        = regexp.MustCompile(`org\.jetbrains\.kotlin\.multiplatform|\bkotlin\s*\(\s*["']multiplatform["']\s*\)`)
	androidPluginPattern              = regexp.MustCompile(`com\.android\.(?:application|library)|libs\.plugins\.[A-Za-z0-9_.-]*android[A-Za-z0-9_.-]*(?:application|library)`)
	namespacePattern                  = regexp.MustCompile(`(?m)\bnamespace\s*(?:=\s*|\s+)["'][^"']+["']`)
	removedAGP9DSLPattern             = regexp.MustCompile(`\b(?:applicationVariants|libraryVariants|testVariants|unitTestVariants|variantFilter|BaseExtension|com\.android\.build\.gradle\.internal|dexOptions|aidlPackagedList|renderscript)\b`)
	buildConfigFieldMutationPattern   = regexp.MustCompile(`\bbuildConfigField\b`)
	buildConfigEnabledMutationPattern = regexp.MustCompile(`(?m)\bbuildConfig\s*(?:=\s*|\s+)true\b`)
	legacySDKMethodPattern            = regexp.MustCompile(`\b(?:compileSdkVersion|minSdkVersion|targetSdkVersion)\b`)
)

type migrationFile struct {
	path     string
	relative string
	content  string
	mode     uint32
}

type catalogVersion struct {
	key, value string
	line       int
}

type catalogEntry struct {
	section      string
	alias        string
	id           string
	module       string
	version      string
	versionRef   string
	line         int
	coordinate   bool
	isAGP        bool
	isKotlin     bool
	isKSP        bool
	isKapt       bool
	isLegacyKapt bool
}

type catalogModel struct {
	versions map[string]catalogVersion
	entries  []catalogEntry
}

func transformLiteralAGP(content, current, target string) (string, int) {
	result := content
	total := 0
	for _, pattern := range []*regexp.Regexp{literalAGPCallPattern, literalAGPGroovyPattern, literalAGPClasspathPattern} {
		var count int
		result, count = replaceCleanGroup(result, pattern, 1, current, target)
		total += count
	}
	return result, total
}

func literalAGPVersions(content string) []string {
	clean := project.StripComments(content)
	var result []string
	for _, pattern := range []*regexp.Regexp{literalAGPCallPattern, literalAGPGroovyPattern, literalAGPClasspathPattern} {
		for _, match := range pattern.FindAllStringSubmatch(clean, -1) {
			result = append(result, match[1])
		}
	}
	return sortAndUniqueStrings(result)
}

func replaceCleanGroup(content string, pattern *regexp.Regexp, group int, oldValue, newValue string) (string, int) {
	clean := project.StripComments(content)
	matches := pattern.FindAllStringSubmatchIndex(clean, -1)
	if len(matches) == 0 {
		return content, 0
	}
	var builder strings.Builder
	last := 0
	count := 0
	for _, match := range matches {
		startIndex := group * 2
		if startIndex+1 >= len(match) || match[startIndex] < 0 {
			continue
		}
		start, end := match[startIndex], match[startIndex+1]
		if content[start:end] != oldValue {
			continue
		}
		builder.WriteString(content[last:start])
		builder.WriteString(newValue)
		last = end
		count++
	}
	if count == 0 {
		return content, 0
	}
	builder.WriteString(content[last:])
	return builder.String(), count
}

func parseCatalog(content string) catalogModel {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	model := catalogModel{versions: map[string]catalogVersion{}}
	sections := make([]string, len(lines))
	section := ""
	for index, raw := range lines {
		code := strings.TrimSpace(tomlCode(raw))
		if strings.HasPrefix(code, "[") && strings.HasSuffix(code, "]") {
			section = strings.Trim(code, "[] ")
			sections[index] = section
			continue
		}
		sections[index] = section
		if section == "versions" {
			if match := catalogVersionPattern.FindStringSubmatch(code); len(match) > 0 {
				model.versions[match[1]] = catalogVersion{key: match[1], value: match[2], line: index}
			}
		}
	}
	for index, raw := range lines {
		section = sections[index]
		if section != "plugins" && section != "libraries" {
			continue
		}
		code := strings.TrimSpace(tomlCode(raw))
		parts := strings.SplitN(code, "=", 2)
		if len(parts) != 2 {
			continue
		}
		entry := catalogEntry{section: section, alias: strings.TrimSpace(parts[0]), line: index}
		if match := catalogVersionRefPattern.FindStringSubmatch(parts[1]); len(match) > 0 {
			entry.versionRef = match[1]
			entry.version = model.versions[entry.versionRef].value
		} else if match := catalogInlineVersionPattern.FindStringSubmatch(parts[1]); len(match) > 0 {
			entry.version = match[1]
		}
		if section == "plugins" {
			if match := catalogIDPattern.FindStringSubmatch(parts[1]); len(match) > 0 {
				entry.id = match[1]
			}
		} else if match := catalogModulePattern.FindStringSubmatch(parts[1]); len(match) > 0 {
			entry.module = match[1]
		} else if match := catalogCoordinatePattern.FindStringSubmatch(code); len(match) > 0 {
			segments := strings.Split(match[1], ":")
			if len(segments) >= 2 {
				entry.module = strings.Join(segments[:2], ":")
			}
			if len(segments) >= 3 {
				entry.version = segments[2]
				entry.coordinate = true
			}
		}
		entry.isAGP = isKnownAGPPluginID(entry.id) || entry.module == "com.android.tools.build:gradle"
		entry.isKotlin = entry.id == "org.jetbrains.kotlin.android" || entry.id == "kotlin-android"
		entry.isKSP = entry.id == "com.google.devtools.ksp"
		entry.isKapt = entry.id == "org.jetbrains.kotlin.kapt" || entry.id == "kotlin-kapt"
		entry.isLegacyKapt = entry.id == "com.android.legacy-kapt"
		model.entries = append(model.entries, entry)
	}
	return model
}

func isKnownAGPPluginID(id string) bool {
	switch id {
	case "com.android.application", "com.android.library", "com.android.test", "com.android.dynamic-feature", "com.android.asset-pack", "com.android.asset-pack-bundle", "com.android.privacy-sandbox-sdk", "com.android.settings", "com.android.lint", "com.android.legacy-kapt", "com.android.kotlin.multiplatform.library":
		return true
	default:
		return false
	}
}

func transformCatalog(content, current, target string) (string, int, []string) {
	model := parseCatalog(content)
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	refUsers := map[string][]catalogEntry{}
	for _, entry := range model.entries {
		if entry.versionRef != "" {
			refUsers[entry.versionRef] = append(refUsers[entry.versionRef], entry)
		}
	}
	refsToUpdate := map[string]bool{}
	var blockers []string
	count := 0
	for _, entry := range model.entries {
		if !entry.isAGP {
			continue
		}
		location := fmt.Sprintf("gradle/libs.versions.toml:%d", entry.line+1)
		if entry.version == "" {
			blockers = append(blockers, location+": AGP catalog entry has no statically resolved version")
			continue
		}
		if entry.version != current {
			blockers = append(blockers, fmt.Sprintf("%s: AGP catalog entry is %s, expected %s", location, entry.version, current))
			continue
		}
		if entry.versionRef != "" {
			for _, user := range refUsers[entry.versionRef] {
				if !user.isAGP {
					blockers = append(blockers, fmt.Sprintf("%s: version.ref %q is shared with non-AGP catalog entry %q", location, entry.versionRef, user.alias))
				}
			}
			refsToUpdate[entry.versionRef] = true
			continue
		}
		updated, replacements := replaceCatalogEntryVersion(lines[entry.line], entry, current, target)
		if replacements != 1 {
			blockers = append(blockers, location+": could not safely rewrite the AGP version")
			continue
		}
		lines[entry.line] = updated
		count++
	}
	for ref := range refsToUpdate {
		version, ok := model.versions[ref]
		if !ok || version.value != current {
			blockers = append(blockers, fmt.Sprintf("gradle/libs.versions.toml: AGP version.ref %q is missing or not %s", ref, current))
			continue
		}
		updated, replacements := replaceCatalogVersionLine(lines[version.line], current, target)
		if replacements != 1 {
			blockers = append(blockers, fmt.Sprintf("gradle/libs.versions.toml:%d: could not safely rewrite version %q", version.line+1, ref))
			continue
		}
		lines[version.line] = updated
		count++
	}
	if len(blockers) > 0 {
		return content, 0, sortAndUniqueStrings(blockers)
	}
	result := strings.Join(lines, lineEnding)
	for _, entry := range parseCatalog(result).entries {
		if entry.isAGP && entry.version != target {
			return content, 0, []string{fmt.Sprintf("gradle/libs.versions.toml:%d: AGP version remained %q after preview", entry.line+1, entry.version)}
		}
	}
	return result, count, nil
}

func replaceCatalogEntryVersion(line string, entry catalogEntry, current, target string) (string, int) {
	if entry.coordinate {
		pattern := regexp.MustCompile(`(com\.android\.tools\.build:gradle:)` + regexp.QuoteMeta(current) + `\b`)
		updated := pattern.ReplaceAllString(line, `${1}`+target)
		if updated != line {
			return updated, 1
		}
		return line, 0
	}
	pattern := regexp.MustCompile(`(\bversion\s*=\s*["'])` + regexp.QuoteMeta(current) + `(["'])`)
	updated := pattern.ReplaceAllString(line, `${1}`+target+`${2}`)
	if updated != line {
		return updated, 1
	}
	return line, 0
}

func replaceCatalogVersionLine(line, current, target string) (string, int) {
	pattern := regexp.MustCompile(`(=\s*["'])` + regexp.QuoteMeta(current) + `(["'])`)
	updated := pattern.ReplaceAllString(line, `${1}`+target+`${2}`)
	if updated != line {
		return updated, 1
	}
	return line, 0
}

func catalogAliases(model catalogModel, predicate func(catalogEntry) bool) []string {
	var aliases []string
	for _, entry := range model.entries {
		if predicate(entry) {
			aliases = append(aliases, catalogAccessor(entry.alias))
		}
	}
	return sortAndUniqueStrings(aliases)
}

func catalogAccessor(alias string) string {
	alias = strings.TrimSpace(alias)
	alias = strings.ReplaceAll(alias, "-", ".")
	alias = strings.ReplaceAll(alias, "_", ".")
	return alias
}

func tomlCode(line string) string {
	quote := byte(0)
	escaped := false
	data := []byte(line)
	for index, current := range data {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' && quote == '"' {
				escaped = true
			} else if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			continue
		}
		if current == '#' {
			return line[:index]
		}
	}
	return line
}

func removeKotlinAndroidPluginLines(content string, aliases []string) (string, int, []int) {
	parts := strings.SplitAfter(content, "\n")
	cleanLines := make([]string, len(parts))
	for index, part := range parts {
		cleanLines[index] = strings.TrimSpace(project.StripComments(strings.TrimSuffix(part, "\n")))
	}
	var builder strings.Builder
	removed := 0
	var unsafe []int
	for index, part := range parts {
		clean := cleanLines[index]
		if clean == "" || !containsKotlinAndroidPlugin(clean, aliases) {
			builder.WriteString(part)
			continue
		}
		continuation := false
		for next := index + 1; next < len(cleanLines); next++ {
			if cleanLines[next] == "" {
				continue
			}
			continuation = strings.HasPrefix(cleanLines[next], "version ") || strings.HasPrefix(cleanLines[next], "apply ")
			break
		}
		if isStandaloneKotlinAndroidPlugin(clean, aliases) && !continuation {
			removed++
			continue
		}
		unsafe = append(unsafe, index+1)
		builder.WriteString(part)
	}
	return builder.String(), removed, unsafe
}

func containsKotlinAndroidPlugin(line string, aliases []string) bool {
	if kotlinAndroidIDPattern.MatchString(line) || kotlinAndroidCallPattern.MatchString(line) {
		return true
	}
	for _, alias := range aliases {
		if strings.Contains(line, "libs.plugins."+alias) {
			return true
		}
	}
	return false
}

func isStandaloneKotlinAndroidPlugin(line string, aliases []string) bool {
	line = strings.TrimSuffix(strings.TrimSpace(line), ";")
	suffix := `(?:\s+version\s+(?:["'][^"']+["']|[A-Za-z0-9_.()]+))?(?:\s+apply\s*(?:\(\s*)?false\s*\)?)?`
	patterns := []string{
		`id\s*\(\s*["'](?:org\.jetbrains\.kotlin\.android|kotlin-android)["']\s*\)` + suffix,
		`id\s+["'](?:org\.jetbrains\.kotlin\.android|kotlin-android)["']` + suffix,
		`kotlin\s*\(\s*["']android["']\s*\)` + suffix,
		`apply\s*\(\s*plugin\s*=\s*["'](?:org\.jetbrains\.kotlin\.android|kotlin-android)["']\s*\)`,
		`apply\s+plugin\s*:\s*["'](?:org\.jetbrains\.kotlin\.android|kotlin-android)["']`,
	}
	for _, alias := range aliases {
		patterns = append(patterns, `alias\s*\(\s*libs\.plugins\.`+regexp.QuoteMeta(alias)+`\s*\)`+suffix)
	}
	for _, raw := range patterns {
		if regexp.MustCompile(`^` + raw + `$`).MatchString(line) {
			return true
		}
	}
	return false
}

func scanKSPVersions(files []migrationFile, model catalogModel) (bool, []string) {
	detected := false
	var versions []string
	aliasVersions := map[string]string{}
	for _, entry := range model.entries {
		if entry.isKSP {
			aliasVersions[catalogAccessor(entry.alias)] = entry.version
		}
	}
	for _, file := range files {
		clean := project.StripComments(file.content)
		if kspIDPattern.MatchString(clean) {
			detected = true
		}
		if kspDependencyPattern.MatchString(clean) {
			detected = true
		}
		for alias, version := range aliasVersions {
			if strings.Contains(clean, "libs.plugins."+alias) {
				detected = true
				if version != "" {
					versions = append(versions, version)
				}
			}
		}
		for _, pattern := range []*regexp.Regexp{kspLiteralCallPattern, kspLiteralGroovyPattern} {
			for _, match := range pattern.FindAllStringSubmatch(clean, -1) {
				versions = append(versions, match[1])
			}
		}
	}
	return detected, sortAndUniqueStrings(versions)
}

func hasKapt(files []migrationFile, aliases, legacyAliases []string) []string {
	var evidence []string
	for _, file := range files {
		clean := project.StripComments(file.content)
		if kaptIDPattern.MatchString(clean) || kaptCallPattern.MatchString(clean) || (kaptDependencyPattern.MatchString(clean) && !hasLegacyKapt(file, legacyAliases)) {
			evidence = append(evidence, file.relative)
			continue
		}
		for _, alias := range aliases {
			if strings.Contains(clean, "libs.plugins."+alias) {
				evidence = append(evidence, file.relative)
				break
			}
		}
	}
	return sortAndUniqueStrings(evidence)
}

func hasLegacyKapt(file migrationFile, aliases []string) bool {
	clean := project.StripComments(file.content)
	if legacyKaptIDPattern.MatchString(clean) {
		return true
	}
	for _, alias := range aliases {
		if strings.Contains(clean, "libs.plugins."+alias) {
			return true
		}
	}
	return false
}

func transformWrapper(content, current, target string) (string, string, error) {
	matches := wrapperDistributionPattern.FindAllStringSubmatch(content, -1)
	if len(matches) != 1 || matches[0][1] != current {
		return content, "", fmt.Errorf("Wrapper must contain exactly one literal Gradle %s distribution URL", current)
	}
	flavor := matches[0][2]
	checksum, err := toolchain.GradleDistributionSHA256(target, flavor)
	if err != nil {
		return content, "", err
	}
	updated := strings.Replace(content, "gradle-"+current+"-"+flavor+".zip", "gradle-"+target+"-"+flavor+".zip", 1)
	checksumMatches := wrapperChecksumPattern.FindAllStringSubmatch(updated, -1)
	if len(checksumMatches) > 1 || (wrapperChecksumKeyPattern.MatchString(updated) && len(checksumMatches) != 1) {
		return content, "", fmt.Errorf("Wrapper distributionSha256Sum is duplicated or not a literal 64-character checksum")
	}
	if len(checksumMatches) == 1 {
		updated = wrapperChecksumPattern.ReplaceAllString(updated, `${1}`+checksum+`${3}`)
	} else {
		lineEnding := "\n"
		if strings.Contains(updated, "\r\n") {
			lineEnding = "\r\n"
		}
		urlLine := regexp.MustCompile(`(?m)^\s*distributionUrl\s*=.*$`)
		location := urlLine.FindStringIndex(updated)
		if location == nil {
			return content, "", fmt.Errorf("Wrapper distributionUrl line could not be located")
		}
		insertAt := location[1]
		insertion := lineEnding + "distributionSha256Sum=" + checksum
		if insertAt < len(updated) && updated[insertAt] == '\n' {
			insertAt++
			insertion = "distributionSha256Sum=" + checksum + lineEnding
		}
		updated = updated[:insertAt] + insertion + updated[insertAt:]
	}
	return updated, checksum, nil
}

func removeObsoleteAGP9Properties(content string) (string, int) {
	keys := map[string]bool{
		"android.builtInKotlin":              true,
		"android.newDsl":                     true,
		"android.uniquePackageNames":         true,
		"android.enableAppCompileTimeRClass": true,
	}
	parts := strings.SplitAfter(content, "\n")
	var builder strings.Builder
	removed := 0
	propertyPattern := regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*[:=]`)
	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.TrimSuffix(part, "\n"))
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			builder.WriteString(part)
			continue
		}
		match := propertyPattern.FindStringSubmatch(trimmed)
		if len(match) > 0 && keys[match[1]] {
			removed++
			continue
		}
		builder.WriteString(part)
	}
	return builder.String(), removed
}

func patternLocations(file migrationFile, pattern *regexp.Regexp, label string) []string {
	clean := project.StripComments(file.content)
	var result []string
	for _, location := range pattern.FindAllStringIndex(clean, -1) {
		line := strings.Count(clean[:location[0]], "\n") + 1
		result = append(result, fmt.Sprintf("%s:%d: %s", file.relative, line, label))
	}
	return result
}

func sortMigrationFiles(files []migrationFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].relative < files[j].relative })
}
