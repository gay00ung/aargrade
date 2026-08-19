# ADR 0004: Compose an agent-style upgrade with evidence and rollback

- Status: Accepted
- Date: 2026-08-14

## Context

The bounded `migrate` command can safely edit known declarations, but users
expect an upgrade assistant to continue through configuration, compilation,
artifact inspection, failure diagnosis, and customer verification. Requiring a
person or MCP agent to manually chain every command makes the safe primitives
harder to use and can leave a failed multi-file migration applied.

A standalone CLI also cannot safely infer arbitrary convention-plugin logic,
kapt/KSP processor policy, removed Variant API semantics, or source-level
intent. Pretending that lexical rewriting is a general Gradle agent would make
the result less trustworthy.

## Decision

- `upgrade` is the recommended entry point and is preview-only by default.
- It composes the existing migration, verification, artifact, and consumer
  matrix domain packages; it does not shell out to other AARGrade commands or
  duplicate their policies.
- Agent repairs are explicit, deterministic recipes with ambiguity checks.
  Unsupported structures remain blockers with structured suggested actions.
- Apply records a whole-project Wrapper `build --dry-run` before mutation,
  repeats it after mutation, and fails on a new or different normalized Gradle
  failure. An exactly matching pre-existing failure is retained as a warning
  only when the selected library task also passes `--dry-run` and real assembly.
- Apply runs Wrapper `help` after mutation before inspecting the produced AAR
  and optional baseline.
- A configured consumer matrix is the final optional evidence stage.
- Failed or incomplete applied runs roll back the hash-owned configuration by
  default. Preserving failed changes requires an explicit option.
- Successful runs retain rollback state for review. `migrate accept` removes
  that state only when every owned file is still the exact applied version.
- The MCP `aargrade_upgrade` tool exposes the same operation. An external agent
  may use blockers and bounded Gradle output to make project-specific changes
  and rerun; the standalone CLI does not claim to contain a general LLM.

## Consequences

The common migration path becomes one reviewable command with real build and
artifact evidence, and configuration failures are recoverable by default.
Recipe coverage is intentionally narrower than arbitrary Android Studio or
human-assisted migrations. New recipes require fixtures, real Gradle evidence,
and a fail-closed rule for ambiguous syntax.

Projects with an unrelated, already-broken root `build` task can still produce
scoped selected-library evidence without hiding the failure. The report keeps
both command outputs and explicitly avoids claiming whole-repository health.
Timeouts, cancellations, empty evidence, and non-identical failures remain
fatal and use the normal automatic rollback path.
