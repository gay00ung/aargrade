#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "Usage: scripts/verify-release.sh VERSION DIST_DIR" >&2
  exit 2
fi

version="$1"
dist_dir="$2"
release_version="${version#v}"
targets=(
  darwin/amd64
  darwin/arm64
  linux/amd64
  linux/arm64
  windows/amd64
  windows/arm64
)

if [[ ! -f "${dist_dir}/checksums.txt" ]]; then
  echo "Missing ${dist_dir}/checksums.txt" >&2
  exit 2
fi

(
  cd "${dist_dir}"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c checksums.txt
  else
    shasum -a 256 -c checksums.txt
  fi
)

for target in "${targets[@]}"; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  archive_root="aargrade_${release_version}_${target_os}_${target_arch}"
  archive_path="${dist_dir}/${archive_root}.tar.gz"
  binary_name="aargrade"

  if [[ "${target_os}" == "windows" ]]; then
    archive_path="${dist_dir}/${archive_root}.zip"
    binary_name="aargrade.exe"
  fi
  if [[ ! -f "${archive_path}" ]]; then
    echo "Missing release archive: ${archive_path}" >&2
    exit 1
  fi

  if [[ "${target_os}" == "windows" ]]; then
    archive_listing="$(unzip -Z1 "${archive_path}")"
  else
    archive_listing="$(tar -tzf "${archive_path}")"
  fi

  for required_file in "${binary_name}" LICENSE README.md README.en.md; do
    if ! grep -Fxq "${archive_root}/${required_file}" <<<"${archive_listing}"; then
      echo "${archive_path} is missing ${archive_root}/${required_file}" >&2
      exit 1
    fi
  done

  for third_party_file in \
    third_party_licenses/go-toolchain/LICENSE \
    third_party_licenses/github.com/modelcontextprotocol/go-sdk/LICENSE \
    third_party_licenses/go.yaml.in/yaml/v3/NOTICE; do
    if ! grep -Fxq "${archive_root}/${third_party_file}" <<<"${archive_listing}"; then
      echo "${archive_path} is missing ${archive_root}/${third_party_file}" >&2
      exit 1
    fi
  done
done

host_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${host_os}" in
  darwin|linux)
    ;;
  *)
    echo "Archive structure verified; native version check skipped on ${host_os}."
    exit 0
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64)
    host_arch="amd64"
    ;;
  arm64|aarch64)
    host_arch="arm64"
    ;;
  *)
    echo "Archive structure verified; native version check skipped on $(uname -m)."
    exit 0
    ;;
esac

native_root="aargrade_${release_version}_${host_os}_${host_arch}"
native_archive="${dist_dir}/${native_root}.tar.gz"
extract_root="$(mktemp -d)"
trap 'rm -rf "${extract_root}"' EXIT
tar -C "${extract_root}" -xzf "${native_archive}"

actual_version="$("${extract_root}/${native_root}/aargrade" version)"
expected_version="aargrade ${version}"
if [[ "${actual_version}" != "${expected_version}" ]]; then
  echo "Embedded version mismatch: got ${actual_version}, want ${expected_version}" >&2
  exit 1
fi

printf 'Verified six archives and embedded version %s.\n' "${version}"
