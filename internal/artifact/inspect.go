package artifact

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	maxArtifactSize            = 1 << 30
	maxArtifactExtractedSize   = 4 << 30
	maxArchiveEntrySize        = 512 << 20
	maxArchiveEntries          = 50000
	maxNestedJarSize           = 256 << 20
	maxNestedJarExtractedSize  = 1 << 30
	maxNestedJarArchiveEntries = 50000
)

var (
	globalRulePattern = regexp.MustCompile(`^-(dontoptimize|dontshrink|dontobfuscate|repackageclasses|allowaccessmodification|processkotlinnullchecks)\b`)
	broadKeepPattern  = regexp.MustCompile(`^-keep(?:classmembers)?(?:,[^ ]+)*\s+(?:public\s+)?(?:class|interface|enum)\s+[^\s{]*\*[^\s{]*`)
)

func Inspect(filePath string) (Snapshot, error) {
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve AAR path: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return Snapshot{}, fmt.Errorf("inspect AAR: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Snapshot{}, fmt.Errorf("AAR must be a regular, non-symlink file: %s", absolute)
	}
	if info.Size() > maxArtifactSize {
		return Snapshot{}, fmt.Errorf("AAR exceeds %d bytes: %s", maxArtifactSize, absolute)
	}
	digest, err := fileSHA256(absolute)
	if err != nil {
		return Snapshot{}, err
	}
	archive, err := zip.OpenReader(absolute)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open AAR zip: %w", err)
	}
	defer archive.Close()
	if err := validateEntryCount("AAR", len(archive.File), maxArchiveEntries); err != nil {
		return Snapshot{}, err
	}
	if err := validateExtractedSize("AAR", archive.File, maxArtifactExtractedSize); err != nil {
		return Snapshot{}, err
	}

	result := Snapshot{
		Inspector: InspectorVersion,
		Path:      absolute,
		SHA256:    digest,
		Size:      info.Size(),
		Metadata:  map[string]string{},
	}
	seenEntries := map[string]bool{}
	classByName := map[string]Class{}
	for _, file := range archive.File {
		if err := validateArchivePath(file.Name); err != nil {
			return Snapshot{}, err
		}
		if seenEntries[file.Name] {
			return Snapshot{}, fmt.Errorf("AAR contains duplicate entry %q", file.Name)
		}
		seenEntries[file.Name] = true
		if file.UncompressedSize64 > maxArchiveEntrySize {
			return Snapshot{}, fmt.Errorf("AAR entry %q exceeds size limit", file.Name)
		}
		content, readErr := readZipFile(file, maxArchiveEntrySize)
		if readErr != nil {
			return Snapshot{}, readErr
		}
		result.Entries = append(result.Entries, Entry{Name: file.Name, Size: file.UncompressedSize64, SHA256: bytesSHA256(content)})

		switch {
		case file.Name == "AndroidManifest.xml":
			result.HasManifest = true
		case file.Name == "classes.jar":
			result.HasClassesJar = true
			if err := inspectJar(file.Name, content, classByName, &result); err != nil {
				return Snapshot{}, err
			}
		case strings.HasPrefix(file.Name, "libs/") && strings.HasSuffix(file.Name, ".jar"):
			if err := inspectJar(file.Name, content, classByName, &result); err != nil {
				return Snapshot{}, err
			}
		case file.Name == "META-INF/com/android/build/gradle/aar-metadata.properties":
			result.Metadata = parseProperties(string(content))
		case isRuleFile(file.Name):
			result.RuleFiles = append(result.RuleFiles, file.Name)
			result.RuleIssues = append(result.RuleIssues, inspectRules(file.Name, string(content))...)
		case strings.HasSuffix(file.Name, ".so"):
			if abi, ok := packagedNativeABI(file.Name); ok {
				result.Native = append(result.Native, NativeLibrary{ABI: abi, Path: file.Name, SHA256: bytesSHA256(content)})
			}
		}
	}
	for _, class := range classByName {
		result.Classes = append(result.Classes, class)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Name < result.Entries[j].Name })
	sort.Slice(result.Classes, func(i, j int) bool { return result.Classes[i].Name < result.Classes[j].Name })
	sort.Strings(result.RuleFiles)
	sort.Slice(result.RuleIssues, func(i, j int) bool {
		if result.RuleIssues[i].Path != result.RuleIssues[j].Path {
			return result.RuleIssues[i].Path < result.RuleIssues[j].Path
		}
		return result.RuleIssues[i].Line < result.RuleIssues[j].Line
	})
	sort.Slice(result.Native, func(i, j int) bool { return result.Native[i].Path < result.Native[j].Path })
	return result, nil
}

func packagedNativeABI(name string) (string, bool) {
	segments := strings.Split(name, "/")
	if len(segments) >= 3 && segments[0] == "jni" {
		return segments[1], true
	}
	if len(segments) >= 6 && segments[0] == "prefab" && segments[1] == "modules" {
		for index := 2; index+1 < len(segments); index++ {
			if segments[index] == "libs" && strings.HasPrefix(segments[index+1], "android.") {
				abi := strings.TrimPrefix(segments[index+1], "android.")
				return abi, abi != ""
			}
		}
	}
	return "", false
}

func inspectJar(name string, content []byte, classes map[string]Class, snapshot *Snapshot) error {
	if len(content) > maxNestedJarSize {
		return fmt.Errorf("nested JAR %q exceeds size limit", name)
	}
	jar, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("open nested JAR %q: %w", name, err)
	}
	if err := validateEntryCount("nested JAR "+name, len(jar.File), maxNestedJarArchiveEntries); err != nil {
		return err
	}
	seen := map[string]bool{}
	var extractedClassBytes uint64
	for _, file := range jar.File {
		if err := validateArchivePath(file.Name); err != nil {
			return fmt.Errorf("nested JAR %q: %w", name, err)
		}
		if seen[file.Name] {
			return fmt.Errorf("nested JAR %q contains duplicate entry %q", name, file.Name)
		}
		seen[file.Name] = true
		if !strings.HasSuffix(file.Name, ".class") || file.Name == "module-info.class" || strings.HasPrefix(file.Name, "META-INF/versions/") {
			continue
		}
		if file.UncompressedSize64 > maxNestedJarExtractedSize-extractedClassBytes {
			return fmt.Errorf("nested JAR %q exceeds extracted class size limit", name)
		}
		extractedClassBytes += file.UncompressedSize64
		data, readErr := readZipFile(file, 16<<20)
		if readErr != nil {
			return fmt.Errorf("read class %q from %q: %w", file.Name, name, readErr)
		}
		class, public, parseErr := parseClassFile(data)
		if parseErr != nil {
			return fmt.Errorf("parse class %q from %q: %w", file.Name, name, parseErr)
		}
		if !public {
			continue
		}
		if _, exists := classes[class.Name]; exists {
			return fmt.Errorf("duplicate public class %q across AAR JARs", class.Name)
		}
		classes[class.Name] = class
		if class.KotlinMetadata {
			snapshot.KotlinMetadata = true
		}
	}
	return nil
}

func validateEntryCount(kind string, count, limit int) error {
	if count > limit {
		return fmt.Errorf("%s contains %d entries; limit is %d", kind, count, limit)
	}
	return nil
}

func validateExtractedSize(kind string, files []*zip.File, limit uint64) error {
	var total uint64
	for _, file := range files {
		if file.UncompressedSize64 > limit-total {
			return fmt.Errorf("%s exceeds extracted size limit of %d bytes", kind, limit)
		}
		total += file.UncompressedSize64
	}
	return nil
}

func validateArchivePath(name string) error {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("unsafe archive entry path %q", name)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") || clean != strings.TrimSuffix(name, "/") {
		return fmt.Errorf("unsafe archive entry path %q", name)
	}
	return nil
}

func readZipFile(file *zip.File, limit uint64) ([]byte, error) {
	if file.UncompressedSize64 > limit {
		return nil, fmt.Errorf("archive entry %q exceeds %d bytes", file.Name, limit)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open archive entry %q: %w", file.Name, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read archive entry %q: %w", file.Name, err)
	}
	if uint64(len(data)) > limit {
		return nil, fmt.Errorf("archive entry %q exceeds %d bytes", file.Name, limit)
	}
	return data, nil
}

func isRuleFile(name string) bool {
	return name == "proguard.txt" || name == "consumer-rules.pro" ||
		(strings.HasPrefix(name, "META-INF/proguard/") && (strings.HasSuffix(name, ".pro") || strings.HasSuffix(name, ".txt")))
}

func inspectRules(name, content string) []RuleIssue {
	var result []RuleIssue
	for index, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		if line == "" {
			continue
		}
		switch {
		case globalRulePattern.MatchString(line):
			result = append(result, RuleIssue{ID: "r8.consumer-global-option", Severity: "error", Path: name, Line: index + 1, Rule: line, Message: "global R8 options do not belong in library consumer rules"})
		case broadKeepPattern.MatchString(line):
			result = append(result, RuleIssue{ID: "r8.consumer-broad-keep", Severity: "warning", Path: name, Line: index + 1, Rule: line, Message: "package or wildcard keep rule needs source-level justification"})
		}
	}
	return result
}

func parseProperties(content string) map[string]string {
	result := map[string]string{}
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func bytesSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
