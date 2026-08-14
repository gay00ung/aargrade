package migration

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func RenderMutationJSON(writer io.Writer, result MutationResult) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func RenderMutationText(writer io.Writer, result MutationResult) error {
	status := "PREVIEW — no files changed"
	if !result.Ready {
		status = "BLOCKED — no files changed"
	} else if result.TransactionStarted && !result.Applied {
		status = "INCOMPLETE — rollback state recorded"
	} else if result.Applied {
		status = "APPLIED"
	}
	if _, err := fmt.Fprintf(writer, "AARGrade %s — %s\nProject: %s\n", result.Operation, status, result.ProjectRoot); err != nil {
		return err
	}
	if result.CurrentAGP != "" || result.TargetAGP != "" {
		if _, err := fmt.Fprintf(writer, "AGP: %s → %s\n", valueOrUnknown(result.CurrentAGP), valueOrUnknown(result.TargetAGP)); err != nil {
			return err
		}
	}
	if result.Toolchain != nil && result.Toolchain.MinimumGradle != "" {
		if _, err := fmt.Fprintf(writer, "Required toolchain: Gradle %s+, JDK %d\n", result.Toolchain.MinimumGradle, result.Toolchain.RecommendedJDK); err != nil {
			return err
		}
	}
	if result.StatePath != "" {
		if _, err := fmt.Fprintf(writer, "Rollback state: %s\n", result.StatePath); err != nil {
			return err
		}
	}
	if len(result.Blockers) > 0 {
		if _, err := fmt.Fprintln(writer, "\nBlockers:"); err != nil {
			return err
		}
		for _, blocker := range result.Blockers {
			if _, err := fmt.Fprintf(writer, "- %s\n", blocker); err != nil {
				return err
			}
		}
	}
	if len(result.Warnings) > 0 {
		if _, err := fmt.Fprintln(writer, "\nWarnings:"); err != nil {
			return err
		}
		for _, warning := range result.Warnings {
			if _, err := fmt.Fprintf(writer, "- %s\n", warning); err != nil {
				return err
			}
		}
	}
	for _, change := range result.Changes {
		if _, err := fmt.Fprintf(writer, "\n[%s] %s\n", strings.ToUpper(change.Action), change.Path); err != nil {
			return err
		}
		if change.Preview != "" {
			if _, err := fmt.Fprintln(writer, change.Preview); err != nil {
				return err
			}
		}
	}
	if len(result.NextSteps) > 0 {
		if _, err := fmt.Fprintln(writer, "\nNext steps:"); err != nil {
			return err
		}
		for index, step := range result.NextSteps {
			if _, err := fmt.Fprintf(writer, "%d. %s\n", index+1, step); err != nil {
				return err
			}
		}
	}
	if result.Ready && !result.Applied {
		_, err := fmt.Fprintln(writer, "\nReview this preview, then re-run the same command with `--apply`.")
		return err
	}
	return nil
}
