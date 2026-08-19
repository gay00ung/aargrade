package upgrade

import (
	"context"
	"time"

	consumer "github.com/gay00ung/aargrade/internal/matrix"
	"github.com/gay00ung/aargrade/internal/migration"
	verification "github.com/gay00ung/aargrade/internal/verify"
)

const SchemaVersion = 1

type FailureAnalysis struct {
	Category        string `json:"category"`
	Summary         string `json:"summary"`
	SuggestedAction string `json:"suggestedAction"`
	Command         string `json:"command,omitempty"`
}

type Report struct {
	SchemaVersion       int                       `json:"schemaVersion"`
	ToolVersion         string                    `json:"toolVersion,omitempty"`
	ProjectRoot         string                    `json:"projectRoot,omitempty"`
	TargetAGP           string                    `json:"targetAgp"`
	Verdict             string                    `json:"verdict"`
	Applied             bool                      `json:"applied"`
	RolledBack          bool                      `json:"rolledBack"`
	Migration           migration.MutationResult  `json:"migration"`
	BeforeUpgradeDryRun *verification.Command     `json:"beforeUpgradeDryRun,omitempty"`
	Verification        *verification.Report      `json:"verification,omitempty"`
	Matrix              *consumer.Report          `json:"matrix,omitempty"`
	Failure             *FailureAnalysis          `json:"failure,omitempty"`
	Rollback            *migration.MutationResult `json:"rollback,omitempty"`
	Limitations         []string                  `json:"limitations,omitempty"`
}

type Options struct {
	Context             context.Context
	ProjectPath         string
	TargetAGP           string
	CurrentAGPOverride  string
	LibraryPath         string
	Variant             string
	BaselineAAR         string
	MatrixConfig        string
	MatrixWorkDirectory string
	SelectedCells       []string
	Apply               bool
	RollbackOnFailure   bool
	AllowDownloads      bool
	KeepGoing           bool
	GradleArgs          []string
	Timeout             time.Duration
	ToolVersion         string
	JavaHomes           map[int]string
	GradleExecutables   map[string]string
}
