package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	consumer "github.com/gay00ung/aargrade/internal/matrix"
	"github.com/gay00ung/aargrade/internal/migration"
	verification "github.com/gay00ung/aargrade/internal/verify"
)

func RenderJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func RenderText(writer io.Writer, report Report) error {
	if _, err := fmt.Fprintf(writer, "AARGrade upgrade — %s\nProject: %s\nTarget AGP: %s\n", strings.ToUpper(report.Verdict), report.ProjectRoot, report.TargetAGP); err != nil {
		return err
	}
	if len(report.Migration.Repairs) > 0 {
		if _, err := fmt.Fprintln(writer, "\nAutomatic repairs:"); err != nil {
			return err
		}
		for _, repair := range report.Migration.Repairs {
			if _, err := fmt.Fprintf(writer, "- %s: %s (%s)\n", repair.ID, repair.Summary, repair.Path); err != nil {
				return err
			}
		}
	}
	if report.Verdict == "preview" || report.Verdict == "blocked" || report.Verdict == "incomplete" && report.Verification == nil {
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
		if err := migration.RenderMutationText(writer, report.Migration); err != nil {
			return err
		}
	}
	if report.Verification != nil {
		if _, err := fmt.Fprintln(writer, "\nProject and AAR verification:"); err != nil {
			return err
		}
		if err := verification.RenderText(writer, *report.Verification); err != nil {
			return err
		}
	}
	if report.Matrix != nil {
		if _, err := fmt.Fprintln(writer, "\nConsumer matrix:"); err != nil {
			return err
		}
		if err := consumer.RenderText(writer, *report.Matrix); err != nil {
			return err
		}
	}
	if report.Failure != nil {
		if _, err := fmt.Fprintf(writer, "\nFailure analysis [%s]: %s\nAction: %s\n", report.Failure.Category, report.Failure.Summary, report.Failure.SuggestedAction); err != nil {
			return err
		}
	}
	if len(report.Limitations) > 0 {
		if _, err := fmt.Fprintln(writer, "\nUpgrade limitations:"); err != nil {
			return err
		}
		for _, limitation := range report.Limitations {
			if _, err := fmt.Fprintf(writer, "- %s\n", limitation); err != nil {
				return err
			}
		}
	}
	if report.RolledBack {
		if _, err := fmt.Fprintln(writer, "\nThe owned Gradle configuration was automatically rolled back."); err != nil {
			return err
		}
	} else if report.Verdict == "pass" {
		if _, err := fmt.Fprintln(writer, "\nUpgrade evidence passed. Review with `aargrade migrate rollback`, then either roll back or run `aargrade migrate accept --apply`."); err != nil {
			return err
		}
	}
	return nil
}
