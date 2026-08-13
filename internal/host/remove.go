package host

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Remove(options Options) (Plan, error) {
	root, err := filepath.Abs(options.ProjectPath)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return Plan{}, fmt.Errorf("project path is not a directory: %s", root)
	}
	statePath, err := safeJoin(root, stateRelativePath)
	if err != nil {
		return Plan{}, err
	}
	if err := ensureNoSymlink(root, statePath); err != nil {
		return Plan{}, err
	}
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Plan{}, fmt.Errorf("no AARGrade host ownership state found at %s", stateRelativePath)
		}
		return Plan{}, fmt.Errorf("read host ownership state: %w", err)
	}
	var state State
	decoder := json.NewDecoder(bytes.NewReader(stateData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return Plan{}, fmt.Errorf("invalid host ownership state: %w", err)
	}
	if state.SchemaVersion != StateSchemaVersion {
		return Plan{}, fmt.Errorf("unsupported host ownership state schema %d", state.SchemaVersion)
	}
	if err := validateState(state); err != nil {
		return Plan{}, err
	}

	settingsPath, err := safeJoin(root, state.SettingsFile)
	if err != nil {
		return Plan{}, fmt.Errorf("invalid settings path in ownership state: %w", err)
	}
	paths := []string{statePath, settingsPath}
	for _, file := range state.GeneratedFiles {
		path, joinErr := safeJoin(root, file.Path)
		if joinErr != nil {
			return Plan{}, fmt.Errorf("invalid generated path in ownership state: %w", joinErr)
		}
		paths = append(paths, path)
	}
	if err := ensureNoSymlink(root, paths...); err != nil {
		return Plan{}, err
	}

	settingsContent, err := os.ReadFile(settingsPath)
	if err != nil {
		return Plan{}, fmt.Errorf("read settings file: %w", err)
	}
	if strings.Count(string(settingsContent), state.SettingsInsertion) != 1 {
		return Plan{}, fmt.Errorf("owned settings block is missing or changed in %s; refusing automatic removal", state.SettingsFile)
	}
	settingsAfter := []byte(strings.Replace(string(settingsContent), state.SettingsInsertion, "", 1))
	settingsMode, err := fileMode(settingsPath)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		SchemaVersion: PlanSchemaVersion,
		Operation:     "remove",
		ProjectRoot:   root,
		LibraryModule: state.LibraryModule,
		HostModule:    state.HostModule,
		AGPVersion:    state.AGPVersion,
		settingsPath:  settingsPath,
		statePath:     statePath,
		state:         state,
	}
	plan.Changes = append(plan.Changes, Change{
		Action: "update", Path: state.SettingsFile, Preview: deletePreview(state.SettingsInsertion),
		before: settingsContent, after: settingsAfter, mode: uint32(settingsMode),
	})
	for _, file := range state.GeneratedFiles {
		absolute, _ := safeJoin(root, file.Path)
		content, readErr := os.ReadFile(absolute)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				return Plan{}, fmt.Errorf("owned generated file is missing: %s", file.Path)
			}
			return Plan{}, fmt.Errorf("read %s: %w", file.Path, readErr)
		}
		if digest(content) != file.SHA256 {
			return Plan{}, fmt.Errorf("owned generated file was modified: %s; refusing automatic removal", file.Path)
		}
		mode, modeErr := fileMode(absolute)
		if modeErr != nil {
			return Plan{}, modeErr
		}
		plan.Changes = append(plan.Changes, Change{
			Action: "delete", Path: file.Path, Preview: deletePreview(string(content)),
			before: content, mode: uint32(mode),
		})
	}
	stateMode, err := fileMode(statePath)
	if err != nil {
		return Plan{}, err
	}
	plan.Changes = append(plan.Changes, Change{
		Action: "delete", Path: stateRelativePath, Preview: deletePreview(string(stateData)),
		before: stateData, mode: uint32(stateMode),
	})

	if options.Apply {
		if err := applyRemove(&plan); err != nil {
			return Plan{}, err
		}
		plan.Applied = true
	}
	return plan, nil
}

func validateState(state State) error {
	if state.HostModule == "" || state.LibraryModule == "" || state.SettingsFile == "" || state.SettingsInsertion == "" {
		return fmt.Errorf("invalid host ownership state: required fields are missing")
	}
	if err := validateModulePath(state.HostModule); err != nil {
		return fmt.Errorf("invalid host ownership state: %w", err)
	}
	if err := validateModulePath(state.LibraryModule); err != nil {
		return fmt.Errorf("invalid host ownership state: %w", err)
	}
	if !strings.Contains(state.SettingsInsertion, markerStart) || !strings.Contains(state.SettingsInsertion, markerEnd) {
		return fmt.Errorf("invalid host ownership state: settings ownership markers are missing")
	}
	kotlin := state.SettingsFile == "settings.gradle.kts"
	if !kotlin && state.SettingsFile != "settings.gradle" {
		return fmt.Errorf("invalid host ownership state: unexpected settings file %q", state.SettingsFile)
	}
	wantBlock := settingsBlock(kotlin, state.HostModule)
	if state.SettingsInsertion != "\n"+wantBlock && state.SettingsInsertion != "\n\n"+wantBlock {
		return fmt.Errorf("invalid host ownership state: settings insertion does not match the owned host block")
	}
	wantFiles := map[string]bool{
		hostBuildRelativePath(kotlin): true,
		hostManifestRelativePath():    true,
	}
	if len(state.GeneratedFiles) != len(wantFiles) {
		return fmt.Errorf("invalid host ownership state: unexpected generated file count")
	}
	for _, file := range state.GeneratedFiles {
		if !wantFiles[file.Path] || len(file.SHA256) != 64 {
			return fmt.Errorf("invalid host ownership state: bad generated file record for %q", file.Path)
		}
		delete(wantFiles, file.Path)
	}
	for _, directory := range state.CreatedDirectories {
		if directory != ".aargrade" && !strings.HasPrefix(directory, ".aargrade/") {
			return fmt.Errorf("invalid host ownership state: unsafe created directory %q", directory)
		}
	}
	return nil
}

func applyRemove(plan *Plan) error {
	currentSettings, err := os.ReadFile(plan.settingsPath)
	if err != nil {
		return fmt.Errorf("re-read settings before apply: %w", err)
	}
	if digest(currentSettings) != digest(plan.Changes[0].before) {
		return fmt.Errorf("settings file changed after preview; run host remove again")
	}
	for _, change := range plan.Changes[1:] {
		absolute, joinErr := safeJoin(plan.ProjectRoot, change.Path)
		if joinErr != nil {
			return joinErr
		}
		content, readErr := os.ReadFile(absolute)
		if readErr != nil {
			return fmt.Errorf("re-read %s before apply: %w", change.Path, readErr)
		}
		if digest(content) != digest(change.before) {
			return fmt.Errorf("owned file changed after preview: %s", change.Path)
		}
	}
	if err := atomicWrite(plan.settingsPath, plan.Changes[0].after, os.FileMode(plan.Changes[0].mode)); err != nil {
		return err
	}
	removed := make([]Change, 0, len(plan.Changes)-1)
	rollback := func(cause error) error {
		for _, change := range removed {
			absolute, joinErr := safeJoin(plan.ProjectRoot, change.Path)
			if joinErr == nil {
				_ = atomicWrite(absolute, change.before, os.FileMode(change.mode))
			}
		}
		_ = atomicWrite(plan.settingsPath, plan.Changes[0].before, os.FileMode(plan.Changes[0].mode))
		return cause
	}
	for _, change := range plan.Changes[1:] {
		absolute, _ := safeJoin(plan.ProjectRoot, change.Path)
		if err := os.Remove(absolute); err != nil {
			return rollback(fmt.Errorf("remove %s: %w", change.Path, err))
		}
		removed = append(removed, change)
	}
	created := append([]string(nil), plan.state.CreatedDirectories...)
	sort.Slice(created, func(i, j int) bool {
		leftDepth := strings.Count(created[i], "/")
		rightDepth := strings.Count(created[j], "/")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return created[i] > created[j]
	})
	for _, directory := range created {
		absolute, joinErr := safeJoin(plan.ProjectRoot, directory)
		if joinErr == nil {
			_ = os.Remove(absolute)
		}
	}
	return nil
}
