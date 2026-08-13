package verify

import (
	"context"
	"time"

	"github.com/gay00ung/aargrade/internal/artifact"
)

const SchemaVersion = 1

type Status string

const (
	StatusPass    Status = "pass"
	StatusWarning Status = "warning"
	StatusFail    Status = "fail"
	StatusSkipped Status = "skipped"
)

type Check struct {
	ID      string   `json:"id"`
	Status  Status   `json:"status"`
	Summary string   `json:"summary"`
	Details []string `json:"details,omitempty"`
}

type Command struct {
	Name       string   `json:"name"`
	Directory  string   `json:"directory"`
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
	ExitCode   int      `json:"exitCode"`
	DurationMS int64    `json:"durationMs"`
	Status     Status   `json:"status"`
	Output     string   `json:"output,omitempty"`
}

type Report struct {
	SchemaVersion int                     `json:"schemaVersion"`
	ToolVersion   string                  `json:"toolVersion"`
	Verdict       string                  `json:"verdict"`
	Scope         string                  `json:"scope"`
	ProjectRoot   string                  `json:"projectRoot,omitempty"`
	LibraryModule string                  `json:"libraryModule,omitempty"`
	Candidate     artifact.Snapshot       `json:"candidate"`
	Baseline      *artifact.Snapshot      `json:"baseline,omitempty"`
	ABI           *artifact.ABIComparison `json:"abi,omitempty"`
	Commands      []Command               `json:"commands,omitempty"`
	Checks        []Check                 `json:"checks"`
	Limitations   []string                `json:"limitations,omitempty"`
}

type Options struct {
	Context      context.Context
	ProjectPath  string
	LibraryPath  string
	Variant      string
	CandidateAAR string
	BaselineAAR  string
	GradleArgs   []string
	Timeout      time.Duration
	ToolVersion  string
}
