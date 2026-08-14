# AARGrade release guide

[한국어](releasing.ko.md)

Pushing a version tag runs the release workflow. It revalidates the source,
builds six CLI archives and their SHA-256 checksums, and publishes a GitHub
Release.

## Assets

| OS | CPU | Asset |
| --- | --- | --- |
| macOS | Intel | `aargrade_<VERSION>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `aargrade_<VERSION>_darwin_arm64.tar.gz` |
| Linux | x86-64 | `aargrade_<VERSION>_linux_amd64.tar.gz` |
| Linux | ARM64 | `aargrade_<VERSION>_linux_arm64.tar.gz` |
| Windows | x86-64 | `aargrade_<VERSION>_windows_amd64.zip` |
| Windows | ARM64 | `aargrade_<VERSION>_windows_arm64.zip` |

Each archive contains the executable, both READMEs, and the Apache-2.0
`LICENSE`. `third_party_licenses` contains notices for the linked modules and
Go toolchain. `checksums.txt` covers every archive.

## Local gate

```bash
make check
make vuln
make release-check
```

To inspect assets with the intended version without publishing anything:

```bash
make release RELEASE_VERSION=v0.1.0-beta.1
```

The files are written under `dist/v0.1.0-beta.1`.

## Publish a beta

Tags must use SemVer with a `v` prefix. Increment prereleases rather than
moving a published tag.

```bash
git switch main
git pull --ff-only
git status --short
git tag -a v0.1.0-beta.1 -m "AARGrade v0.1.0-beta.1"
git push origin v0.1.0-beta.1
```

`.github/workflows/release.yml` then:

1. validates the tag and requires its commit to be merged into `main`;
2. runs tests, vet, vulnerability scanning, workflow linting, and demo;
3. cross-compiles macOS, Linux, and Windows for amd64 and arm64;
4. verifies archive contents, SHA-256 hashes, and the embedded version; and
5. creates a GitHub prerelease for tags containing a hyphen.

Inspect it with:

```bash
gh run list --workflow Release --limit 5
gh release view v0.1.0-beta.1
```

## Publish stable

After beta validation, push a tag without prerelease metadata:

```bash
git tag -a v0.1.0 -m "AARGrade v0.1.0"
git push origin v0.1.0
```

The workflow publishes it as a normal GitHub Release.

## Failure behavior

- A failed quality or packaging gate creates no release.
- Rerunning the same workflow replaces assets on an existing release.
- Never move a published tag; fix the source and publish the next prerelease
  or patch version.
- Publishing uses the workflow's scoped `GITHUB_TOKEN`, not a stored personal
  token.
