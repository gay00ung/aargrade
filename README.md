# AARGrade

> Upgrade your AAR without breaking consumers.

AARGrade is an evidence-first CLI for Android library maintainers. It is being
built to diagnose Android Gradle Plugin (AGP) migration risks, create a
reversible temporary application host when one is genuinely needed, and later
prove a candidate AAR against real Java and Kotlin consumer builds.

The project is intentionally CLI-first. CI integrations, a Gradle plugin, and
an MCP server will reuse the same core instead of becoming separate sources of
truth.

## Current scope

AARGrade is in its first implementation milestone:

- `aargrade doctor`: static, read-only Gradle project diagnosis.
- `aargrade host add/remove`: owned, preview-first temporary host lifecycle.
- Stable text and JSON output for humans, CI, and future MCP clients.

`plan`, `verify`, `matrix`, and `mcp serve` are roadmap commands and are not
claimed as implemented yet.

## Build from source

```bash
go test ./...
go build ./cmd/aargrade
./aargrade doctor --project /path/to/android-library
./aargrade host add --project /path/to/android-library
```

See [the CLI contract](docs/cli.md), [product plan](docs/product/plan.md),
[validation log](docs/product/validation.md), and
[Upgrade Assistant experiment](docs/product/upgrade-assistant-experiment.md)
before relying on roadmap features. The deliberately unsafe Consumer R8
fixture is documented in [the R8 configuration analysis](R8_Configuration_Analysis.md).
Architecture decisions start with [the CLI runtime ADR](docs/adr/0001-cli-runtime.md).

## Status

The name **AARGrade** is provisional until trademark review is complete. The
technical namespace checks performed on 2026-08-13 are recorded in the
validation log.
