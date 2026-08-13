package migration

import (
	"github.com/gay00ung/aargrade/internal/model"
	"github.com/gay00ung/aargrade/internal/toolchain"
)

const SchemaVersion = 1

type StepKind string

const (
	StepRequired StepKind = "required"
	StepReview   StepKind = "review"
	StepOptional StepKind = "optional"
)

type Step struct {
	ID       string           `json:"id"`
	Order    int              `json:"order"`
	Kind     StepKind         `json:"kind"`
	Title    string           `json:"title"`
	Why      string           `json:"why"`
	Action   string           `json:"action"`
	Evidence []model.Evidence `json:"evidence,omitempty"`
}

type Plan struct {
	SchemaVersion int                     `json:"schemaVersion"`
	ToolVersion   string                  `json:"toolVersion"`
	ProjectRoot   string                  `json:"projectRoot"`
	CurrentAGP    string                  `json:"currentAgp,omitempty"`
	TargetAGP     string                  `json:"targetAgp"`
	Toolchain     toolchain.Compatibility `json:"toolchain"`
	Ready         bool                    `json:"ready"`
	Blockers      []string                `json:"blockers,omitempty"`
	Steps         []Step                  `json:"steps"`
}

type Options struct {
	ProjectPath        string
	TargetAGP          string
	CurrentAGPOverride string
	ToolVersion        string
}
