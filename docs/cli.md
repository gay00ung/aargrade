# CLI contract

## `doctor`

```bash
aargrade doctor \
  --project /path/to/project \
  --format text \
  --fail-on error
```

Options:

- `--project`: Gradle root containing one settings file.
- `--format`: `text` or versioned `json`.
- `--fail-on`: `info`, `warn`, `error`, or `never`.

Exit codes:

- `0`: a trustworthy report was produced and no finding met the threshold.
- `1`: a trustworthy report was produced and a finding met the threshold.
- `2`: arguments, discovery, analysis, or report writing failed.

`doctor` never invokes Gradle and never writes to the target project. It is a
conservative static inventory, so unresolved dynamic build logic is a finding,
not an invitation to guess.

## `host add`

```bash
# Preview
aargrade host add --project /path/to/library

# Apply the reviewed plan
aargrade host add --project /path/to/library --apply
```

The command refuses to auto-select when an application module already exists,
when more than one library exists, or when AGP/compileSdk cannot be resolved.
Use `--library`, `--agp-version`, or `--compile-sdk` to supply reviewed values.

The generated module is mapped to `.aargrade/upgrade-host` and its ownership
state is stored at `.aargrade/state/upgrade-host.json`.

## `host remove`

```bash
# Preview
aargrade host remove --project /path/to/library

# Remove unchanged owned content
aargrade host remove --project /path/to/library --apply
```

Removal verifies the exact settings block and SHA-256 of every generated file.
It fails closed on modification or missing content. Settings edits outside the
owned block are preserved. If removal refuses, inspect the reported path and
reconcile it manually; do not delete user-modified content blindly.

## JSON compatibility

`doctor` reports and host plans currently use `schemaVersion: 1`. Within a
schema version, fields are additive and verdict semantics do not change. A
breaking rename or semantic change requires a new schema version.
