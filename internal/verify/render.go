package verify

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func RenderJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func RenderText(writer io.Writer, report Report) error {
	if _, err := fmt.Fprintf(writer, "AARGrade verify — %s\nScope: %s\nCandidate: %s\n", strings.ToUpper(report.Verdict), report.Scope, report.Candidate.Path); err != nil {
		return err
	}
	if report.Baseline != nil {
		if _, err := fmt.Fprintf(writer, "Baseline: %s\n", report.Baseline.Path); err != nil {
			return err
		}
	}
	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(writer, "\n[%s] %s — %s\n", strings.ToUpper(string(check.Status)), check.ID, check.Summary); err != nil {
			return err
		}
		for _, detail := range check.Details {
			if _, err := fmt.Fprintf(writer, "  - %s\n", detail); err != nil {
				return err
			}
		}
	}
	if len(report.Limitations) > 0 {
		if _, err := fmt.Fprintln(writer, "\nLimitations:"); err != nil {
			return err
		}
		for _, limitation := range report.Limitations {
			if _, err := fmt.Fprintf(writer, "- %s\n", limitation); err != nil {
				return err
			}
		}
	}
	return nil
}
