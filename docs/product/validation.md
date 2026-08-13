# Validation log

Checked on **2026-08-13**. This document separates verified facts from product
hypotheses; it is not a claim that market demand has already been proven.

## Verified ecosystem facts

| Claim | Evidence | Product consequence |
| --- | --- | --- |
| Upgrade Assistant uses static Gradle-file analysis, has documented unsupported structures, and may still fail on conforming projects. | [Official Upgrade Assistant guide](https://developer.android.com/build/agp-upgrade-assistant) | `doctor` should explain analyzability and uncertainty before suggesting a workaround. |
| AGP 9 enables built-in Kotlin and removes access to important legacy DSL/Variant API surfaces by default. | [AGP 9.0 release notes](https://developer.android.com/build/releases/agp-9-0-0-release-notes) | Kotlin plugins, opt-outs, and legacy APIs are first-class diagnostics. |
| AGP 9.3 is the current stable line; AGP 9.4 changes are in preview and AGP 10 is planned for late 2026. | [AGP 9.3 release notes](https://developer.android.com/build/releases/agp-9-3-0-release-notes), [AGP roadmap](https://developer.android.com/build/releases/gradle-plugin-roadmap) | Compatibility knowledge must be versioned data, not scattered conditionals. |
| AAR metadata can declare minimum AGP and compile SDK requirements. | [AAR metadata API](https://developer.android.com/reference/tools/gradle-api/9.2/com/android/build/api/variant/AarMetadata) | `verify` must inspect metadata before running expensive consumers. |
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
- Go available: 1.26.0.
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

The reusable check is `scripts/gradle-smoke.sh`; CI runs both toolchain cells.
This proves that the generated Gradle models configure at the tested endpoints.
It does **not** yet prove that the host changes Upgrade Assistant behavior.

### External repository trials

`doctor` was run read-only against shallow clones of these public repositories:

| Repository and commit | Useful result | Noise or limitation found | Change made |
| --- | --- | --- | --- |
| [JakeWharton/timber](https://github.com/JakeWharton/timber) `5ddea1c9c912ea785ab752f244168260ae8dabd8` | Classified the KMP Android library and sample app; resolved AGP 9.3.1. | Initially warned about kapt in a non-Android lint module. | Restricted kapt/KSP migration findings to resolved Android module kinds. |
| [square/picasso](https://github.com/square/picasso) `e94d6116e3fff99b3932103e0ab22ff5b44a1273` | Classified five libraries and one sample app; resolved AGP 8.7.2. | Initially missed Groovy `include 'module'` and classpath aliases declared in catalog `[libraries]`. | Added colonless include normalization and catalog library-coordinate resolution. |
| [airbnb/lottie-android](https://github.com/airbnb/lottie-android) `05ea92e90381eb8a8ae06855ea2b74f322bebbec` | Classified application, library, and test modules; found Android Kotlin, kapt, and KSP migration signals. | AGP stays unresolved because RefreshVersions owns its effective mapping. | Added an explicit RefreshVersions limitation instead of guessing the AGP version; classified `com.android.test` separately. |

These trials validate useful structural coverage across Kotlin DSL, Groovy DSL,
KMP Android, version catalogs, classic buildscript classpaths, Android test
modules, and dynamic version management. They are technical trials, not demand
validation or proof of Upgrade Assistant failure.

### Upgrade Assistant A/B status

The controlled protocol is documented in
`docs/product/upgrade-assistant-experiment.md` with a dedicated AGP 7.4.2
library-only fixture. An Android Studio UI run was attempted on 2026-08-13, but
the macOS session was locked, so no UI actions were performed and no result was
inferred. Gate A and Gate C remain open.

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
- The Consumer R8 negative fixture was reviewed separately: its global
  `-dontoptimize` option is a high-confidence `doctor` finding, while the
  remaining member rule requires source-level evidence and is not automatically
  rewritten.

## Unverified hypotheses

1. A library-only project currently causes Upgrade Assistant to miss an Android
   model, and adding a minimal application host reliably fixes it.
2. SDK maintainers will adopt a new CLI instead of maintaining bespoke CI
   sample apps.
3. A default compatibility matrix can be small enough for routine pull-request
   use.
4. Static diagnostics will remain useful across convention plugins and dynamic
   build logic.
5. Teams will want MCP after CI output is available.

The delivery gates in the product plan are designed to test these hypotheses
without pretending they are facts.
