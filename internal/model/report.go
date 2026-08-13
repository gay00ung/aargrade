package model

import "strings"

const ReportSchemaVersion = 1

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

func ParseSeverity(value string) (Severity, bool) {
	switch strings.ToLower(value) {
	case string(SeverityInfo):
		return SeverityInfo, true
	case string(SeverityWarn), "warning":
		return SeverityWarn, true
	case string(SeverityError):
		return SeverityError, true
	default:
		return "", false
	}
}

func SeverityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 3
	case SeverityWarn:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

type Evidence struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type Finding struct {
	ID             string     `json:"id"`
	Severity       Severity   `json:"severity"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	Recommendation string     `json:"recommendation,omitempty"`
	Evidence       []Evidence `json:"evidence,omitempty"`
}

type Module struct {
	GradlePath string   `json:"gradlePath"`
	Directory  string   `json:"directory"`
	BuildFile  string   `json:"buildFile,omitempty"`
	Kind       string   `json:"kind"`
	Plugins    []string `json:"plugins,omitempty"`
}

type Version struct {
	Value  string `json:"value,omitempty"`
	Source string `json:"source,omitempty"`
}

type Inventory struct {
	SettingsFile string   `json:"settingsFile"`
	Gradle       Version  `json:"gradle"`
	AGP          Version  `json:"agp"`
	Modules      []Module `json:"modules"`
}

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	ToolVersion   string    `json:"toolVersion"`
	ProjectRoot   string    `json:"projectRoot"`
	Inventory     Inventory `json:"inventory"`
	Findings      []Finding `json:"findings"`
}

func (r Report) Counts() map[Severity]int {
	counts := map[Severity]int{
		SeverityInfo:  0,
		SeverityWarn:  0,
		SeverityError: 0,
	}
	for _, finding := range r.Findings {
		counts[finding.Severity]++
	}
	return counts
}
