package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/project"
)

func Add(options Options) (Plan, error) {
	discovered, err := project.Discover(options.ProjectPath)
	if err != nil {
		return Plan{}, err
	}
	statePath := filepath.Join(discovered.Root, filepath.FromSlash(stateRelativePath))
	if _, err := os.Lstat(statePath); err == nil {
		return Plan{}, fmt.Errorf("an AARGrade host is already recorded in %s", stateRelativePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Plan{}, fmt.Errorf("inspect %s: %w", statePath, err)
	}
	if strings.Contains(discovered.SettingsContent, markerStart) || strings.Contains(discovered.SettingsContent, markerEnd) {
		return Plan{}, fmt.Errorf("settings file contains an untracked AARGrade host marker; remove or reconcile it manually")
	}

	var applications, libraries []project.Module
	for _, module := range discovered.Modules {
		switch module.Kind() {
		case "application":
			applications = append(applications, module)
		case "library":
			libraries = append(libraries, module)
		}
	}
	if len(applications) > 0 {
		return Plan{}, fmt.Errorf("refusing to add a temporary host: project already has %d Android application module(s)", len(applications))
	}
	library, err := selectLibrary(libraries, options.LibraryPath)
	if err != nil {
		return Plan{}, err
	}

	hostModule := options.ModulePath
	if hostModule == "" {
		hostModule = defaultModulePath
	}
	if err := validateModulePath(hostModule); err != nil {
		return Plan{}, err
	}
	if hostModule == library.GradlePath {
		return Plan{}, fmt.Errorf("host module path %s conflicts with the selected library", hostModule)
	}
	for _, module := range discovered.Modules {
		if module.GradlePath == hostModule {
			return Plan{}, fmt.Errorf("module %s already exists in settings", hostModule)
		}
	}

	agpRaw, err := chooseAGPVersion(discovered, options.AGPVersion)
	if err != nil {
		return Plan{}, err
	}
	version, err := parseAGPVersion(agpRaw)
	if err != nil {
		return Plan{}, err
	}
	compileSDK := options.CompileSDK
	if compileSDK == 0 {
		compileSDK = parseLiteralSDK(project.StripComments(library.Content), compileSDKPatterns)
	}
	if compileSDK == 0 {
		return Plan{}, fmt.Errorf("could not resolve a literal compileSdk from %s; pass --compile-sdk explicitly", library.GradlePath)
	}
	if compileSDK < 1 {
		return Plan{}, fmt.Errorf("compile SDK must be positive, got %d", compileSDK)
	}
	minSDK := options.MinSDK
	if minSDK == 0 {
		minSDK = parseLiteralSDK(project.StripComments(library.Content), minSDKPatterns)
	}
	if minSDK == 0 {
		minSDK = 21
	}
	if minSDK < 1 {
		return Plan{}, fmt.Errorf("minimum SDK must be positive, got %d", minSDK)
	}
	if minSDK > compileSDK {
		return Plan{}, fmt.Errorf("min SDK %d cannot exceed compile SDK %d", minSDK, compileSDK)
	}

	kotlin := strings.HasSuffix(discovered.SettingsFile, ".kts")
	block := settingsBlock(kotlin, hostModule)
	insertion := settingsInsertion(discovered.SettingsContent, block)
	settingsAfter := []byte(discovered.SettingsContent + insertion)
	settingsBefore := []byte(discovered.SettingsContent)
	settingsMode, err := fileMode(discovered.SettingsFile)
	if err != nil {
		return Plan{}, fmt.Errorf("inspect settings permissions: %w", err)
	}

	generated := []struct {
		path    string
		content []byte
	}{
		{hostBuildRelativePath(kotlin), []byte(hostBuildFile(kotlin, version, library.GradlePath, compileSDK, minSDK))},
		{hostManifestRelativePath(), []byte(hostManifest(version))},
	}
	for _, file := range generated {
		absolute, joinErr := safeJoin(discovered.Root, file.path)
		if joinErr != nil {
			return Plan{}, joinErr
		}
		if _, statErr := os.Lstat(absolute); statErr == nil {
			return Plan{}, fmt.Errorf("refusing to overwrite existing path %s", file.path)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return Plan{}, fmt.Errorf("inspect %s: %w", file.path, statErr)
		}
	}

	directories, err := missingDirectories(
		discovered.Root,
		filepath.Join(discovered.Root, filepath.FromSlash(hostRelativeDir)),
		filepath.Join(discovered.Root, filepath.FromSlash(hostRelativeDir), "src", "main"),
		filepath.Dir(statePath),
	)
	if err != nil {
		return Plan{}, err
	}
	settingsRelative, err := slashRelative(discovered.Root, discovered.SettingsFile)
	if err != nil {
		return Plan{}, err
	}
	state := State{
		SchemaVersion:      StateSchemaVersion,
		HostModule:         hostModule,
		LibraryModule:      library.GradlePath,
		AGPVersion:         agpRaw,
		SettingsFile:       settingsRelative,
		SettingsInsertion:  insertion,
		CreatedDirectories: directories,
	}
	for _, file := range generated {
		state.GeneratedFiles = append(state.GeneratedFiles, OwnedFile{Path: file.path, SHA256: digest(file.content)})
	}
	stateData, err := marshalState(state)
	if err != nil {
		return Plan{}, fmt.Errorf("encode host ownership state: %w", err)
	}

	plan := Plan{
		SchemaVersion: PlanSchemaVersion,
		Operation:     "add",
		ProjectRoot:   discovered.Root,
		LibraryModule: library.GradlePath,
		HostModule:    hostModule,
		AGPVersion:    agpRaw,
		CompileSDK:    compileSDK,
		MinSDK:        minSDK,
		settingsPath:  discovered.SettingsFile,
		statePath:     statePath,
		state:         state,
	}
	plan.Changes = append(plan.Changes, Change{
		Action: "update", Path: settingsRelative, Preview: addPreview(insertion),
		before: settingsBefore, after: settingsAfter, mode: uint32(settingsMode),
	})
	for _, file := range generated {
		plan.Changes = append(plan.Changes, Change{
			Action: "create", Path: file.path, Preview: addPreview(string(file.content)),
			after: file.content, mode: 0o644,
		})
	}
	plan.Changes = append(plan.Changes, Change{
		Action: "create", Path: stateRelativePath, Preview: addPreview(string(stateData)),
		after: stateData, mode: 0o600,
	})

	if options.Apply {
		if err := applyAdd(&plan); err != nil {
			return Plan{}, err
		}
		plan.Applied = true
	}
	return plan, nil
}

func selectLibrary(libraries []project.Module, requested string) (project.Module, error) {
	if requested != "" {
		if err := validateModulePath(requested); err != nil {
			return project.Module{}, err
		}
		for _, library := range libraries {
			if library.GradlePath == requested {
				return library, nil
			}
		}
		return project.Module{}, fmt.Errorf("%s is not a statically resolved Android library module", requested)
	}
	if len(libraries) == 0 {
		return project.Module{}, fmt.Errorf("no statically resolved Android library module found")
	}
	if len(libraries) > 1 {
		paths := make([]string, 0, len(libraries))
		for _, library := range libraries {
			paths = append(paths, library.GradlePath)
		}
		sort.Strings(paths)
		return project.Module{}, fmt.Errorf("multiple Android libraries found (%s); select one with --library", strings.Join(paths, ", "))
	}
	return libraries[0], nil
}

func chooseAGPVersion(discovered *project.Project, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	versions := make([]string, 0, len(discovered.AGPVersions))
	for _, item := range discovered.AGPVersions {
		versions = append(versions, item.Value)
	}
	versions = uniqueStrings(versions)
	if len(versions) == 0 {
		return "", fmt.Errorf("could not resolve one literal AGP version; pass --agp-version explicitly")
	}
	if len(versions) > 1 {
		return "", fmt.Errorf("multiple AGP versions found (%s); pass --agp-version explicitly", strings.Join(versions, ", "))
	}
	return versions[0], nil
}

func applyAdd(plan *Plan) error {
	paths := []string{plan.settingsPath, plan.statePath}
	for _, change := range plan.Changes {
		if change.Action == "create" {
			absolute, err := safeJoin(plan.ProjectRoot, change.Path)
			if err != nil {
				return err
			}
			paths = append(paths, absolute)
		}
	}
	if err := ensureNoSymlink(plan.ProjectRoot, paths...); err != nil {
		return err
	}
	currentSettings, err := os.ReadFile(plan.settingsPath)
	if err != nil {
		return fmt.Errorf("re-read settings before apply: %w", err)
	}
	if digest(currentSettings) != digest(plan.Changes[0].before) {
		return fmt.Errorf("settings file changed after preview; run host add again")
	}
	createdFiles := make([]string, 0, len(plan.Changes)-1)
	settingsWritten := false
	rollback := func(cause error) error {
		for index := len(createdFiles) - 1; index >= 0; index-- {
			_ = os.Remove(createdFiles[index])
		}
		if settingsWritten {
			_ = atomicWrite(plan.settingsPath, plan.Changes[0].before, os.FileMode(plan.Changes[0].mode))
		}
		for index := len(plan.state.CreatedDirectories) - 1; index >= 0; index-- {
			if directory, err := safeJoin(plan.ProjectRoot, plan.state.CreatedDirectories[index]); err == nil {
				_ = os.Remove(directory)
			}
		}
		return cause
	}
	for _, change := range plan.Changes[1:] {
		absolute, err := safeJoin(plan.ProjectRoot, change.Path)
		if err != nil {
			return rollback(err)
		}
		if _, err := os.Lstat(absolute); err == nil {
			return rollback(fmt.Errorf("path appeared after preview; refusing to overwrite %s", change.Path))
		} else if !errors.Is(err, os.ErrNotExist) {
			return rollback(fmt.Errorf("inspect %s: %w", change.Path, err))
		}
		if err := atomicWrite(absolute, change.after, os.FileMode(change.mode)); err != nil {
			return rollback(err)
		}
		createdFiles = append(createdFiles, absolute)
	}
	if err := atomicWrite(plan.settingsPath, plan.Changes[0].after, os.FileMode(plan.Changes[0].mode)); err != nil {
		return rollback(err)
	}
	settingsWritten = true
	return nil
}
