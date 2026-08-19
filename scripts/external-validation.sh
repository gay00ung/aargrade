#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repository_root="$(cd "${script_dir}/.." && pwd)"
owns_workspace=0

if [[ -n "${AARGRADE_EXTERNAL_ROOT:-}" ]]; then
  validation_root="${AARGRADE_EXTERNAL_ROOT}"
  mkdir -p "${validation_root}"
else
  validation_root="$(mktemp -d)"
  owns_workspace=1
fi

cleanup() {
  if [[ "${owns_workspace}" != 1 || "${AARGRADE_KEEP_EXTERNAL:-0}" == 1 ]]; then
    printf 'External validation workspace: %s\n' "${validation_root}"
    return
  fi
  case "${validation_root}" in
    /tmp/*|/private/tmp/*|/var/folders/*) rm -rf "${validation_root}" ;;
    *) printf 'Refusing to remove unexpected workspace: %s\n' "${validation_root}" >&2 ;;
  esac
}
trap cleanup EXIT

clone_at() {
  local name="$1"
  local url="$2"
  local commit="$3"
  local destination="${validation_root}/${name}"
  if [[ -e "${destination}" ]]; then
    printf 'Refusing to reuse existing path: %s\n' "${destination}" >&2
    exit 2
  fi
  git init --quiet "${destination}"
  git -C "${destination}" remote add origin "${url}"
  git -C "${destination}" fetch --quiet --depth 1 origin "${commit}"
  git -C "${destination}" checkout --quiet --detach FETCH_HEAD
}

binary="${validation_root}/aargrade"
(
  cd "${repository_root}"
  go build -trimpath -o "${binary}" ./cmd/aargrade
)

clone_at timber https://github.com/JakeWharton/timber.git c51c0cad85bfd4c4c452222dae5ec49e8988303a
clone_at picasso https://github.com/square/picasso.git e94d6116e3fff99b3932103e0ab22ff5b44a1273
clone_at lottie https://github.com/airbnb/lottie-android.git 05ea92e90381eb8a8ae06855ea2b74f322bebbec
clone_at glide https://github.com/bumptech/glide.git bf4901de78c206ed21ce73f5d25aa2bbf35ebf3c
clone_at adjust https://github.com/adjust/android_sdk.git b31ee274a2d189c4ff705c2ee6c47ad09ca5eb62

for project_name in timber picasso lottie glide; do
  printf '\n== doctor: %s ==\n' "${project_name}"
  "${binary}" doctor \
    --project "${validation_root}/${project_name}" \
    --fail-on never
done

printf '\n== doctor: adjust ==\n'
"${binary}" doctor \
  --project "${validation_root}/adjust/Adjust" \
  --fail-on never

printf '\n== upgrade preview: Timber AGP 9.2.1 -> 9.3.1 ==\n'
"${binary}" upgrade \
  --project "${validation_root}/timber" \
  --target-agp 9.3.1 \
  --library :timber

printf '\n== upgrade preview: Picasso AGP 8.7.2 -> 9.3.1 ==\n'
"${binary}" upgrade \
  --project "${validation_root}/picasso" \
  --target-agp 9.3.1 \
  --library :picasso

printf '\n== expected fail-closed preview: Lottie RefreshVersions ==\n'
if "${binary}" upgrade \
  --project "${validation_root}/lottie" \
  --target-agp 9.3.1 \
  --library :lottie; then
  printf 'Expected Lottie to stop at its RefreshVersions boundary.\n' >&2
  exit 1
else
  status=$?
  if [[ "${status}" != 1 ]]; then
    printf 'Lottie returned unexpected exit code %s.\n' "${status}" >&2
    exit "${status}"
  fi
fi

printf '\n== upgrade preview: Glide AGP 9.2.0 -> 9.3.1 ==\n'
"${binary}" upgrade \
  --project "${validation_root}/glide" \
  --target-agp 9.3.1 \
  --library :library

printf '\n== upgrade preview: Adjust AGP 9.2.1 -> 9.3.1 ==\n'
"${binary}" upgrade \
  --project "${validation_root}/adjust/Adjust" \
  --target-agp 9.3.1 \
  --library :sdk-core

if [[ "${AARGRADE_EXTERNAL_APPLY:-0}" == 1 ]]; then
  printf '\n== applied verification: Timber ==\n'
  "${binary}" upgrade \
    --project "${validation_root}/timber" \
    --target-agp 9.3.1 \
    --library :timber \
    --apply
  "${binary}" migrate accept --project "${validation_root}/timber" --apply

  printf '\n== applied verification: Glide ==\n'
  "${binary}" upgrade \
    --project "${validation_root}/glide" \
    --target-agp 9.3.1 \
    --library :library \
    --apply
  "${binary}" migrate accept --project "${validation_root}/glide" --apply
fi

printf '\nPinned external validation completed successfully.\n'
