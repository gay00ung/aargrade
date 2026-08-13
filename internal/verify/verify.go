package verify

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gay00ung/aargrade/internal/artifact"
	"github.com/gay00ung/aargrade/internal/toolchain"
)

func Run(options Options) (Report, error) {
	if options.ProjectPath == "" {
		options.ProjectPath = "."
	}
	result := Report{
		SchemaVersion: SchemaVersion,
		ToolVersion:   options.ToolVersion,
		Verdict:       "evidence",
		Scope:         "AAR structure and JVM linkage compatibility",
		Limitations: []string{
			"The built-in ABI engine reads JVM class files; it does not prove Kotlin or Java source compatibility.",
			"JNI symbol-level and runtime behavior require consumer tests outside static AAR inspection.",
		},
	}

	candidatePath := options.CandidateAAR
	if candidatePath == "" {
		build, err := buildCandidate(options)
		if err != nil {
			return Report{}, err
		}
		result.ProjectRoot = build.projectRoot
		result.LibraryModule = build.libraryModule
		result.Commands = build.commands
		if build.failed {
			result.Verdict = "fail"
			result.Checks = append(result.Checks, Check{ID: "gradle.commands", Status: StatusFail, Summary: "Gradle verification command failed", Details: failedCommandDetails(build.commands)})
			return result, nil
		}
		candidatePath = build.aarPath
		result.Checks = append(result.Checks, Check{ID: "gradle.commands", Status: StatusPass, Summary: "Gradle help, dry-run, and AAR assembly succeeded"})
	} else {
		result.Checks = append(result.Checks, Check{ID: "gradle.commands", Status: StatusSkipped, Summary: "Candidate AAR was supplied; project build commands were not run"})
	}

	candidate, err := artifact.Inspect(candidatePath)
	if err != nil {
		return Report{}, err
	}
	result.Candidate = candidate
	result.Checks = append(result.Checks, candidateChecks(candidate)...)

	if options.BaselineAAR == "" {
		result.Checks = append(result.Checks,
			Check{ID: "abi.binary", Status: StatusSkipped, Summary: "No baseline AAR was supplied"},
			Check{ID: "metadata.requirements", Status: StatusSkipped, Summary: "No baseline AAR was supplied"},
			Check{ID: "jni.compatibility", Status: StatusSkipped, Summary: "No baseline AAR was supplied"},
		)
	} else {
		baseline, inspectErr := artifact.Inspect(options.BaselineAAR)
		if inspectErr != nil {
			return Report{}, inspectErr
		}
		result.Baseline = &baseline
		comparison := artifact.CompareABI(baseline, candidate)
		result.ABI = &comparison
		result.Checks = append(result.Checks, abiCheck(comparison))
		result.Checks = append(result.Checks, metadataCheck(baseline, candidate))
		result.Checks = append(result.Checks, nativeCheck(baseline, candidate))
		if baseline.KotlinMetadata || candidate.KotlinMetadata {
			result.Checks = append(result.Checks, Check{
				ID: "api.kotlin-source", Status: StatusSkipped,
				Summary: "Kotlin metadata is present; Kotlin source compatibility needs an external source/API engine",
			})
		}
	}

	sort.SliceStable(result.Checks, func(i, j int) bool { return result.Checks[i].ID < result.Checks[j].ID })
	if hasFailedCheck(result.Checks) {
		result.Verdict = "fail"
	} else if result.Baseline != nil {
		result.Verdict = "pass"
	}
	return result, nil
}

func candidateChecks(snapshot artifact.Snapshot) []Check {
	var checks []Check
	structure := Check{ID: "aar.structure", Status: StatusPass, Summary: "Required AAR structure is present"}
	if !snapshot.HasManifest {
		structure.Status = StatusFail
		structure.Summary = "AndroidManifest.xml is missing"
	}
	if !snapshot.HasClassesJar {
		if structure.Status != StatusFail {
			structure.Status = StatusWarning
		}
		structure.Details = append(structure.Details, "classes.jar is missing; this may be intentional only for a resource-only AAR")
	}
	checks = append(checks, structure)
	if len(snapshot.Metadata) == 0 {
		checks = append(checks, Check{ID: "aar.metadata", Status: StatusWarning, Summary: "AAR metadata properties were not found"})
	} else {
		checks = append(checks, validateMetadata(snapshot.Metadata))
	}
	r8 := Check{ID: "r8.consumer-rules", Status: StatusPass, Summary: "No unsafe Consumer R8 rule was detected"}
	for _, issue := range snapshot.RuleIssues {
		r8.Details = append(r8.Details, fmt.Sprintf("%s:%d %s — %s", issue.Path, issue.Line, issue.Rule, issue.Message))
		if issue.Severity == "error" {
			r8.Status = StatusFail
			r8.Summary = "Unsafe global option exists in Consumer R8 rules"
		} else if r8.Status != StatusFail {
			r8.Status = StatusWarning
			r8.Summary = "Broad Consumer R8 rule needs review"
		}
	}
	checks = append(checks, r8)
	return checks
}

func abiCheck(comparison artifact.ABIComparison) Check {
	check := Check{ID: "abi.binary", Status: StatusPass, Summary: "Built-in JVM binary surface is compatible"}
	check.Details = append(check.Details, comparison.RemovedClasses...)
	check.Details = append(check.Details, comparison.RemovedMembers...)
	check.Details = append(check.Details, comparison.IncompatibleChanges...)
	if !comparison.Compatible {
		check.Status = StatusFail
		check.Summary = "JVM binary compatibility regression detected"
	} else if len(comparison.Warnings) > 0 {
		check.Status = StatusWarning
		check.Summary = "JVM binary surface is compatible with source-level warnings"
	}
	check.Details = append(check.Details, comparison.Warnings...)
	return check
}

func metadataCheck(baseline, candidate artifact.Snapshot) Check {
	check := Check{ID: "metadata.requirements", Status: StatusPass, Summary: "Consumer requirements did not increase"}
	if baselineValidation := validateMetadata(baseline.Metadata); baselineValidation.Status == StatusFail {
		check.Status = StatusFail
		for _, detail := range baselineValidation.Details {
			check.Details = append(check.Details, "baseline: "+detail)
		}
	}
	for _, key := range []string{"minCompileSdk", "minCompileSdkExtension"} {
		before, beforeOK := integerProperty(baseline.Metadata, key)
		after, afterOK := integerProperty(candidate.Metadata, key)
		if afterOK && !beforeOK && after > 0 {
			check.Status = StatusFail
			check.Details = append(check.Details, fmt.Sprintf("%s is newly declared as %d; the baseline declared no value", key, after))
		} else if beforeOK && afterOK && after > before {
			check.Status = StatusFail
			check.Details = append(check.Details, fmt.Sprintf("%s increased from %d to %d", key, before, after))
		}
	}
	beforeAGP, beforeOK := baseline.Metadata["minAndroidGradlePluginVersion"]
	afterAGP, afterOK := candidate.Metadata["minAndroidGradlePluginVersion"]
	if afterOK {
		if !beforeOK {
			beforeAGP = "1.0.0"
		}
		beforeVersion, beforeErr := toolchain.ParseVersion(beforeAGP)
		afterVersion, afterErr := toolchain.ParseVersion(afterAGP)
		if beforeErr == nil && afterErr == nil && afterVersion.Compare(beforeVersion) > 0 {
			check.Status = StatusFail
			if beforeOK {
				check.Details = append(check.Details, fmt.Sprintf("minAndroidGradlePluginVersion increased from %s to %s", beforeAGP, afterAGP))
			} else {
				check.Details = append(check.Details, fmt.Sprintf("minAndroidGradlePluginVersion is newly declared as %s; the baseline declared no value", afterAGP))
			}
		}
	}
	if check.Status == StatusFail {
		check.Summary = "AAR metadata requirements are invalid or increased"
	}
	return check
}

func validateMetadata(properties map[string]string) Check {
	check := Check{ID: "aar.metadata", Status: StatusPass, Summary: "AAR metadata properties were parsed"}
	for _, item := range []struct {
		key     string
		minimum int
	}{{"minCompileSdk", 1}, {"minCompileSdkExtension", 0}} {
		value, ok := properties[item.key]
		if !ok {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < item.minimum {
			check.Status = StatusFail
			check.Details = append(check.Details, fmt.Sprintf("%s has invalid value %q", item.key, value))
		}
	}
	if value, ok := properties["minAndroidGradlePluginVersion"]; ok {
		if _, err := toolchain.ParseVersion(value); err != nil {
			check.Status = StatusFail
			check.Details = append(check.Details, fmt.Sprintf("minAndroidGradlePluginVersion has invalid value %q", value))
		}
	}
	if check.Status == StatusFail {
		check.Summary = "AAR metadata contains an invalid consumer requirement"
	}
	return check
}

func nativeCheck(baseline, candidate artifact.Snapshot) Check {
	check := Check{ID: "jni.compatibility", Status: StatusPass, Summary: "No packaged native library was removed"}
	candidateNative := map[string]artifact.NativeLibrary{}
	for _, library := range candidate.Native {
		candidateNative[library.Path] = library
	}
	for _, library := range baseline.Native {
		candidateLibrary, ok := candidateNative[library.Path]
		if !ok {
			check.Status = StatusFail
			check.Details = append(check.Details, "removed "+library.Path)
			continue
		}
		if candidateLibrary.SHA256 != library.SHA256 && check.Status != StatusFail {
			check.Status = StatusWarning
			check.Details = append(check.Details, "binary changed; symbol/runtime validation required: "+library.Path)
		}
	}
	if check.Status == StatusFail {
		check.Summary = "Packaged JNI library or ABI was removed"
	} else if check.Status == StatusWarning {
		check.Summary = "JNI packaging is intact but native binaries changed"
	}
	return check
}

func integerProperty(properties map[string]string, key string) (int, bool) {
	value, ok := properties[key]
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func failedCommandDetails(commands []Command) []string {
	var details []string
	for _, command := range commands {
		if command.Status == StatusFail {
			details = append(details, fmt.Sprintf("%s exited %d: %s %s", command.Name, command.ExitCode, command.Executable, strings.Join(command.Arguments, " ")))
		}
	}
	return details
}

func hasFailedCheck(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}
