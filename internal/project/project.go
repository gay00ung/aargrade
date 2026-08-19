package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxBuildFileSize = 4 << 20

var errAmbiguousBuildFiles = errors.New("ambiguous Gradle build files")

var (
	quotedGradlePathPattern        = regexp.MustCompile(`["'](:?[A-Za-z0-9_.-]+(?::[A-Za-z0-9_.-]+)*)["']`)
	includeCallPattern             = regexp.MustCompile(`(?m)\binclude\s*\(([^)]*)\)`)
	includeStatementPattern        = regexp.MustCompile(`(?m)^\s*include\s+([^\n]+)$`)
	projectDirPattern              = regexp.MustCompile(`(?m)project\s*\(\s*["'](:[^"']+)["']\s*\)\s*\.projectDir\s*=\s*file\s*\(\s*["']([^"']+)["']\s*\)`)
	projectDirGroovyPattern        = regexp.MustCompile(`(?m)project\s*\(\s*["'](:[^"']+)["']\s*\)\s*\.projectDir\s*=\s*file\s+["']([^"']+)["']`)
	wrapperVersionPattern          = regexp.MustCompile(`gradle-([0-9][0-9A-Za-z.+-]*)-(?:bin|all)\.zip`)
	pluginIDCallPattern            = regexp.MustCompile(`\bid\s*\(\s*["']([^"']+)["']\s*\)`)
	pluginIDGroovyPattern          = regexp.MustCompile(`\bid\s+["']([^"']+)["']`)
	applyPluginCallPattern         = regexp.MustCompile(`\bapply\s*\(\s*plugin\s*=\s*["']([^"']+)["']\s*\)`)
	applyPluginGroovyPattern       = regexp.MustCompile(`\bapply\s+plugin\s*:\s*["']([^"']+)["']`)
	pluginAliasPattern             = regexp.MustCompile(`\balias\s*\(\s*libs\.plugins\.([A-Za-z0-9_.-]+)\s*\)`)
	applyFalsePattern              = regexp.MustCompile(`\bapply\s*(?:\(\s*)?false\b`)
	pluginVersionCallPattern       = regexp.MustCompile(`(?s)\bid\s*\(\s*["'](com\.android\.(?:application|library))["']\s*\)\s*version\s*["']([^"']+)["']`)
	pluginVersionGroovyPattern     = regexp.MustCompile(`(?m)\bid\s+["'](com\.android\.(?:application|library))["']\s+version\s+["']([^"']+)["']`)
	classpathAGPPattern            = regexp.MustCompile(`com\.android\.tools\.build:gradle:([0-9][0-9A-Za-z.+-]*)`)
	classpathAGPVariablePattern    = regexp.MustCompile(`["']com\.android\.tools\.build:gradle:\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))["']`)
	gradleLiteralAssignmentPattern = regexp.MustCompile(`(?m)^\s*(?:ext\.)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*["']([0-9][0-9A-Za-z.+-]*)["']\s*$`)
)

type Plugin struct {
	ID      string
	Version string
	Source  string
	Line    int
}

type Module struct {
	GradlePath string
	Directory  string
	BuildFile  string
	Content    string
	Plugins    []Plugin
}

func (m Module) Kind() string {
	for _, plugin := range m.Plugins {
		switch plugin.ID {
		case "com.android.application":
			return "application"
		case "com.android.library", "com.android.kotlin.multiplatform.library":
			return "library"
		case "com.android.test":
			return "test"
		case "com.android.dynamic-feature":
			return "dynamic-feature"
		}
	}
	return "unknown"
}

func (m Module) HasPlugin(id string) bool {
	for _, plugin := range m.Plugins {
		if plugin.ID == id {
			return true
		}
	}
	return false
}

type VersionEvidence struct {
	Value  string
	Path   string
	Line   int
	Source string
}

// AGPBuildscriptVariable is a literal Gradle variable whose string
// interpolations are confined to the exact version segment of an Android
// Gradle plugin buildscript classpath. Keeping this representation deliberately
// narrow lets callers recognize common Groovy ext variables without evaluating
// arbitrary Gradle code.
type AGPBuildscriptVariable struct {
	Name  string
	Value string
	Line  int
}

type Project struct {
	Root                       string
	SettingsFile               string
	SettingsContent            string
	WrapperFile                string
	WrapperVersion             string
	Modules                    []Module
	AGPVersions                []VersionEvidence
	HasBuildSrc                bool
	HasSettingsVersionCatalogs bool
	HasRefreshVersions         bool
	HasAmbiguousBuildFiles     []string
	GradlePropertiesFile       string
	GradleProperties           string
}

type CatalogPlugin struct {
	ID      string
	Version string
	Line    int
}

type CatalogLibrary struct {
	Module  string
	Version string
	Line    int
}

type Catalog struct {
	Path      string
	Plugins   map[string]CatalogPlugin
	Libraries map[string]CatalogLibrary
}

func Discover(root string) (*Project, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("open project path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project path is not a directory: %s", absRoot)
	}

	settingsFile, err := chooseSingleFile(absRoot, "settings.gradle.kts", "settings.gradle")
	if err != nil {
		return nil, err
	}
	if settingsFile == "" {
		return nil, fmt.Errorf("no settings.gradle.kts or settings.gradle found in %s", absRoot)
	}
	settingsContent, err := readBuildFile(settingsFile)
	if err != nil {
		return nil, err
	}

	catalog, err := readDefaultCatalog(absRoot)
	if err != nil {
		return nil, err
	}
	cleanSettings := StripComments(settingsContent)
	modulePaths := parseIncludes(cleanSettings)
	customDirs := parseProjectDirs(cleanSettings)

	result := &Project{
		Root:                       absRoot,
		SettingsFile:               settingsFile,
		SettingsContent:            settingsContent,
		HasSettingsVersionCatalogs: strings.Contains(cleanSettings, "versionCatalogs"),
		HasRefreshVersions:         strings.Contains(cleanSettings, "de.fayard.refreshVersions"),
	}
	if info, statErr := os.Stat(filepath.Join(absRoot, "buildSrc")); statErr == nil && info.IsDir() {
		result.HasBuildSrc = true
	}

	wrapperFile := filepath.Join(absRoot, "gradle", "wrapper", "gradle-wrapper.properties")
	if wrapperContent, readErr := readOptionalFile(wrapperFile); readErr != nil {
		return nil, readErr
	} else if wrapperContent != "" {
		result.WrapperFile = wrapperFile
		if match := wrapperVersionPattern.FindStringSubmatch(wrapperContent); len(match) > 1 {
			result.WrapperVersion = match[1]
		}
	}

	propertiesFile := filepath.Join(absRoot, "gradle.properties")
	if properties, readErr := readOptionalFile(propertiesFile); readErr != nil {
		return nil, readErr
	} else if properties != "" {
		result.GradlePropertiesFile = propertiesFile
		result.GradleProperties = properties
	}

	for _, gradlePath := range modulePaths {
		directory := filepath.Join(absRoot, gradlePathToDirectory(gradlePath))
		if custom, ok := customDirs[gradlePath]; ok {
			directory = filepath.Join(absRoot, filepath.FromSlash(custom))
		}
		module, ambiguous, moduleErr := readModule(absRoot, gradlePath, directory, catalog)
		if moduleErr != nil {
			return nil, moduleErr
		}
		if ambiguous {
			result.HasAmbiguousBuildFiles = append(result.HasAmbiguousBuildFiles, gradlePath)
		}
		result.Modules = append(result.Modules, module)
	}

	rootModule, ambiguous, rootErr := readModule(absRoot, ":", absRoot, catalog)
	if rootErr != nil {
		return nil, rootErr
	}
	if ambiguous {
		result.HasAmbiguousBuildFiles = append(result.HasAmbiguousBuildFiles, ":")
	}
	if rootModule.Kind() != "unknown" {
		result.Modules = append(result.Modules, rootModule)
	}

	result.AGPVersions = collectAGPVersions(result, catalog)
	sort.Slice(result.Modules, func(i, j int) bool {
		return result.Modules[i].GradlePath < result.Modules[j].GradlePath
	})
	return result, nil
}

func chooseSingleFile(root string, names ...string) (string, error) {
	var matches []string
	for _, name := range names {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			matches = append(matches, path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%w: both %s and %s exist", errAmbiguousBuildFiles, matches[0], matches[1])
	}
	if len(matches) == 0 {
		return "", nil
	}
	return matches[0], nil
}

func readModule(root, gradlePath, directory string, catalog Catalog) (Module, bool, error) {
	buildFile, err := chooseSingleFile(directory, "build.gradle.kts", "build.gradle")
	if err != nil {
		if errors.Is(err, errAmbiguousBuildFiles) {
			return Module{GradlePath: gradlePath, Directory: directory}, true, nil
		}
		return Module{}, false, err
	}
	module := Module{GradlePath: gradlePath, Directory: directory, BuildFile: buildFile}
	if buildFile == "" {
		return module, false, nil
	}
	content, err := readBuildFile(buildFile)
	if err != nil {
		return Module{}, false, err
	}
	module.Content = content
	module.Plugins = parseAppliedPlugins(content, relativePath(root, buildFile), catalog)
	return module, false, nil
}

func readBuildFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Size() > maxBuildFileSize {
		return "", fmt.Errorf("refusing to analyze build file larger than %d bytes: %s", maxBuildFileSize, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

func readOptionalFile(path string) (string, error) {
	content, err := readBuildFile(path)
	if errors.Is(err, os.ErrNotExist) || (err != nil && strings.Contains(err.Error(), "no such file or directory")) {
		return "", nil
	}
	if err != nil {
		if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return content, nil
}

func parseIncludes(content string) []string {
	set := map[string]bool{}
	for _, match := range includeCallPattern.FindAllStringSubmatch(content, -1) {
		for _, pathMatch := range quotedGradlePathPattern.FindAllStringSubmatch(match[1], -1) {
			path := normalizeGradlePath(pathMatch[1])
			if path != ":" {
				set[path] = true
			}
		}
	}
	for _, match := range includeStatementPattern.FindAllStringSubmatch(content, -1) {
		for _, pathMatch := range quotedGradlePathPattern.FindAllStringSubmatch(match[1], -1) {
			path := normalizeGradlePath(pathMatch[1])
			if path != ":" {
				set[path] = true
			}
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func normalizeGradlePath(value string) string {
	if strings.HasPrefix(value, ":") {
		return value
	}
	return ":" + value
}

func parseProjectDirs(content string) map[string]string {
	result := map[string]string{}
	for _, pattern := range []*regexp.Regexp{projectDirPattern, projectDirGroovyPattern} {
		for _, match := range pattern.FindAllStringSubmatch(content, -1) {
			result[match[1]] = match[2]
		}
	}
	return result
}

func gradlePathToDirectory(path string) string {
	return filepath.Join(strings.Split(strings.TrimPrefix(path, ":"), ":")...)
}

func parseAppliedPlugins(content, source string, catalog Catalog) []Plugin {
	clean := StripComments(content)
	plugins := map[string]Plugin{}
	lines := strings.Split(clean, "\n")
	patterns := []*regexp.Regexp{pluginIDCallPattern, pluginIDGroovyPattern, applyPluginCallPattern, applyPluginGroovyPattern}
	for index, line := range lines {
		for _, pattern := range patterns {
			for _, match := range pattern.FindAllStringSubmatch(line, -1) {
				if applyFalsePattern.MatchString(line) && strings.HasPrefix(match[1], "com.android.") {
					continue
				}
				plugins[match[1]] = Plugin{ID: match[1], Source: source, Line: index + 1}
			}
		}
		for _, match := range pluginAliasPattern.FindAllStringSubmatch(line, -1) {
			alias := normalizeAlias(match[1])
			if catalogPlugin, ok := catalog.Plugins[alias]; ok {
				if applyFalsePattern.MatchString(line) && strings.HasPrefix(catalogPlugin.ID, "com.android.") {
					continue
				}
				plugins[catalogPlugin.ID] = Plugin{ID: catalogPlugin.ID, Version: catalogPlugin.Version, Source: source, Line: index + 1}
			}
		}
	}
	result := make([]Plugin, 0, len(plugins))
	for _, plugin := range plugins {
		result = append(result, plugin)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func collectAGPVersions(project *Project, catalog Catalog) []VersionEvidence {
	seen := map[string]bool{}
	var result []VersionEvidence
	add := func(value, path string, line int, source string) {
		key := value + "\x00" + path + "\x00" + strconv.Itoa(line)
		if value == "" || seen[key] {
			return
		}
		seen[key] = true
		result = append(result, VersionEvidence{Value: value, Path: path, Line: line, Source: source})
	}

	files := []struct {
		path    string
		content string
	}{
		{relativePath(project.Root, project.SettingsFile), project.SettingsContent},
	}
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		path := filepath.Join(project.Root, name)
		if content, err := readOptionalFile(path); err == nil && content != "" {
			files = append(files, struct {
				path    string
				content string
			}{relativePath(project.Root, path), content})
		}
	}
	for _, module := range project.Modules {
		if module.BuildFile != "" {
			files = append(files, struct {
				path    string
				content string
			}{relativePath(project.Root, module.BuildFile), module.Content})
		}
	}
	for _, file := range files {
		clean := StripComments(file.content)
		for _, pattern := range []*regexp.Regexp{pluginVersionCallPattern, pluginVersionGroovyPattern, classpathAGPPattern} {
			for _, index := range pattern.FindAllStringSubmatchIndex(clean, -1) {
				valueGroup := 2
				if pattern == classpathAGPPattern {
					valueGroup = 1
				}
				value := clean[index[valueGroup*2]:index[valueGroup*2+1]]
				add(value, file.path, lineAt(clean, index[0]), "literal")
			}
		}
		for _, variable := range FindAGPBuildscriptVariables(file.content) {
			add(variable.Value, file.path, variable.Line, "buildscript-variable")
		}
	}
	for _, plugin := range catalog.Plugins {
		if (plugin.ID == "com.android.application" || plugin.ID == "com.android.library") && plugin.Version != "" {
			add(plugin.Version, relativePath(project.Root, catalog.Path), plugin.Line, "version-catalog")
		}
	}
	for _, library := range catalog.Libraries {
		if library.Module == "com.android.tools.build:gradle" && library.Version != "" {
			add(library.Version, relativePath(project.Root, catalog.Path), library.Line, "version-catalog")
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Value != result[j].Value {
			return result[i].Value < result[j].Value
		}
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Line < result[j].Line
	})
	return result
}

// FindAGPBuildscriptVariables resolves the conservative Groovy pattern used by
// projects such as:
//
//	ext { agp_version = '9.2.1' }
//	classpath "com.android.tools.build:gradle:$agp_version"
//
// A variable is returned only when it has one literal assignment and every
// interpolation of that variable is an AGP classpath version. Dynamic,
// reassigned, or shared variables intentionally remain unresolved.
func FindAGPBuildscriptVariables(content string) []AGPBuildscriptVariable {
	clean := StripComments(content)
	assignments := map[string][]AGPBuildscriptVariable{}
	for _, match := range gradleLiteralAssignmentPattern.FindAllStringSubmatchIndex(clean, -1) {
		name := clean[match[2]:match[3]]
		value := clean[match[4]:match[5]]
		assignments[name] = append(assignments[name], AGPBuildscriptVariable{
			Name: name, Value: value, Line: lineAt(clean, match[0]),
		})
	}

	agpReferences := map[string]int{}
	for _, match := range classpathAGPVariablePattern.FindAllStringSubmatchIndex(clean, -1) {
		name := ""
		if match[2] >= 0 {
			name = clean[match[2]:match[3]]
		} else if match[4] >= 0 {
			name = clean[match[4]:match[5]]
		}
		if name != "" {
			agpReferences[name]++
		}
	}

	var result []AGPBuildscriptVariable
	for name, agpReferenceCount := range agpReferences {
		values := assignments[name]
		if len(values) != 1 || countGradleAssignments(clean, name) != 1 {
			continue
		}
		if countGradleInterpolations(clean, name) != agpReferenceCount {
			continue
		}
		result = append(result, values[0])
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func countGradleAssignments(content, name string) int {
	pattern := regexp.MustCompile(`(?m)^\s*(?:ext\.)?` + regexp.QuoteMeta(name) + `\s*=`)
	return len(pattern.FindAllStringIndex(content, -1))
}

func countGradleInterpolations(content, name string) int {
	pattern := regexp.MustCompile(`\$(?:\{` + regexp.QuoteMeta(name) + `\}|` + regexp.QuoteMeta(name) + `\b)`)
	return len(pattern.FindAllStringIndex(content, -1))
}

func readDefaultCatalog(root string) (Catalog, error) {
	path := filepath.Join(root, "gradle", "libs.versions.toml")
	content, err := readOptionalFile(path)
	if err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{Path: path, Plugins: map[string]CatalogPlugin{}, Libraries: map[string]CatalogLibrary{}}
	if content == "" {
		return catalog, nil
	}
	versions := map[string]string{}
	section := ""
	versionPattern := regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*=\s*["']([^"']+)["']\s*$`)
	idPattern := regexp.MustCompile(`\bid\s*=\s*["']([^"']+)["']`)
	modulePattern := regexp.MustCompile(`\bmodule\s*=\s*["']([^"']+)["']`)
	groupPattern := regexp.MustCompile(`\bgroup\s*=\s*["']([^"']+)["']`)
	namePattern := regexp.MustCompile(`\bname\s*=\s*["']([^"']+)["']`)
	inlineVersionPattern := regexp.MustCompile(`\bversion\s*=\s*["']([^"']+)["']`)
	versionRefPattern := regexp.MustCompile(`\bversion\.ref\s*=\s*["']([^"']+)["']`)
	stringCoordinatePattern := regexp.MustCompile(`^\s*[A-Za-z0-9_.-]+\s*=\s*["']([^"']+)["']\s*$`)
	for index, rawLine := range strings.Split(stripTOMLComments(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[] ")
			continue
		}
		switch section {
		case "versions":
			if match := versionPattern.FindStringSubmatch(line); len(match) > 0 {
				versions[match[1]] = match[2]
			}
		case "plugins", "libraries":
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			alias := normalizeAlias(strings.TrimSpace(parts[0]))
			version := ""
			if match := inlineVersionPattern.FindStringSubmatch(parts[1]); len(match) > 0 {
				version = match[1]
			} else if match := versionRefPattern.FindStringSubmatch(parts[1]); len(match) > 0 {
				version = versions[match[1]]
			}
			if section == "plugins" {
				idMatch := idPattern.FindStringSubmatch(parts[1])
				if len(idMatch) == 0 {
					continue
				}
				catalog.Plugins[alias] = CatalogPlugin{ID: idMatch[1], Version: version, Line: index + 1}
				continue
			}
			library := CatalogLibrary{Version: version, Line: index + 1}
			if moduleMatch := modulePattern.FindStringSubmatch(parts[1]); len(moduleMatch) > 0 {
				library.Module = moduleMatch[1]
			} else if groupMatch, nameMatch := groupPattern.FindStringSubmatch(parts[1]), namePattern.FindStringSubmatch(parts[1]); len(groupMatch) > 0 && len(nameMatch) > 0 {
				library.Module = groupMatch[1] + ":" + nameMatch[1]
			} else if coordinateMatch := stringCoordinatePattern.FindStringSubmatch(line); len(coordinateMatch) > 0 {
				segments := strings.Split(coordinateMatch[1], ":")
				if len(segments) >= 2 {
					library.Module = strings.Join(segments[:2], ":")
				}
				if len(segments) >= 3 {
					library.Version = segments[2]
				}
			}
			if library.Module != "" {
				catalog.Libraries[alias] = library
			}
		}
	}
	return catalog, nil
}

func stripTOMLComments(content string) string {
	var result strings.Builder
	result.Grow(len(content))
	quote := byte(0)
	for index := 0; index < len(content); index++ {
		current := content[index]
		if quote != 0 {
			result.WriteByte(current)
			if current == '\\' && index+1 < len(content) {
				index++
				result.WriteByte(content[index])
			} else if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			result.WriteByte(current)
			continue
		}
		if current == '#' {
			for index < len(content) && content[index] != '\n' {
				result.WriteByte(' ')
				index++
			}
			if index < len(content) {
				result.WriteByte('\n')
			}
			continue
		}
		result.WriteByte(current)
	}
	return result.String()
}

func normalizeAlias(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "-", ".")
	value = strings.ReplaceAll(value, "_", ".")
	return value
}

func relativePath(root, path string) string {
	if path == "" {
		return ""
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func lineAt(content string, offset int) int {
	if offset < 0 || offset > len(content) {
		return 0
	}
	return strings.Count(content[:offset], "\n") + 1
}

// StripComments removes line and block comments while preserving newlines and
// quoted strings. It is deliberately a small Gradle-script lexer, not a Gradle
// evaluator.
func StripComments(content string) string {
	var result strings.Builder
	result.Grow(len(content))
	state := byte('n')
	for i := 0; i < len(content); i++ {
		current := content[i]
		next := byte(0)
		if i+1 < len(content) {
			next = content[i+1]
		}
		switch state {
		case 'n':
			switch {
			case current == '/' && next == '/':
				result.WriteString("  ")
				i++
				state = 'l'
			case current == '/' && next == '*':
				result.WriteString("  ")
				i++
				state = 'b'
			case current == '\'':
				result.WriteByte(current)
				state = 's'
			case current == '"':
				result.WriteByte(current)
				state = 'd'
			default:
				result.WriteByte(current)
			}
		case 'l':
			if current == '\n' {
				result.WriteByte('\n')
				state = 'n'
			} else {
				result.WriteByte(' ')
			}
		case 'b':
			if current == '*' && next == '/' {
				result.WriteString("  ")
				i++
				state = 'n'
			} else if current == '\n' {
				result.WriteByte('\n')
			} else {
				result.WriteByte(' ')
			}
		case 's', 'd':
			result.WriteByte(current)
			if current == '\\' && i+1 < len(content) {
				i++
				result.WriteByte(content[i])
			} else if (state == 's' && current == '\'') || (state == 'd' && current == '"') {
				state = 'n'
			}
		}
	}
	return result.String()
}
