# R8 Configuration Analysis

## Scope

This analysis covers the deliberately diagnostic Android library fixture at
`testdata/projects/kotlin-library`. It is not a production AAR configuration.

## Build configuration

- The fixture uses AGP 8.8.0 and Gradle 8.10.2.
- It is a library-only project. App-release settings such as
  `isMinifyEnabled`, `isShrinkResources`, and
  `proguard-android-optimize.txt` therefore do not apply inside this module.
- R8 full mode is not disabled. AGP 8 enables full mode by default.
- The fixture is intentionally below AGP 9 so `aargrade doctor` can exercise
  migration diagnostics. Keep that baseline isolated from the separate AGP 9
  smoke fixture; a production SDK should be verified on AGP 9 or newer.

## Consumer keep-rule actions

### `-dontoptimize`

Action: remove this option from any production library's consumer rules. It is
a global option rather than a library-scoped compatibility rule. Older consumer
toolchains can apply it to the whole consuming app, while AGP 9 filters global
options from library consumer rules. Its presence here is useful only as an
intentional negative test for `aargrade doctor`.

### `-keepclassmembers class dev.aargrade.fixture.Example { public *; }`

Action: remove this rule from a production configuration unless source-level
evidence proves that named members are reached dynamically. The fixture has no
`Example` implementation and no adjacent reflection, JNI, manual Parcelable,
or annotation-scanning code that could justify retaining every public member.
If a real SDK does use reflection, replace the wildcard with the exact fields,
methods, constructors, or annotated members reached at runtime.

## Subsumption

The two directives express different behavior, so neither is a narrower
replacement for the other. Both require independent action.

## Verification after applying these actions to a real SDK

Run the SDK's unit and integration tests, then build an optimized consumer app.
Use UI Automator for customer-visible flows that exercise
`dev.aargrade.fixture` (or the corresponding production package), concentrating
on reflective registration, serialization, JNI entry points, and component
startup where applicable. Compare runtime behavior and mapping/usage outputs
before publishing the candidate AAR.
