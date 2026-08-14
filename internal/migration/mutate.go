package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/doctor"
	"github.com/gay00ung/aargrade/internal/model"
	"github.com/gay00ung/aargrade/internal/project"
	"github.com/gay00ung/aargrade/internal/toolchain"
)

// Migrate builds a deterministic, preview-first mutation plan and optionally
// applies it. It deliberately handles only statically proven transformations;
// unsupported Gradle logic is returned as a blocker instead of being guessed.
func Migrate(options MutationOptions) (MutationResult, error) {
	compatibility, err := toolchain.ForAGP(options.TargetAGP)
	if err != nil {
		return MutationResult{}, err
	}
	targetVersion, _ := toolchain.ParseVersion(options.TargetAGP)
	discovered, err := project.Discover(options.ProjectPath)
	if err != nil {
		return MutationResult{}, err
	}
	statePath := filepath.Join(discovered.Root, filepath.FromSlash(stateRelativePath))
	if _, err := os.Lstat(statePath); err == nil {
		return MutationResult{}, fmt.Errorf("an active migration state already exists at %s; run `aargrade migrate rollback` first", stateRelativePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return MutationResult{}, fmt.Errorf("inspect migration state: %w", err)
	}

	result := MutationResult{
		SchemaVersion: MutationSchemaVersion,
		Operation:     "migrate",
		ToolVersion:   options.ToolVersion,
		ProjectRoot:   discovered.Root,
		TargetAGP:     options.TargetAGP,
		Toolchain:     &compatibility,
		StatePath:     stateRelativePath,
		statePath:     statePath,
	}

	diagnosis, err := doctor.Analyze(discovered.Root, options.ToolVersion)
	if err != nil {
		return MutationResult{}, err
	}
	currentVersions := uniqueProjectVersions(discovered.AGPVersions)
	if len(currentVersions) == 1 {
		result.CurrentAGP = currentVersions[0]
	}
	if options.CurrentAGPOverride != "" {
		if len(currentVersions) == 1 && currentVersions[0] != options.CurrentAGPOverride {
			result.Blockers = append(result.Blockers, fmt.Sprintf("--current-agp %s conflicts with the literal project declaration %s", options.CurrentAGPOverride, currentVersions[0]))
		} else {
			result.CurrentAGP = options.CurrentAGPOverride
		}
	}
	if len(currentVersions) == 0 {
		result.Blockers = append(result.Blockers, "AGP 선언 위치와 버전을 정적으로 확인할 수 없어 자동 변경할 수 없습니다.")
	} else if len(currentVersions) > 1 {
		result.Blockers = append(result.Blockers, "서로 다른 AGP 선언이 있습니다: "+strings.Join(currentVersions, ", "))
	}
	if result.CurrentAGP == "" {
		result.Blockers = append(result.Blockers, "현재 AGP 버전을 확인할 수 없습니다.")
	} else if currentVersion, parseErr := toolchain.ParseVersion(result.CurrentAGP); parseErr != nil {
		result.Blockers = append(result.Blockers, fmt.Sprintf("현재 AGP 버전 %q을 비교할 수 없습니다.", result.CurrentAGP))
	} else if currentVersion.Compare(targetVersion) >= 0 {
		result.Blockers = append(result.Blockers, fmt.Sprintf("목표 AGP %s는 현재 버전 %s보다 높아야 합니다.", options.TargetAGP, result.CurrentAGP))
	}
	for _, finding := range diagnosis.Findings {
		if finding.Severity == model.SeverityError {
			result.Blockers = append(result.Blockers, finding.ID+": "+finding.Title)
		}
	}

	buildFiles, catalogFile, err := loadMigrationFiles(discovered)
	if err != nil {
		return MutationResult{}, err
	}
	allBuildFiles := append([]migrationFile(nil), buildFiles...)
	catalog := catalogModel{versions: map[string]catalogVersion{}}
	if catalogFile != nil {
		catalog = parseCatalog(catalogFile.content)
	}

	if targetVersion.Major >= 8 {
		result.Blockers = append(result.Blockers, namespaceBlockers(discovered)...)
		if hasFindingID(diagnosis, "android.buildconfig.feature-implicit") {
			result.Blockers = append(result.Blockers, "android.buildconfig.feature-implicit: custom BuildConfig fields require an explicit buildFeatures.buildConfig = true migration")
		}
	}
	result.Blockers = append(result.Blockers, mutationStructureBlockers(discovered, diagnosis)...)
	if targetVersion.Major >= 9 {
		result.Blockers = append(result.Blockers, agp9MutationBlockers(discovered, diagnosis, allBuildFiles, catalog)...)
	}

	result.Blockers = sortAndUniqueStrings(result.Blockers)
	if len(result.Blockers) > 0 {
		result.Ready = false
		return result, nil
	}

	currentVersion, _ := toolchain.ParseVersion(result.CurrentAGP)
	contents := map[string]string{}
	filesByRelative := map[string]migrationFile{}
	for _, file := range buildFiles {
		contents[file.relative] = file.content
		filesByRelative[file.relative] = file
	}
	if catalogFile != nil {
		contents[catalogFile.relative] = catalogFile.content
		filesByRelative[catalogFile.relative] = *catalogFile
	}

	agpReplacements := 0
	for _, file := range buildFiles {
		updated, replacements := transformLiteralAGP(contents[file.relative], result.CurrentAGP, options.TargetAGP)
		contents[file.relative] = updated
		agpReplacements += replacements
	}
	if catalogFile != nil {
		updated, replacements, blockers := transformCatalog(contents[catalogFile.relative], result.CurrentAGP, options.TargetAGP)
		if len(blockers) > 0 {
			result.Blockers = append(result.Blockers, blockers...)
			result.Ready = false
			return result, nil
		}
		contents[catalogFile.relative] = updated
		agpReplacements += replacements
	}
	if agpReplacements == 0 {
		result.Blockers = []string{"AGP 버전 선언을 안전하게 바꿀 수 있는 위치를 찾지 못했습니다."}
		return result, nil
	}
	for _, file := range buildFiles {
		for _, version := range literalAGPVersions(contents[file.relative]) {
			if version != options.TargetAGP {
				result.Blockers = append(result.Blockers, fmt.Sprintf("%s: AGP 선언 %s가 미변경 상태로 남았습니다.", file.relative, version))
			}
		}
	}
	if catalogFile != nil {
		for _, entry := range parseCatalog(contents[catalogFile.relative]).entries {
			if entry.isAGP && entry.version != options.TargetAGP {
				result.Blockers = append(result.Blockers, fmt.Sprintf("%s:%d: AGP catalog 선언이 %q로 남았습니다.", catalogFile.relative, entry.line+1, entry.version))
			}
		}
	}
	if len(result.Blockers) > 0 {
		result.Blockers = sortAndUniqueStrings(result.Blockers)
		return result, nil
	}

	if targetVersion.Major >= 9 {
		kotlinAliases := catalogAliases(catalog, func(entry catalogEntry) bool { return entry.isKotlin })
		removed := 0
		for _, file := range buildFiles {
			updated, count, unsafe := removeKotlinAndroidPluginLines(contents[file.relative], kotlinAliases)
			if len(unsafe) > 0 {
				for _, line := range unsafe {
					result.Blockers = append(result.Blockers, fmt.Sprintf("%s:%d: Kotlin Android plugin declaration is not a standalone line", file.relative, line))
				}
				continue
			}
			contents[file.relative] = updated
			removed += count
		}
		if hasFindingID(diagnosis, "agp9.kotlin-android-plugin") && removed == 0 {
			result.Blockers = append(result.Blockers, "AGP 9 Built-in Kotlin 전환에 필요한 Kotlin Android plugin 선언을 안전하게 제거하지 못했습니다.")
		}
		if len(result.Blockers) > 0 {
			result.Blockers = sortAndUniqueStrings(result.Blockers)
			return result, nil
		}
		if removed > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("AGP 9 Built-in Kotlin을 위해 Kotlin Android plugin 선언 %d개를 제거합니다. Kotlin 컴파일 옵션이 없는 단순 구성만 자동 지원합니다.", removed))
		}
		if discovered.GradlePropertiesFile != "" {
			relative, relErr := migrationRelative(discovered.Root, discovered.GradlePropertiesFile)
			if relErr != nil {
				return MutationResult{}, relErr
			}
			propertiesFile, ok := filesByRelative[relative]
			if !ok {
				mode, modeErr := migrationFileMode(discovered.GradlePropertiesFile)
				if modeErr != nil {
					return MutationResult{}, modeErr
				}
				propertiesFile = migrationFile{path: discovered.GradlePropertiesFile, relative: relative, content: discovered.GradleProperties, mode: uint32(mode)}
				filesByRelative[relative] = propertiesFile
				contents[relative] = propertiesFile.content
			}
			updated, removedProperties := removeObsoleteAGP9Properties(contents[relative])
			contents[relative] = updated
			if removedProperties > 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("완전한 AGP 9 전환을 위해 obsolete Android Gradle property %d개를 제거합니다.", removedProperties))
			}
		}
	}

	minimumGradle, _ := toolchain.ParseVersion(compatibility.MinimumGradle)
	if currentGradle, parseErr := toolchain.ParseVersion(discovered.WrapperVersion); parseErr != nil {
		result.Blockers = append(result.Blockers, "Gradle Wrapper 버전을 확인할 수 없어 자동 업그레이드할 수 없습니다.")
	} else if currentGradle.Compare(minimumGradle) < 0 {
		if discovered.WrapperFile == "" {
			result.Blockers = append(result.Blockers, "Gradle Wrapper 파일이 없어 자동 업그레이드할 수 없습니다.")
		} else {
			wrapperData, readErr := os.ReadFile(discovered.WrapperFile)
			if readErr != nil {
				return MutationResult{}, fmt.Errorf("read Gradle Wrapper: %w", readErr)
			}
			mode, modeErr := migrationFileMode(discovered.WrapperFile)
			if modeErr != nil {
				return MutationResult{}, modeErr
			}
			relative, relErr := migrationRelative(discovered.Root, discovered.WrapperFile)
			if relErr != nil {
				return MutationResult{}, relErr
			}
			updated, _, transformErr := transformWrapper(string(wrapperData), discovered.WrapperVersion, compatibility.MinimumGradle)
			if transformErr != nil {
				result.Blockers = append(result.Blockers, transformErr.Error())
			} else {
				filesByRelative[relative] = migrationFile{path: discovered.WrapperFile, relative: relative, content: string(wrapperData), mode: uint32(mode)}
				contents[relative] = updated
				result.Warnings = append(result.Warnings, "Wrapper URL과 공식 distribution SHA-256을 함께 갱신합니다. Wrapper JAR·스크립트 재생성은 별도 검토가 필요합니다.")
			}
		}
	}
	if len(result.Blockers) > 0 {
		result.Blockers = sortAndUniqueStrings(result.Blockers)
		return result, nil
	}

	var relatives []string
	for relative := range filesByRelative {
		relatives = append(relatives, relative)
	}
	sort.Strings(relatives)
	for _, relative := range relatives {
		file := filesByRelative[relative]
		after, ok := contents[relative]
		if !ok || after == file.content {
			continue
		}
		beforeData, afterData := []byte(file.content), []byte(after)
		result.Changes = append(result.Changes, FileChange{
			Action:       "update",
			Path:         relative,
			BeforeSHA256: migrationDigest(beforeData),
			AfterSHA256:  migrationDigest(afterData),
			Preview:      migrationDiff(file.content, after),
			before:       beforeData,
			after:        afterData,
			mode:         file.mode,
		})
	}
	if len(result.Changes) == 0 {
		result.Blockers = []string{"목표 버전으로 바꿀 수 있는 파일 변경이 없습니다."}
		return result, nil
	}

	result.Warnings = append(result.Warnings,
		fmt.Sprintf("AARGrade는 JDK를 설치하거나 JAVA_HOME을 바꾸지 않습니다. 검증 시 JDK %d을 사용해야 합니다.", compatibility.RecommendedJDK),
		"이 자동 변경은 정적으로 증명한 선언만 다룹니다. 실제 Gradle 구성과 AAR 고객 호환성 검증은 별도 단계입니다.",
	)
	if currentVersion.Major < targetVersion.Major {
		result.Warnings = append(result.Warnings, "AGP 메이저 버전이 바뀝니다. 미사용 코드 경로와 동적으로 적용되는 convention plugin은 정적 분석 범위 밖입니다.")
	}
	result.Warnings = sortAndUniqueStrings(result.Warnings)
	result.NextSteps = []string{
		fmt.Sprintf("JDK %d에서 `./gradlew help --no-daemon`을 실행하세요.", compatibility.RecommendedJDK),
		"`./gradlew build --dry-run --no-daemon`으로 전체 task graph를 확인하세요.",
		"후보 AAR을 빌드한 뒤 `aargrade verify`로 기준 AAR과 비교하세요.",
		"마지막으로 `aargrade matrix`로 실제 Java·Kotlin 고객 셀을 빌드하세요.",
	}
	result.state = migrationState{
		SchemaVersion: StateSchemaVersion,
		Status:        "prepared",
		ToolVersion:   options.ToolVersion,
		CurrentAGP:    result.CurrentAGP,
		TargetAGP:     result.TargetAGP,
	}
	for _, change := range result.Changes {
		result.state.Files = append(result.state.Files, migrationStateFile{
			Path:           change.Path,
			Mode:           change.mode,
			BeforeSHA256:   change.BeforeSHA256,
			AfterSHA256:    change.AfterSHA256,
			OriginalBase64: encodeMigrationOriginal(change.before),
		})
	}
	result.Ready = true
	if options.Apply {
		if err := applyMigration(&result); err != nil {
			return MutationResult{}, err
		}
		result.Applied = true
	}
	return result, nil
}

func loadMigrationFiles(discovered *project.Project) ([]migrationFile, *migrationFile, error) {
	paths := map[string]bool{discovered.SettingsFile: true}
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		path := filepath.Join(discovered.Root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			paths[path] = true
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	for _, module := range discovered.Modules {
		if module.BuildFile != "" {
			paths[module.BuildFile] = true
		}
	}
	var files []migrationFile
	for path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
		mode, err := migrationFileMode(path)
		if err != nil {
			return nil, nil, err
		}
		relative, err := migrationRelative(discovered.Root, path)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, migrationFile{path: path, relative: relative, content: string(data), mode: uint32(mode)})
	}
	sortMigrationFiles(files)
	catalogPath := filepath.Join(discovered.Root, "gradle", "libs.versions.toml")
	data, err := os.ReadFile(catalogPath)
	if errors.Is(err, os.ErrNotExist) {
		return files, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read version catalog: %w", err)
	}
	mode, err := migrationFileMode(catalogPath)
	if err != nil {
		return nil, nil, err
	}
	catalog := &migrationFile{path: catalogPath, relative: "gradle/libs.versions.toml", content: string(data), mode: uint32(mode)}
	return files, catalog, nil
}

func namespaceBlockers(discovered *project.Project) []string {
	var blockers []string
	for _, module := range discovered.Modules {
		if module.BuildFile == "" || module.HasPlugin("com.android.kotlin.multiplatform.library") {
			continue
		}
		switch module.Kind() {
		case "application", "library", "test", "dynamic-feature":
			if !namespacePattern.MatchString(project.StripComments(module.Content)) {
				relative, _ := migrationRelative(discovered.Root, module.BuildFile)
				blockers = append(blockers, fmt.Sprintf("%s: AGP 8+ requires an explicit Android namespace", relative))
			}
		}
	}
	return blockers
}

func mutationStructureBlockers(discovered *project.Project, diagnosis model.Report) []string {
	var blockers []string
	if discovered.HasBuildSrc {
		blockers = append(blockers, "buildSrc가 있어 적용될 AGP 선언과 DSL을 정적으로 증명할 수 없습니다.")
	}
	if discovered.HasSettingsVersionCatalogs {
		blockers = append(blockers, "settings.gradle에서 만든 custom version catalog는 자동 변경 범위 밖입니다.")
	}
	if discovered.HasRefreshVersions {
		blockers = append(blockers, "RefreshVersions가 관리하는 AGP 선언은 자동 변경하지 않습니다.")
	}
	if hasFindingID(diagnosis, "android.model.unresolved") {
		blockers = append(blockers, "Android-looking module의 plugin이 convention/dynamic logic에 숨겨져 있습니다.")
	}
	return blockers
}

func agp9MutationBlockers(discovered *project.Project, diagnosis model.Report, files []migrationFile, catalog catalogModel) []string {
	var blockers []string
	if hasFindingID(diagnosis, "agp9.legacy-api") {
		blockers = append(blockers, "agp9.legacy-api: legacy Variant/internal API는 androidComponents/public DSL로 수동 전환해야 합니다.")
	}
	for _, module := range discovered.Modules {
		if module.HasPlugin("org.jetbrains.kotlin.multiplatform") && (module.HasPlugin("com.android.application") || module.HasPlugin("com.android.library")) {
			blockers = append(blockers, module.GradlePath+": Kotlin Multiplatform with a legacy Android plugin requires the AGP 9 KMP plugin migration")
		}
	}
	for _, file := range files {
		blockers = append(blockers, patternLocations(file, kotlinOptionsPattern, "Kotlin compiler/source-set DSL requires a manual Built-in Kotlin migration")...)
		clean := project.StripComments(file.content)
		if kotlinBlockPattern.MatchString(clean) && sourceSetsPattern.MatchString(clean) {
			blockers = append(blockers, file.relative+": kotlin { sourceSets ... } requires migration to android.sourceSets Kotlin directories")
		}
		blockers = append(blockers, patternLocations(file, javaKotlinSourceDirPattern, "Kotlin sources configured through Java source dirs require android.sourceSets Kotlin migration")...)
		blockers = append(blockers, patternLocations(file, removedAGP9DSLPattern, "legacy or removed AGP 9 DSL/API requires manual migration")...)
		blockers = append(blockers, patternLocations(file, legacySDKMethodPattern, "legacy Android SDK setter DSL requires manual migration")...)
		if kmpPattern.MatchString(clean) && androidPluginPattern.MatchString(clean) {
			blockers = append(blockers, file.relative+": Kotlin Multiplatform with a legacy Android plugin requires the AGP 9 KMP plugin migration")
		}
		if buildConfigFieldMutationPattern.MatchString(clean) && !buildConfigEnabledMutationPattern.MatchString(clean) {
			blockers = append(blockers, file.relative+": buildConfigField requires explicit buildFeatures.buildConfig = true")
		}
	}
	kaptAliases := catalogAliases(catalog, func(entry catalogEntry) bool { return entry.isKapt })
	legacyKaptAliases := catalogAliases(catalog, func(entry catalogEntry) bool { return entry.isLegacyKapt })
	for _, path := range hasKapt(files, kaptAliases, legacyKaptAliases) {
		blockers = append(blockers, path+": kapt processor migration cannot be inferred safely; choose validated KSP or com.android.legacy-kapt first")
	}
	kspDetected, kspVersions := scanKSPVersions(files, catalog)
	if kspDetected {
		minimum, _ := toolchain.ParseVersion("2.3.6")
		if len(kspVersions) == 0 {
			blockers = append(blockers, "KSP plugin version could not be resolved; AGP 9 automatic migration requires a verified KSP 2.3.6+")
		}
		for _, raw := range kspVersions {
			version, err := toolchain.ParseVersion(raw)
			if err != nil || version.Compare(minimum) < 0 {
				blockers = append(blockers, fmt.Sprintf("KSP %s is below the supported AGP 9 migration floor 2.3.6", raw))
			}
		}
	}
	kotlinAliases := catalogAliases(catalog, func(entry catalogEntry) bool { return entry.isKotlin })
	removed := 0
	for _, file := range files {
		_, count, unsafe := removeKotlinAndroidPluginLines(file.content, kotlinAliases)
		removed += count
		for _, line := range unsafe {
			blockers = append(blockers, fmt.Sprintf("%s:%d: Kotlin Android plugin declaration must be a standalone line for safe removal", file.relative, line))
		}
	}
	if hasFindingID(diagnosis, "agp9.kotlin-android-plugin") && removed == 0 {
		blockers = append(blockers, "Kotlin Android plugin is applied but its declaration could not be safely located")
	}
	return blockers
}

func applyMigration(result *MutationResult) error {
	paths := []string{result.statePath}
	for _, change := range result.Changes {
		absolute, err := migrationJoin(result.ProjectRoot, change.Path)
		if err != nil {
			return err
		}
		paths = append(paths, absolute)
	}
	if err := ensureMigrationNoSymlink(result.ProjectRoot, paths...); err != nil {
		return err
	}
	if _, err := os.Lstat(result.statePath); err == nil {
		return fmt.Errorf("migration state appeared after preview; refusing to overwrite %s", stateRelativePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect migration state before apply: %w", err)
	}
	for _, change := range result.Changes {
		absolute, _ := migrationJoin(result.ProjectRoot, change.Path)
		content, err := os.ReadFile(absolute)
		if err != nil {
			return fmt.Errorf("re-read %s before apply: %w", change.Path, err)
		}
		if migrationDigest(content) != change.BeforeSHA256 {
			return fmt.Errorf("%s changed after preview; run migrate again", change.Path)
		}
	}
	stateData, err := marshalMigrationState(result.state)
	if err != nil {
		return fmt.Errorf("encode migration state: %w", err)
	}
	if len(stateData) > maxMigrationStateSize {
		return fmt.Errorf("migration state exceeds %d bytes", maxMigrationStateSize)
	}
	if err := atomicMigrationWrite(result.statePath, stateData, 0o600); err != nil {
		return fmt.Errorf("write prepared migration state: %w", err)
	}
	for _, change := range result.Changes {
		absolute, _ := migrationJoin(result.ProjectRoot, change.Path)
		current, readErr := os.ReadFile(absolute)
		if readErr != nil {
			return fmt.Errorf("re-read %s during apply: %w; ownership state was preserved, run `aargrade migrate rollback --apply`", change.Path, readErr)
		}
		if migrationDigest(current) != change.BeforeSHA256 {
			return fmt.Errorf("%s changed during apply; ownership state was preserved, run `aargrade migrate rollback --apply`", change.Path)
		}
		if err := atomicMigrationWrite(absolute, change.after, os.FileMode(change.mode)); err != nil {
			return fmt.Errorf("apply %s: %w; ownership state was preserved, run `aargrade migrate rollback --apply`", change.Path, err)
		}
	}
	result.state.Status = "applied"
	stateData, err = marshalMigrationState(result.state)
	if err != nil {
		return fmt.Errorf("encode applied migration state: %w", err)
	}
	if err := atomicMigrationWrite(result.statePath, stateData, 0o600); err != nil {
		return fmt.Errorf("mark migration state applied: %w; run `aargrade migrate rollback --apply` if needed", err)
	}
	return nil
}

func uniqueProjectVersions(values []project.VersionEvidence) []string {
	var versions []string
	for _, value := range values {
		versions = append(versions, value.Value)
	}
	return sortAndUniqueStrings(versions)
}

func hasFindingID(report model.Report, id string) bool {
	for _, finding := range report.Findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
