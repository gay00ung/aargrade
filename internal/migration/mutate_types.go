package migration

import "github.com/gay00ung/aargrade/internal/toolchain"

const (
	MutationSchemaVersion = 1
	StateSchemaVersion    = 1
	stateRelativePath     = ".aargrade/state/migration.json"
)

type MutationOptions struct {
	ProjectPath        string
	TargetAGP          string
	CurrentAGPOverride string
	ToolVersion        string
	Apply              bool
}

type RollbackOptions struct {
	ProjectPath string
	Apply       bool
}

type FileChange struct {
	Action       string `json:"action"`
	Path         string `json:"path"`
	BeforeSHA256 string `json:"beforeSha256,omitempty"`
	AfterSHA256  string `json:"afterSha256,omitempty"`
	Preview      string `json:"preview,omitempty"`

	before []byte
	after  []byte
	mode   uint32
}

type MutationResult struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Operation     string                   `json:"operation"`
	ToolVersion   string                   `json:"toolVersion,omitempty"`
	ProjectRoot   string                   `json:"projectRoot"`
	CurrentAGP    string                   `json:"currentAgp,omitempty"`
	TargetAGP     string                   `json:"targetAgp,omitempty"`
	Toolchain     *toolchain.Compatibility `json:"toolchain,omitempty"`
	Ready         bool                     `json:"ready"`
	Applied       bool                     `json:"applied"`
	StatePath     string                   `json:"statePath,omitempty"`
	Blockers      []string                 `json:"blockers,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	Changes       []FileChange             `json:"changes,omitempty"`
	NextSteps     []string                 `json:"nextSteps,omitempty"`

	statePath string
	stateHash string
	state     migrationState
}

type migrationState struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Status        string               `json:"status"`
	ToolVersion   string               `json:"toolVersion,omitempty"`
	CurrentAGP    string               `json:"currentAgp"`
	TargetAGP     string               `json:"targetAgp"`
	Files         []migrationStateFile `json:"files"`
}

type migrationStateFile struct {
	Path           string `json:"path"`
	Mode           uint32 `json:"mode"`
	BeforeSHA256   string `json:"beforeSha256"`
	AfterSHA256    string `json:"afterSha256"`
	OriginalBase64 string `json:"originalBase64"`
}
