# Library-only example

[한국어](README.md) | English

This is a normal, deliberately small Android library project with no
application module. It exists so users can try the current AARGrade workflow
without pointing the CLI at a production repository.

From the AARGrade repository root:

```bash
make demo
```

The demo runs a read-only diagnosis, creates an AGP 9.3 plan, and previews the
agent-style upgrade plus a temporary host. It does not change this project.

To exercise the complete reversible host lifecycle on a disposable copy:

```bash
cp -R examples/library-only /tmp/aargrade-library-example
go run ./cmd/aargrade host add \
  --project /tmp/aargrade-library-example \
  --apply
go run ./cmd/aargrade host remove \
  --project /tmp/aargrade-library-example \
  --apply
```

To build the example AAR, provide JDK 17 and Android SDK platform 35, then run:

```bash
make example-build
```

The checksum-pinned wrapper downloads Gradle 9.4.1 on first use.
The output is `examples/library-only/sdk/build/outputs/aar/sdk-release.aar`.

Use the same file as baseline and candidate to exercise the static verification
flow:

```bash
go run ./cmd/aargrade verify \
  --baseline-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar \
  --candidate-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar
```

The repository-level `aargrade.yml` contains the four AGP 4.2/9.2 ×
Java/Kotlin cells run by CI. See the root README for JDK and Android SDK
provisioning.
