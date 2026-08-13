package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/gay00ung/aargrade/internal/mcpserver"
)

func runMCP(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printMCPUsage(stdout)
		return 0
	}
	if args[0] != "serve" {
		_, _ = fmt.Fprintf(stderr, "aargrade mcp: unknown operation %q\n\n", args[0])
		printMCPUsage(stderr)
		return 2
	}
	flags := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printMCPUsage(flags.Output()) }
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "aargrade mcp serve: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if err := mcpserver.ServeStdio(context.Background(), version); err != nil {
		_, _ = fmt.Fprintf(stderr, "aargrade mcp serve: %s\n", err)
		return 2
	}
	return 0
}

func printMCPUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, `Usage:
  aargrade mcp serve

Serve AARGrade tools over MCP stdio. Do not write human output to stdout while
the server is running; stdout is reserved for newline-delimited MCP messages.`)
}
