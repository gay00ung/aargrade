# AARGrade

[한국어](README.md) | English

[![CI](https://github.com/gay00ung/aargrade/actions/workflows/ci.yml/badge.svg)](https://github.com/gay00ung/aargrade/actions/workflows/ci.yml)

> Upgrade your AAR without breaking consumers.

AARGrade is an AGP migration safety net for Android library and SDK
maintainers. It turns a migration into reviewable evidence: diagnose the
project, plan the toolchain change, inspect the candidate AAR, and build that
artifact in isolated old and current Java/Kotlin consumers.

```text
diagnose → plan → build/compare AAR → build consumer matrix → report to people, CI, or MCP
```

AARGrade does not rewrite an arbitrary Gradle project or claim universal
compatibility. `plan` describes the work; `verify` and `matrix` test its result.
Only `host` edits project files, and it is preview-only unless `--apply` is
explicitly supplied.

## Implemented commands

| Command | Outcome | Writes |
| --- | --- | --- |
| `doctor` | Read-only project and AGP migration diagnosis | None |
| `plan` | Ordered target-AGP, Gradle, and JDK migration plan | None |
| `verify` | Build/inspect an AAR and compare it with a baseline | May create normal Gradle build outputs |
| `matrix` | Build isolated Java/Kotlin consumer applications | Evidence under `.aargrade/matrix` |
| `host add/remove` | Safely preview, add, or remove an owned temporary app | Only with `--apply` |
| `mcp serve` | Expose the same domain operations over MCP stdio | Depends on the called tool |

The built-in verifier reads the public/protected JVM linkage surface. It does
not prove complete Java/Kotlin source compatibility or native runtime behavior;
the real consumer builds are intentionally a separate final layer.

## Try it in one minute

The read-only demo needs Git and Go 1.25 or newer, but not Android Studio or the
Android SDK.

```bash
git clone https://github.com/gay00ung/aargrade.git
cd aargrade
make demo
```

It runs `doctor`, creates an AGP 9.3 `plan`, and previews `host add` against the
public [`examples/library-only`](examples/library-only/README.en.md) project.
It does not change the example.

## Install

```bash
go install github.com/gay00ung/aargrade/cmd/aargrade@latest
export PATH="$(go env GOPATH)/bin:$PATH"
aargrade version
```

There is no tagged release yet. Pin a commit if adopting the development
version in CI. To build a checkout instead, run `make build` and use
`./bin/aargrade`.

## End-to-end workflow

Diagnose the Gradle root without running Gradle or changing files:

```bash
aargrade doctor --project .
```

Create an ordered, version-aware plan. Unknown AGP lines fail closed instead of
being guessed:

```bash
aargrade plan --project . --target-agp 9.3.0
```

Compare the released baseline with a candidate AAR:

```bash
aargrade verify \
  --baseline-aar releases/sdk-1.4.0.aar \
  --candidate-aar sdk/build/outputs/aar/sdk-release.aar
```

When `--candidate-aar` is omitted, AARGrade discovers the library and runs the
project wrapper through `help`, `build --dry-run`, and variant assembly:

```bash
aargrade verify \
  --project . \
  --library :sdk \
  --baseline-aar releases/sdk-1.4.0.aar
```

`verify` checks bounded AAR ZIP structure, metadata requirement increases,
public/protected class linkage and sealed hierarchy, unsafe Consumer R8
options, and packaged JNI/Prefab libraries. Without a baseline, comparison checks are explicitly `SKIPPED` and
the verdict is `EVIDENCE`, not a compatibility pass.

Declare actual consumer toolchains in `aargrade.yml`:

```yaml
schemaVersion: 1
candidateAar: sdk/build/outputs/aar/sdk-release.aar
baselineAar: releases/sdk-1.4.0.aar

cells:
  - name: legacy-java
    agp: 4.2.2
    gradle: 6.7.1
    jdk: 11
    compileSdk: 30
    minSdk: 21
    language: java

  - name: current-kotlin
    agp: 9.2.0
    gradle: 9.4.1
    jdk: 17
    compileSdk: 35
    minSdk: 21
    language: kotlin
```

Then provision each JDK and run the matrix:

```bash
export AARGRADE_JAVA_HOME_11=/path/to/jdk-11
export AARGRADE_JAVA_HOME_17=/path/to/jdk-17
aargrade matrix --config aargrade.yml --allow-downloads
```

`--allow-downloads` applies only to checksum-verified Gradle distributions.
JDKs and Android SDK platforms must already exist. To keep the run completely
offline, repeat `--gradle-bin VERSION=/path/to/gradle` for each version.

Every cell generates an isolated, minified release consumer project. With a
baseline present, the result distinguishes `REGRESSION` (baseline passes,
candidate fails), `IMPROVED`, `UNSUPPORTED` (both fail), and `PASS`. Generated
projects, exact commands, checksums, sanitized logs, and durations are retained
under `.aargrade/matrix/run-*`.

A direct file dependency does not restore transitive dependencies from a Maven
POM. Add the SDK's published runtime dependencies to each cell's `dependencies`
when they are required. Common secret values and local paths are masked on a
best-effort basis, but retained build logs should still be treated as sensitive.

## Temporary Upgrade Assistant host

Use this only after reproducing a problem caused by the lack of an application
model:

```bash
aargrade host add --project .            # preview
aargrade host add --project . --apply    # reviewed apply
./gradlew :aargrade-upgrade-host:tasks --all

aargrade host remove --project .         # preview
aargrade host remove --project . --apply # remove unchanged owned content
```

Removal rechecks the owned settings block and every generated file SHA-256. It
refuses modified or missing owned content and never deletes an existing app.

## MCP

Register this stdio command in any compatible MCP client:

```text
aargrade mcp serve
```

A generic client configuration looks like this; use the exact key names from
your client's documentation:

```json
{
  "mcpServers": {
    "aargrade": {
      "command": "aargrade",
      "args": ["mcp", "serve"]
    }
  }
}
```

The server exposes `aargrade_doctor`, `aargrade_plan`, `aargrade_verify`,
`aargrade_matrix`, `aargrade_host_add`, and `aargrade_host_remove` with inferred
input/output schemas and structured results. Host `apply` and matrix
`allowDownloads` both default to `false`.

## CI and tests

All evidence commands support `--format json`. See the complete exit-code and
flag contract in [docs/cli.md](docs/cli.md).

```bash
make test
make vet
make test-race
make vuln
make check
make example-build
make example-verify
```

User-facing flows live in [`tests/integration`](tests/integration); focused Go
unit tests are colocated with each `internal/*` package. Android-dependent
consumer builds run in the CI `consumer-matrix` job.

CI runs Go tests on Linux, macOS, and Windows, builds the public example AAR,
then assembles it in four consumers: AGP 4.2.2/9.2.0 × Java/Kotlin with explicit
Gradle and JDK provisioning. This proves the declared cells, not every customer
project or device runtime.

## Design and validation

- [CLI contract](docs/cli.md)
- [Product and delivery plan](docs/product/plan.md)
- [Validation evidence](docs/product/validation.md)
- [Upgrade Assistant A/B protocol](docs/product/upgrade-assistant-experiment.md)
- [Consumer R8 fixture analysis](R8_Configuration_Analysis.md)
- [CLI runtime decision](docs/adr/0001-cli-runtime.md)

The name **AARGrade** remains provisional until trademark review.
