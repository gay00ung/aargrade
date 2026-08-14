package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/toolchain"
)

const maxMigrationStateSize = 64 << 20

func migrationDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func migrationRelative(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes project root: %s", path)
	}
	return filepath.ToSlash(relative), nil
}

func migrationJoin(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("unsafe migration path %q", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe migration path %q", relative)
	}
	path := filepath.Join(root, clean)
	if _, err := migrationRelative(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func ensureMigrationNoSymlink(root string, paths ...string) error {
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

func migrationFileMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("migration target is not a regular file: %s", path)
	}
	return info.Mode().Perm(), nil
}

func atomicMigrationWrite(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".aargrade-migrate-*")
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
	if err := replaceMigrationFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func marshalMigrationState(state migrationState) ([]byte, error) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func readMigrationState(path string) (migrationState, []byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return migrationState{}, nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMigrationStateSize+1))
	if err != nil {
		return migrationState{}, nil, err
	}
	if len(data) > maxMigrationStateSize {
		return migrationState{}, nil, fmt.Errorf("migration state exceeds %d bytes", maxMigrationStateSize)
	}
	var state migrationState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return migrationState{}, nil, fmt.Errorf("invalid migration state: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return migrationState{}, nil, fmt.Errorf("invalid migration state: trailing JSON content")
	}
	return state, data, nil
}

func validateMigrationState(root string, state migrationState) error {
	if state.SchemaVersion != StateSchemaVersion {
		return fmt.Errorf("unsupported migration state schema %d", state.SchemaVersion)
	}
	if state.Status != "prepared" && state.Status != "applied" {
		return fmt.Errorf("invalid migration state status %q", state.Status)
	}
	if state.CurrentAGP == "" || state.TargetAGP == "" || len(state.Files) == 0 {
		return fmt.Errorf("invalid migration state: required fields are missing")
	}
	if _, err := toolchain.ParseVersion(state.CurrentAGP); err != nil {
		return fmt.Errorf("invalid migration state current AGP: %w", err)
	}
	if _, err := toolchain.ParseVersion(state.TargetAGP); err != nil {
		return fmt.Errorf("invalid migration state target AGP: %w", err)
	}
	seen := map[string]bool{}
	for _, file := range state.Files {
		if seen[file.Path] {
			return fmt.Errorf("invalid migration state: duplicate path %q", file.Path)
		}
		seen[file.Path] = true
		if _, err := migrationJoin(root, file.Path); err != nil {
			return fmt.Errorf("invalid migration state path: %w", err)
		}
		if !allowedMigrationStatePath(file.Path) {
			return fmt.Errorf("invalid migration state: path %q is outside the supported Gradle configuration set", file.Path)
		}
		if !validMigrationSHA256(file.BeforeSHA256) || !validMigrationSHA256(file.AfterSHA256) {
			return fmt.Errorf("invalid migration state hashes for %q", file.Path)
		}
		original, err := decodeMigrationOriginal(file)
		if err != nil {
			return fmt.Errorf("invalid migration state original content for %q: %w", file.Path, err)
		}
		if migrationDigest(original) != file.BeforeSHA256 {
			return fmt.Errorf("invalid migration state original content for %q", file.Path)
		}
		if len(original) > 4<<20 {
			return fmt.Errorf("invalid migration state: original content for %q is too large", file.Path)
		}
		if file.BeforeSHA256 == file.AfterSHA256 {
			return fmt.Errorf("invalid migration state: unchanged hash pair for %q", file.Path)
		}
		if file.Mode > 0o777 {
			return fmt.Errorf("invalid migration state permissions for %q", file.Path)
		}
	}
	return nil
}

func validMigrationSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func encodeMigrationOriginal(content []byte) string {
	return base64.StdEncoding.EncodeToString(content)
}

func decodeMigrationOriginal(file migrationStateFile) ([]byte, error) {
	return base64.StdEncoding.DecodeString(file.OriginalBase64)
}

func allowedMigrationStatePath(path string) bool {
	path = filepath.ToSlash(path)
	switch path {
	case "settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts", "gradle.properties", "gradle/libs.versions.toml", "gradle/wrapper/gradle-wrapper.properties":
		return true
	}
	return strings.HasSuffix(path, "/build.gradle") || strings.HasSuffix(path, "/build.gradle.kts")
}

func sortAndUniqueStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func migrationDiff(before, after string) string {
	left := splitPreviewLines(before)
	right := splitPreviewLines(after)
	var output []string
	for i, j := 0, 0; i < len(left) || j < len(right); {
		switch {
		case i < len(left) && j < len(right) && left[i] == right[j]:
			i++
			j++
		case i+1 < len(left) && j < len(right) && left[i+1] == right[j]:
			output = append(output, "- "+left[i])
			i++
		case i < len(left) && j+1 < len(right) && left[i] == right[j+1]:
			output = append(output, "+ "+right[j])
			j++
		case i < len(left) && j < len(right):
			output = append(output, "- "+left[i], "+ "+right[j])
			i++
			j++
		case i < len(left):
			output = append(output, "- "+left[i])
			i++
		default:
			output = append(output, "+ "+right[j])
			j++
		}
	}
	const maxPreviewLines = 200
	if len(output) > maxPreviewLines {
		omitted := len(output) - maxPreviewLines
		output = append(output[:maxPreviewLines], fmt.Sprintf("… %d additional changed line(s) omitted", omitted))
	}
	return strings.Join(output, "\n")
}

func splitPreviewLines(content string) []string {
	content = strings.TrimSuffix(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}
