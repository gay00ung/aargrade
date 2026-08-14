#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repository_root="$(cd "${script_dir}/.." && pwd)"
upgrade_root="$(mktemp -d)"

case "${upgrade_root}" in
  /tmp/*|/private/tmp/*|/var/folders/*) ;;
  *)
    echo "Refusing unexpected temporary directory: ${upgrade_root}" >&2
    exit 1
    ;;
esac
trap 'rm -rf "${upgrade_root}"' EXIT

fixture="${repository_root}/testdata/projects/upgrade-agent"
project="${upgrade_root}/project"
cp -R "${fixture}" "${project}"
cp "${repository_root}/testdata/projects/upgrade-assistant-repro/gradlew" "${project}/gradlew"
cp "${repository_root}/testdata/projects/upgrade-assistant-repro/gradlew.bat" "${project}/gradlew.bat"
cp "${repository_root}/testdata/projects/upgrade-assistant-repro/gradle/wrapper/gradle-wrapper.jar" "${project}/gradle/wrapper/gradle-wrapper.jar"
chmod 0755 "${project}/gradlew"

if [[ -n "${AARGRADE_JAVA_HOME:-}" ]]; then
  export JAVA_HOME="${AARGRADE_JAVA_HOME}"
fi

go run "${repository_root}/cmd/aargrade" upgrade \
  --project "${project}" \
  --target-agp 9.2.0 \
  --library :sdk \
  --timeout 15m \
  --apply

test -f "${project}/sdk/build/outputs/aar/sdk-release.aar"
grep -Fq 'namespace = "dev.aargrade.agentfixture"' "${project}/sdk/build.gradle.kts"
grep -Fq 'compileSdk = 35' "${project}/sdk/build.gradle.kts"
grep -Fq 'minSdk = 21' "${project}/sdk/build.gradle.kts"
grep -Fq 'buildConfig = true' "${project}/sdk/build.gradle.kts"
grep -Fq 'JvmTarget.fromTarget("17")' "${project}/sdk/build.gradle.kts"
if grep -Fq 'package=' "${project}/sdk/src/main/AndroidManifest.xml"; then
  echo "Legacy source manifest package remains after namespace migration" >&2
  exit 1
fi
if grep -Fq 'org.jetbrains.kotlin.android' "${project}/gradle/libs.versions.toml"; then
  echo "Kotlin Android plugin catalog entry remains after Built-in Kotlin migration" >&2
  exit 1
fi

go run "${repository_root}/cmd/aargrade" migrate accept \
  --project "${project}"
test -f "${project}/.aargrade/state/migration.json"

go run "${repository_root}/cmd/aargrade" migrate rollback \
  --project "${project}" \
  --apply

for relative in \
  settings.gradle.kts \
  build.gradle.kts \
  gradle.properties \
  gradle/libs.versions.toml \
  gradle/wrapper/gradle-wrapper.properties \
  sdk/build.gradle.kts \
  sdk/src/main/AndroidManifest.xml \
  sdk/src/main/kotlin/dev/aargrade/agentfixture/Greeting.kt; do
  diff -u "${fixture}/${relative}" "${project}/${relative}"
done

test ! -e "${project}/.aargrade/state/migration.json"
printf 'Agent-style AGP 9 repairs, migration, build evidence, AAR inspection, and exact rollback succeeded.\n'
