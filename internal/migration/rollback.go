package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Rollback previews or restores the exact pre-migration contents recorded by
// Migrate. Any user edit after apply causes the whole rollback to fail closed.
func Rollback(options RollbackOptions) (MutationResult, error) {
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
		Operation:     "migrate rollback",
		ToolVersion:   state.ToolVersion,
		ProjectRoot:   root,
		CurrentAGP:    state.TargetAGP,
		TargetAGP:     state.CurrentAGP,
		Ready:         true,
		StatePath:     stateRelativePath,
		statePath:     statePath,
		stateHash:     migrationDigest(stateData),
		state:         state,
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
		hash := migrationDigest(content)
		switch hash {
		case owned.AfterSHA256:
			original, decodeErr := decodeMigrationOriginal(owned)
			if decodeErr != nil {
				return MutationResult{}, fmt.Errorf("decode original %s: %w", owned.Path, decodeErr)
			}
			result.Changes = append(result.Changes, FileChange{
				Action:       "restore",
				Path:         owned.Path,
				BeforeSHA256: hash,
				AfterSHA256:  owned.BeforeSHA256,
				Preview:      migrationDiff(string(content), string(original)),
				before:       content,
				after:        original,
				mode:         owned.Mode,
			})
		case owned.BeforeSHA256:
			result.Warnings = append(result.Warnings, owned.Path+": already matches its pre-migration content")
		default:
			result.Ready = false
			result.Blockers = append(result.Blockers, owned.Path+": content changed after migration; refusing automatic rollback")
		}
	}
	if err := ensureMigrationNoSymlink(root, paths...); err != nil {
		return MutationResult{}, err
	}
	result.Blockers = sortAndUniqueStrings(result.Blockers)
	result.Warnings = sortAndUniqueStrings(result.Warnings)
	if !result.Ready {
		return result, nil
	}
	if options.Apply {
		if err := applyRollback(&result); err != nil {
			return MutationResult{}, err
		}
		result.Applied = true
	}
	return result, nil
}

func applyRollback(result *MutationResult) error {
	stateData, err := os.ReadFile(result.statePath)
	if err != nil {
		return fmt.Errorf("re-read migration state before rollback: %w", err)
	}
	if migrationDigest(stateData) != result.stateHash {
		return fmt.Errorf("migration state changed after rollback preview; refusing automatic rollback")
	}
	for _, owned := range result.state.Files {
		absolute, _ := migrationJoin(result.ProjectRoot, owned.Path)
		content, err := os.ReadFile(absolute)
		if err != nil {
			return fmt.Errorf("re-read %s before rollback: %w", owned.Path, err)
		}
		hash := migrationDigest(content)
		if hash != owned.BeforeSHA256 && hash != owned.AfterSHA256 {
			return fmt.Errorf("%s changed after rollback preview; refusing automatic rollback", owned.Path)
		}
	}
	for _, owned := range result.state.Files {
		absolute, _ := migrationJoin(result.ProjectRoot, owned.Path)
		content, err := os.ReadFile(absolute)
		if err != nil {
			return err
		}
		if migrationDigest(content) == owned.BeforeSHA256 {
			continue
		}
		if migrationDigest(content) != owned.AfterSHA256 {
			return fmt.Errorf("%s changed during rollback; migration state was preserved", owned.Path)
		}
		original, decodeErr := decodeMigrationOriginal(owned)
		if decodeErr != nil {
			return fmt.Errorf("decode original %s: %w", owned.Path, decodeErr)
		}
		if err := atomicMigrationWrite(absolute, original, os.FileMode(owned.Mode)); err != nil {
			return fmt.Errorf("restore %s: %w; migration state was preserved so rollback can be retried", owned.Path, err)
		}
	}
	stateData, err = os.ReadFile(result.statePath)
	if err != nil {
		return fmt.Errorf("re-read migration state before removal: %w", err)
	}
	if migrationDigest(stateData) != result.stateHash {
		return fmt.Errorf("migration state changed during rollback; restored files were preserved and state was not removed")
	}
	if err := os.Remove(result.statePath); err != nil {
		return fmt.Errorf("remove migration state: %w", err)
	}
	stateDirectory := filepath.Dir(result.statePath)
	_ = os.Remove(stateDirectory)
	_ = os.Remove(filepath.Dir(stateDirectory))
	return nil
}
