# ADR 0002: Make project mutation preview-first and ownership-scoped

- Status: Accepted for Milestone 1
- Date: 2026-08-13

## Context

The temporary application host is a workaround, not user-owned application
code. Removing an existing or edited module would be materially worse than
leaving a stale generated host behind.

## Decision

- `host add` and `host remove` show a deterministic preview by default.
- Mutation requires `--apply`.
- The generated host lives under `.aargrade/` and each file includes an
  ownership marker where its format allows one.
- Settings changes are enclosed in a unique, bounded marker block.
- State records content digests for generated files and the exact owned
  settings block.
- Removal fails closed if owned content differs. It preserves unrelated edits
  outside the owned block.
- Existing application modules and ambiguous library selection are refusal
  conditions in Milestone 1.

## Consequences

Automation needs an explicit apply flag, but previews are safe in local and CI
experiments. A failed cleanup is recoverable and explainable instead of risking
user data.
