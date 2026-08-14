#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "Usage: scripts/package-release.sh VERSION GOOS GOARCH OUTPUT_DIR" >&2
  exit 2
fi

version="$1"
target_os="$2"
target_arch="$3"
output_dir="$4"

if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Release version must be SemVer with a v prefix (for example v0.1.0-beta.1)." >&2
  exit 2
fi

case "${target_os}/${target_arch}" in
  darwin/amd64|darwin/arm64|linux/amd64|linux/arm64|windows/amd64|windows/arm64)
    ;;
  *)
    echo "Unsupported release target: ${target_os}/${target_arch}" >&2
    exit 2
    ;;
esac

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd "${script_dir}/.." && pwd)"
go_bin="${GO:-go}"
release_version="${version#v}"
archive_root="aargrade_${release_version}_${target_os}_${target_arch}"
stage_root="$(mktemp -d)"
package_root="${stage_root}/${archive_root}"
trap 'rm -rf "${stage_root}"' EXIT

mkdir -p "${output_dir}" "${package_root}"
output_dir="$(cd "${output_dir}" && pwd)"

binary_name="aargrade"
if [[ "${target_os}" == "windows" ]]; then
  binary_name="aargrade.exe"
fi

(
  cd "${project_root}"
  CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
    "${go_bin}" build \
      -trimpath \
      -ldflags "-s -w -X=main.version=${version}" \
      -o "${package_root}/${binary_name}" \
      ./cmd/aargrade
)

cp "${project_root}/LICENSE" "${package_root}/LICENSE"
cp "${project_root}/README.md" "${package_root}/README.md"
cp "${project_root}/README.en.md" "${package_root}/README.en.md"
if [[ -n "${AARGRADE_RELEASE_LICENSES_DIR:-}" ]]; then
  if [[ ! -d "${AARGRADE_RELEASE_LICENSES_DIR}" ]]; then
    echo "Shared release license directory does not exist: ${AARGRADE_RELEASE_LICENSES_DIR}" >&2
    exit 2
  fi
  cp -R "${AARGRADE_RELEASE_LICENSES_DIR}" "${package_root}/third_party_licenses"
else
  "${script_dir}/collect-licenses.sh" "${package_root}/third_party_licenses"
fi

if [[ "${target_os}" == "windows" ]]; then
  archive_path="${output_dir}/${archive_root}.zip"
  rm -f "${archive_path}"
  (
    cd "${stage_root}"
    zip -q -r "${archive_path}" "${archive_root}"
  )
else
  archive_path="${output_dir}/${archive_root}.tar.gz"
  rm -f "${archive_path}"
  tar -C "${stage_root}" -czf "${archive_path}" "${archive_root}"
fi

printf '%s\n' "${archive_path}"
