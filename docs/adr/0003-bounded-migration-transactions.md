# ADR 0003: Bound automatic migration and make rollback hash-owned

- Status: Accepted
- Date: 2026-08-14

## Context

An AGP migration can involve executable Gradle logic, convention plugins,
compiler plugins, removed DSL, JDK selection, artifacts, and consumer projects.
A lexical rewrite that claims to handle arbitrary builds would be unsafe.
However, several high-value changes have narrow, statically provable forms:
literal AGP declarations, the default version catalog, the Wrapper distribution,
and standalone Kotlin Android plugin declarations for Built-in Kotlin.

Multi-file edits also need a recovery contract. Git cannot be assumed, and a
blind rollback could overwrite work performed after the migration.

## Decision

- `migrate` is preview-only unless `--apply` is supplied.
- Only enumerated, statically proven syntax is transformed. Unknown or shared
  ownership is a blocker, not a best-effort edit.
- AGP 9 migration blocks legacy Kotlin kapt unless it was explicitly migrated
  to `com.android.legacy-kapt`, old KSP, compiler/source-set DSL, legacy
  Variant/internal APIs, buildSrc, dynamic catalogs, and other structures that
  require semantic decisions.
- A below-minimum Wrapper update changes the distribution URL and pinned
  official checksum together. Wrapper scripts and JAR are not silently
  regenerated.
- Before any project file is written, `.aargrade/state/migration.json` records
  a prepared transaction containing exact originals, modes, and before/after
  SHA-256 values. It is marked applied only after all writes complete.
- Rollback accepts a file only when its current hash is the recorded before or
  after hash. Any third value blocks the entire operation.
- Ownership-state paths are restricted to known Gradle configuration names and
  symlink traversal is refused.
- Gradle execution, JDK selection, artifact verification, and consumer builds
  remain explicit post-migration evidence steps.

## Consequences

The feature handles fewer projects automatically, but a ready preview has a
clear meaning and a rollback cannot silently erase later edits. Teams with
complex build logic receive actionable blockers and can resolve them before
rerunning the same deterministic operation.

The local state duplicates original Gradle configuration, which may be
sensitive. It is permission-restricted, ignored from version control, removed
after rollback, and documented as sensitive local evidence.
