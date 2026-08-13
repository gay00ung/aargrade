# ADR 0001: Use a standalone Go CLI as the product core

- Status: Accepted; amended through Milestone 4
- Date: 2026-08-13

## Context

AARGrade must eventually orchestrate consumer builds whose Gradle versions
require different JDKs. If the product core itself runs inside Gradle or on a
fixed JVM, CLI startup and matrix execution become coupled to the toolchain
under test. The CLI also needs straightforward local, CI, and MCP distribution.

## Decision

Implement the command and domain core in Go. Milestone 1 initially used only
the standard library. Build Gradle integration as subprocess adapters with
explicit JDK and working-directory inputs. Keep Android/Gradle compatibility
rules in versioned data once version-dependent execution is introduced.

The Milestone 2–4 amendment sets Go 1.25 as the source baseline and accepts two
focused dependencies: `go.yaml.in/yaml/v3` for strict matrix configuration and
the official `github.com/modelcontextprotocol/go-sdk` for protocol-compliant
MCP transport and schemas. Project analysis, policy, artifact inspection,
matrix generation, and mutation safety remain owned domain code.

## Consequences

- Release artifacts can be single native binaries for common CI platforms.
- Old consumer builds can run with a per-cell JDK independent of the CLI.
- Static Gradle analysis must be conservative; AARGrade will not pretend that
  regex or lightweight parsing can evaluate arbitrary build logic.
- JVM-native tools such as Metalava will be invoked as declared external
  engines rather than embedded into the CLI process.
- The implemented MCP server is a thin adapter over shared domain operations;
  a future Gradle plugin must keep the same boundary.

## Revisit when

- Reliable analysis requires the Gradle Tooling API for the majority of useful
  findings, or
- distributing native binaries proves materially harder than a JVM tool for
  the target users.
