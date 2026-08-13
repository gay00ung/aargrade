package host

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	modulePathPattern  = regexp.MustCompile(`^:(?:[A-Za-z0-9_.-]+)(?::[A-Za-z0-9_.-]+)*$`)
	versionPattern     = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.\d+)?(?:[-+][0-9A-Za-z.-]+)?$`)
	compileSDKPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?m)\bcompileSdk\s*=\s*(\d+)\b`),
		regexp.MustCompile(`(?m)\bcompileSdk\s+(\d+)\b`),
		regexp.MustCompile(`(?m)\bcompileSdkVersion\s*\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`(?m)\bcompileSdkVersion\s+(\d+)\b`),
	}
	minSDKPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?m)\bminSdk\s*=\s*(\d+)\b`),
		regexp.MustCompile(`(?m)\bminSdk\s+(\d+)\b`),
		regexp.MustCompile(`(?m)\bminSdkVersion\s*\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`(?m)\bminSdkVersion\s+(\d+)\b`),
	}
)

type agpVersion struct {
	raw          string
	major, minor int
}

func parseAGPVersion(raw string) (agpVersion, error) {
	match := versionPattern.FindStringSubmatch(raw)
	if len(match) == 0 {
		return agpVersion{}, fmt.Errorf("AGP version %q is not a supported literal version", raw)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	if major < 4 || major > 9 {
		return agpVersion{}, fmt.Errorf("host generation currently supports AGP 4.x through 9.x, got %q", raw)
	}
	return agpVersion{raw: raw, major: major, minor: minor}, nil
}

func (v agpVersion) supportsNamespace() bool {
	return v.major > 7 || (v.major == 7 && v.minor >= 3)
}

func (v agpVersion) usesModernDSL() bool {
	return v.major >= 7
}

func parseLiteralSDK(content string, patterns []*regexp.Regexp) int {
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(content); len(match) > 0 {
			value, _ := strconv.Atoi(match[1])
			return value
		}
	}
	return 0
}

func validateModulePath(path string) error {
	if !modulePathPattern.MatchString(path) {
		return fmt.Errorf("invalid Gradle module path %q; expected a path such as :sdk", path)
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func slashRelative(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes project root: %s", path)
	}
	return filepath.ToSlash(relative), nil
}

func safeJoin(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("unsafe owned path %q", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe owned path %q", relative)
	}
	path := filepath.Join(root, clean)
	if _, err := slashRelative(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func ensureNoSymlink(root string, paths ...string) error {
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("path escapes project root: %s", path)
		}
		current := root
		for _, part := range strings.Split(relative, string(filepath.Separator)) {
			if part == "." || part == "" {
				continue
			}
			current = filepath.Join(current, part)
			info, statErr := os.Lstat(current)
			if errors.Is(statErr, os.ErrNotExist) {
				break
			}
			if statErr != nil {
				return fmt.Errorf("inspect %s: %w", current, statErr)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to mutate through symlink: %s", current)
			}
		}
	}
	return nil
}

func missingDirectories(root string, targetDirs ...string) ([]string, error) {
	set := map[string]bool{}
	for _, target := range targetDirs {
		relative, err := filepath.Rel(root, target)
		if err != nil {
			return nil, err
		}
		current := root
		for _, part := range strings.Split(relative, string(filepath.Separator)) {
			if part == "." || part == "" {
				continue
			}
			current = filepath.Join(current, part)
			_, statErr := os.Lstat(current)
			if errors.Is(statErr, os.ErrNotExist) {
				relativeCurrent, relErr := slashRelative(root, current)
				if relErr != nil {
					return nil, relErr
				}
				set[relativeCurrent] = true
			} else if statErr != nil {
				return nil, fmt.Errorf("inspect %s: %w", current, statErr)
			}
		}
	}
	result := make([]string, 0, len(set))
	for path := range set {
		result = append(result, path)
	}
	sort.Slice(result, func(i, j int) bool {
		leftDepth := strings.Count(result[i], "/")
		rightDepth := strings.Count(result[j], "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return result[i] < result[j]
	})
	return result, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".aargrade-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set permissions for %s: %w", path, err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func addPreview(content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	for index := range lines {
		lines[index] = "+ " + lines[index]
	}
	return strings.Join(lines, "\n")
}

func deletePreview(content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	for index := range lines {
		lines[index] = "- " + lines[index]
	}
	return strings.Join(lines, "\n")
}

func fileMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Mode().Perm(), nil
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
