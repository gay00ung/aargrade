package host

import "encoding/json"

const (
	PlanSchemaVersion  = 1
	StateSchemaVersion = 1
	defaultModulePath  = ":aargrade-upgrade-host"
	stateRelativePath  = ".aargrade/state/upgrade-host.json"
	hostRelativeDir    = ".aargrade/upgrade-host"
	markerStart        = "// aargrade:upgrade-host:start v1"
	markerEnd          = "// aargrade:upgrade-host:end v1"
)

type Options struct {
	ProjectPath string
	LibraryPath string
	ModulePath  string
	AGPVersion  string
	CompileSDK  int
	MinSDK      int
	Apply       bool
}

type Change struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Preview string `json:"preview,omitempty"`

	before []byte
	after  []byte
	mode   uint32
}

type Plan struct {
	SchemaVersion int      `json:"schemaVersion"`
	Operation     string   `json:"operation"`
	ProjectRoot   string   `json:"projectRoot"`
	LibraryModule string   `json:"libraryModule,omitempty"`
	HostModule    string   `json:"hostModule"`
	AGPVersion    string   `json:"agpVersion,omitempty"`
	CompileSDK    int      `json:"compileSdk,omitempty"`
	MinSDK        int      `json:"minSdk,omitempty"`
	Applied       bool     `json:"applied"`
	Changes       []Change `json:"changes"`

	settingsPath       string
	settingsBeforeHash string
	statePath          string
	state              State
}

type OwnedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type State struct {
	SchemaVersion      int         `json:"schemaVersion"`
	HostModule         string      `json:"hostModule"`
	LibraryModule      string      `json:"libraryModule"`
	AGPVersion         string      `json:"agpVersion"`
	SettingsFile       string      `json:"settingsFile"`
	SettingsInsertion  string      `json:"settingsInsertion"`
	GeneratedFiles     []OwnedFile `json:"generatedFiles"`
	CreatedDirectories []string    `json:"createdDirectories"`
}

func marshalState(state State) ([]byte, error) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
