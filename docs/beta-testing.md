# AARGrade v1 external beta validation

[한국어](beta-testing.ko.md)

AARGrade needs evidence from real Android SDK or AAR maintainers before
`v1.0.0`, not only trials that the tool author selected and ran. Private
projects are welcome; you do not need to disclose source code or customer
names.

## Useful validation paths

Run at least one of these paths in a real project:

1. review an `upgrade` preview, run `--apply`, and confirm either an explicit
   rollback after success or the default automatic rollback after failure;
2. compare a currently released AAR and candidate AAR with `verify`; or
3. build real Java or Kotlin consumer cells with `matrix`.

A preview-only result is still useful demand and diagnostic evidence. Apply and
restore or consumer-build evidence is more important for the v1 safety gate.

## Before you begin

- Test only a project you maintain or are authorized to evaluate.
- Start from a clean branch or temporary worktree.
- Keep the currently released baseline AAR when possible.
- Review AARGrade output and `.aargrade` logs before sharing them.
- Do not publish tokens, signing material, customer code, internal repository
  addresses, or private local paths.

```bash
git status --short
aargrade version
```

Use the latest public beta, currently `v0.1.0-beta.2`.

## 1. Read-only diagnosis and preview

Run these commands from the Gradle project root. Neither command changes Gradle
configuration.

```bash
aargrade doctor --project . --fail-on never
aargrade upgrade --project . --target-agp 9.3.1
```

If module selection is ambiguous, select the Gradle project that publishes the
AAR:

```bash
aargrade upgrade \
  --project . \
  --target-agp 9.3.1 \
  --library :sdk
```

A `BLOCKED` preview and exit code `1` may mean AARGrade refused to guess about
unknown Gradle logic. Record the exact blocker.

## 2. Apply and restore

After reviewing the preview, run only in a clean branch or worktree. Omit
`--baseline-aar` when no released artifact is available.

```bash
aargrade upgrade \
  --project . \
  --target-agp 9.3.1 \
  --library :sdk \
  --baseline-aar releases/sdk-current.aar \
  --apply
```

AARGrade records the whole-project dry-run before mutation, compares the
post-migration result, and verifies the selected library task graph and AAR
assembly. A failed apply rolls back AARGrade-owned configuration by default.

A successful apply leaves rollback state. Preview and apply restoration when
you do not intend to keep the migration:

```bash
aargrade migrate rollback --project .
aargrade migrate rollback --project . --apply
git status --short
```

To keep reviewed changes, accept them instead:

```bash
aargrade migrate accept --project .
aargrade migrate accept --project . --apply
```

Avoid `--keep-failed-changes` during routine beta validation because it disables
automatic rollback.

## 3. Consumer matrix (optional)

Adapt the repository [`aargrade.yml`](../aargrade.yml) to declare the baseline,
candidate, and supported AGP/Gradle/JDK/language cells. You must provide the
JDKs. `--allow-downloads` permits only official checksum-verified Gradle
distribution downloads.

```bash
export AARGRADE_JAVA_HOME_11=/path/to/jdk-11
export AARGRADE_JAVA_HOME_17=/path/to/jdk-17

aargrade matrix --config aargrade.yml --allow-downloads
```

A direct AAR dependency does not carry Maven POM transitives. Add the SDK's
published runtime dependencies to each cell's `dependencies` list.

## Share the result

Use the
[beta validation form](https://github.com/gay00ung/aargrade/issues/new?template=beta-validation.yml)
and share only:

- AARGrade version and OS;
- current/target AGP, Gradle, JDK, and DSL;
- the executed workflow and final verdict;
- what worked, where AARGrade stopped safely, or what failed unexpectedly; and
- reviewed, sanitized findings or log excerpts.

Link a project only when it is public. An anonymous description such as
“private multi-module payment SDK” is sufficient. Report vulnerabilities or
actual secret exposure through the [Security Policy](../SECURITY.md), not a
public issue.

## What counts toward v1

- The reporter maintains/contributes to the SDK or is authorized to evaluate
  it.
- A published `v0.1.0-beta.2` or newer binary was used.
- A real apply/restore or consumer-build result was observed.
- Safety blockers and suspected false verdicts are reported rather than hidden.

Independent dogfood runs performed by the AARGrade author remain technical
evidence, but they do not count as external-user adoption.
