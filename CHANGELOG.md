# Changelog

Notable changes to AARGrade are recorded here. The project follows Semantic
Versioning while its public interfaces are still in beta.

## [Unreleased]

## [0.1.0-beta.2] - 2026-08-19

### Added

- Compare whole-project Gradle dry-run evidence before and after an applied
  upgrade. An exactly matching pre-existing failure is retained as a warning
  only after the selected library dry-run and AAR assembly pass; new,
  different, timed-out, canceled, and unrecognized failures remain fatal.
- Preserve pre-migration command evidence and normalized failure fingerprints
  in structured upgrade and verification reports.

### Fixed

- Resolve and migrate a single literal Groovy `buildscript.ext` AGP variable
  when every interpolation is confined to an AGP classpath coordinate. Shared,
  reassigned, and dynamic variables remain fail-closed.

### Validation

- Migrated the released Adjust Android SDK 5.8.0 source from AGP 9.2.1 to
  9.3.1, compared its rebuilt AAR with Maven Central, and passed AGP 4.2.2 and
  9.3.1 Java/Kotlin consumer builds.
- Re-ran Adjust through one `upgrade --apply`: its identical pre-existing root
  dry-run failure was isolated as a warning while `:sdk-core` dry-run, AAR
  assembly, JVM ABI, metadata, R8, and JNI verification completed.

## [0.1.0-beta.1] - 2026-08-18

### Added

- Preview-first `doctor`, `plan`, `upgrade`, `migrate`, `verify`, `matrix`,
  `host`, and `mcp serve` workflows.
- Hash-owned migration transactions with exact rollback and explicit accept.
- AGP/Gradle/JDK policy for AGP 4.2, 7.x, 8.x, and 9.0–9.3.
- Bounded AAR, JVM binary surface, metadata, Consumer R8, JNI, and consumer
  matrix evidence.
- Six-platform release archives with SHA-256 checksums and bundled notices.

### Safety and compatibility

- Added deterministic Upgrade Assistant-style repairs for namespace,
  `BuildConfig`, legacy SDK setters, Built-in Kotlin, and matching Java/Kotlin
  targets.
- Recognize version-catalog libraries declared with `group` plus `name` and
  narrowly support `libs.versions.*.get()` in safe Groovy migration recipes.
- Preserve existing Kotlin/KMP configuration during AGP 9 minor upgrades.
- Build and locate AARs produced by the AGP Kotlin Multiplatform Android
  library plugin.
- Keep RefreshVersions, arbitrary Gradle expressions, convention-plugin
  internals, legacy Variant APIs, and other ambiguous changes fail-closed.

[Unreleased]: https://github.com/gay00ung/aargrade/compare/v0.1.0-beta.2...HEAD
[0.1.0-beta.2]: https://github.com/gay00ung/aargrade/compare/v0.1.0-beta.1...v0.1.0-beta.2
[0.1.0-beta.1]: https://github.com/gay00ung/aargrade/releases/tag/v0.1.0-beta.1
