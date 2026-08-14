#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: scripts/checksum-release.sh DIST_DIR" >&2
  exit 2
fi

dist_dir="$1"
if [[ ! -d "${dist_dir}" ]]; then
  echo "Release directory does not exist: ${dist_dir}" >&2
  exit 2
fi

(
  cd "${dist_dir}"
  archive_names="$({
    find . -maxdepth 1 -type f -name 'aargrade_*.tar.gz' -print
    find . -maxdepth 1 -type f -name 'aargrade_*.zip' -print
  } | sed 's#^\./##' | LC_ALL=C sort)"

  if [[ -z "${archive_names}" ]]; then
    echo "No release archives found in ${dist_dir}." >&2
    exit 2
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    while IFS= read -r archive_name; do
      sha256sum "${archive_name}"
    done <<<"${archive_names}" >checksums.txt
  else
    while IFS= read -r archive_name; do
      shasum -a 256 "${archive_name}"
    done <<<"${archive_names}" >checksums.txt
  fi
)

printf '%s\n' "${dist_dir}/checksums.txt"
