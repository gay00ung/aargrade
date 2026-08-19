package integration_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type issueForm struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Body        []map[string]any `yaml:"body"`
}

func TestGitHubIssueFormsAreStructurallyValid(t *testing.T) {
	t.Parallel()

	directory, err := filepath.Abs(filepath.Join("..", "..", ".github", "ISSUE_TEMPLATE"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}

	formCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" || entry.Name() == "config.yml" {
			continue
		}
		formCount++
		validateIssueForm(t, filepath.Join(directory, entry.Name()))
	}
	if formCount < 2 {
		t.Fatalf("issue form count = %d, want at least beta validation and bug report forms", formCount)
	}

	validateIssueTemplateConfig(t, filepath.Join(directory, "config.yml"))
}

func validateIssueForm(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var form issueForm
	if err := yaml.Unmarshal(data, &form); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if strings.TrimSpace(form.Name) == "" || strings.TrimSpace(form.Description) == "" {
		t.Fatalf("%s must define a non-empty name and description", path)
	}
	if len(form.Body) == 0 {
		t.Fatalf("%s must define at least one body item", path)
	}

	knownTypes := map[string]bool{
		"markdown":   true,
		"input":      true,
		"textarea":   true,
		"dropdown":   true,
		"checkboxes": true,
		"upload":     true,
	}
	validID := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	ids := map[string]bool{}
	inputCount := 0
	for index, item := range form.Body {
		itemType, _ := item["type"].(string)
		if !knownTypes[itemType] {
			t.Fatalf("%s body[%d] has unsupported type %q", path, index, itemType)
		}
		attributes, ok := item["attributes"].(map[string]any)
		if !ok {
			t.Fatalf("%s body[%d] has no attributes map", path, index)
		}
		if itemType == "markdown" {
			if value, _ := attributes["value"].(string); strings.TrimSpace(value) == "" {
				t.Fatalf("%s body[%d] markdown is empty", path, index)
			}
			continue
		}

		inputCount++
		id, _ := item["id"].(string)
		if strings.TrimSpace(id) == "" {
			t.Fatalf("%s body[%d] must define an id", path, index)
		}
		if !validID.MatchString(id) {
			t.Fatalf("%s body[%d] has invalid id %q", path, index, id)
		}
		if ids[id] {
			t.Fatalf("%s repeats body id %q", path, id)
		}
		ids[id] = true
		if label, _ := attributes["label"].(string); strings.TrimSpace(label) == "" {
			t.Fatalf("%s body[%d] must define an attributes.label", path, index)
		}
		if validations, exists := item["validations"]; exists {
			validationMap, ok := validations.(map[string]any)
			if !ok {
				t.Fatalf("%s body[%d] validations must be a map", path, index)
			}
			if required, exists := validationMap["required"]; exists {
				if _, ok := required.(bool); !ok {
					t.Fatalf("%s body[%d] validations.required must be boolean", path, index)
				}
			}
		}
		if itemType == "dropdown" {
			validateDropdownOptions(t, path, index, attributes["options"])
		}
		if itemType == "checkboxes" {
			validateCheckboxOptions(t, path, index, attributes["options"])
		}
	}
	if inputCount == 0 {
		t.Fatalf("%s must contain at least one user input", path)
	}
}

func validateDropdownOptions(t *testing.T, path string, index int, value any) {
	t.Helper()

	options, ok := value.([]any)
	if !ok || len(options) == 0 {
		t.Fatalf("%s body[%d] must define non-empty dropdown options", path, index)
	}
	seen := map[string]bool{}
	for optionIndex, option := range options {
		text, ok := option.(string)
		if !ok || strings.TrimSpace(text) == "" {
			t.Fatalf("%s body[%d] dropdown option[%d] must be a non-empty string", path, index, optionIndex)
		}
		if seen[text] {
			t.Fatalf("%s body[%d] repeats dropdown option %q", path, index, text)
		}
		seen[text] = true
	}
}

func validateCheckboxOptions(t *testing.T, path string, index int, value any) {
	t.Helper()

	options, ok := value.([]any)
	if !ok || len(options) == 0 {
		t.Fatalf("%s body[%d] must define non-empty checkbox options", path, index)
	}
	for optionIndex, option := range options {
		optionMap, ok := option.(map[string]any)
		if !ok {
			t.Fatalf("%s body[%d] checkbox option[%d] must be a map", path, index, optionIndex)
		}
		label, _ := optionMap["label"].(string)
		if strings.TrimSpace(label) == "" {
			t.Fatalf("%s body[%d] checkbox option[%d] must define a label", path, index, optionIndex)
		}
		if required, exists := optionMap["required"]; exists {
			if _, ok := required.(bool); !ok {
				t.Fatalf("%s body[%d] checkbox option[%d] required must be boolean", path, index, optionIndex)
			}
		}
	}
}

func validateIssueTemplateConfig(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		BlankIssuesEnabled *bool `yaml:"blank_issues_enabled"`
		ContactLinks       []struct {
			Name  string `yaml:"name"`
			URL   string `yaml:"url"`
			About string `yaml:"about"`
		} `yaml:"contact_links"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if config.BlankIssuesEnabled == nil {
		t.Fatalf("%s must explicitly set blank_issues_enabled", path)
	}
	for index, link := range config.ContactLinks {
		if link.Name == "" || link.URL == "" || link.About == "" {
			t.Fatalf("%s contact_links[%d] is incomplete", path, index)
		}
	}
}
