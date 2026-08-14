#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "Usage: scripts/build-release.sh VERSION DIST_DIR" >&2
  exit 2
fi

version="$1"
dist_dir="$2"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
license_stage="$(mktemp -d)"
trap 'rm -rf "${license_stage}"' EXIT
targets=(
  darwin/amd64
  darwin/arm64
  linux/amd64
  linux/arm64
  windows/amd64
  windows/arm64
)

mkdir -p "${dist_dir}"
"${script_dir}/collect-licenses.sh" "${license_stage}/third_party_licenses"
export AARGRADE_RELEASE_LICENSES_DIR="${license_stage}/third_party_licenses"

for target in "${targets[@]}"; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  "${script_dir}/package-release.sh" \
    "${version}" "${target_os}" "${target_arch}" "${dist_dir}"
done

"${script_dir}/checksum-release.sh" "${dist_dir}"
"${script_dir}/verify-release.sh" "${version}" "${dist_dir}"
