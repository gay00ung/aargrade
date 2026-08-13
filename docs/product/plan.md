# AARGrade product and delivery plan

## Implementation status (2026-08-13)

- Milestone 1 is implemented: read-only diagnosis and ownership-safe host
  preview/apply/removal.
- Milestone 2 is implemented as a conservative first engine: version-aware
  planning, wrapper-driven build evidence, bounded AAR inspection, JVM linkage,
  metadata, Consumer R8, and JNI packaging comparison.
- Milestone 3 is implemented and Gate D passed locally: the public AAR built in
  AGP 4.2.2 and 9.2.0 Java/Kotlin consumers with explicit JDK 11/17. CI is
  configured to repeat all four cells.
- The MCP portion of Milestone 4 is implemented over the shared domain
  operations and tested through real stdio negotiation. A Gradle plugin is not
  implemented.
- Upgrade Assistant UI Gates A and C and real external-user demand remain open.
  Implementing the toolchain is not evidence that the market hypothesis is
  already validated.

## Product contract

### Problem

Android SDK maintainers need more than proof that their own library project
builds after an AGP migration. They need reproducible evidence that the
resulting artifact can still be consumed by the customer toolchains they
support.

The existing ecosystem has strong point tools for API/ABI checks, artifact
diffing, Gradle build-logic tests, and wrapper upgrades. AARGrade's intended
value is orchestration and one compatibility verdict across those seams.

### Primary users

- Teams publishing AARs or Android SDKs to multiple customers.
- Payment, identity, security, camera, and ML SDK maintainers.
- Android Gradle plugin authors.
- Enterprise teams supporting several AGP, JDK, Java, and Kotlin combinations.

### Job to be done

> When migrating the build toolchain used to publish an Android library, give
> me reviewable evidence that the candidate artifact preserves the consumer
> compatibility policy my team has declared.

### Product boundary

AARGrade is a CLI first. The Gradle plugin, CI adapters, and MCP server must call
the same domain operations and consume the same versioned report schema.

AARGrade does not replace Android Studio's AGP Upgrade Assistant, Metalava,
binary compatibility validators, Diffuse, Gradle, R8, or Android Lint. It will
detect, invoke, and consolidate those tools where their contracts are stable.

## Command delivery status

| Command | Outcome | Status |
| --- | --- | --- |
| `doctor` | Read-only inventory and migration-risk findings | Implemented |
| `host add/remove` | Reversible temporary application host with ownership proof | Implemented |
| `plan --target-agp` | Version-aware migration plan, without source mutation | Implemented |
| `verify` | Candidate build, AAR/JVM linkage, metadata, R8, and JNI packaging evidence | Implemented with documented limits |
| `matrix` | Real Java/Kotlin consumer builds across declared toolchains | Implemented; four endpoint cells proven |
| `mcp serve` | Agent interface over the shared domain contract | Implemented |

Every delivered command must distinguish a pass, failure, skipped evidence, and
inability to run. Placeholder success is forbidden.

## Milestone 1: diagnostic and host MVP

### Observable success criteria

1. `doctor` accepts a project path and never writes to the target project.
2. It identifies settings/build files, included modules, Android application
   and library plugins, literal AGP versions, Gradle Wrapper versions, and
   common migration-risk signals in Groovy and Kotlin DSL projects.
3. It explains unsupported dynamic build logic instead of guessing.
4. Text and JSON reports contain the same ordered findings. JSON carries a
   schema version so CI and the MCP adapter can depend on it.
5. Exit code `0` means analysis completed without findings at or above the
   configured threshold, `1` means the threshold was met, and `2` means the
   command could not produce a trustworthy report.
6. `host add` defaults to preview. `--apply` is required for mutation.
7. A host is auto-selected only when there is exactly one Android library and
   no application module. Ambiguity stops with an actionable error.
8. Every generated file has an ownership marker and recorded content digest.
   Settings edits use a bounded owned block.
9. `host remove` removes only an unchanged owned block and unchanged generated
   files. Any owned-content modification stops removal and reports the exact
   path; unrelated settings edits are preserved.
10. Unit and fixture tests cover Kotlin DSL, Groovy DSL, version catalogs,
    ambiguous modules, preview/apply/remove, and tamper refusal.

### Initial doctor checks

- Missing settings file or Gradle Wrapper.
- No Android module, library-only structure, or application module present.
- AGP and Gradle versions that are literal, catalog-backed, or unresolved.
- `buildSrc` and settings-defined version catalogs that reduce Upgrade
  Assistant static-analysis reliability.
- Kotlin Android, KSP, and kapt plugin signals relevant to AGP 9 migration.
- Legacy Variant APIs and explicit AGP 9 opt-out flags.
- Custom `BuildConfig` fields without an explicit build feature.
- Global/unsafe options in consumer R8 rules.
- NDK and CMake configuration inventory.

Static checks are evidence, not a build verdict. Findings must say when a match
is lexical or incomplete.

### Non-goals

- Rewriting arbitrary Gradle build logic.
- Running Android Studio UI automation inside `doctor`.
- Claiming binary or runtime compatibility from static analysis alone.
- Downloading JDKs, SDKs, containers, or Gradle without explicit opt-in.
- Supporting every historical AGP syntax in the first host generator.

## Milestone 2: migration and artifact verification

This milestone started after Milestone 1 was exercised against Timber, Picasso,
and Lottie Android and the useful/noisy/unresolved findings were recorded in
the validation log.

The `verify` result must distinguish:

- command execution evidence (`help`, dry-run, AAR assembly);
- artifact structure and metadata evidence;
- public/protected JVM linkage evidence from a versioned declared engine;
- consumer R8 and JNI packaging evidence;
- checks that were skipped, including the reason.

No skipped check may be rendered as passing.

## Milestone 3: consumer matrix

Each matrix cell is an explicit tuple:

```text
AGP + Gradle + JDK + compileSdk + consumer language + candidate artifact
```

Cells run in isolated work directories and emit the generated project,
dependency coordinates or artifact checksum, exact command, sanitized log,
duration, and verdict. A baseline/candidate comparison must differentiate a
pre-existing unsupported cell from a regression introduced by the candidate.

JDK selection is per cell. The AARGrade process must not assume that the JDK
running the CLI can run old Gradle versions.

## Milestone 4: integrations

The MCP server and a future Gradle plugin are adapters only. They may not
duplicate project detection, policy evaluation, mutation safety, or report
rendering. The MCP server now calls the same domain packages directly; the
Gradle plugin remains a future integration.

## Validation gates

- **Gate A — problem reproduction:** publish a minimal library-only fixture and
  record the exact Android Studio/Upgrade Assistant behavior and version.
- **Gate B — diagnostic usefulness:** run against 3–5 external libraries;
  classify every finding as useful, noisy, or missing.
- **Gate C — host necessity:** keep `host` only if it changes a reproduced
  failure into a usable Upgrade Assistant flow.
- **Gate D — matrix feasibility (passed technically):** AGP 4.2.2/9.2.0 ×
  Java/Kotlin passed with Gradle 6.7.1/9.4.1 and JDK 11/17. This is not external
  adoption evidence.
- **Gate E — integration demand (partially satisfied):** versioned JSON and CI
  exist and MCP was explicitly requested for this repository. Broader external
  demand remains unverified.

## Re-plan triggers

- The library-only Upgrade Assistant failure cannot be reproduced on any
  supported Android Studio version.
- External-project trials show that lexical Gradle analysis is too noisy to
  produce trustworthy actions.
- A consumer matrix requires redistribution or licensing terms incompatible
  with the intended open-source distribution.
- A registry or trademark conflict makes the AARGrade name unsafe.
