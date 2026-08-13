package matrix

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	maxGradleArchiveSize    = 1 << 30
	maxGradleExtractedSize  = 2 << 30
	maxGradleArchiveEntries = 20000
)

var gradleVersionOutputPattern = regexp.MustCompile(`(?m)^Gradle\s+([0-9][0-9A-Za-z.+-]*)\s*$`)

func resolveGradle(ctx context.Context, cell CellConfig, options Options) (string, error) {
	if executable := options.GradleExecutables[cell.Gradle]; executable != "" {
		return validateGradleExecutable(ctx, executable, cell.Gradle)
	}
	if cell.GradleExecutable != "" {
		return validateGradleExecutable(ctx, cell.GradleExecutable, cell.Gradle)
	}
	environmentKey := "AARGRADE_GRADLE_" + strings.NewReplacer(".", "_", "-", "_").Replace(cell.Gradle)
	if executable := os.Getenv(environmentKey); executable != "" {
		return validateGradleExecutable(ctx, executable, cell.Gradle)
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	distributionRoot := filepath.Join(cacheRoot, "aargrade", "gradle", "gradle-"+cell.Gradle)
	executable := gradleExecutable(distributionRoot)
	if _, err := os.Stat(executable); err == nil {
		return validateGradleExecutable(ctx, executable, cell.Gradle)
	}
	if !options.AllowDownloads {
		return "", fmt.Errorf("Gradle %s is not configured or cached; pass --allow-downloads or --gradle-bin %s=/path/to/gradle", cell.Gradle, cell.Gradle)
	}
	if err := downloadGradle(ctx, cell.Gradle, filepath.Dir(distributionRoot)); err != nil {
		return "", err
	}
	return validateGradleExecutable(ctx, executable, cell.Gradle)
}

func validateGradleExecutable(parent context.Context, value, expected string) (string, error) {
	executable := value
	if !strings.ContainsRune(value, filepath.Separator) {
		resolved, err := exec.LookPath(value)
		if err != nil {
			return "", fmt.Errorf("find Gradle executable %q: %w", value, err)
		}
		executable = resolved
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, absolute, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s --version: %w", absolute, err)
	}
	match := gradleVersionOutputPattern.FindStringSubmatch(string(output))
	if len(match) == 0 || match[1] != expected {
		return "", fmt.Errorf("Gradle executable %s reports %q; expected %s", absolute, reportedValue(match), expected)
	}
	return absolute, nil
}

func downloadGradle(ctx context.Context, version, cacheParent string) error {
	if err := os.MkdirAll(cacheParent, 0o755); err != nil {
		return err
	}
	target := filepath.Join(cacheParent, "gradle-"+version)
	if _, err := os.Stat(gradleExecutable(target)); err == nil {
		return nil
	}
	temporary, err := os.MkdirTemp(cacheParent, ".gradle-"+version+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	archivePath := filepath.Join(temporary, "gradle.zip")
	baseURL := "https://services.gradle.org/distributions/gradle-" + version + "-bin.zip"
	expectedChecksum, err := downloadText(ctx, baseURL+".sha256", 1024)
	if err != nil {
		return fmt.Errorf("download Gradle checksum: %w", err)
	}
	checksumFields := strings.Fields(expectedChecksum)
	if len(checksumFields) == 0 {
		return fmt.Errorf("Gradle %s checksum response is empty", version)
	}
	expectedChecksum = checksumFields[0]
	if len(expectedChecksum) != 64 {
		return fmt.Errorf("Gradle %s checksum response is invalid", version)
	}
	if err := downloadFile(ctx, baseURL, archivePath, maxGradleArchiveSize); err != nil {
		return fmt.Errorf("download Gradle %s: %w", version, err)
	}
	actualChecksum, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("Gradle %s checksum mismatch", version)
	}
	extractRoot := filepath.Join(temporary, "extract")
	if err := extractZip(archivePath, extractRoot); err != nil {
		return err
	}
	extracted := filepath.Join(extractRoot, "gradle-"+version)
	if _, err := os.Stat(gradleExecutable(extracted)); err != nil {
		return fmt.Errorf("Gradle %s archive did not contain the expected executable", version)
	}
	if err := os.Rename(extracted, target); err != nil {
		if _, statErr := os.Stat(gradleExecutable(target)); statErr == nil {
			return nil
		}
		return fmt.Errorf("install Gradle %s in cache: %w", version, err)
	}
	return nil
}

func downloadText(ctx context.Context, url string, limit int64) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "AARGrade")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit {
		return "", fmt.Errorf("response exceeds %d bytes", limit)
	}
	return string(data), nil
}

func downloadFile(ctx context.Context, url, destination string, limit int64) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "AARGrade")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", response.Status)
	}
	if response.ContentLength > limit {
		return fmt.Errorf("response exceeds %d bytes", limit)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, limit+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		return fmt.Errorf("response exceeds %d bytes", limit)
	}
	return nil
}

func extractZip(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	if len(archive.File) > maxGradleArchiveEntries {
		return fmt.Errorf("Gradle archive contains too many entries")
	}
	var total uint64
	for _, file := range archive.File {
		total += file.UncompressedSize64
		if total > maxGradleExtractedSize {
			return fmt.Errorf("Gradle archive exceeds extracted size limit")
		}
		clean := filepath.Clean(filepath.FromSlash(file.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe Gradle archive entry %q", file.Name)
		}
		target := filepath.Join(destination, clean)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe Gradle archive entry %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Gradle archive contains symlink %q", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		mode := file.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			reader.Close()
			return err
		}
		_, copyErr := io.Copy(output, reader)
		closeErr := output.Close()
		readCloseErr := reader.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if readCloseErr != nil {
			return readCloseErr
		}
	}
	return nil
}

func gradleExecutable(root string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "bin", "gradle.bat")
	}
	return filepath.Join(root, "bin", "gradle")
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, bufio.NewReader(file)); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func reportedValue(match []string) string {
	if len(match) > 1 {
		return match[1]
	}
	return "unknown"
}
