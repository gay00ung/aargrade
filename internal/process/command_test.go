package process

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsBatchCommandLineQuotesCommandAndArguments(t *testing.T) {
	got := windowsBatchCommandLine(`C:\project with space\gradlew.bat`, "help", "argument with space")
	want := `/d /s /c ""C:\project with space\gradlew.bat" "help" "argument with space""`
	if got != want {
		t.Fatalf("command line = %q, want %q", got, want)
	}
}

func TestWindowsBatchLauncherWithSpaceInPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows batch launcher coverage")
	}
	directory := filepath.Join(t.TempDir(), "directory with space")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(directory, "launcher.bat")
	if err := os.WriteFile(launcher, []byte("@echo off\r\necho batch-ok:%~1\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := CommandContext(context.Background(), launcher, "argument with space").CombinedOutput()
	if err != nil {
		t.Fatalf("run batch launcher: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "batch-ok:argument with space") {
		t.Fatalf("batch output = %q", output)
	}
}
