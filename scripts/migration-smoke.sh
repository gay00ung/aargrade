#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repository_root="$(cd "${script_dir}/.." && pwd)"
migration_root="$(mktemp -d)"

case "${migration_root}" in
  /tmp/*|/private/tmp/*|/var/folders/*) ;;
  *)
    echo "Refusing unexpected temporary directory: ${migration_root}" >&2
    exit 1
    ;;
esac
trap 'rm -rf "${migration_root}"' EXIT

fixture="${repository_root}/testdata/projects/migrate-kotlin-catalog"
project="${migration_root}/project"
cp -R "${fixture}" "${project}"
cp "${repository_root}/testdata/projects/upgrade-assistant-repro/gradlew" "${project}/gradlew"
cp "${repository_root}/testdata/projects/upgrade-assistant-repro/gradlew.bat" "${project}/gradlew.bat"
cp "${repository_root}/testdata/projects/upgrade-assistant-repro/gradle/wrapper/gradle-wrapper.jar" "${project}/gradle/wrapper/gradle-wrapper.jar"
chmod 0755 "${project}/gradlew"

go run "${repository_root}/cmd/aargrade" migrate \
  --project "${project}" \
  --target-agp 9.2.0 \
  --apply

if [[ -n "${AARGRADE_JAVA_HOME:-}" ]]; then
  export JAVA_HOME="${AARGRADE_JAVA_HOME}"
fi

"${project}/gradlew" -p "${project}" help --no-daemon
"${project}/gradlew" -p "${project}" build --dry-run --no-daemon
"${project}/gradlew" -p "${project}" :sdk:assembleRelease --no-daemon
test -f "${project}/sdk/build/outputs/aar/sdk-release.aar"

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
  sdk/src/main/kotlin/dev/aargrade/migratefixture/Greeting.kt; do
  diff -u "${fixture}/${relative}" "${project}/${relative}"
done

test ! -e "${project}/.aargrade/state/migration.json"
printf 'AGP 9 migration, Built-in Kotlin compilation, AAR assembly, and exact rollback succeeded.\n'
