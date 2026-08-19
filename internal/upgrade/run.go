package upgrade

import (
	"fmt"
	"strings"

	consumer "github.com/gay00ung/aargrade/internal/matrix"
	"github.com/gay00ung/aargrade/internal/migration"
	verification "github.com/gay00ung/aargrade/internal/verify"
)

func Run(options Options) (Report, error) {
	if options.ProjectPath == "" {
		options.ProjectPath = "."
	}
	if options.Variant == "" {
		options.Variant = "release"
	}
	report := Report{
		SchemaVersion: SchemaVersion,
		ToolVersion:   options.ToolVersion,
		TargetAGP:     options.TargetAGP,
		Verdict:       "incomplete",
		Limitations: []string{
			"Known recipes are deterministic; arbitrary convention plugins, third-party plugin internals, and source-level semantic changes still require an MCP agent or maintainer.",
			"A successful project build and AAR inspection do not replace the optional configured consumer matrix.",
		},
	}

	migrationOptions := migration.MutationOptions{
		ProjectPath:        options.ProjectPath,
		TargetAGP:          options.TargetAGP,
		CurrentAGPOverride: options.CurrentAGPOverride,
		ToolVersion:        options.ToolVersion,
		AutoRepair:         true,
	}
	mutation, err := migration.Migrate(migrationOptions)
	if err != nil {
		return Report{}, err
	}
	report.ProjectRoot = mutation.ProjectRoot
	report.Migration = mutation
	if !mutation.Ready {
		report.Verdict = "blocked"
		report.Failure = &FailureAnalysis{
			Category: "static-analysis", Summary: "안전하게 자동화할 수 없는 마이그레이션 항목이 있습니다.",
			SuggestedAction: "blockers를 MCP 에이전트 또는 관리자가 해결한 뒤 upgrade를 다시 실행하세요.",
		}
		return report, nil
	}
	if !options.Apply {
		report.Verdict = "preview"
		return report, nil
	}

	beforeUpgradeDryRun, err := verification.RunRootDryRun(verification.Options{
		Context:     options.Context,
		ProjectPath: mutation.ProjectRoot,
		GradleArgs:  options.GradleArgs,
		Timeout:     options.Timeout,
		ToolVersion: options.ToolVersion,
	})
	if err != nil {
		report.Verdict = "incomplete"
		report.Failure = analyzeFailure("gradle-dry-run-before", err)
		return report, nil
	}
	report.BeforeUpgradeDryRun = &beforeUpgradeDryRun
	if options.Context != nil && options.Context.Err() != nil {
		report.Verdict = "incomplete"
		report.Failure = analyzeFailure("gradle-dry-run-before", options.Context.Err())
		return report, nil
	}

	migrationOptions.Apply = true
	mutation, err = migration.Migrate(migrationOptions)
	if mutation.ProjectRoot != "" {
		report.ProjectRoot = mutation.ProjectRoot
		report.Migration = mutation
	}
	if err != nil {
		report.Verdict = "incomplete"
		report.Failure = analyzeFailure("migration-apply", err)
		if mutation.TransactionStarted {
			report.Applied = true
			if rollbackErr := rollbackFailedUpgrade(&report, options); rollbackErr != nil {
				return report, rollbackErr
			}
		}
		return report, nil
	}
	if !mutation.Ready {
		report.Verdict = "blocked"
		report.Failure = &FailureAnalysis{
			Category: "static-analysis", Summary: "사전 검증 이후 프로젝트가 달라져 자동 마이그레이션을 적용할 수 없습니다.",
			SuggestedAction: "blockers를 확인하고 upgrade를 다시 실행하세요.",
		}
		return report, nil
	}
	report.Applied = mutation.Applied

	verificationReport, verifyErr := verification.Run(verification.Options{
		Context:             options.Context,
		ProjectPath:         mutation.ProjectRoot,
		LibraryPath:         options.LibraryPath,
		Variant:             options.Variant,
		BaselineAAR:         options.BaselineAAR,
		GradleArgs:          options.GradleArgs,
		BeforeUpgradeDryRun: &beforeUpgradeDryRun,
		Timeout:             options.Timeout,
		ToolVersion:         options.ToolVersion,
	})
	if verifyErr != nil {
		report.Verdict = "incomplete"
		report.Failure = analyzeFailure("", verifyErr)
		if rollbackErr := rollbackFailedUpgrade(&report, options); rollbackErr != nil {
			return report, rollbackErr
		}
		return report, nil
	}
	report.Verification = &verificationReport
	if verificationReport.Verdict == "fail" {
		report.Verdict = "fail"
		report.Failure = analyzeVerificationFailure(verificationReport)
		if rollbackErr := rollbackFailedUpgrade(&report, options); rollbackErr != nil {
			return report, rollbackErr
		}
		return report, nil
	}

	if options.MatrixConfig != "" {
		matrixReport, matrixErr := consumer.Run(consumer.Options{
			Context:           options.Context,
			ConfigPath:        options.MatrixConfig,
			CandidateAAR:      verificationReport.Candidate.Path,
			BaselineAAR:       options.BaselineAAR,
			WorkDirectory:     options.MatrixWorkDirectory,
			SelectedCells:     options.SelectedCells,
			AllowDownloads:    options.AllowDownloads,
			KeepGoing:         options.KeepGoing,
			Timeout:           options.Timeout,
			ToolVersion:       options.ToolVersion,
			JavaHomes:         options.JavaHomes,
			GradleExecutables: options.GradleExecutables,
		})
		if matrixErr != nil {
			report.Verdict = "incomplete"
			report.Failure = analyzeFailure("consumer-matrix", matrixErr)
			if rollbackErr := rollbackFailedUpgrade(&report, options); rollbackErr != nil {
				return report, rollbackErr
			}
			return report, nil
		}
		report.Matrix = &matrixReport
		switch matrixReport.Verdict {
		case "pass":
		case "fail":
			report.Verdict = "fail"
			report.Failure = &FailureAnalysis{
				Category: "consumer-regression", Summary: "후보 AAR이 하나 이상의 고객 환경에서 실패했습니다.",
				SuggestedAction: "실패 셀의 baseline/candidate 출력을 비교하고 호환성 회귀를 수정하세요.",
			}
			if rollbackErr := rollbackFailedUpgrade(&report, options); rollbackErr != nil {
				return report, rollbackErr
			}
			return report, nil
		default:
			report.Verdict = "incomplete"
			report.Failure = &FailureAnalysis{
				Category: "consumer-environment", Summary: "고객 매트릭스 일부를 완료하지 못했습니다.",
				SuggestedAction: "필요한 JDK, Android SDK, Gradle 배포판을 준비하고 해당 셀을 다시 실행하세요.",
			}
			if rollbackErr := rollbackFailedUpgrade(&report, options); rollbackErr != nil {
				return report, rollbackErr
			}
			return report, nil
		}
	}

	report.Verdict = "pass"
	return report, nil
}

func rollbackFailedUpgrade(report *Report, options Options) error {
	if !options.RollbackOnFailure {
		return nil
	}
	rollback, err := migration.Rollback(migration.RollbackOptions{ProjectPath: report.ProjectRoot, Apply: true})
	if err != nil {
		return fmt.Errorf("upgrade failed and automatic rollback also failed: %w", err)
	}
	report.Rollback = &rollback
	report.RolledBack = rollback.Applied
	return nil
}

func analyzeVerificationFailure(report verification.Report) *FailureAnalysis {
	for _, command := range report.Commands {
		if command.Status == verification.StatusFail {
			return analyzeFailure(command.Name, fmt.Errorf("%s", command.Output))
		}
	}
	for _, check := range report.Checks {
		if check.Status == verification.StatusFail {
			return &FailureAnalysis{
				Category: "aar-compatibility", Summary: check.Summary,
				SuggestedAction: "기준 AAR과 후보 AAR의 실패 상세를 검토하고 공개 API·메타데이터·R8/JNI 회귀를 수정하세요.",
			}
		}
	}
	return &FailureAnalysis{Category: "verification", Summary: "AAR 검증이 실패했습니다.", SuggestedAction: "verify 보고서의 실패 항목을 확인하세요."}
}

func analyzeFailure(command string, failure error) *FailureAnalysis {
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	lower := strings.ToLower(message)
	analysis := &FailureAnalysis{Category: "gradle", Summary: firstFailureLine(message), SuggestedAction: "전체 Gradle 출력을 MCP 에이전트에 전달해 프로젝트별 수정을 진행하세요.", Command: command}
	if command == "migration-apply" {
		analysis.Category = "migration-apply"
		analysis.SuggestedAction = "부분 적용의 rollback 결과를 확인하고 파일 권한·디스크 상태를 해결한 뒤 다시 실행하세요."
	} else if command == "consumer-matrix" {
		analysis.Category = "consumer-matrix"
		analysis.SuggestedAction = "matrix 설정과 JDK·Gradle·Android SDK 환경을 확인한 뒤 다시 실행하세요."
	}
	switch {
	case strings.Contains(lower, "namespace not specified") || strings.Contains(lower, "namespace is not specified"):
		analysis.Category = "namespace"
		analysis.SuggestedAction = "manifest package가 literal인지 확인하고 해당 Android module에 namespace를 명시하세요."
	case strings.Contains(lower, "buildconfig") && strings.Contains(lower, "disabled"):
		analysis.Category = "buildconfig"
		analysis.SuggestedAction = "custom BuildConfig field가 있는 module에 buildFeatures.buildConfig = true를 명시하세요."
	case strings.Contains(lower, "kotlin-android") || strings.Contains(lower, "org.jetbrains.kotlin.android"):
		analysis.Category = "built-in-kotlin"
		analysis.SuggestedAction = "남아 있는 Kotlin Android plugin 선언과 지원되지 않는 Kotlin source-set DSL을 확인하세요."
	case strings.Contains(lower, "kapt"):
		analysis.Category = "kapt"
		analysis.SuggestedAction = "processor가 KSP를 지원하는지 검증하고, 불가능하면 같은 module에 com.android.legacy-kapt를 적용하세요."
	case strings.Contains(lower, "ksp"):
		analysis.Category = "ksp"
		analysis.SuggestedAction = "KSP 2.3.6 이상과 processor 호환성을 확인하세요."
	case strings.Contains(lower, "inconsistent jvm targets"):
		analysis.Category = "jvm-target"
		analysis.SuggestedAction = "Java compileOptions와 Kotlin compilerOptions의 JVM target을 같은 값으로 맞추세요."
	case strings.Contains(lower, "baseextension") || strings.Contains(lower, "applicationvariants") || strings.Contains(lower, "libraryvariants"):
		analysis.Category = "agp9-dsl"
		analysis.SuggestedAction = "legacy DSL/Variant API를 public DSL과 androidComponents API로 전환하세요."
	case strings.Contains(lower, "sdk location not found") || strings.Contains(lower, "failed to find target with hash string"):
		analysis.Category = "android-sdk"
		analysis.SuggestedAction = "ANDROID_HOME/local.properties와 필요한 compileSdk platform 설치 상태를 확인하세요."
	case strings.Contains(lower, "java_home") || strings.Contains(lower, "requires java") || strings.Contains(lower, "jdk"):
		analysis.Category = "jdk"
		analysis.SuggestedAction = "목표 AGP가 요구하는 JDK를 JAVA_HOME으로 선택한 뒤 다시 실행하세요."
	case strings.Contains(lower, "could not resolve") || strings.Contains(lower, "unknownhost") || strings.Contains(lower, "network"):
		analysis.Category = "dependency-resolution"
		analysis.SuggestedAction = "네트워크·저장소·프록시와 dependency version을 확인한 뒤 재시도하세요."
	case strings.Contains(lower, "task with name") && strings.Contains(lower, "not found"):
		analysis.Category = "gradle-task"
		analysis.SuggestedAction = "실패한 task 의존성이 현재 AGP에서 존재하는지 확인하고 public task/artifact API로 전환하세요."
	}
	if analysis.Summary == "" {
		analysis.Summary = "Gradle 검증을 완료하지 못했습니다."
	}
	return analysis
}

func firstFailureLine(message string) string {
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">"))
		lower := strings.ToLower(line)
		if strings.Contains(lower, "namespace not specified") ||
			strings.Contains(lower, "inconsistent jvm targets") ||
			strings.Contains(lower, "buildconfig") && strings.Contains(lower, "disabled") ||
			strings.Contains(lower, "sdk location not found") ||
			strings.Contains(lower, "could not resolve") ||
			strings.Contains(lower, "could not determine the dependencies") ||
			strings.Contains(lower, "task with name") && strings.Contains(lower, "not found") {
			return truncateFailureLine(line)
		}
	}
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateFailureLine(line)
		}
	}
	return ""
}

func truncateFailureLine(line string) string {
	const maximum = 240
	if len(line) > maximum {
		return line[:maximum] + "…"
	}
	return line
}
