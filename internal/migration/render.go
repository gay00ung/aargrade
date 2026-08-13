package migration

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func RenderJSON(writer io.Writer, plan Plan) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(plan)
}

func RenderText(writer io.Writer, plan Plan) error {
	status := "READY"
	if !plan.Ready {
		status = "BLOCKED"
	}
	if _, err := fmt.Fprintf(writer, "AARGrade migration plan — %s\nProject: %s\nAGP: %s → %s\nRequired toolchain: Gradle %s+, JDK %d\n", status, plan.ProjectRoot, valueOrUnknown(plan.CurrentAGP), plan.TargetAGP, plan.Toolchain.MinimumGradle, plan.Toolchain.RecommendedJDK); err != nil {
		return err
	}
	if len(plan.Blockers) > 0 {
		if _, err := fmt.Fprintln(writer, "\nBlockers:"); err != nil {
			return err
		}
		for _, blocker := range plan.Blockers {
			if _, err := fmt.Fprintf(writer, "- %s\n", blocker); err != nil {
				return err
			}
		}
	}
	for _, step := range plan.Steps {
		if _, err := fmt.Fprintf(writer, "\n%d. [%s] %s\n   Why: %s\n   Action: %s\n", step.Order, strings.ToUpper(string(step.Kind)), step.Title, step.Why, step.Action); err != nil {
			return err
		}
		for _, evidence := range step.Evidence {
			location := evidence.Path
			if evidence.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, evidence.Line)
			}
			if _, err := fmt.Fprintf(writer, "   Evidence: %s\n", location); err != nil {
				return err
			}
		}
	}
	return nil
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
