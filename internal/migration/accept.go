package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Accept previews or removes rollback ownership state after a successful
// migration. It accepts only the exact applied hashes, so it cannot hide a
// partial rollback or silently discard protection for later user edits.
func Accept(options AcceptOptions) (MutationResult, error) {
	root, err := filepath.Abs(options.ProjectPath)
	if err != nil {
		return MutationResult{}, fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return MutationResult{}, fmt.Errorf("project path is not a directory: %s", root)
	}
	statePath, err := migrationJoin(root, stateRelativePath)
	if err != nil {
		return MutationResult{}, err
	}
	if err := ensureMigrationNoSymlink(root, statePath); err != nil {
		return MutationResult{}, err
	}
	state, stateData, err := readMigrationState(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MutationResult{}, fmt.Errorf("no AARGrade migration state found at %s", stateRelativePath)
		}
		return MutationResult{}, fmt.Errorf("read migration state: %w", err)
	}
	if err := validateMigrationState(root, state); err != nil {
		return MutationResult{}, err
	}
	result := MutationResult{
		SchemaVersion: MutationSchemaVersion,
		Operation:     "migrate accept",
		ToolVersion:   state.ToolVersion,
		ProjectRoot:   root,
		CurrentAGP:    state.CurrentAGP,
		TargetAGP:     state.TargetAGP,
		Ready:         state.Status == "applied",
		StatePath:     stateRelativePath,
		statePath:     statePath,
		stateHash:     migrationDigest(stateData),
		state:         state,
	}
	if state.Status != "applied" {
		result.Blockers = append(result.Blockers, "migration transaction is not fully applied; use rollback instead")
	}
	paths := []string{statePath}
	for _, owned := range state.Files {
		absolute, joinErr := migrationJoin(root, owned.Path)
		if joinErr != nil {
			return MutationResult{}, joinErr
		}
		paths = append(paths, absolute)
		content, readErr := os.ReadFile(absolute)
		if readErr != nil {
			return MutationResult{}, fmt.Errorf("read %s: %w", owned.Path, readErr)
		}
		if migrationDigest(content) != owned.AfterSHA256 {
			result.Ready = false
			result.Blockers = append(result.Blockers, owned.Path+": content no longer matches the exact applied migration; refusing to discard rollback state")
			continue
		}
		result.Changes = append(result.Changes, FileChange{Action: "accept", Path: owned.Path})
	}
	if err := ensureMigrationNoSymlink(root, paths...); err != nil {
		return MutationResult{}, err
	}
	result.Blockers = sortAndUniqueStrings(result.Blockers)
	if !result.Ready {
		return result, nil
	}
	result.Warnings = []string{"Applying this operation removes rollback state but keeps every migrated project file unchanged."}
	if options.Apply {
		if err := applyAccept(&result); err != nil {
			return MutationResult{}, err
		}
		result.Applied = true
	}
	return result, nil
}

func applyAccept(result *MutationResult) error {
	stateData, err := os.ReadFile(result.statePath)
	if err != nil {
		return fmt.Errorf("re-read migration state before accept: %w", err)
	}
	if migrationDigest(stateData) != result.stateHash {
		return fmt.Errorf("migration state changed after accept preview; refusing to remove it")
	}
	for _, owned := range result.state.Files {
		absolute, _ := migrationJoin(result.ProjectRoot, owned.Path)
		content, readErr := os.ReadFile(absolute)
		if readErr != nil {
			return fmt.Errorf("re-read %s before accept: %w", owned.Path, readErr)
		}
		if migrationDigest(content) != owned.AfterSHA256 {
			return fmt.Errorf("%s changed after accept preview; refusing to remove rollback state", owned.Path)
		}
	}
	if err := os.Remove(result.statePath); err != nil {
		return fmt.Errorf("remove accepted migration state: %w", err)
	}
	stateDirectory := filepath.Dir(result.statePath)
	_ = os.Remove(stateDirectory)
	_ = os.Remove(filepath.Dir(stateDirectory))
	return nil
}
