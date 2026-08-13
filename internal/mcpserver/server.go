package mcpserver

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gay00ung/aargrade/internal/doctor"
	"github.com/gay00ung/aargrade/internal/host"
	consumer "github.com/gay00ung/aargrade/internal/matrix"
	"github.com/gay00ung/aargrade/internal/migration"
	"github.com/gay00ung/aargrade/internal/model"
	verification "github.com/gay00ung/aargrade/internal/verify"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "aargrade"

var (
	boolTrue  = true
	boolFalse = false
)

type DoctorInput struct {
	ProjectPath string `json:"projectPath,omitempty" jsonschema:"Gradle project root. Defaults to the current directory."`
}

type PlanInput struct {
	ProjectPath string `json:"projectPath,omitempty" jsonschema:"Gradle project root. Defaults to the current directory."`
	TargetAGP   string `json:"targetAgp" jsonschema:"Target Android Gradle Plugin version, for example 9.3.0."`
	CurrentAGP  string `json:"currentAgp,omitempty" jsonschema:"Verified current AGP override. Leave empty to discover it from the project."`
}

type VerifyInput struct {
	ProjectPath    string   `json:"projectPath,omitempty" jsonschema:"Gradle project root used when candidateAar is omitted."`
	LibraryPath    string   `json:"libraryPath,omitempty" jsonschema:"Android library Gradle path, for example :sdk."`
	Variant        string   `json:"variant,omitempty" jsonschema:"Android build variant. Defaults to release."`
	CandidateAAR   string   `json:"candidateAar,omitempty" jsonschema:"Existing candidate AAR. When omitted AARGrade builds the project."`
	BaselineAAR    string   `json:"baselineAar,omitempty" jsonschema:"Released baseline AAR to compare against."`
	GradleArgs     []string `json:"gradleArgs,omitempty" jsonschema:"Extra arguments appended to each Gradle command."`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" jsonschema:"Timeout for each Gradle command in seconds. Defaults to 600."`
}

type MatrixInput struct {
	ConfigPath        string            `json:"configPath,omitempty" jsonschema:"Matrix YAML path. Defaults to aargrade.yml."`
	CandidateAAR      string            `json:"candidateAar,omitempty" jsonschema:"Candidate AAR override."`
	BaselineAAR       string            `json:"baselineAar,omitempty" jsonschema:"Baseline AAR override."`
	WorkDirectory     string            `json:"workDirectory,omitempty" jsonschema:"Owned directory for generated consumer projects and evidence."`
	SelectedCells     []string          `json:"selectedCells,omitempty" jsonschema:"Only run these named matrix cells."`
	AllowDownloads    bool              `json:"allowDownloads,omitempty" jsonschema:"Allow checksum-verified Gradle downloads. Defaults to false."`
	FailFast          bool              `json:"failFast,omitempty" jsonschema:"Stop after the first failed or regressed cell."`
	TimeoutSeconds    int               `json:"timeoutSeconds,omitempty" jsonschema:"Timeout for each consumer build in seconds. Defaults to 900."`
	JavaHomes         map[string]string `json:"javaHomes,omitempty" jsonschema:"JDK major to JAVA_HOME mapping, for example {\"17\":\"/path/to/jdk\"}."`
	GradleExecutables map[string]string `json:"gradleExecutables,omitempty" jsonschema:"Gradle version to executable mapping."`
}

type HostAddInput struct {
	ProjectPath string `json:"projectPath,omitempty" jsonschema:"Gradle project root. Defaults to the current directory."`
	LibraryPath string `json:"libraryPath,omitempty" jsonschema:"Android library Gradle path when selection is ambiguous."`
	ModulePath  string `json:"modulePath,omitempty" jsonschema:"Temporary host Gradle path. Defaults to :aargrade-upgrade-host."`
	AGPVersion  string `json:"agpVersion,omitempty" jsonschema:"Literal current AGP version override."`
	CompileSDK  int    `json:"compileSdk,omitempty" jsonschema:"Literal compile SDK override."`
	MinSDK      int    `json:"minSdk,omitempty" jsonschema:"Minimum SDK override."`
	Apply       bool   `json:"apply,omitempty" jsonschema:"Write the previewed owned changes. Defaults to false."`
}

type HostRemoveInput struct {
	ProjectPath string `json:"projectPath,omitempty" jsonschema:"Gradle project root. Defaults to the current directory."`
	Apply       bool   `json:"apply,omitempty" jsonschema:"Remove only unchanged AARGrade-owned content. Defaults to false."`
}

// New returns a fully configured MCP server. The server exposes the same
// domain operations as the CLI rather than spawning the CLI as a subprocess.
func New(version string) *mcp.Server {
	if version == "" {
		version = "devel"
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:        serverName,
		Title:       "AARGrade",
		Description: "Android library AGP migration and AAR consumer compatibility evidence",
		Version:     version,
		WebsiteURL:  "https://github.com/gay00ung/aargrade",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "aargrade_doctor",
		Title:       "Diagnose Android project",
		Description: "Read-only static diagnosis of an Android Gradle project before an AGP migration.",
		Annotations: readOnlyAnnotations("Diagnose Android project"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input DoctorInput) (*mcp.CallToolResult, model.Report, error) {
		projectPath := defaultString(input.ProjectPath, ".")
		report, err := doctor.Analyze(projectPath, version)
		return nil, report, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "aargrade_plan",
		Title:       "Plan AGP migration",
		Description: "Create a read-only ordered migration plan using versioned AGP, Gradle, and JDK compatibility policy.",
		Annotations: readOnlyAnnotations("Plan AGP migration"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input PlanInput) (*mcp.CallToolResult, migration.Plan, error) {
		plan, err := migration.Create(migration.Options{
			ProjectPath:        defaultString(input.ProjectPath, "."),
			TargetAGP:          input.TargetAGP,
			CurrentAGPOverride: input.CurrentAGP,
			ToolVersion:        version,
		})
		return nil, plan, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "aargrade_verify",
		Title:       "Verify AAR",
		Description: "Build or inspect a candidate AAR and compare JVM linkage, metadata, Consumer R8 rules, and native packaging with a baseline.",
		Annotations: additiveAnnotations("Verify AAR", true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input VerifyInput) (*mcp.CallToolResult, verification.Report, error) {
		timeout, err := timeoutDuration(input.TimeoutSeconds, 10*time.Minute)
		if err != nil {
			return nil, verification.Report{}, err
		}
		report, err := verification.Run(verification.Options{
			Context:      ctx,
			ProjectPath:  defaultString(input.ProjectPath, "."),
			LibraryPath:  input.LibraryPath,
			Variant:      defaultString(input.Variant, "release"),
			CandidateAAR: input.CandidateAAR,
			BaselineAAR:  input.BaselineAAR,
			GradleArgs:   input.GradleArgs,
			Timeout:      timeout,
			ToolVersion:  version,
		})
		return nil, report, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "aargrade_matrix",
		Title:       "Run consumer matrix",
		Description: "Generate isolated Java/Kotlin consumer projects and build the candidate and optional baseline AAR across configured toolchains.",
		Annotations: additiveAnnotations("Run consumer matrix", true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input MatrixInput) (*mcp.CallToolResult, consumer.Report, error) {
		timeout, err := timeoutDuration(input.TimeoutSeconds, 15*time.Minute)
		if err != nil {
			return nil, consumer.Report{}, err
		}
		javaHomes, err := parseJavaHomes(input.JavaHomes)
		if err != nil {
			return nil, consumer.Report{}, err
		}
		report, err := consumer.Run(consumer.Options{
			Context:           ctx,
			ConfigPath:        defaultString(input.ConfigPath, "aargrade.yml"),
			CandidateAAR:      input.CandidateAAR,
			BaselineAAR:       input.BaselineAAR,
			WorkDirectory:     input.WorkDirectory,
			SelectedCells:     input.SelectedCells,
			AllowDownloads:    input.AllowDownloads,
			KeepGoing:         !input.FailFast,
			Timeout:           timeout,
			ToolVersion:       version,
			JavaHomes:         javaHomes,
			GradleExecutables: input.GradleExecutables,
		})
		return nil, report, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "aargrade_host_add",
		Title:       "Preview or add temporary host",
		Description: "Preview an owned temporary Android application host. Set apply=true only after reviewing the proposed changes.",
		Annotations: additiveAnnotations("Preview or add temporary host", false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input HostAddInput) (*mcp.CallToolResult, host.Plan, error) {
		plan, err := host.Add(host.Options{
			ProjectPath: defaultString(input.ProjectPath, "."),
			LibraryPath: input.LibraryPath,
			ModulePath:  input.ModulePath,
			AGPVersion:  input.AGPVersion,
			CompileSDK:  input.CompileSDK,
			MinSDK:      input.MinSDK,
			Apply:       input.Apply,
		})
		return nil, plan, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "aargrade_host_remove",
		Title:       "Preview or remove temporary host",
		Description: "Preview removal of unchanged AARGrade-owned host content. Set apply=true only after reviewing the plan.",
		Annotations: destructiveAnnotations("Preview or remove temporary host"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input HostRemoveInput) (*mcp.CallToolResult, host.Plan, error) {
		plan, err := host.Remove(host.Options{
			ProjectPath: defaultString(input.ProjectPath, "."),
			Apply:       input.Apply,
		})
		return nil, plan, err
	})

	return server
}

// ServeStdio serves one MCP session over stdin/stdout. Protocol data is the
// only output written to stdout.
func ServeStdio(ctx context.Context, version string) error {
	return New(version).Run(ctx, &mcp.StdioTransport{})
}

func readOnlyAnnotations(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:         title,
		ReadOnlyHint:  true,
		OpenWorldHint: &boolFalse,
	}
}

func additiveAnnotations(title string, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:           title,
		DestructiveHint: &boolFalse,
		OpenWorldHint:   boolPointer(openWorld),
	}
}

func destructiveAnnotations(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:           title,
		DestructiveHint: &boolTrue,
		OpenWorldHint:   &boolFalse,
	}
}

func boolPointer(value bool) *bool {
	if value {
		return &boolTrue
	}
	return &boolFalse
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func timeoutDuration(seconds int, fallback time.Duration) (time.Duration, error) {
	if seconds == 0 {
		return fallback, nil
	}
	if seconds < 0 {
		return 0, fmt.Errorf("timeoutSeconds must be positive")
	}
	const maxDurationSeconds = int64((1<<63 - 1) / int64(time.Second))
	if int64(seconds) > maxDurationSeconds {
		return 0, fmt.Errorf("timeoutSeconds is too large")
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseJavaHomes(values map[string]string) (map[int]string, error) {
	result := make(map[int]string, len(values))
	for major, path := range values {
		parsed, err := strconv.Atoi(major)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("invalid JDK major %q in javaHomes", major)
		}
		result[parsed] = path
	}
	return result, nil
}
