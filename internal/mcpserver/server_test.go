package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gay00ung/aargrade/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerListsToolsAndRunsDoctor(t *testing.T) {
	ctx := context.Background()
	server := New("test")
	client := mcp.NewClient(&mcp.Implementation{Name: "aargrade-test", Version: "test"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("tool %s is missing an inferred schema", tool.Name)
		}
	}
	sort.Strings(names)
	want := []string{
		"aargrade_doctor",
		"aargrade_host_add",
		"aargrade_host_remove",
		"aargrade_matrix",
		"aargrade_plan",
		"aargrade_verify",
	}
	if !equalStrings(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}

	project, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "kotlin-library"))
	if err != nil {
		t.Fatal(err)
	}
	called, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "aargrade_doctor",
		Arguments: DoctorInput{ProjectPath: project},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called.IsError {
		t.Fatalf("doctor returned a tool error: %v", called.GetError())
	}
	encoded, err := json.Marshal(called.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var report model.Report
	if err := json.Unmarshal(encoded, &report); err != nil {
		t.Fatal(err)
	}
	if report.ToolVersion != "test" || report.ProjectRoot != project || len(report.Findings) == 0 {
		t.Fatalf("unexpected doctor report: %#v", report)
	}
}

func TestInvalidMCPInputsBecomeToolErrors(t *testing.T) {
	ctx := context.Background()
	server := New("test")
	client := mcp.NewClient(&mcp.Implementation{Name: "aargrade-test", Version: "test"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	called, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "aargrade_verify",
		Arguments: VerifyInput{CandidateAAR: "missing.aar", TimeoutSeconds: -1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called.IsError {
		t.Fatal("negative timeout should be returned as a tool error")
	}
}

func TestStdioServerNegotiatesAndListsTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.Command(os.Args[0], "-test.run=^TestMCPStdioHelper$")
	command.Env = append(os.Environ(), "AARGRADE_MCP_TEST_HELPER=1")
	client := mcp.NewClient(&mcp.Implementation{Name: "aargrade-stdio-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		t.Fatal(err)
	}
	if len(listed.Tools) != 6 {
		_ = session.Close()
		t.Fatalf("stdio tool count = %d, want 6", len(listed.Tools))
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMCPStdioHelper(t *testing.T) {
	if os.Getenv("AARGRADE_MCP_TEST_HELPER") != "1" {
		return
	}
	if err := ServeStdio(context.Background(), "test"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
