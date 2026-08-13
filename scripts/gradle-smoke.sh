#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd "${script_dir}/.." && pwd)"
gradle_bin="${AARGRADE_GRADLE_BIN:-}"
fixture_name="${AARGRADE_SMOKE_FIXTURE:-gradle-smoke}"

if [[ -z "${gradle_bin}" ]]; then
  gradle_bin="$(command -v gradle || true)"
fi
if [[ -z "${gradle_bin}" ]]; then
  echo "Set AARGRADE_GRADLE_BIN to a Gradle 9.4.1 executable." >&2
  exit 2
fi

smoke_root="$(mktemp -d)"
trap 'rm -rf "${smoke_root}"' EXIT
fixture_path="${project_root}/testdata/projects/${fixture_name}"
if [[ ! -d "${fixture_path}" ]]; then
  echo "Unknown smoke fixture: ${fixture_name}" >&2
  exit 2
fi
cp -R "${fixture_path}" "${smoke_root}/project"

cd "${project_root}"

go run ./cmd/aargrade host add \
  --project "${smoke_root}/project" \
  --apply \
  --format json >/dev/null

"${gradle_bin}" \
  -p "${smoke_root}/project" \
  :aargrade-upgrade-host:tasks \
  --all \
  --no-daemon

go run ./cmd/aargrade host remove \
  --project "${smoke_root}/project" \
  --apply \
  --format json >/dev/null

settings_name="settings.gradle.kts"
if [[ ! -f "${fixture_path}/${settings_name}" ]]; then
  settings_name="settings.gradle"
fi
cmp \
  "${fixture_path}/${settings_name}" \
  "${smoke_root}/project/${settings_name}"

test ! -e "${smoke_root}/project/.aargrade/state/upgrade-host.json"
