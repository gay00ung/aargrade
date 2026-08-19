# Validation log

Checked through **2026-08-18**. This document separates verified facts from product
hypotheses; it is not a claim that market demand has already been proven.

## Verified ecosystem facts

| Claim | Evidence | Product consequence |
| --- | --- | --- |
| Upgrade Assistant uses static Gradle-file analysis, has documented unsupported structures, and may still fail on conforming projects. | [Official Upgrade Assistant guide](https://developer.android.com/build/agp-upgrade-assistant) | `doctor` should explain analyzability and uncertainty before suggesting a workaround. |
| AGP 9 enables built-in Kotlin and removes access to important legacy DSL/Variant API surfaces by default. | [AGP 9.0 release notes](https://developer.android.com/build/releases/agp-9-0-0-release-notes) | Kotlin plugins, opt-outs, and legacy APIs are first-class diagnostics. |
| AGP 9.3 is the current stable line; AGP 9.4 changes are in preview and AGP 10 is planned for late 2026. | [AGP 9.3 release notes](https://developer.android.com/build/releases/agp-9-3-0-release-notes), [AGP roadmap](https://developer.android.com/build/releases/gradle-plugin-roadmap) | Compatibility knowledge must be versioned data, not scattered conditionals. |
| AAR metadata can declare minimum AGP and compile SDK requirements. | [AAR publishing guide](https://developer.android.com/build/publish-library/prep-lib-release), [AAR metadata DSL](https://developer.android.com/reference/tools/gradle-api/9.2/com/android/build/api/dsl/AarMetadata) | `verify` must inspect metadata before running expensive consumers. |
| Gradle TestKit can select Gradle versions for cross-version plugin tests. | [Gradle TestKit guide](https://docs.gradle.org/current/userguide/test_kit.html) | Useful for a future Gradle plugin, but it does not replace isolated AAR consumer builds. |
| Diffuse compares APK/AAB/AAR/JAR contents. | [Diffuse repository](https://github.com/JakeWharton/diffuse) | Reuse or complement artifact diffing; do not rebuild it. |
| Metalava and Kotlin binary compatibility tools cover API/ABI surfaces. | [Metalava](https://android.googlesource.com/platform/tools/metalava/), [Kotlin BCV](https://kotlin.github.io/binary-compatibility-validator/) | Delegate specialist analysis and consolidate evidence. |

## Name checks

The provisional name `AARGrade` was checked as follows:

- GitHub repository name search for `aargrade`: no exact repository result.
- `github.com/gay00ung/aargrade`: not found.
- Maven Central full-text search: zero results.
- Gradle Plugin Portal search: no plugins found.
- `aargrade.io` and `aargrade.com`: WHOIS returned no registered domain.
- General web search found unrelated uses of “AAR grade” in the railway
  industry, so the phrase is not globally unique.

These checks do **not** reserve a repository, package, plugin ID, or domain and
do not constitute trademark clearance. Keep the name provisional until those
actions are explicitly approved.

## Environment baseline

- Local architecture: macOS arm64.
- Go initially available: 1.26.0; security validation also used patched Go
  1.26.5.
- JDKs available: 11, 17, 21, and 25; no JDK 8 detected.
- Android SDK command-line tools are available.
- No global Gradle command is installed; builds must be wrapper-driven.
- Gradle's current distribution API reported 9.7.0.

This confirms that a meaningful old-AGP matrix will need explicit JDK
provisioning rather than inheriting the CLI environment.

## Milestone 1 implementation evidence

### Real Gradle smoke tests

The generated host was applied, configured by Gradle, and removed from copies
of two minimal library-only fixtures:

| Fixture | Toolchain | Result |
| --- | --- | --- |
| Groovy DSL legacy endpoint | AGP 4.2.2, Gradle 6.7.1, JDK 11, compileSdk 30 | `:aargrade-upgrade-host:tasks --all` succeeded; removal restored the original settings file. |
| Kotlin DSL current endpoint | AGP 9.2.0, Gradle 9.4.1, JDK 17, compileSdk 35 | `:aargrade-upgrade-host:tasks --all` succeeded; removal restored the original settings file. |

The reusable check is `scripts/gradle-smoke.sh`; CI runs the legacy and current
endpoints plus the dedicated Upgrade Assistant fixture.
This proves that the generated Gradle models configure at the tested endpoints.
It does not by itself prove Android Studio behavior; that causal claim was
tested separately in the controlled A/B experiment below.

### External repository trials

`doctor` and preview-first `upgrade` were run against pinned clones of these
public repositories. Applied verification was limited to projects already on
AGP 9:

| Repository and commit | Validation result | Product correction |
| --- | --- | --- | --- |
| [JakeWharton/timber](https://github.com/JakeWharton/timber) `c51c0cad85bfd4c4c452222dae5ec49e8988303a` | AGP 9.2.1 → 9.3.1 applied; `help`, full dry-run, KMP AAR assembly, and AAR inspection passed. | Preserve Kotlin/KMP setup on AGP 9 minor upgrades; use `bundleAndroidMainAar` and locate suffixless KMP AARs. |
| [square/picasso](https://github.com/square/picasso) `e94d6116e3fff99b3932103e0ab22ff5b44a1273` | AGP 8.7.2 → 9.3.1 preview became ready across six Groovy Android modules. | Handle consecutive Groovy plugin lines and narrowly migrate matching `libs.versions.*.get()` SDK/JVM target expressions. |
| [airbnb/lottie-android](https://github.com/airbnb/lottie-android) `05ea92e90381eb8a8ae06855ea2b74f322bebbec` | Diagnosis succeeded; upgrade stopped because RefreshVersions owns the effective AGP mapping. | Keep the dynamic-version boundary explicit and fail-closed instead of guessing. |
| [bumptech/glide](https://github.com/bumptech/glide) `bf4901de78c206ed21ce73f5d25aa2bbf35ebf3c` | AGP 9.2.0 → 9.3.1 and Gradle 9.4.1 → 9.5.0 applied; large dry-run, release AAR assembly, and inspection passed. | Resolve catalog libraries declared with separate `group` and `name`; preserve existing AGP 9 Kotlin/KSP setup. |
| [adjust/android_sdk](https://github.com/adjust/android_sdk) `b31ee274a2d189c4ff705c2ee6c47ad09ca5eb62` | Released 5.8.0 source migrated from AGP 9.2.1 → 9.3.1; selected SDK AAR, baseline comparison, and four consumer cells passed. | Resolve a single literal Groovy `buildscript.ext` AGP variable while keeping shared and dynamic variables fail-closed. |

The Timber AAR was 32,705 bytes with SHA-256
`3270214ff2784195326b0a67da50404f7cb36291e30b50690e6cb0dba38d19f1`.
The Glide AAR was 719,394 bytes with SHA-256
`59e1c99532535b4ce15973539a9359af0efbe903c8215dad415eb74b64cc52c7`;
its three broad Consumer R8 rules were retained as review warnings. Both
successful runs finished with hash-checked migration acceptance. The pinned
protocol is executable through `make test-external`, with applied Gradle work
requiring explicit `AARGRADE_EXTERNAL_APPLY=1`. See the
[Korean detailed record](external-validation-2026-08.ko.md).

The Adjust baseline AAR from Maven Central had SHA-256
`d00ddde91b0bfe46fb7c5675fa5a52fe3393e990a334f14804b01c2e4f0affea`;
the AGP 9.3.1 rebuild had SHA-256
`69457f6f2b09c8d9daeda7a3432ef7c78ef3ad2576764d9e84e3b6f6d75fca61`.
Both passed AGP 4.2.2/Gradle 6.7.1/JDK 11 and AGP 9.3.1/Gradle 9.5.0/JDK 17
Java/Kotlin release consumers. The repository-wide dry-run failed identically
before and after migration because an unrelated plugin task references missing
`packageReleaseAssets`; therefore the record claims selected SDK compatibility,
not a clean whole-repository build. See the
[Adjust dogfood record](adjust-sdk-dogfood-2026-08-19.ko.md).

These trials validate useful structural coverage across Kotlin DSL, Groovy DSL,
KMP Android, version catalogs, classic buildscript classpaths, Android test
modules, and dynamic version management. They are technical trials, not demand
validation or proof of every source/runtime compatibility property.

### Upgrade Assistant A/B status

Gate A and Gate C passed on 2026-08-14 in a controlled A→B→A run using Android
Studio build `AI-253.30387.90.2532.14901460`, Microsoft OpenJDK 17.0.16, and the
dedicated AGP 7.4.2/Gradle 7.5 library-only fixture.

| State | Gradle/IDE observation | Upgrade Assistant observation |
| --- | --- | --- |
| A: library only | Wrapper `help` and Studio sync succeeded. | No usable plan; IDE log: `Unable to obtain application's Android Project`. |
| B: after `host add --apply` | Sync succeeded and the generated application run configuration appeared. | Opened `Upgrade project from AGP 7.4.2`, selected 8.13.2, and listed concrete migration steps. |
| A: after `host remove --apply` | Settings returned to its exact original SHA-256 and the host disappeared. | The open panel returned to `unknown AGP` / `Android Gradle Plugin not present`. |

No migration step was run. This validates the workaround for that exact fixture
and Studio build, not every future IDE or dynamic Gradle structure. The full
environment, checksums, logs, protocol, and bounded conclusion are in the
[experiment record](upgrade-assistant-experiment.md) and its
[Korean version](upgrade-assistant-experiment.ko.md).

### Automated safety coverage

- `doctor` tree digest is identical before and after analysis.
- Human text and schema-versioned JSON share one ordered finding model.
- Host preview creates no files.
- Host removal preserves settings edits outside the owned block.
- Modified generated files or settings blocks stop automatic removal.
- Existing application modules and ambiguous library selection stop host
  creation.
- Unix and Windows replacement paths compile; CI is configured to run unit
  tests on all three major desktop operating systems.
- `TestUpgradeAssistantReproHostLifecycle` keeps the published fixture
  runnable, verifies the library-only → application-present → library-only
  diagnosis transition, and checks exact settings restoration. CI also adds
  the AGP 7.4.2/Gradle 7.5/JDK 17 fixture to the real Gradle host smoke matrix.
- The Consumer R8 negative fixture was reviewed separately: its global
  `-dontoptimize` option is a high-confidence `doctor` finding, while the
  remaining member rule requires source-level evidence and is not automatically
  rewritten.
- Strict matrix YAML parsing rejects unknown fields and extra documents. AAR,
  nested JAR, and downloaded Gradle ZIP readers enforce entry, path, compressed,
  and extracted-size boundaries.
- Recorded Gradle arguments and logs mask common password, token, credential,
  API-key, private-key, project-path, and user-home patterns. This is a
  best-effort safeguard rather than permission to publish arbitrary build logs.

### Public user journey

- `go install github.com/gay00ung/aargrade/cmd/aargrade@latest` successfully
  installed the pushed repository into an empty temporary binary directory.
- `make demo` diagnoses, plans, and previews the agent-style upgrade plus host
  creation against `examples/library-only` without changing the example tree.
- `TestPublicExampleUserJourney` runs the public example through doctor, host
  preview, host apply, removal preview, and removal apply, then verifies exact
  settings restoration.
- A local wrapper-driven build with Gradle 9.4.1, JDK 17, and Android SDK
  platform 35 completed all 25 tasks and produced `sdk-release.aar` containing
  `classes.jar`, the manifest, and AAR metadata.
- CI is configured to repeat that release AAR build so the documented example
  cannot silently decay into a static-only fixture.
- The example includes a Gradle Wrapper whose 9.4.1 distribution checksum was
  matched against the official published SHA-256 before generation and is
  pinned in `gradle-wrapper.properties`.
- Android CLI 1.0.15857036 described the public project successfully and
  resolved both the `:sdk` debug and release variants against the configured
  local Android SDK.

## Milestone 2 implementation evidence

### Version-aware plan

- `plan --target-agp` consumes the same read-only diagnosis as `doctor` and a
  centralized compatibility table instead of duplicating project discovery.
- The table currently covers AGP 4.2, 7.0–7.4, 8.0–8.13, and 9.0–9.3 using the
  [official AGP/Gradle compatibility table](https://developer.android.com/build/releases/about-agp)
  and endpoint release notes.
- Unknown policy lines, unresolved current AGP, a non-upgrade target, and
  high-confidence diagnostic errors fail closed. Unit tests cover current/target
  ordering and toolchain requirements.
- The public AGP 9.2.0 example produces a ready AGP 9.3.0 plan that correctly
  requires Gradle 9.5.0 and JDK 17, while still requiring later artifact and
  consumer verification.

### Bounded migration and rollback

- `migrate` is read-only by default and produces deterministic per-file diffs,
  before/after SHA-256 values, blockers, warnings, and required verification
  steps. `--apply` rechecks every input hash before writing.
- A Kotlin DSL fixture backed by `gradle/libs.versions.toml` was migrated from
  AGP 8.8.0/Gradle 8.10.2 to AGP 9.2.0/Gradle 9.4.1. The engine updated the
  catalog AGP ref, Wrapper URL, and official distribution checksum; removed two
  standalone Kotlin Android plugin declarations and four obsolete AGP 9
  opt-out properties.
- With JDK 17 and Android platform 35, the migrated fixture passed real
  `help`, `build --dry-run`, `compileReleaseKotlin`, and `assembleRelease` tasks.
  The Kotlin source was therefore compiled through AGP 9 Built-in Kotlin and a
  release AAR was produced.
- `migrate rollback --apply` restored every owned configuration file byte for
  byte and removed the transaction state. Unit and integration tests repeat
  preview/apply/rollback and prove that a post-migration user edit blocks the
  entire restore.
- Negative fixtures prove fail-closed behavior for shared catalog refs, KSP
  below 2.3.6, buildSrc, implicit BuildConfig, legacy Variant API, missing AGP
  8+ namespace, and multiline Kotlin plugin declarations.
- CI repeats the real migration/build/rollback lifecycle in the
  `migration-smoke` job. Wrapper checksum transformation is covered for both
  `bin` and `all` distributions without weakening an existing pin.

### AAR verification

- The inspector accepts only regular non-symlink AARs, bounds archive and entry
  sizes, rejects duplicate or unsafe paths, and records SHA-256 evidence.
- A pure-Go class-file reader inventories public/protected classes, fields, and
  methods including descriptors, signatures, constants, exceptions,
  superclass/interfaces, access semantics, and Kotlin metadata presence.
- Baseline comparison fails on removed classes/members, narrowed access,
  incompatible superclass/interface/sealed/static/abstract/final changes, raised declared
  consumer requirements, removed JNI/Prefab libraries, and global Consumer R8 options.
- Final/abstract/static and access-change classifications follow the
  [Java Language Specification binary-compatibility rules](https://docs.oracle.com/javase/specs/jls/se25/html/jls-13.html),
  including the distinction that making an instance method final may break
  existing subclasses while making a static method final does not.
- Source-level and runtime claims outside the built-in engine are explicitly
  rendered as warnings or skipped checks. No skipped check contributes a pass.
- The public `sdk-release.aar` was inspected and compared with itself: AAR
  structure, metadata, JVM linkage, Consumer R8, and JNI packaging checks
  passed. Negative tests cover unsafe ZIP entries, raised metadata requirements,
  and unsafe R8 global options.

## Milestone 3 consumer-matrix evidence

The same public candidate AAR was placed in generated, isolated, minified
Android application consumers. Explicit JDK and Gradle executables were used:

| Cell | Toolchain | Result |
| --- | --- | --- |
| `agp4-java` | AGP 4.2.2, Gradle 6.7.1, JDK 11, compileSdk 30, Java | PASS |
| `agp4-kotlin` | AGP 4.2.2, Gradle 6.7.1, JDK 11, compileSdk 30, Kotlin 1.6.21 | PASS |
| `agp9-java` | AGP 9.2.0, Gradle 9.4.1, JDK 17, compileSdk 35, Java | PASS |
| `agp9-kotlin` | AGP 9.2.0, Gradle 9.4.1, JDK 17, compileSdk 35, built-in Kotlin | PASS |

The first legacy run found a real metadata problem: AGP had emitted the
library's default `minCompileSdk=35`, so the API-30 consumer correctly rejected
the AAR. Raising the old consumer to API 35 then exposed that AGP 4.2's AAPT2
cannot consume that modern platform. The example uses no API/resource newer
than 30, so its library DSL now deliberately declares
`aarMetadata.minCompileSdk = 30`; the rebuilt AAR then passed both legacy cells.
This demonstrates that
the matrix can catch an incorrectly declared consumer floor rather than merely
checking that the producer builds.

The matrix validates toolchain tuples, verifies the selected `java -version`
and `gradle --version`, preserves exact generated projects and bounded logs,
and classifies baseline/candidate outcomes as pass, regression, improved,
unsupported, fail, or incomplete. CI now builds the public AAR once and repeats
all four consumer cells independently.

Gate D is technically complete. Routine cost, broader API usage, runtime/device
behavior, and external-project adoption remain separate questions.

The generated projects use direct AAR file dependencies. Teams must declare
the dependencies normally supplied by their Maven POM in the matrix
`dependencies` list; otherwise a failure may describe an incomplete test
fixture rather than a consumer regression.

## Milestone 4 MCP evidence

- The server uses the official Tier-1
  [Model Context Protocol Go SDK](https://modelcontextprotocol.io/docs/sdk),
  currently pinned to `github.com/modelcontextprotocol/go-sdk` v1.7.0.
- Ten tools call the same domain packages as the CLI: doctor, plan, upgrade,
  migrate, migrate accept, migrate rollback, verify, matrix, host add, and host
  remove. There is no subprocess parsing or duplicate policy engine.
- Typed input/output schemas are inferred by the SDK. Read-only, additive,
  open-world, and destructive annotations reflect the operations; host apply
  and Gradle downloads remain explicit opt-ins.
- Tests negotiate an in-memory session, list and call typed tools, verify that
  invalid input becomes a tool-visible error, and launch a separate real stdio
  server process to negotiate and list all ten tools.
- Verification and matrix operations propagate MCP cancellation into Gradle,
  JDK, Gradle validation, and HTTP download contexts.

This proves protocol behavior, not broad external demand for the integration.

## Milestone 5 agent-style upgrade evidence

- `upgrade` is preview-only unless `--apply` is supplied. Preview produces the
  complete owned diff without invoking Gradle or changing the fixture tree.
- The deterministic repair layer handles a literal manifest-derived namespace
  plus removal of that legacy source-manifest package attribute, implicit
  custom BuildConfig enablement, numeric legacy SDK setter syntax,
  simple `android.kotlinOptions.jvmTarget`, matching Java compile targets,
  Kotlin Android plugin/catalog removal, obsolete AGP 9 properties, AGP, and
  the checksum-pinned Wrapper.
- The dedicated `upgrade-agent` fixture begins on AGP 8.8/Gradle 8.10.2 with no
  namespace, custom BuildConfig while disabled, `compileSdkVersion`,
  `minSdkVersion`, and `kotlinOptions`. One `upgrade --apply` migrated it to AGP
  9.2/Gradle 9.4.1, passed Wrapper `help`, `build --dry-run`, Built-in Kotlin
  release compilation, AAR assembly, structure, metadata, and Consumer R8
  inspection with JDK 17 and Android platform 35.
- The first real run exposed inconsistent Java 11/Kotlin 17 targets. That
  evidence led to the explicit Java/Kotlin alignment recipe and a dedicated
  failure category instead of weakening the fixture.
- A failed applied verification is categorized and exact owned configuration is
  automatically rolled back by default. Unit tests inject a Gradle namespace
  failure and verify both the category and byte-exact restoration.
- Applied upgrades retain a bounded whole-project dry-run before mutation and
  compare it with the post-migration run. Tests cover an identical pre-existing
  failure, a newly introduced failure, a different failure, a resolved failure,
  and timeout/cancellation fail-closed behavior. An accepted pre-existing root
  failure still requires the selected library dry-run and AAR assembly.
- Adjust Android SDK 5.8.0 exercises that branch with real Gradle: the same
  missing `packageReleaseAssets` task failed before and after AGP 9.2.1 → 9.3.1,
  while `:sdk-core` dry-run, assembly, released-AAR ABI, metadata, R8, and JNI
  evidence passed. The report remained explicit that this was not a clean
  whole-repository build.
- A separate permission-failure test stops the multi-file write after ownership
  state and earlier files were written, then verifies that `upgrade` classifies
  the incomplete apply and restores every partial change automatically.
- A passed run keeps rollback state. `migrate accept` removes it only when all
  owned files still match their exact applied hashes; later edits and partial
  transactions fail closed.
- `scripts/upgrade-agent-smoke.sh` and CI repeat the complete repair, migration,
  real build/AAR inspection, and exact rollback lifecycle.

This is assistant-style orchestration, not a claim that the standalone CLI can
semantically rewrite arbitrary Gradle programs. Project-specific blockers are
returned through CLI JSON and `aargrade_upgrade` so an MCP agent or maintainer
can continue the loop.

## Dependency and toolchain security evidence

- `go mod verify` passed for the pinned module graph.
- `govulncheck` under the initially installed Go 1.26.0 correctly reported
  reachable standard-library vulnerabilities fixed in later Go patch releases.
- Re-running the scan with the CI/release toolchain Go 1.25.13 reported zero
  vulnerabilities called by AARGrade. One imported/required-module advisory
  was unreachable from this code. CI and release builds remain pinned to that
  fixed patch and repeat the reachable-vulnerability scan; Go 1.26 builders
  must use 1.26.6 or newer for the same fixes.

## Release automation evidence

- `make release-check` cross-compiled macOS, Linux, and Windows binaries for
  amd64 and arm64 with CGO disabled.
- All six archives contained the executable, both READMEs, the exact
  Apache-2.0 license text, and licenses/notices collected for linked modules
  and the Go toolchain. Generated SHA-256 entries verified successfully.
- The archive matching the local host executed and reported the SemVer value
  injected at link time rather than `dev`.
- CI repeats this dry-run packaging check on Linux. Tag-triggered publishing
  runs the same packaging scripts only after tests, vet, vulnerability
  scanning, workflow linting, and the public demo pass.
- The release job has `contents: write`; all earlier jobs retain read-only
  repository access. Prerelease tags such as `v0.1.0-beta.1` are marked as
  GitHub prereleases, while tags such as `v0.1.0` create stable releases.
- Publishing rejects a tag whose commit is not an ancestor of `origin/main`.
- The `v0.1.0-beta.1` workflow passed its quality gate, built all six archives,
  verified checksums and embedded versions, and published seven assets as a
  GitHub prerelease on 2026-08-18.
- A fresh download of the macOS arm64 archive matched `checksums.txt` and
  executed as `aargrade v0.1.0-beta.1`. A separate temporary
  `go install github.com/gay00ung/aargrade/cmd/aargrade@v0.1.0-beta.1` also
  reported the same version.

This validates the packaging and publication path. The GitHub Releases page
and the tag-triggered workflow are the authoritative publication record.

## Unverified hypotheses

1. The validated host workaround remains reliable across other supported
   Android Studio builds and real-world Gradle structures.
2. SDK maintainers will adopt a new CLI instead of maintaining bespoke CI
   sample apps.
3. The four-cell default matrix can be cheap and representative enough for
   routine pull-request use outside this small example.
4. Static diagnostics will remain useful across convention plugins and dynamic
   build logic.
5. Teams beyond the initial repository owner will want MCP after CI output is
   available.

The delivery gates in the product plan are designed to test these hypotheses
without pretending they are facts.
