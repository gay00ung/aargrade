package matrix

import (
	"context"
	"time"
)

const SchemaVersion = 1

type Config struct {
	SchemaVersion int          `yaml:"schemaVersion"`
	CandidateAAR  string       `yaml:"candidateAar"`
	BaselineAAR   string       `yaml:"baselineAar,omitempty"`
	Cells         []CellConfig `yaml:"cells"`
}

type CellConfig struct {
	Name             string   `yaml:"name"`
	AGP              string   `yaml:"agp"`
	Gradle           string   `yaml:"gradle"`
	JDK              int      `yaml:"jdk"`
	CompileSDK       int      `yaml:"compileSdk"`
	MinSDK           int      `yaml:"minSdk,omitempty"`
	Language         string   `yaml:"language"`
	Kotlin           string   `yaml:"kotlin,omitempty"`
	JavaHome         string   `yaml:"javaHome,omitempty"`
	JavaHomeEnv      string   `yaml:"javaHomeEnv,omitempty"`
	GradleExecutable string   `yaml:"gradleExecutable,omitempty"`
	GradleArgs       []string `yaml:"gradleArgs,omitempty"`
	Dependencies     []string `yaml:"dependencies,omitempty"`
}

type Execution struct {
	Artifact   string   `json:"artifact"`
	SHA256     string   `json:"sha256"`
	ProjectDir string   `json:"projectDir"`
	ProbeClass string   `json:"probeClass,omitempty"`
	Command    []string `json:"command"`
	ExitCode   int      `json:"exitCode"`
	DurationMS int64    `json:"durationMs"`
	Status     string   `json:"status"`
	Output     string   `json:"output,omitempty"`
}

type CellResult struct {
	Name       string     `json:"name"`
	AGP        string     `json:"agp"`
	Gradle     string     `json:"gradle"`
	JDK        int        `json:"jdk"`
	CompileSDK int        `json:"compileSdk"`
	Language   string     `json:"language"`
	Verdict    string     `json:"verdict"`
	Reason     string     `json:"reason"`
	Baseline   *Execution `json:"baseline,omitempty"`
	Candidate  *Execution `json:"candidate,omitempty"`
}

type Report struct {
	SchemaVersion     int          `json:"schemaVersion"`
	ToolVersion       string       `json:"toolVersion"`
	Verdict           string       `json:"verdict"`
	ConfigPath        string       `json:"configPath"`
	WorkDirectory     string       `json:"workDirectory"`
	CandidateAAR      string       `json:"candidateAar"`
	CandidateSHA256   string       `json:"candidateSha256"`
	BaselineAAR       string       `json:"baselineAar,omitempty"`
	BaselineSHA256    string       `json:"baselineSha256,omitempty"`
	Cells             []CellResult `json:"cells"`
	SelectedCellCount int          `json:"selectedCellCount"`
}

type Options struct {
	Context           context.Context
	ConfigPath        string
	CandidateAAR      string
	BaselineAAR       string
	WorkDirectory     string
	SelectedCells     []string
	AllowDownloads    bool
	KeepGoing         bool
	Timeout           time.Duration
	ToolVersion       string
	JavaHomes         map[int]string
	GradleExecutables map[string]string
}
