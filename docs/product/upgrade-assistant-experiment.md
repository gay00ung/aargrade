# Upgrade Assistant host experiment

## Question

Does adding an AARGrade-owned application host change a reproducible AGP
Upgrade Assistant failure in a library-only project into a usable migration
flow?

This experiment tests only that causal claim. A successful Gradle sync or host
build alone is insufficient.

## Controlled fixture

Use a fresh copy of `testdata/projects/upgrade-assistant-repro`, which contains
one Android library, AGP 7.4.2, Gradle 7.5, and no application module. Never run
the experiment in a user project with uncommitted work.

Record:

- Android Studio full version and build number.
- JDK selected for Gradle.
- OS and architecture.
- Fixture SHA-256 or Git commit.
- Target AGP offered by Upgrade Assistant.
- Exact visible status/error text and IDE log excerpt.

## A — library-only baseline

1. Copy the fixture to a new temporary directory.
2. Open that directory in Android Studio and wait for sync to finish.
3. Open **Tools → AGP Upgrade Assistant**.
4. Record whether it loads a migration plan, remains unavailable, or reports
   that it cannot resolve an application Android model.
5. Do not run the upgrade.

## B — host treatment

In the same temporary copy:

```bash
aargrade host add --project /temporary/fixture --apply
```

Then:

1. Sync Gradle and wait for indexing to finish.
2. Refresh or reopen AGP Upgrade Assistant.
3. Record the same evidence as in A.
4. Do not run the upgrade; the dependent variable is plan availability, not a
   completed migration.
5. Remove the host and confirm the settings file returns to its original bytes:

```bash
aargrade host remove --project /temporary/fixture --apply
```

## Verdict

- **Validated:** A fails for an application-model reason and B succeeds on the
  same Studio version and fixture.
- **Rejected:** A already succeeds, or B fails in the same way.
- **Inconclusive:** sync/indexing failures, unsupported Studio/AGP pairing, or
  materially different environment state prevents a controlled comparison.

Only a validated result justifies positioning `host` as an Upgrade Assistant
workaround. Otherwise it remains a generic temporary consumer host and should
not drive the product narrative.
