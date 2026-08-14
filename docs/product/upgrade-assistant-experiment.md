# Upgrade Assistant host experiment

[한국어](upgrade-assistant-experiment.ko.md)

**Status: validated in the environment below on 2026-08-14.** The result proves
the causal claim for one controlled fixture and Android Studio build; it is not
a promise that every Studio version or Gradle structure behaves identically.

## Question

Does adding an AARGrade-owned application host change a reproducible AGP
Upgrade Assistant failure in a library-only project into a usable migration
plan?

The dependent variable is plan availability. A Gradle sync or generated-host
build alone is not sufficient, and no upgrade step was executed.

## Recorded environment

| Item | Value |
| --- | --- |
| Android Studio | Panda 2, 2025.3.2 RC 1; bundle version `2025.3`; build `AI-253.30387.90.2532.14901460` |
| OS | macOS 26.3.1 (`25D2128`), arm64 |
| Gradle JDK | Microsoft OpenJDK 17.0.16 (`17.0.16+8-LTS`) |
| Fixture | one `com.android.library` module `:sdk`; no application module |
| Toolchain | AGP 7.4.2, Gradle 7.5, compileSdk 33 |
| Original settings digest | `4d52544df9939fa68da4e905b0c316b5ff5eb3faa71f63f1d8b49250e7c3dd4c` |
| Upgrade target selected by Studio | AGP 8.13.2; the IDE reported 9.1.0-rc01 as its latest known version |

The first IDE sync selected JDK 21, which Gradle 7.5 does not support. Android
Studio's project-specific **Apply compatible Gradle JDK configuration and
sync** action selected JDK 17; the fixture then synced successfully. The JDK
selection failure was classified as environment setup and excluded from the
product verdict.

## A — library-only baseline

The runnable fixture in `testdata/projects/upgrade-assistant-repro` was copied
to a temporary Git repository and its wrapper completed `help` successfully.
Android Studio sync then completed with `BUILD SUCCESSFUL`.

Opening **Tools → AGP Upgrade Assistant** produced no usable plan. The IDE log
recorded the reason twice:

```text
WARN - Upgrade Assistant - Unable to obtain application's Android Project
```

This satisfies Gate A: the problem was reproduced in a clean library-only
project, independently of AARGrade.

## B — AARGrade host treatment

The following command was run in the same temporary project:

```bash
aargrade host add --project /temporary/fixture --apply
```

It added only the owned settings block and `.aargrade/upgrade-host`, with an
application module depending on `:sdk`. The next sync completed successfully
and Android Studio immediately reported:

```text
Android Gradle plugin version 7.4.2 has an upgrade available.
Start the AGP Upgrade Assistant to update this project's AGP version.
```

The assistant opened a real plan titled `Upgrade project from AGP 7.4.2`,
selected AGP 8.13.2, and listed concrete Gradle, R8, BuildConfig, R-class, and
AGP dependency steps. The corresponding log changed from the baseline warning
to:

```text
INFO - Upgrade Assistant - Starting AGP Upgrade Assistant
INFO - Upgrade Assistant - Gradle model version: 7.4.2, latest known version for IDE: 9.1.0-rc01
INFO - Upgrade Assistant - Gradle upgrade state: GradlePluginUpgradeState(importance=RECOMMEND, target=8.13.2)
```

No **Run selected steps** action was executed.

## Removal and reversal

```bash
aargrade host remove --project /temporary/fixture --apply
```

Removal deleted only AARGrade-owned files. The settings digest returned exactly
to `4d52544df9939fa68da4e905b0c316b5ff5eb3faa71f63f1d8b49250e7c3dd4c`.
After syncing the restored library-only model, the still-open assistant changed
to `Upgrade project from unknown AGP` and `Android Gradle Plugin not present`.
This A→B→A reversal strengthens the causal result.

## Verdict and product consequence

**Validated for this fixture and Studio build.** Baseline A failed because the
assistant could not obtain an application Android project; treatment B exposed
a usable migration plan; removing the treatment restored the failure state.
Gate C therefore passed and `host add/remove` remains a supported Upgrade
Assistant workaround.

The command must still remain preview-first and ownership-safe. Documentation
must tell users to reproduce their own failure first because a future Studio
version, unsupported Gradle structure, or unrelated sync problem can produce a
different result.

## Reproducing it

The fixture includes its Gradle 7.5 Wrapper and pinned distribution checksum.
Follow its [Korean reproduction guide](../../testdata/projects/upgrade-assistant-repro/README.md),
or use this controlled sequence:

1. Copy the fixture to a fresh temporary directory and commit the baseline.
2. Run `./gradlew help --no-daemon` with a Gradle-7.5-compatible JDK.
3. Sync in Android Studio and record the baseline Upgrade Assistant result.
4. Preview, then apply `aargrade host add`; sync and record the result again.
5. Do not run the upgrade.
6. Preview, then apply `aargrade host remove`; verify exact settings
   restoration.

CI verifies that the fixture remains runnable and that the generated host
configures under AGP 7.4.2. The Android Studio UI observation remains a manual
validation because a headless Gradle build cannot truthfully substitute for
the IDE behavior under test.
