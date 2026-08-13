package host

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
	mode := "PREVIEW — no files changed"
	if plan.Applied {
		mode = "APPLIED"
	}
	if _, err := fmt.Fprintf(writer, "AARGrade host %s\n%s\n", plan.Operation, mode); err != nil {
		return err
	}
	if plan.LibraryModule != "" {
		if _, err := fmt.Fprintf(writer, "Library: %s\n", plan.LibraryModule); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "Host: %s\n", plan.HostModule); err != nil {
		return err
	}
	for _, change := range plan.Changes {
		if _, err := fmt.Fprintf(writer, "\n[%s] %s\n", strings.ToUpper(change.Action), change.Path); err != nil {
			return err
		}
		if change.Preview != "" {
			if _, err := fmt.Fprintln(writer, change.Preview); err != nil {
				return err
			}
		}
	}
	if !plan.Applied {
		_, err := fmt.Fprintln(writer, "\nRe-run the reviewed command with `--apply`, preserving any selection or version override flags.")
		return err
	}
	return nil
}
