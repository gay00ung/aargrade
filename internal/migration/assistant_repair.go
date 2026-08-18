package migration

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gay00ung/aargrade/internal/project"
)

const maxManifestSize = 4 << 20

var (
	androidNamespaceValuePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	legacySDKLinePattern         = regexp.MustCompile(`^(\s*)(compileSdkVersion|minSdkVersion|targetSdkVersion)\b(.*)$`)
	integerLiteralPattern        = regexp.MustCompile(`^[0-9]+$`)
	catalogProviderPattern       = regexp.MustCompile(`^libs\.versions(?:\.[A-Za-z_][A-Za-z0-9_]*)+\.get\(\)$`)
	simpleJVMTargetPattern       = regexp.MustCompile(`^jvmTarget\s*(?:=\s*|\s+)["']([0-9]+(?:\.[0-9]+)?)["']\s*;?$`)
	simpleJVMProviderPattern     = regexp.MustCompile(`^jvmTarget\s*(?:=\s*|\s+)(libs\.versions(?:\.[A-Za-z_][A-Za-z0-9_]*)+\.get\(\))\s*;?$`)
	compilerOptionsPattern       = regexp.MustCompile(`\bcompilerOptions\s*\{`)
	javaSourceTargetPattern      = regexp.MustCompile(`\b(sourceCompatibility|targetCompatibility)\b`)
	javaTargetLinePattern        = regexp.MustCompile(`^\s*(sourceCompatibility|targetCompatibility)\s*(?:=\s*|\s+)(?:JavaVersion\.VERSION_([0-9_]+)|JavaVersion\.toVersion\(\s*["']?([0-9]+(?:\.[0-9]+)?)["']?\s*\)|["']([0-9]+(?:\.[0-9]+)?)["'])\s*;?\s*$`)
	javaProviderTargetPattern    = regexp.MustCompile(`^\s*(sourceCompatibility|targetCompatibility)\s*(?:=\s*|\s+)(libs\.versions(?:\.[A-Za-z_][A-Za-z0-9_]*)+\.get\(\))\s*;?\s*$`)
	javaProviderToVersionPattern = regexp.MustCompile(`^\s*(sourceCompatibility|targetCompatibility)\s*(?:=\s*|\s+)JavaVersion\.toVersion\(\s*(libs\.versions(?:\.[A-Za-z_][A-Za-z0-9_]*)+\.get\(\))\s*\)\s*;?\s*$`)
)

type scriptBlock struct {
	nameStart int
	open      int
	close     int
	lineStart int
	indent    string
}

type manifestNamespaceEvidence struct {
	value        string
	path         string
	content      string
	mode         uint32
	packageStart int
	packageEnd   int
}

func applyAssistantRepairs(discovered *project.Project, targetMajor int, crossesIntoAGP9 bool, files []migrationFile, contents map[string]string, filesByRelative map[string]migrationFile, result *MutationResult) {
	if targetMajor >= 8 {
		for _, module := range discovered.Modules {
			if module.BuildFile == "" || module.HasPlugin("com.android.kotlin.multiplatform.library") {
				continue
			}
			switch module.Kind() {
			case "application", "library", "test", "dynamic-feature":
			default:
				continue
			}
			relative, err := migrationRelative(discovered.Root, module.BuildFile)
			if err != nil {
				result.Blockers = append(result.Blockers, err.Error())
				continue
			}
			content := contents[relative]
			if !namespacePattern.MatchString(project.StripComments(content)) {
				evidence, inferErr := inferManifestNamespace(module.Directory)
				if inferErr != nil {
					result.Blockers = append(result.Blockers, relative+": "+inferErr.Error())
					continue
				}
				updated, ok := insertIntoSingleBlock(content, "android", []string{`namespace = "` + evidence.value + `"`})
				if !ok {
					result.Blockers = append(result.Blockers, relative+": Android namespace insertion point is ambiguous")
					continue
				}
				contents[relative] = updated
				result.Repairs = append(result.Repairs, Repair{
					ID: "android.namespace", Path: relative,
					Summary: fmt.Sprintf("AndroidManifest package에서 namespace %q 추가", evidence.value),
				})
				manifestRelative, relativeErr := migrationRelative(discovered.Root, evidence.path)
				if relativeErr != nil {
					result.Blockers = append(result.Blockers, relativeErr.Error())
					continue
				}
				manifestAfter := evidence.content[:evidence.packageStart] + evidence.content[evidence.packageEnd:]
				filesByRelative[manifestRelative] = migrationFile{
					path: evidence.path, relative: manifestRelative, content: evidence.content, mode: evidence.mode,
				}
				contents[manifestRelative] = manifestAfter
				result.Repairs = append(result.Repairs, Repair{
					ID: "android.manifest-package", Path: manifestRelative,
					Summary: "namespace로 이전한 source manifest package 속성을 제거",
				})
			}

			content = contents[relative]
			clean := project.StripComments(content)
			if buildConfigFieldMutationPattern.MatchString(clean) && !buildConfigEnabledMutationPattern.MatchString(clean) {
				updated, ok := enableBuildConfig(content)
				if !ok {
					result.Blockers = append(result.Blockers, relative+": buildFeatures.buildConfig insertion point is ambiguous")
					continue
				}
				contents[relative] = updated
				result.Repairs = append(result.Repairs, Repair{
					ID: "android.buildconfig.enable", Path: relative,
					Summary: "custom BuildConfig field를 위해 buildFeatures.buildConfig = true 추가",
				})
			}
		}
	}

	if !crossesIntoAGP9 {
		return
	}
	for _, file := range files {
		content := contents[file.relative]
		if updated, count := transformLegacySDKSetters(content); count > 0 {
			contents[file.relative] = updated
			content = updated
			result.Repairs = append(result.Repairs, Repair{
				ID: "agp9.sdk-dsl", Path: file.relative,
				Summary: fmt.Sprintf("구형 SDK setter %d개를 AGP 9 property DSL로 전환", count),
			})
		}
		if updated, target, ok := migrateSimpleKotlinOptions(content); ok {
			aligned, javaChanged, alignErr := ensureJavaCompileTarget(updated, target)
			if alignErr != nil {
				result.Blockers = append(result.Blockers, file.relative+": "+alignErr.Error())
				continue
			}
			contents[file.relative] = aligned
			result.Repairs = append(result.Repairs, Repair{
				ID: "agp9.kotlin-options", Path: file.relative,
				Summary: fmt.Sprintf("android.kotlinOptions.jvmTarget %q을 kotlin.compilerOptions로 전환", target),
			})
			if javaChanged {
				result.Repairs = append(result.Repairs, Repair{
					ID: "agp9.java-kotlin-target", Path: file.relative,
					Summary: fmt.Sprintf("Java와 Kotlin compile target을 %q로 정렬", target),
				})
			}
		}
	}
}

func assistantRepairBlockers(discovered *project.Project, targetMajor int, crossesIntoAGP9 bool, files []migrationFile, contents map[string]string) []string {
	var blockers []string
	if targetMajor >= 8 {
		for _, module := range discovered.Modules {
			if module.BuildFile == "" || module.HasPlugin("com.android.kotlin.multiplatform.library") {
				continue
			}
			switch module.Kind() {
			case "application", "library", "test", "dynamic-feature":
			default:
				continue
			}
			relative, err := migrationRelative(discovered.Root, module.BuildFile)
			if err != nil {
				blockers = append(blockers, err.Error())
				continue
			}
			clean := project.StripComments(contents[relative])
			if !namespacePattern.MatchString(clean) {
				blockers = append(blockers, relative+": AGP 8+ requires an explicit Android namespace and it could not be inferred safely")
			}
			if buildConfigFieldMutationPattern.MatchString(clean) && !buildConfigEnabledMutationPattern.MatchString(clean) {
				blockers = append(blockers, relative+": buildConfigField still requires explicit buildFeatures.buildConfig = true")
			}
		}
	}
	if crossesIntoAGP9 {
		for _, file := range files {
			updatedFile := file
			updatedFile.content = contents[file.relative]
			blockers = append(blockers, patternLocations(updatedFile, kotlinOptionsPattern, "Kotlin compiler/source-set DSL could not be migrated by a safe recipe")...)
			blockers = append(blockers, patternLocations(updatedFile, legacySDKMethodPattern, "legacy Android SDK setter could not be migrated by a safe recipe")...)
		}
	}
	return sortAndUniqueStrings(blockers)
}

func inferManifestNamespace(moduleDirectory string) (manifestNamespaceEvidence, error) {
	manifest := filepath.Join(moduleDirectory, "src", "main", "AndroidManifest.xml")
	info, err := os.Lstat(manifest)
	if err != nil {
		return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace: read %s: %w", filepath.ToSlash(filepath.Join("src", "main", "AndroidManifest.xml")), err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace from a non-regular manifest")
	}
	if info.Size() > maxManifestSize {
		return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace: manifest exceeds %d bytes", maxManifestSize)
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace: %w", err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		before := int(decoder.InputOffset())
		token, decodeErr := decoder.Token()
		if decodeErr == io.EOF {
			break
		}
		if decodeErr != nil {
			return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace: invalid AndroidManifest.xml: %w", decodeErr)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "manifest" {
			continue
		}
		for _, attribute := range start.Attr {
			if attribute.Name.Space == "" && attribute.Name.Local == "package" {
				value := strings.TrimSpace(attribute.Value)
				if !androidNamespaceValuePattern.MatchString(value) {
					return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace from manifest package %q", value)
				}
				start, end, rangeErr := manifestPackageAttributeRange(data[before:int(decoder.InputOffset())])
				if rangeErr != nil {
					return manifestNamespaceEvidence{}, fmt.Errorf("cannot remove manifest package safely: %w", rangeErr)
				}
				return manifestNamespaceEvidence{
					value: value, path: manifest, content: string(data), mode: uint32(info.Mode().Perm()),
					packageStart: before + start, packageEnd: before + end,
				}, nil
			}
		}
		break
	}
	return manifestNamespaceEvidence{}, fmt.Errorf("cannot infer namespace: the main manifest has no literal package attribute")
}

func manifestPackageAttributeRange(startElement []byte) (int, int, error) {
	manifestStart := bytes.Index(startElement, []byte("<manifest"))
	if manifestStart < 0 {
		return 0, 0, fmt.Errorf("manifest start element bytes were not found")
	}
	cursor := manifestStart + len("<manifest")
	matchStart, matchEnd := -1, -1
	for cursor < len(startElement) {
		spaceStart := cursor
		for cursor < len(startElement) && isXMLSpace(startElement[cursor]) {
			cursor++
		}
		if cursor >= len(startElement) || startElement[cursor] == '>' || startElement[cursor] == '/' {
			break
		}
		nameStart := cursor
		for cursor < len(startElement) && isXMLNameByte(startElement[cursor]) {
			cursor++
		}
		if nameStart == cursor {
			return 0, 0, fmt.Errorf("invalid attribute name in manifest start element")
		}
		name := string(startElement[nameStart:cursor])
		for cursor < len(startElement) && isXMLSpace(startElement[cursor]) {
			cursor++
		}
		if cursor >= len(startElement) || startElement[cursor] != '=' {
			return 0, 0, fmt.Errorf("manifest attribute %q has no literal value", name)
		}
		cursor++
		for cursor < len(startElement) && isXMLSpace(startElement[cursor]) {
			cursor++
		}
		if cursor >= len(startElement) || startElement[cursor] != '\'' && startElement[cursor] != '"' {
			return 0, 0, fmt.Errorf("manifest attribute %q is not quoted", name)
		}
		quote := startElement[cursor]
		cursor++
		for cursor < len(startElement) && startElement[cursor] != quote {
			cursor++
		}
		if cursor >= len(startElement) {
			return 0, 0, fmt.Errorf("manifest attribute %q is unterminated", name)
		}
		cursor++
		if name == "package" {
			if matchStart >= 0 {
				return 0, 0, fmt.Errorf("multiple package attributes are ambiguous")
			}
			matchStart, matchEnd = spaceStart, cursor
		}
	}
	if matchStart < 0 {
		return 0, 0, fmt.Errorf("literal package attribute bytes were not found")
	}
	return matchStart, matchEnd, nil
}

func isXMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isXMLNameByte(value byte) bool {
	return value == ':' || value == '_' || value == '-' || value == '.' ||
		value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func enableBuildConfig(content string) (string, bool) {
	androidBlocks := findScriptBlocks(content, "android")
	if len(androidBlocks) != 1 {
		return content, false
	}
	androidBlock := androidBlocks[0]
	var features []scriptBlock
	for _, block := range findScriptBlocks(content, "buildFeatures") {
		if block.open > androidBlock.open && block.close < androidBlock.close {
			features = append(features, block)
		}
	}
	switch len(features) {
	case 0:
		return insertIntoBlock(content, androidBlock, []string{"buildFeatures {", "    buildConfig = true", "}"})
	case 1:
		return insertIntoBlock(content, features[0], []string{"buildConfig = true"})
	default:
		return content, false
	}
}

func transformLegacySDKSetters(content string) (string, int) {
	androidBlocks := findScriptBlocks(content, "android")
	if len(androidBlocks) != 1 {
		return content, 0
	}
	androidBlock := androidBlocks[0]
	parts := strings.SplitAfter(content, "\n")
	count := 0
	offset := 0
	for index, part := range parts {
		lineStart := offset
		offset += len(part)
		line := part
		ending := ""
		if strings.HasSuffix(part, "\r\n") {
			line = strings.TrimSuffix(part, "\r\n")
			ending = "\r\n"
		} else if strings.HasSuffix(part, "\n") {
			line = strings.TrimSuffix(part, "\n")
			ending = "\n"
		}
		if project.StripComments(line) != line {
			continue
		}
		match := legacySDKLinePattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		statement := lineStart + len(match[1])
		if statement <= androidBlock.open || statement >= androidBlock.close {
			continue
		}
		value, ok := safeLegacySDKValue(match[3])
		if !ok {
			continue
		}
		property := map[string]string{
			"compileSdkVersion": "compileSdk",
			"minSdkVersion":     "minSdk",
			"targetSdkVersion":  "targetSdk",
		}[match[2]]
		parts[index] = match[1] + property + " = " + value + ending
		count++
	}
	return strings.Join(parts, ""), count
}

func safeLegacySDKValue(raw string) (string, bool) {
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ";"))
	if strings.HasPrefix(value, "=") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "="))
	}
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	if integerLiteralPattern.MatchString(value) {
		return value, true
	}
	provider := strings.TrimSpace(strings.TrimSuffix(value, " as int"))
	if catalogProviderPattern.MatchString(provider) {
		return value, true
	}
	return "", false
}

func ensureJavaCompileTarget(content, target string) (string, bool, error) {
	androidBlocks := findScriptBlocks(content, "android")
	if len(androidBlocks) != 1 {
		return content, false, fmt.Errorf("Java/Kotlin target alignment requires exactly one android block")
	}
	androidBlock := androidBlocks[0]
	var compileBlocks []scriptBlock
	for _, block := range findScriptBlocks(content, "compileOptions") {
		if block.open > androidBlock.open && block.close < androidBlock.close {
			compileBlocks = append(compileBlocks, block)
		}
	}
	if len(compileBlocks) > 1 {
		return content, false, fmt.Errorf("multiple android.compileOptions blocks make Java/Kotlin target alignment ambiguous")
	}
	providerTarget := catalogProviderPattern.MatchString(target)
	targetArgument := `"` + target + `"`
	if providerTarget {
		targetArgument = target
	}
	lines := []string{
		`sourceCompatibility = JavaVersion.toVersion(` + targetArgument + `)`,
		`targetCompatibility = JavaVersion.toVersion(` + targetArgument + `)`,
	}
	if len(compileBlocks) == 0 {
		updated, ok := insertIntoBlock(content, androidBlock, compileOptionsLines(lines))
		if !ok {
			return content, false, fmt.Errorf("cannot insert android.compileOptions safely")
		}
		return updated, true, nil
	}
	body := content[compileBlocks[0].open+1 : compileBlocks[0].close]
	cleanBody := project.StripComments(body)
	found := map[string]bool{}
	for _, line := range strings.Split(cleanBody, "\n") {
		if !javaSourceTargetPattern.MatchString(line) {
			continue
		}
		property, value, ok := javaCompileTarget(line, providerTarget)
		if !ok {
			return content, false, fmt.Errorf("existing Java sourceCompatibility/targetCompatibility must be reviewed before aligning Kotlin jvmTarget %q", target)
		}
		if found[property] {
			return content, false, fmt.Errorf("duplicate %s declarations make Java/Kotlin target alignment ambiguous", property)
		}
		found[property] = true
		if (providerTarget && value != target) || (!providerTarget && canonicalJavaTarget(value) != canonicalJavaTarget(target)) {
			return content, false, fmt.Errorf("existing Java %s %q conflicts with Kotlin jvmTarget %q", property, value, target)
		}
	}
	var missing []string
	if !found["sourceCompatibility"] {
		missing = append(missing, lines[0])
	}
	if !found["targetCompatibility"] {
		missing = append(missing, lines[1])
	}
	if len(missing) == 0 {
		return content, false, nil
	}
	updated, ok := insertIntoBlock(content, compileBlocks[0], missing)
	if !ok {
		return content, false, fmt.Errorf("cannot update android.compileOptions safely")
	}
	return updated, true, nil
}

func javaCompileTarget(line string, providerTarget bool) (string, string, bool) {
	if providerTarget {
		for _, pattern := range []*regexp.Regexp{javaProviderTargetPattern, javaProviderToVersionPattern} {
			if match := pattern.FindStringSubmatch(line); len(match) > 0 {
				return match[1], match[2], true
			}
		}
		return "", "", false
	}
	match := javaTargetLinePattern.FindStringSubmatch(line)
	if len(match) == 0 {
		return "", "", false
	}
	value := match[2]
	if value != "" {
		value = strings.ReplaceAll(value, "_", ".")
	}
	if value == "" {
		value = match[3]
	}
	if value == "" {
		value = match[4]
	}
	return match[1], value, true
}

func canonicalJavaTarget(value string) string {
	value = strings.TrimSpace(value)
	if value == "1.8" {
		return "8"
	}
	return value
}

func compileOptionsLines(lines []string) []string {
	result := make([]string, 0, len(lines)+2)
	result = append(result, "compileOptions {")
	for _, line := range lines {
		result = append(result, "    "+line)
	}
	result = append(result, "}")
	return result
}

func migrateSimpleKotlinOptions(content string) (string, string, bool) {
	clean := project.StripComments(content)
	if !kotlinOptionsPattern.MatchString(clean) || compilerOptionsPattern.MatchString(clean) {
		return content, "", false
	}
	blocks := findScriptBlocks(content, "kotlinOptions")
	androidBlocks := findScriptBlocks(content, "android")
	if len(blocks) != 1 || len(androidBlocks) != 1 {
		return content, "", false
	}
	block := blocks[0]
	if block.open <= androidBlocks[0].open || block.close >= androidBlocks[0].close {
		return content, "", false
	}
	body := content[block.open+1 : block.close]
	if project.StripComments(body) != body {
		return content, "", false
	}
	bodyStatement := strings.TrimSpace(body)
	target := ""
	targetArgument := ""
	if match := simpleJVMTargetPattern.FindStringSubmatch(bodyStatement); len(match) > 0 {
		target = match[1]
		targetArgument = `"` + target + `"`
	} else if match := simpleJVMProviderPattern.FindStringSubmatch(bodyStatement); len(match) > 0 {
		target = match[1]
		targetArgument = target
	} else {
		return content, "", false
	}
	lineEnd := strings.IndexByte(content[block.close:], '\n')
	removeEnd := len(content)
	if lineEnd >= 0 {
		removeEnd = block.close + lineEnd + 1
	}
	if strings.TrimSpace(content[block.lineStart:block.nameStart]) != "" || strings.TrimSpace(content[block.close+1:removeEnd]) != "" {
		return content, "", false
	}
	updated := content[:block.lineStart] + content[removeEnd:]
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}
	updated = strings.TrimRight(updated, "\r\n") + lineEnding + lineEnding +
		"kotlin {" + lineEnding +
		"    compilerOptions {" + lineEnding +
		"        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.fromTarget(" + targetArgument + ")" + lineEnding +
		"    }" + lineEnding +
		"}" + lineEnding
	return updated, target, true
}

func insertIntoSingleBlock(content, name string, lines []string) (string, bool) {
	blocks := findScriptBlocks(content, name)
	if len(blocks) != 1 {
		return content, false
	}
	return insertIntoBlock(content, blocks[0], lines)
}

func insertIntoBlock(content string, block scriptBlock, lines []string) (string, bool) {
	position := block.open + 1
	lineEnding := "\n"
	switch {
	case strings.HasPrefix(content[position:], "\r\n"):
		position += 2
		lineEnding = "\r\n"
	case strings.HasPrefix(content[position:], "\n"):
		position++
	default:
		return content, false
	}
	bodyIndent := block.indent + "    "
	var rendered strings.Builder
	for _, line := range lines {
		rendered.WriteString(bodyIndent)
		rendered.WriteString(line)
		rendered.WriteString(lineEnding)
	}
	return content[:position] + rendered.String() + content[position:], true
}

func findScriptBlocks(content, name string) []scriptBlock {
	var blocks []scriptBlock
	for index := 0; index < len(content); {
		next, normal := nextScriptToken(content, index)
		if !normal {
			index = next
			continue
		}
		if !isScriptIdentifierStart(content[index]) {
			index++
			continue
		}
		end := index + 1
		for end < len(content) && isScriptIdentifierPart(content[end]) {
			end++
		}
		if content[index:end] != name {
			index = end
			continue
		}
		open := end
		for open < len(content) && (content[open] == ' ' || content[open] == '\t' || content[open] == '\r' || content[open] == '\n') {
			open++
		}
		if open >= len(content) || content[open] != '{' {
			index = end
			continue
		}
		close := matchingScriptBrace(content, open)
		if close < 0 {
			index = end
			continue
		}
		lineStart := strings.LastIndex(content[:index], "\n") + 1
		indentEnd := lineStart
		for indentEnd < index && (content[indentEnd] == ' ' || content[indentEnd] == '\t') {
			indentEnd++
		}
		blocks = append(blocks, scriptBlock{
			nameStart: index, open: open, close: close, lineStart: lineStart,
			indent: content[lineStart:indentEnd],
		})
		index = end
	}
	return blocks
}

// nextScriptToken skips comments and quoted strings. It returns the next byte
// after a skipped token, or marks the current byte as normal Gradle code.
func nextScriptToken(content string, index int) (int, bool) {
	switch {
	case strings.HasPrefix(content[index:], "//"):
		if end := strings.IndexByte(content[index+2:], '\n'); end >= 0 {
			return index + 2 + end + 1, false
		}
		return len(content), false
	case strings.HasPrefix(content[index:], "/*"):
		if end := strings.Index(content[index+2:], "*/"); end >= 0 {
			return index + 2 + end + 2, false
		}
		return len(content), false
	case strings.HasPrefix(content[index:], `"""`):
		if end := strings.Index(content[index+3:], `"""`); end >= 0 {
			return index + 3 + end + 3, false
		}
		return len(content), false
	case strings.HasPrefix(content[index:], `'''`):
		if end := strings.Index(content[index+3:], `'''`); end >= 0 {
			return index + 3 + end + 3, false
		}
		return len(content), false
	case content[index] == '"' || content[index] == '\'':
		quote := content[index]
		for cursor := index + 1; cursor < len(content); cursor++ {
			if content[cursor] == '\\' {
				cursor++
				continue
			}
			if cursor < len(content) && content[cursor] == quote {
				return cursor + 1, false
			}
		}
		return len(content), false
	default:
		return index + 1, true
	}
}

func matchingScriptBrace(content string, open int) int {
	depth := 1
	for index := open + 1; index < len(content); {
		next, normal := nextScriptToken(content, index)
		if !normal {
			index = next
			continue
		}
		switch content[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
		index++
	}
	return -1
}

func isScriptIdentifierStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isScriptIdentifierPart(value byte) bool {
	return isScriptIdentifierStart(value) || value >= '0' && value <= '9'
}
