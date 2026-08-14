#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: scripts/collect-licenses.sh OUTPUT_DIR" >&2
  exit 2
fi

output_dir="$1"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd "${script_dir}/.." && pwd)"
go_bin="${GO:-go}"

if [[ -e "${output_dir}" ]]; then
  echo "License output path already exists: ${output_dir}" >&2
  exit 2
fi
output_parent="$(dirname "${output_dir}")"
output_name="$(basename "${output_dir}")"
mkdir -p "${output_parent}"
output_parent="$(cd "${output_parent}" && pwd)"
output_dir="${output_parent}/${output_name}"

(
  cd "${project_root}"
  "${go_bin}" run github.com/google/go-licenses/v2@v2.0.1 save \
    ./cmd/aargrade \
    --ignore github.com/gay00ung/aargrade \
    --save_path="${output_dir}"
)

go_root="$("${go_bin}" env GOROOT)"
go_license="${go_root}/LICENSE"
if [[ ! -f "${go_license}" && -f "${go_root}/../LICENSE" ]]; then
  go_license="${go_root}/../LICENSE"
fi
if [[ ! -f "${go_license}" ]]; then
  echo "Go toolchain license was not found under ${go_root}." >&2
  exit 1
fi
mkdir -p "${output_dir}/go-toolchain"
cp "${go_license}" "${output_dir}/go-toolchain/LICENSE"
if [[ -f "${go_root}/PATENTS" ]]; then
  cp "${go_root}/PATENTS" "${output_dir}/go-toolchain/PATENTS"
fi

if [[ ! -f "${output_dir}/github.com/modelcontextprotocol/go-sdk/LICENSE" ]]; then
  echo "MCP SDK license was not collected." >&2
  exit 1
fi
if [[ ! -f "${output_dir}/go.yaml.in/yaml/v3/NOTICE" ]]; then
  echo "YAML attribution notice was not collected." >&2
  exit 1
fi
