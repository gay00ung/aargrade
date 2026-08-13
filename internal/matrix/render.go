package matrix

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
	if _, err := fmt.Fprintf(writer, "AARGrade consumer matrix — %s\nCandidate: %s\nWork directory: %s\n", strings.ToUpper(report.Verdict), report.CandidateAAR, report.WorkDirectory); err != nil {
		return err
	}
	if report.BaselineAAR != "" {
		if _, err := fmt.Fprintf(writer, "Baseline: %s\n", report.BaselineAAR); err != nil {
			return err
		}
	}
	for _, cell := range report.Cells {
		if _, err := fmt.Fprintf(writer, "\n[%s] %s — AGP %s / Gradle %s / JDK %d / API %d / %s\n  %s\n", strings.ToUpper(cell.Verdict), cell.Name, cell.AGP, cell.Gradle, cell.JDK, cell.CompileSDK, cell.Language, cell.Reason); err != nil {
			return err
		}
		if cell.Baseline != nil {
			if _, err := fmt.Fprintf(writer, "  baseline: %s (%d ms)\n", cell.Baseline.Status, cell.Baseline.DurationMS); err != nil {
				return err
			}
		}
		if cell.Candidate != nil {
			if _, err := fmt.Fprintf(writer, "  candidate: %s (%d ms)\n", cell.Candidate.Status, cell.Candidate.DurationMS); err != nil {
				return err
			}
		}
	}
	return nil
}
