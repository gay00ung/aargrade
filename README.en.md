# AARGrade

[한국어](README.md) | English

[![CI](https://github.com/gay00ung/aargrade/actions/workflows/ci.yml/badge.svg)](https://github.com/gay00ung/aargrade/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

> Upgrade your AAR without breaking consumers.

AARGrade is an AGP migration safety net for Android library and SDK
maintainers. It turns a migration into reviewable evidence: diagnose the
project, plan the toolchain change, inspect the candidate AAR, and build that
artifact in isolated old and current Java/Kotlin consumers.

```text
diagnose → preview repairs → migrate → build/inspect AAR → optional consumer matrix → classify failure/roll back
```

Start with `upgrade`. It orchestrates diagnosis, known AGP repairs, migration,
real Gradle/AAR evidence, and an optional consumer matrix. Failed applies roll
back owned changes by default. Use `migrate` when you want to control only the
configuration transaction without running Gradle.

AARGrade does not guess how to rewrite arbitrary Gradle logic or claim universal
compatibility. Automatic repairs cover only statically provable forms from
official AGP migration rules. kapt, legacy APIs, convention-plugin internals,
and other ambiguous structures stop with a reason and suggested next action.
Over MCP, an agent can consume that structured result, make project-specific
changes, and rerun the same evidence loop.

## Implemented commands

| Command | Outcome | Writes |
| --- | --- | --- |
| `doctor` | Read-only project and AGP migration diagnosis | None |
| `plan` | Ordered target-AGP, Gradle, and JDK migration plan | None |
| `upgrade` | Repair, migrate, build, inspect, and optionally test consumers in one flow | Preview writes nothing; `--apply` changes configuration/builds |
| `migrate` | Preview/apply/accept/rollback supported AGP, Wrapper, and Built-in Kotlin edits | Only with `--apply` |
| `verify` | Build/inspect an AAR and compare it with a baseline | May create normal Gradle build outputs |
| `matrix` | Build isolated Java/Kotlin consumer applications | Evidence under `.aargrade/matrix` |
| `host add/remove` | Add/remove an owned app model for Upgrade Assistant | Only with `--apply` |
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

It runs `doctor`, creates an AGP 9.3 `plan`, and previews both `upgrade` and
`host add` against the public
[`examples/library-only`](examples/library-only/README.en.md) project. It does
not change the example.

## Install

Download the archive for your OS and CPU from
[GitHub Releases](https://github.com/gay00ung/aargrade/releases):

| Platform | Asset |
| --- | --- |
| Apple Silicon Mac | `aargrade_<VERSION>_darwin_arm64.tar.gz` |
| Intel Mac | `aargrade_<VERSION>_darwin_amd64.tar.gz` |
| Linux x86-64 | `aargrade_<VERSION>_linux_amd64.tar.gz` |
| Linux ARM64 | `aargrade_<VERSION>_linux_arm64.tar.gz` |
| Windows x86-64 | `aargrade_<VERSION>_windows_amd64.zip` |
| Windows ARM64 | `aargrade_<VERSION>_windows_arm64.zip` |

The first public beta is `v0.1.0-beta.1`. An Apple Silicon installation looks
like:

```bash
AARGRADE_VERSION=v0.1.0-beta.1
AARGRADE_ASSET="aargrade_${AARGRADE_VERSION#v}_darwin_arm64"
curl -fLO "https://github.com/gay00ung/aargrade/releases/download/${AARGRADE_VERSION}/${AARGRADE_ASSET}.tar.gz"
curl -fLO "https://github.com/gay00ung/aargrade/releases/download/${AARGRADE_VERSION}/checksums.txt"
grep " ${AARGRADE_ASSET}.tar.gz$" checksums.txt | shasum -a 256 -c -
tar -xzf "${AARGRADE_ASSET}.tar.gz"
sudo install -m 0755 "${AARGRADE_ASSET}/aargrade" /usr/local/bin/aargrade
aargrade version
```

Each archive also carries the Go toolchain and linked module license notices
under `third_party_licenses`.

With Go 1.25 or newer, install the exact tag instead:

```bash
go install github.com/gay00ung/aargrade/cmd/aargrade@v0.1.0-beta.1
export PATH="$(go env GOPATH)/bin:$PATH"
aargrade version
```

Before that tag exists, or when testing the development checkout, run
`make build VERSION=dev` and use `./bin/aargrade`.

## End-to-end workflow

Preview the recommended end-to-end operation first. It does not write project
files or run Gradle:

```bash
aargrade upgrade --project . --target-agp 9.2.0
```

Then apply the reviewed repairs and produce real evidence:

```bash
aargrade upgrade \
  --project . \
  --target-agp 9.2.0 \
  --library :sdk \
  --baseline-aar releases/sdk-current.aar \
  --apply
```

The baseline is optional. Apply mode updates AGP and the Wrapper, repairs safe
forms of missing namespace plus its legacy manifest package, custom BuildConfig
enablement, legacy SDK setters, AGP 9 Built-in Kotlin, and Java/Kotlin JVM
target alignment. It then runs `help`, `build --dry-run`, classic release or
KMP Android AAR assembly, and AAR inspection. Add
`--matrix-config aargrade.yml` to run declared consumers too. A failure is
classified and rolls the owned configuration back unless
`--keep-failed-changes` is explicitly supplied.

After a pass, either preview/apply exact rollback or accept the migration and
remove its rollback state while leaving the migrated files intact:

```bash
aargrade migrate rollback --project .
aargrade migrate rollback --project . --apply

aargrade migrate accept --project .
aargrade migrate accept --project . --apply
```

The lower-level commands below remain useful when each stage should be run
separately.

Diagnose the Gradle root without running Gradle or changing files:

```bash
aargrade doctor --project .
```

Create an ordered, version-aware plan. Unknown AGP lines fail closed instead of
being guessed:

```bash
aargrade plan --project . --target-agp 9.3.0
```

Preview the bounded edits. This command does not write anything:

```bash
aargrade migrate --project . --target-agp 9.2.0
```

Apply only after reviewing every diff and blocker:

```bash
aargrade migrate --project . --target-agp 9.2.0 --apply
```

The current automatic surface covers literal Kotlin/Groovy AGP declarations,
legacy `buildscript` classpaths, the default version catalog, the minimum
compatible Gradle Wrapper URL plus official distribution checksum, standalone
Kotlin Android plugin declarations for AGP 9 Built-in Kotlin, and obsolete AGP
9 opt-out properties.

It fails closed on missing namespaces, implicit custom BuildConfig fields,
legacy Kotlin kapt unless migrated to `com.android.legacy-kapt`, KSP below
2.3.6, Kotlin compiler/source-set DSL, legacy Variant/internal
APIs, buildSrc, custom settings catalogs, RefreshVersions, dynamic convention
plugins, and catalog version refs shared with non-AGP entries.

An apply stores exact originals and before/after hashes under
`.aargrade/state/migration.json`. Rollback proceeds only while every owned file
still matches one of those hashes:

```bash
aargrade migrate rollback --project .         # preview
aargrade migrate rollback --project . --apply # exact restore
```

Keep `.aargrade/` out of version control. The local state contains original
Gradle configuration and should be handled as potentially sensitive.

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
model. A controlled 2026-08-14 A→B→A experiment reproduced the missing
application-project failure, opened a real AGP 7.4.2 → 8.13.2 plan after host
addition, and restored the failure after removal. That validates the tested
fixture and Studio build, not every IDE version.

```bash
aargrade host add --project .            # preview
aargrade host add --project . --apply    # reviewed apply
./gradlew :aargrade-upgrade-host:tasks --all

aargrade host remove --project .         # preview
aargrade host remove --project . --apply # remove unchanged owned content
```

Removal rechecks the owned settings block and every generated file SHA-256. It
refuses modified or missing owned content and never deletes an existing app.
The runnable fixture and full evidence are in
[`testdata/projects/upgrade-assistant-repro`](testdata/projects/upgrade-assistant-repro)
and the [experiment record](docs/product/upgrade-assistant-experiment.md).

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

The server exposes `aargrade_doctor`, `aargrade_plan`, `aargrade_upgrade`,
`aargrade_migrate`, `aargrade_migrate_accept`,
`aargrade_migrate_rollback`, `aargrade_verify`, `aargrade_matrix`,
`aargrade_host_add`, and `aargrade_host_remove` with inferred input/output
schemas and structured results. Mutation `apply` and `allowDownloads` default
to `false`.

## CI and tests

All evidence commands support `--format json`. See the complete exit-code and
flag contract in [docs/cli.md](docs/cli.md).

```bash
make test
make vet
make test-race
make vuln
make check
make release-check
make example-build
make example-verify
```

User-facing flows live in [`tests/integration`](tests/integration); focused Go
unit tests are colocated with each `internal/*` package. Android-dependent
consumer builds run in the CI `consumer-matrix` job.

Run the reproducible host lifecycle alone with:

```bash
make test-upgrade-assistant-fixture
make test-migration-smoke
make test-upgrade-agent-smoke
make test-external
```

CI runs Go tests on Linux, macOS, and Windows, builds the public example AAR,
then assembles it in four consumers: AGP 4.2.2/9.2.0 × Java/Kotlin with explicit
Gradle and JDK provisioning. This proves the declared cells, not every customer
project or device runtime.

A separate Gradle smoke matrix configures generated hosts on AGP 4.2.2, the
AGP 7.4.2 Upgrade Assistant fixture, and AGP 9.2.0. The Android Studio UI result
remains explicitly manual evidence rather than a headless-build substitute.

The `migration-smoke` job migrates a real Kotlin/version-catalog fixture from
AGP 8.8 to AGP 9.2/Gradle 9.4.1, compiles Kotlin through AGP's Built-in Kotlin,
assembles the release AAR, and then verifies an exact owned-file rollback.
The agent smoke starts from missing namespace, implicit BuildConfig, legacy SDK
setters, and `kotlinOptions`; one `upgrade --apply` repairs them, builds and
inspects a real release AAR, then proves byte-exact rollback.

An opt-in pinned external corpus also validates Timber, Picasso, Lottie, and
Glide. Applied AGP 9 upgrades passed Gradle and AAR verification for Timber and
Glide; Picasso produced a complete preview, while Lottie's RefreshVersions
boundary stopped safely. See the
[Korean evidence record](docs/product/external-validation-2026-08.ko.md).

## Design and validation

- [CLI contract](docs/cli.md)
- [Release guide](docs/releasing.md)
- [릴리스 방법](docs/releasing.ko.md)
- [Product and delivery plan](docs/product/plan.md)
- [Validation evidence](docs/product/validation.md)
- [Pinned external-project evidence (Korean)](docs/product/external-validation-2026-08.ko.md)
- [Upgrade Assistant A/B evidence](docs/product/upgrade-assistant-experiment.md)
- [Upgrade Assistant A/B evidence (Korean)](docs/product/upgrade-assistant-experiment.ko.md)
- [Consumer R8 fixture analysis](R8_Configuration_Analysis.md)
- [CLI runtime decision](docs/adr/0001-cli-runtime.md)
- [Bounded migration transaction decision](docs/adr/0003-bounded-migration-transactions.md)
- [Agent-style upgrade orchestration decision](docs/adr/0004-agent-upgrade-orchestration.md)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

Source and release binaries are provided under the [Apache License 2.0](LICENSE).

The name **AARGrade** remains provisional until trademark review.
