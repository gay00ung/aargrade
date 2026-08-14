# CLI contract

All paths are resolved by the command that consumes them and reports use
absolute evidence paths. Human-readable text is the default; `--format json`
emits a schema-versioned structured result. Exit code `2` always means the
requested verdict could not be produced reliably.

## `version`

```bash
aargrade version
# aargrade v0.1.0-beta.1
```

Tagged release binaries receive the exact tag at link time. A binary installed
with `go install ...@VERSION` uses the Go module version, while an untagged
local build reports `dev` unless `make build VERSION=...` is used.

## `doctor`

```bash
aargrade doctor \
  --project /path/to/project \
  --format text \
  --fail-on error
```

Options:

- `--project`: Gradle root containing one settings file. Default: `.`.
- `--format`: `text` or `json`.
- `--fail-on`: `info`, `warn`, `error`, or `never`. Default: `error`.

Exit codes:

- `0`: trustworthy report; no finding met the configured threshold.
- `1`: trustworthy report; at least one finding met the threshold.
- `2`: invalid input, failed discovery/analysis, or report write failure.

`doctor` never invokes Gradle and never writes to the target project. Dynamic
build logic that cannot be resolved statically is reported as uncertainty
instead of being guessed.

## `plan`

```bash
aargrade plan \
  --project /path/to/project \
  --target-agp 9.3.0 \
  --format text
```

Options:

- `--project`: Gradle root. Default: `.`.
- `--target-agp`: required target AGP version.
- `--current-agp`: reviewed current-version override when static discovery is
  unresolved.
- `--format`: `text` or `json`.

The plan combines `doctor` findings with versioned AGP/Gradle/JDK policy. It
contains blockers and ordered `required`, `review`, and `optional` steps. It is
read-only; it does not edit plugin declarations or the Gradle Wrapper.

The current compatibility policy covers AGP 4.2, 7.0–7.4, 8.0–8.13, and
9.0–9.3 lines. Unknown lines fail closed until their official toolchain policy
is added and tested.

Exit codes:

- `0`: the plan has no known blocker.
- `1`: a trustworthy plan was produced with blockers.
- `2`: invalid input, unsupported policy line, failed analysis, or output
  failure.

## `verify`

Inspect existing artifacts:

```bash
aargrade verify \
  --baseline-aar /path/to/released.aar \
  --candidate-aar /path/to/candidate.aar
```

Or build the candidate first:

```bash
aargrade verify \
  --project /path/to/project \
  --library :sdk \
  --variant release \
  --baseline-aar /path/to/released.aar
```

Options:

- `--project`: Gradle root used when `--candidate-aar` is omitted. Default: `.`.
- `--library`: Android library Gradle path. Required only when discovery is
  ambiguous.
- `--variant`: assembled library variant. Default: `release`.
- `--candidate-aar`: existing candidate AAR; skips project build commands.
- `--baseline-aar`: released AAR used for comparison.
- `--gradle-arg`: extra argument appended to every Gradle invocation; repeatable.
- `--timeout`: timeout for each Gradle command. Default: `10m`.
- `--format`: `text` or `json`.

Without an existing candidate, the project wrapper runs these commands in
order and stops on the first failure:

1. `help --no-daemon`
2. `build --dry-run --no-daemon`
3. `<library>:assemble<Variant> --no-daemon`

The artifact inspector rejects symlinked/non-regular AARs, unsafe or duplicate
ZIP paths, and bounded-size violations. It records entry and artifact SHA-256,
AAR metadata, public/protected JVM classes and members, Consumer R8 issues, and
packaged `jni/<abi>/*.so` and Prefab native-library files.

With a baseline it detects removed linkage elements, access narrowing,
superclass/interface and sealed-hierarchy changes, static/abstract/final semantic changes, increased
`minCompileSdk`, `minCompileSdkExtension`, or minimum AGP, removed JNI
libraries, and unsafe Consumer R8 global options. Generic signature, final,
constant-value, broad keep-rule, and changed native-binary evidence is rendered
as a warning where the built-in engine cannot prove breakage. Changing the
declared exceptions of a method is also a source-level warning, not a JVM
linkage failure.

Limitations are part of every report: class-file linkage is not complete
Java/Kotlin source compatibility, annotation semantics are not compared, and
JNI symbols/runtime behavior are not executed. A missing baseline causes the
comparison checks to be `skipped` and the verdict to be `evidence`.

Exit codes:

- `0`: evidence was produced and no check failed (`pass` or `evidence`).
- `1`: at least one verification check failed.
- `2`: invalid input, build/discovery error that prevented a report, unsafe or
  unreadable artifact, or output failure.

## `matrix`

```bash
aargrade matrix \
  --config /path/to/aargrade.yml \
  --allow-downloads
```

Options:

- `--config`: strict YAML configuration. Default: `aargrade.yml`.
- `--candidate-aar`: override the config candidate path.
- `--baseline-aar`: override the config baseline path.
- `--cell`: run only one named cell; repeatable.
- `--work-dir`: owned evidence root. Default: `.aargrade/matrix` beside the
  config.
- `--allow-downloads`: allow official Gradle distribution and checksum
  downloads. Default: false.
- `--fail-fast`: stop after the first failed/regressed cell. Default: false.
- `--timeout`: timeout per consumer build. Default: `15m`.
- `--java-home MAJOR=PATH`: JDK override; repeatable.
- `--gradle-bin VERSION=EXECUTABLE`: Gradle override; repeatable.
- `--format`: `text` or `json`.

Configuration schema version 1:

```yaml
schemaVersion: 1
candidateAar: artifacts/candidate.aar
baselineAar: artifacts/baseline.aar # optional

cells:
  - name: legacy-kotlin
    agp: 4.2.2
    gradle: 6.7.1
    jdk: 11
    compileSdk: 30
    minSdk: 21
    language: kotlin
    kotlin: 1.6.21
    javaHome: /optional/direct/path
    javaHomeEnv: OPTIONAL_ENV_NAME
    gradleExecutable: /optional/path/to/gradle
    gradleArgs: ["--offline"]
    dependencies: ["com.example:required-runtime:1.0.0"]
```

Candidate and baseline paths are resolved relative to the config file. Unknown
YAML fields, multiple YAML documents, duplicate/unsafe cell names, incompatible
AGP/Gradle/JDK tuples, unknown selected cells, and missing Kotlin versions for
pre-AGP-9 Kotlin cells are rejected.

JDK resolution order:

1. `--java-home`
2. cell `javaHome`
3. environment named by cell `javaHomeEnv`
4. `AARGRADE_JAVA_HOME_<MAJOR>`
5. `JAVA_HOME`

The selected `java -version` must report the declared major version.

Gradle resolution order:

1. `--gradle-bin`
2. cell `gradleExecutable`
3. `AARGRADE_GRADLE_<VERSION_WITH_UNDERSCORES>`
4. AARGrade user cache
5. official download, only with `--allow-downloads`

Downloaded archives are paired with the official `.sha256`, checked before
bounded and path-safe extraction, and cached outside the project. The resolved
`gradle --version` must exactly match the cell.

Each artifact/cell pair gets an isolated Android application. It uses the AAR
as a direct file dependency, references one public top-level class when
available, and assembles a minified/resource-shrunk release build. The evidence
contains checksums, generated paths, exact command, exit status, bounded and
sanitized log, and duration.

Because a direct file AAR has no Maven POM, transitive dependencies are not
resolved automatically. Declare the SDK's required published dependencies in
the cell `dependencies` list. Sanitization masks common secret-shaped arguments,
`key=value` output, evidence paths, and the user home on a best-effort basis;
retained logs must still be handled as potentially sensitive evidence.

Cell verdicts:

- `pass`: candidate builds (and baseline also builds when present).
- `regression`: baseline builds but candidate fails.
- `improved`: baseline fails but candidate builds.
- `unsupported`: baseline and candidate both fail.
- `fail`: candidate fails when no baseline exists.
- `incomplete`: a required JDK/Gradle/build could not be executed.

Exit codes:

- `0`: overall verdict `pass`.
- `1`: overall verdict `fail` due to candidate failure or regression.
- `2`: incomplete/unsupported matrix or a configuration/runtime/output error.

The matrix does not install JDKs or Android SDK platforms. A generated compile
probe is deliberately small, so teams should add representative sample apps or
tests when runtime behavior and a larger source surface matter.

## `host add`

Use this workaround only after reproducing an Upgrade Assistant failure caused
by the absence of an application Android model. The controlled AGP 7.4.2
fixture changed from that exact failure to a usable migration plan after host
addition; see the [bounded A/B evidence](product/upgrade-assistant-experiment.md).
Other Studio versions or Gradle structures can still behave differently.

```bash
# Preview
aargrade host add --project /path/to/library

# Apply the reviewed plan
aargrade host add --project /path/to/library --apply
```

Options include `--library`, `--module`, `--agp-version`, `--compile-sdk`,
`--min-sdk`, `--apply`, and `--format`. The command refuses an existing
application module, ambiguous library selection, unresolved literal AGP or
compile SDK, and any path it would overwrite.

The generated module is mapped to `.aargrade/upgrade-host`; its ownership state
is stored at `.aargrade/state/upgrade-host.json`. Preview is the default and
does not create either path.

Exit codes: `0` for a preview/applied plan, `2` for refusal or input/output
failure.

## `host remove`

```bash
# Preview
aargrade host remove --project /path/to/library

# Remove unchanged owned content
aargrade host remove --project /path/to/library --apply
```

Removal verifies the exact owned settings insertion and SHA-256 of every
generated file, then rechecks immediately before applying. It fails closed on
modification, missing content, invalid ownership state, or symlinks. Settings
edits outside the bounded block are preserved.

Exit codes: `0` for a preview/applied plan, `2` for refusal or input/output
failure.

## `mcp serve`

```bash
aargrade mcp serve
```

The MCP server uses newline-delimited JSON over stdio. Stdout is reserved for
protocol traffic; operational failures go to stderr. It exposes:

- `aargrade_doctor`
- `aargrade_plan`
- `aargrade_verify`
- `aargrade_matrix`
- `aargrade_host_add`
- `aargrade_host_remove`

Each tool calls the same Go domain operation as the CLI and returns structured
output with inferred input/output schemas. Long-running verification and matrix
commands observe MCP request cancellation. Tool annotations distinguish
read-only diagnosis/planning, additive build/evidence work, optional network
access, and destructive host removal.

Host `apply` and matrix `allowDownloads` default to false in the MCP input
schemas. Closing stdin ends the server session. Process exit code is `0` after a
normal client disconnect and `2` when the server cannot run.

## JSON compatibility

Doctor reports, migration plans, host plans, verification reports, and matrix
reports currently use `schemaVersion: 1`. Within a schema version, new fields
may be additive, but existing verdict semantics must not change. A breaking
rename or semantic change requires a new schema version.
