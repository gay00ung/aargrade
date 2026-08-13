package migration

import (
	"fmt"
	"sort"

	"github.com/gay00ung/aargrade/internal/doctor"
	"github.com/gay00ung/aargrade/internal/model"
	"github.com/gay00ung/aargrade/internal/toolchain"
)

func Create(options Options) (Plan, error) {
	compatibility, err := toolchain.ForAGP(options.TargetAGP)
	if err != nil {
		return Plan{}, err
	}
	diagnosis, err := doctor.Analyze(options.ProjectPath, options.ToolVersion)
	if err != nil {
		return Plan{}, err
	}

	currentAGP := diagnosis.Inventory.AGP.Value
	if options.CurrentAGPOverride != "" {
		currentAGP = options.CurrentAGPOverride
	}
	result := Plan{
		SchemaVersion: SchemaVersion,
		ToolVersion:   options.ToolVersion,
		ProjectRoot:   diagnosis.ProjectRoot,
		CurrentAGP:    currentAGP,
		TargetAGP:     options.TargetAGP,
		Toolchain:     compatibility,
		Ready:         true,
	}

	if currentAGP == "" {
		result.Ready = false
		result.Blockers = append(result.Blockers, "현재 AGP 버전을 확인할 수 없습니다. --current-agp로 검증된 값을 지정하세요.")
	} else {
		currentVersion, parseErr := toolchain.ParseVersion(currentAGP)
		targetVersion, _ := toolchain.ParseVersion(options.TargetAGP)
		if parseErr != nil {
			result.Ready = false
			result.Blockers = append(result.Blockers, fmt.Sprintf("현재 AGP 버전 %q을 비교할 수 없습니다.", currentAGP))
		} else if currentVersion.Compare(targetVersion) >= 0 {
			result.Ready = false
			result.Blockers = append(result.Blockers, fmt.Sprintf("목표 AGP %s는 현재 버전 %s보다 높아야 합니다.", options.TargetAGP, currentAGP))
		}
	}

	for _, finding := range diagnosis.Findings {
		if finding.Severity == model.SeverityError {
			result.Ready = false
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s: %s", finding.ID, finding.Title))
		}
	}

	result.Steps = append(result.Steps, baseSteps(diagnosis, compatibility, options.TargetAGP)...)
	targetVersion, _ := toolchain.ParseVersion(options.TargetAGP)
	result.Steps = append(result.Steps, stepsForFindings(diagnosis.Findings, targetVersion)...)
	result.Steps = append(result.Steps, Step{
		ID:     "verify.candidate",
		Kind:   StepRequired,
		Title:  "후보 AAR 검증",
		Why:    "프로젝트 빌드 성공만으로 기존 고객과의 AAR 호환성을 증명할 수 없습니다.",
		Action: "기준 AAR과 후보 AAR을 `aargrade verify --baseline-aar ... --candidate-aar ...`로 비교하세요.",
	})
	result.Steps = append(result.Steps, Step{
		ID:     "matrix.consumers",
		Kind:   StepRequired,
		Title:  "고객 툴체인 매트릭스 실행",
		Why:    "정적 분석과 ABI 비교가 실제 Gradle 소비자 빌드를 대신할 수 없습니다.",
		Action: "지원 정책의 Java/Kotlin 셀을 `aargrade matrix --config aargrade.yml`로 실행하세요.",
	})

	result.Steps = deduplicateAndOrder(result.Steps)
	return result, nil
}

func baseSteps(diagnosis model.Report, compatibility toolchain.Compatibility, targetAGP string) []Step {
	steps := []Step{{
		ID:     "toolchain.jdk",
		Kind:   StepRequired,
		Title:  fmt.Sprintf("JDK %d 실행 환경 준비", compatibility.RecommendedJDK),
		Why:    fmt.Sprintf("AGP %s 검증은 JDK %d 기준으로 수행해야 합니다.", targetAGP, compatibility.RecommendedJDK),
		Action: "로컬과 CI에서 Gradle이 사용하는 JAVA_HOME을 명시하고 `java -version`을 증거로 남기세요.",
	}}

	currentGradle := diagnosis.Inventory.Gradle.Value
	minimumGradle, _ := toolchain.ParseVersion(compatibility.MinimumGradle)
	currentVersion, err := toolchain.ParseVersion(currentGradle)
	switch {
	case currentGradle == "" || err != nil:
		steps = append(steps, Step{
			ID: "toolchain.gradle-wrapper", Kind: StepRequired, Title: "Gradle Wrapper 버전 확정",
			Why:    "현재 Wrapper 버전을 신뢰할 수 있게 확인하지 못했습니다.",
			Action: fmt.Sprintf("Wrapper를 AGP %s의 공식 최소 버전인 Gradle %s 이상으로 설정하세요.", targetAGP, compatibility.MinimumGradle),
		})
	case currentVersion.Compare(minimumGradle) < 0:
		steps = append(steps, Step{
			ID: "toolchain.gradle-wrapper", Kind: StepRequired, Title: "Gradle Wrapper 업그레이드",
			Why:    fmt.Sprintf("현재 Gradle %s는 AGP %s의 공식 최소 버전 %s보다 낮습니다.", currentGradle, targetAGP, compatibility.MinimumGradle),
			Action: fmt.Sprintf("Wrapper를 Gradle %s 이상으로 올리고 Wrapper 검증을 다시 실행하세요.", compatibility.MinimumGradle),
		})
	default:
		steps = append(steps, Step{
			ID: "toolchain.gradle-wrapper", Kind: StepReview, Title: "Gradle Wrapper 조합 확인",
			Why:    fmt.Sprintf("공식 정책은 AGP %s에 Gradle %s 이상을 요구하며 현재 선언은 %s입니다.", targetAGP, compatibility.MinimumGradle, currentGradle),
			Action: "선택한 AGP 패치 릴리스의 release note와 실제 `./gradlew help` 결과를 함께 기록하세요.",
		})
	}

	steps = append(steps, Step{
		ID:     "agp.declaration",
		Kind:   StepRequired,
		Title:  fmt.Sprintf("AGP 선언을 %s로 변경", targetAGP),
		Why:    "플러그인 선언, buildscript classpath, version catalog가 한 버전을 가리켜야 합니다.",
		Action: fmt.Sprintf("모든 유효 AGP 선언을 %s로 맞춘 뒤 `./gradlew help`와 `./gradlew build --dry-run`을 실행하세요.", targetAGP),
	})
	return steps
}

func stepsForFindings(findings []model.Finding, target toolchain.Version) []Step {
	var steps []Step
	for _, finding := range findings {
		if target.Major < 9 && len(finding.ID) >= 4 && finding.ID[:4] == "agp9" {
			continue
		}
		step, ok := findingStep(finding)
		if ok {
			steps = append(steps, step)
		}
	}
	return steps
}

func findingStep(finding model.Finding) (Step, bool) {
	step := Step{ID: finding.ID, Kind: StepReview, Title: finding.Title, Why: finding.Message, Action: finding.Recommendation, Evidence: finding.Evidence}
	switch finding.ID {
	case "agp9.kotlin-android-plugin", "agp9.kapt-plugin", "agp9.legacy-api", "android.buildconfig.feature-implicit", "r8.consumer-global-option":
		step.Kind = StepRequired
		return step, true
	case "agp9.ksp-plugin", "android.native-build.present", "upgrade-assistant.buildsrc", "upgrade-assistant.settings-version-catalog", "gradle.version-manager.refresh-versions":
		return step, true
	case "android.library-only":
		step.Kind = StepOptional
		return step, true
	default:
		if finding.Severity == model.SeverityError {
			step.Kind = StepRequired
			return step, true
		}
		return Step{}, false
	}
}

func deduplicateAndOrder(steps []Step) []Step {
	seen := map[string]bool{}
	filtered := steps[:0]
	for _, step := range steps {
		if step.ID == "" || seen[step.ID] {
			continue
		}
		seen[step.ID] = true
		filtered = append(filtered, step)
	}
	steps = filtered
	priority := func(kind StepKind) int {
		switch kind {
		case StepRequired:
			return 0
		case StepReview:
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if priority(steps[i].Kind) != priority(steps[j].Kind) {
			return priority(steps[i].Kind) < priority(steps[j].Kind)
		}
		return steps[i].ID < steps[j].ID
	})
	for index := range steps {
		steps[index].Order = index + 1
	}
	return steps
}
