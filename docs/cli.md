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

## `upgrade`

```bash
# No-write, no-build preview
aargrade upgrade \
  --project /path/to/project \
  --target-agp 9.2.0

# Apply repairs and run project/AAR evidence
aargrade upgrade \
  --project /path/to/project \
  --target-agp 9.2.0 \
  --library :sdk \
  --baseline-aar /path/to/released.aar \
  --apply
```

Options:

- `--project`: Gradle root. Default: `.`.
- `--target-agp`: required target AGP version.
- `--current-agp`: reviewed current-version override.
- `--library`: Android library Gradle path when discovery is ambiguous.
- `--variant`: library variant to assemble. Default: `release`.
- `--baseline-aar`: optional released AAR for ABI/metadata/JNI comparison.
- `--matrix-config`: optional strict consumer-matrix YAML. When omitted, no
  consumer cells run.
- `--matrix-work-dir`: optional owned matrix evidence directory.
- `--cell`: selected matrix cell; repeatable.
- `--apply`: apply configuration repairs and run Gradle/AAR evidence. Default:
  false.
- `--keep-failed-changes`: disable the default automatic rollback after a
  failed or incomplete applied run.
- `--allow-downloads`: allow checksum-verified Gradle downloads for matrix
  cells. Default: false.
- `--fail-fast`: stop the optional matrix at its first regression.
- `--gradle-arg`: extra argument appended to each project Gradle command;
  repeatable.
- `--java-home MAJOR=PATH` and `--gradle-bin VERSION=EXECUTABLE`: optional
  matrix toolchain overrides; repeatable.
- `--timeout`: timeout for each Gradle/build operation. Default: `15m`.
- `--format`: `text` or `json`.

Preview calls the same migration engine with agent repairs enabled but writes
nothing and does not invoke Gradle. Apply mode performs this sequence:

1. discover and statically diagnose the project;
2. prepare the supported AGP, Wrapper, and AGP 9 changes without writing;
3. run the unmodified project's Wrapper `build --dry-run --no-daemon` and retain
   the complete bounded command evidence as the pre-migration baseline;
4. revalidate every input and apply one hash-owned transaction, including
   safely recognized forms of a missing namespace and its legacy source
   manifest package, implicit custom BuildConfig feature, numeric or narrowly
   version-catalog-backed legacy SDK setters, simple
   `android.kotlinOptions.jvmTarget`, and matching Java target alignment;
5. run Wrapper `help`, repeat the whole-project `build --dry-run`, and compare
   its normalized failure evidence with the pre-migration result;
6. run the selected classic or KMP Android library assembly task first with
   `--dry-run` and then normally;
7. inspect the candidate AAR and compare the optional baseline;
8. run the optional consumer matrix; and
9. classify a failure and automatically roll back the owned configuration,
   unless `--keep-failed-changes` was explicitly supplied.

If both whole-project dry-runs fail, only a non-empty and exactly equal
normalized Gradle `What went wrong` block with the same exit code is classified as
`pre-existing-failure`. The post-migration command remains visible with its
non-zero exit code but receives warning evidence, and the selected library
dry-run and assembly must still pass. A newly failing or different block is a
`regression`. Timeouts, cancellations, truncated output, signal termination,
and failures without comparable evidence also fail closed. A pre-existing warning therefore scopes the pass to the
selected library and produced AAR; it is not a whole-repository success claim.

JSON reports retain the full bounded pre-migration command in
`beforeUpgradeDryRun`. `verification.rootDryRun` contains the verdict, before
and after statuses, SHA-256 fingerprints of normalized evidence, and bounded
failure excerpts. Expected verdicts are `pass`, `improved`,
`pre-existing-failure`, and `regression`.

The repair engine is deterministic, not a bundled language model. Existing
conflicting Java targets, complex Kotlin compiler/source-set blocks, kapt/KSP
decisions, convention-plugin internals, legacy Variant APIs, and ambiguous
dynamic logic remain blockers or project-specific agent work. Over MCP, the
structured blockers and bounded Gradle output let an external agent edit those
project-specific cases and rerun `aargrade_upgrade`.

A successful run leaves rollback state for review. Use `migrate rollback` to
restore exact originals or `migrate accept` to keep exact applied files and
remove that state.

Exit codes:

- `0`: applicable preview or all requested applied evidence passed.
- `1`: static blocker, Gradle/AAR failure, or consumer regression. Applied
  owned changes are rolled back by default.
- `2`: incomplete environment/matrix, invalid input or state, or another error
  prevented a trustworthy requested verdict.

## `migrate`

```bash
# Preview only
aargrade migrate \
  --project /path/to/project \
  --target-agp 9.2.0

# Apply the reviewed diff
aargrade migrate \
  --project /path/to/project \
  --target-agp 9.2.0 \
  --apply
```

Options:

- `--project`: Gradle root. Default: `.`.
- `--target-agp`: required target AGP version.
- `--current-agp`: reviewed current version, accepted only when it does not
  conflict with concrete declarations. An override cannot make an unresolved
  mutation location safe.
- `--apply`: write the exact previewed changes. Default: false.
- `--format`: `text` or `json`.

The bounded mutation engine currently supports:

- literal Kotlin/Groovy `com.android.*` plugin versions;
- `com.android.tools.build:gradle` classpath coordinates;
- one literal Groovy `buildscript.ext` variable whose string interpolations are
  all AGP classpath coordinates; shared, reassigned, or dynamic variables stay
  unresolved;
- AGP entries and unshared `version.ref` keys in the default
  `gradle/libs.versions.toml` catalog;
- a below-minimum Gradle Wrapper URL and its pinned official `bin`/`all`
  distribution SHA-256;
- standalone Kotlin Android plugin declarations when moving to AGP 9 Built-in
  Kotlin without compiler/source-set DSL; and
- removal of obsolete full-migration properties including
  `android.builtInKotlin` and `android.newDsl`.

The operation fails closed before writing when static evidence finds an
unresolved/conflicting AGP declaration, missing AGP 8+ namespace, implicit
custom BuildConfig feature, legacy Kotlin kapt without `com.android.legacy-kapt`,
KSP below 2.3.6, Kotlin compiler/source-set
DSL, legacy Variant/internal APIs, removed AGP 9 DSL, mixed legacy KMP Android
plugins, buildSrc, settings-defined catalogs, RefreshVersions, unresolved
convention logic, or an AGP catalog version ref shared with unrelated entries.

`--apply` re-reads every input hash, writes a prepared ownership transaction,
atomically replaces each supported file, and marks the transaction applied.
Exact original content, permissions, and before/after hashes are stored in the
ignored local file `.aargrade/state/migration.json`. The state can contain
original Gradle configuration and must be handled as potentially sensitive.

The command does not install or select a JDK, regenerate Wrapper scripts/JAR,
run Gradle, build an AAR, or claim compatibility. Its result always lists
`./gradlew help`, `build --dry-run`, `verify`, and `matrix` as separate evidence
steps.

Exit codes:

- `0`: a trustworthy applicable preview was produced, or it was applied.
- `1`: a trustworthy analysis found a blocker; no files were written.
- `2`: invalid input, unsupported policy/state, discovery, race, or output
  failure.

## `migrate rollback`

```bash
aargrade migrate rollback --project /path/to/project
aargrade migrate rollback --project /path/to/project --apply
```

Options are `--project`, `--apply`, and `--format`. Preview is the default.
Rollback first validates the strict state schema and limits owned paths to
Gradle configuration files. Every current file must match either its recorded
pre-migration hash or exact applied hash. Any other content means the user or a
tool changed that file, so the entire rollback stops without restoring
anything. Apply restores exact original bytes and modes, then removes the
ownership state. A partially restored prepared transaction can be retried.

Exit codes:

- `0`: an applicable rollback preview was produced, or it was applied.
- `1`: at least one owned file changed and automatic rollback was refused.
- `2`: state is missing/invalid, an owned path is unsafe, or another
  input/output failure prevented a trustworthy result.

## `migrate accept`

```bash
aargrade migrate accept --project /path/to/project
aargrade migrate accept --project /path/to/project --apply
```

Options are `--project`, `--apply`, and `--format`. Preview is the default.
Accept requires a fully applied transaction and requires every owned project
file to still match its exact recorded post-migration SHA-256. Apply removes
only `.aargrade/state/migration.json` and empty ownership directories; it keeps
all migrated Gradle files unchanged. A partial rollback or any later edit
blocks automatic acceptance so rollback protection is not silently discarded.

Exit codes:

- `0`: applicable accept preview or applied state removal.
- `1`: the transaction or an owned file no longer matches the exact applied
  state.
- `2`: state is missing/invalid, a path is unsafe, or another input/output
  failure prevented a trustworthy result.

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
3. `<library>:assemble<Variant> --dry-run --no-daemon`, or
   `<library>:bundleAndroidMainAar --dry-run --no-daemon` for an AGP Kotlin
   Multiplatform Android library
4. `<library>:assemble<Variant> --no-daemon`, or
   `<library>:bundleAndroidMainAar --no-daemon` for an AGP Kotlin Multiplatform
   Android library

Standalone `verify` has no pre-migration baseline command. Its whole-project
dry-run therefore remains strict: any failure stops verification rather than
being classified as pre-existing.

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
- `aargrade_upgrade`
- `aargrade_migrate`
- `aargrade_migrate_accept`
- `aargrade_migrate_rollback`
- `aargrade_verify`
- `aargrade_matrix`
- `aargrade_host_add`
- `aargrade_host_remove`

Each tool calls the same Go domain operation as the CLI and returns structured
output with inferred input/output schemas. Long-running verification and matrix
commands observe MCP request cancellation. Tool annotations distinguish
read-only diagnosis/planning, additive build/evidence work, optional network
access, and destructive migration/rollback/host operations.

All mutation `apply` values and matrix `allowDownloads` default to false in the
MCP input schemas. Closing stdin ends the server session. Process exit code is
`0` after a normal client disconnect and `2` when the server cannot run.

## JSON compatibility

Doctor reports, migration plans, upgrade reports, mutation/accept/rollback
results, host plans, verification reports, and matrix reports currently use
`schemaVersion: 1`.
Within a schema version, new fields may be additive, but existing verdict
semantics must not change. A breaking rename or semantic change requires a new
schema version.
