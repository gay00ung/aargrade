package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gay00ung/aargrade/internal/model"
)

func JSON(writer io.Writer, value model.Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func Text(writer io.Writer, value model.Report) error {
	counts := value.Counts()
	if _, err := fmt.Fprintln(writer, "AARGrade doctor"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Project: %s\n", value.ProjectRoot); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "AGP: %s\n", printableVersion(value.Inventory.AGP)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Gradle: %s\n", printableVersion(value.Inventory.Gradle)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Modules: %d\n", len(value.Inventory.Modules)); err != nil {
		return err
	}

	for _, finding := range value.Findings {
		if _, err := fmt.Fprintf(writer, "\n[%s] %s — %s\n", strings.ToUpper(string(finding.Severity)), finding.ID, finding.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "  %s\n", finding.Message); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			location := evidence.Path
			if evidence.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, evidence.Line)
			}
			if evidence.Snippet == "" {
				if _, err := fmt.Fprintf(writer, "  at %s\n", location); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(writer, "  at %s: %s\n", location, evidence.Snippet); err != nil {
				return err
			}
		}
		if finding.Recommendation != "" {
			if _, err := fmt.Fprintf(writer, "  Action: %s\n", finding.Recommendation); err != nil {
				return err
			}
		}
	}

	_, err := fmt.Fprintf(
		writer,
		"\nSummary: %d error(s), %d warning(s), %d info\n",
		counts[model.SeverityError],
		counts[model.SeverityWarn],
		counts[model.SeverityInfo],
	)
	return err
}

func printableVersion(version model.Version) string {
	if version.Value == "" {
		return "unknown"
	}
	if version.Source == "" {
		return version.Value
	}
	return fmt.Sprintf("%s (%s)", version.Value, version.Source)
}
