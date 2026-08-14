#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd "${script_dir}/.." && pwd)"
example_project="${project_root}/examples/library-only"

run_aargrade() {
  if [[ -n "${AARGRADE_BIN:-}" ]]; then
    "${AARGRADE_BIN}" "$@"
    return
  fi
  (
    cd "${project_root}"
    go run ./cmd/aargrade "$@"
  )
}

printf 'AARGrade public example: read-only diagnosis\n\n'
run_aargrade doctor \
  --project "${example_project}" \
  --fail-on never

printf '\nAARGrade public example: AGP 9.3 migration plan\n\n'
run_aargrade plan \
  --project "${example_project}" \
  --target-agp 9.3.0

printf '\nAARGrade public example: bounded migration preview\n\n'
run_aargrade migrate \
  --project "${example_project}" \
  --target-agp 9.3.0

printf '\nAARGrade public example: temporary-host preview\n\n'
run_aargrade host add \
  --project "${example_project}"

printf '\nDemo complete. The example project was not modified.\n'
