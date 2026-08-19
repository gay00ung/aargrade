package verify

import (
	"strings"
	"testing"
)

func TestCompareRootDryRunsRecognizesEquivalentGradleFailures(t *testing.T) {
	before := Command{Status: StatusFail, ExitCode: 1, Output: `
FAILURE: Build failed with an exception.

* What went wrong:
Could not determine the dependencies of task ':plugins:sdk-plugin-google-lvl:adjustLvlPluginJar'.
> Task with name 'packageReleaseAssets' not found in project ':plugins:sdk-plugin-google-lvl'.

* Try:
Run with --stacktrace option.
BUILD FAILED in 2s
`}
	after := Command{Status: StatusFail, ExitCode: 1, Output: `
Starting a Gradle Daemon, 1 stopped Daemon could not be reused
FAILURE: Build failed with an exception.
* What went wrong:
  Could not   determine the dependencies of task ':plugins:sdk-plugin-google-lvl:adjustLvlPluginJar'.
  > Task with name 'packageReleaseAssets' not found in project ':plugins:sdk-plugin-google-lvl'.
* Try:
Run with --info option.
BUILD FAILED in 5s
`}

	comparison := compareRootDryRuns(before, after)
	if comparison.Verdict != rootDryRunPreExisting {
		t.Fatalf("comparison = %#v", comparison)
	}
	if comparison.BeforeFingerprint == "" || comparison.BeforeFingerprint != comparison.AfterFingerprint {
		t.Fatalf("fingerprints = %q / %q", comparison.BeforeFingerprint, comparison.AfterFingerprint)
	}
	if !strings.Contains(comparison.BeforeFailure, "packageReleaseAssets") {
		t.Fatalf("failure evidence = %q", comparison.BeforeFailure)
	}
}

func TestCompareRootDryRunsRejectsDifferentOrInterruptedFailures(t *testing.T) {
	existing := Command{Status: StatusFail, ExitCode: 1, Output: "* What went wrong:\nTask with name 'packageReleaseAssets' not found\n* Try:\nRetry"}
	different := Command{Status: StatusFail, ExitCode: 1, Output: "* What went wrong:\nNamespace not specified\n* Try:\nRetry"}
	interrupted := existing
	interrupted.Output += "\ncommand timed out"
	canceled := existing
	canceled.Output += "\ncommand canceled"
	unstructured := Command{Status: StatusFail, ExitCode: 1, Output: "Namespace not specified"}
	truncated := existing
	truncated.Output += "\n[output truncated by AARGrade]"
	signaled := existing
	signaled.ExitCode = -1
	differentExit := existing
	differentExit.ExitCode = 2
	caseChanged := existing
	caseChanged.Output = strings.Replace(caseChanged.Output, "Task with name", "task with name", 1)
	longPrefix := strings.Repeat("same detail\n", 24)
	longBefore := Command{Status: StatusFail, ExitCode: 1, Output: "* What went wrong:\n" + longPrefix + "before tail\n* Try:\nRetry"}
	longAfter := Command{Status: StatusFail, ExitCode: 1, Output: "* What went wrong:\n" + longPrefix + "after tail\n* Try:\nRetry"}

	if got := compareRootDryRuns(existing, different).Verdict; got != rootDryRunRegression {
		t.Fatalf("different failure verdict = %q", got)
	}
	if got := compareRootDryRuns(existing, interrupted).Verdict; got != rootDryRunRegression {
		t.Fatalf("interrupted failure verdict = %q", got)
	}
	if got := compareRootDryRuns(existing, canceled).Verdict; got != rootDryRunRegression {
		t.Fatalf("canceled failure verdict = %q", got)
	}
	if got := compareRootDryRuns(unstructured, unstructured).Verdict; got != rootDryRunRegression {
		t.Fatalf("unstructured failure verdict = %q", got)
	}
	if got := compareRootDryRuns(existing, truncated).Verdict; got != rootDryRunRegression {
		t.Fatalf("truncated failure verdict = %q", got)
	}
	if got := compareRootDryRuns(signaled, signaled).Verdict; got != rootDryRunRegression {
		t.Fatalf("signaled failure verdict = %q", got)
	}
	if got := compareRootDryRuns(existing, differentExit).Verdict; got != rootDryRunRegression {
		t.Fatalf("different exit-code verdict = %q", got)
	}
	if got := compareRootDryRuns(existing, caseChanged).Verdict; got != rootDryRunRegression {
		t.Fatalf("case-changed failure verdict = %q", got)
	}
	if got := compareRootDryRuns(longBefore, longAfter).Verdict; got != rootDryRunRegression {
		t.Fatalf("different long-block tail verdict = %q", got)
	}
}

func TestCompareRootDryRunsClassifiesNewAndResolvedFailures(t *testing.T) {
	pass := Command{Status: StatusPass}
	fail := Command{Status: StatusFail, ExitCode: 1, Output: "Namespace not specified"}

	if got := compareRootDryRuns(pass, fail).Verdict; got != rootDryRunRegression {
		t.Fatalf("new failure verdict = %q", got)
	}
	if got := compareRootDryRuns(fail, pass).Verdict; got != rootDryRunImproved {
		t.Fatalf("resolved failure verdict = %q", got)
	}
	if got := compareRootDryRuns(pass, pass).Verdict; got != rootDryRunPass {
		t.Fatalf("passing verdict = %q", got)
	}
}
