# AARGrade

한국어 | [English](README.en.md)

[![CI](https://github.com/gay00ung/aargrade/actions/workflows/ci.yml/badge.svg)](https://github.com/gay00ung/aargrade/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

> 고객 프로젝트를 깨뜨리지 않고 AAR을 업그레이드하세요.

AARGrade는 **Android 라이브러리와 SDK 제작자를 위한 AGP 마이그레이션
안전망**입니다. 새 Android Gradle Plugin(AGP)으로 만든 AAR이 기존 고객
환경에서도 빌드되는지, 감으로 판단하지 않고 재현 가능한 증거로 확인합니다.

- **AAR**: Android 라이브러리를 고객에게 배포하는 파일 형식
- **AGP**: Android 프로젝트를 빌드하는 Gradle 플러그인
- **기준 AAR**: 현재 고객에게 배포 중인 정상 버전
- **후보 AAR**: AGP 업그레이드 후 새로 만든 버전

## 아주 간단히 말하면

```text
현재 프로젝트 진단
→ AGP 업그레이드 변경안과 자동 수리 미리보기
→ 적용 후 Gradle 구성·dry-run·AAR 빌드 자동 검증
→ 기준 AAR과 후보 AAR 비교 및 선택적 고객 매트릭스 빌드
→ 실패 원인 분류와 안전한 자동 롤백
→ 결과를 사람, CI, MCP 에이전트가 같은 형식으로 사용
```

처음에는 `upgrade`를 쓰면 됩니다. 이 명령이 진단, 알려진 AGP 수리,
마이그레이션, 실제 Gradle/AAR 검증과 선택적 고객 매트릭스를 한 흐름으로
묶습니다. 실패하면 AARGrade가 소유한 변경을 기본적으로 자동 롤백합니다.
`migrate`는 Gradle 실행 없이 변경만 세밀하게 다루고 싶을 때 쓰는 하위
명령입니다.

AARGrade가 임의의 Gradle 코드를 추측해서 고치지는 않습니다. 자동 수리는
공식 AGP 규칙에 근거해 정적으로 증명 가능한 형태만 다루며, kapt, 구형
Variant API, convention plugin 내부처럼 프로젝트별 판단이 필요한 항목은
이유와 다음 조치를 반환합니다. MCP로 연결하면 에이전트가 그 구조화된
결과를 읽고 프로젝트별 수정과 재실행을 이어갈 수 있습니다.

## 지금 실제로 되는 기능

| 명령 | 하는 일 | 프로젝트 파일 변경 |
| --- | --- | --- |
| `doctor` | 프로젝트 구조와 AGP 마이그레이션 위험 진단 | 없음 |
| `plan` | 목표 AGP에 맞는 Gradle·JDK와 작업 순서 작성 | 없음 |
| `upgrade` | 자동 수리·마이그레이션·실제 빌드·AAR 검사·선택적 매트릭스를 한 번에 실행 | 미리보기는 변경 없음, `--apply` 때만 설정 변경/빌드 |
| `migrate` | 지원하는 AGP·Wrapper·Built-in Kotlin 변경 미리보기/적용/채택/롤백 | `--apply`를 쓴 경우만 |
| `verify` | AAR 빌드/검사 및 기준 AAR과 정적 호환성 비교 | 소스는 안 바꾸지만 Gradle 빌드 결과 생성 가능 |
| `matrix` | 격리된 Java·Kotlin 고객 앱을 만들어 실제 빌드 | `.aargrade/matrix` 아래에 증거 생성 |
| `host add/remove` | 앱 model이 없을 때 Upgrade Assistant용 임시 앱을 안전하게 생성/제거 | `--apply`를 쓴 경우만 |
| `mcp serve` | 위 기능을 MCP 호환 에이전트에 제공 | 호출한 도구의 규칙을 따름 |

`verify`의 내장 비교기는 JVM 클래스 파일의 public/protected 연결 표면을
검사합니다. Kotlin/Java **소스 호환성 전체**나 JNI 런타임 동작까지
증명하지는 않습니다. 그래서 마지막에 실제 고객 프로젝트를 빌드하는
`matrix`가 필요합니다.

## 1분 체험

Git과 Go 1.25 이상만 있으면 됩니다. 이 읽기 전용 데모에는 Android Studio나
Android SDK가 필요하지 않습니다.

```bash
git clone https://github.com/gay00ung/aargrade.git
cd aargrade
make demo
```

공개 예제 [`examples/library-only`](examples/library-only)를 대상으로 다음을
보여줍니다.

1. `doctor`가 앱 없는 Android 라이브러리 프로젝트를 진단합니다.
2. `plan`이 AGP 9.3 마이그레이션 순서를 만듭니다.
3. `upgrade`가 자동 수리·마이그레이션·검증 전체 변경안을 미리 보여줍니다.
4. `host add`가 만들 임시 앱과 Gradle 변경을 미리 보여줍니다.
5. 실제 파일은 하나도 변경하지 않습니다.

## 설치

### 릴리스 바이너리 (권장)

[GitHub Releases](https://github.com/gay00ung/aargrade/releases)에서 운영체제와
CPU에 맞는 파일을 받습니다.

| 내 환경 | 받을 파일 |
| --- | --- |
| Apple Silicon Mac | `aargrade_<VERSION>_darwin_arm64.tar.gz` |
| Intel Mac | `aargrade_<VERSION>_darwin_amd64.tar.gz` |
| 일반 64비트 Linux | `aargrade_<VERSION>_linux_amd64.tar.gz` |
| ARM64 Linux | `aargrade_<VERSION>_linux_arm64.tar.gz` |
| 일반 64비트 Windows | `aargrade_<VERSION>_windows_amd64.zip` |
| ARM64 Windows | `aargrade_<VERSION>_windows_arm64.zip` |

첫 베타가 공개된 뒤 Apple Silicon Mac에서는 다음처럼 설치할 수 있습니다.

```bash
AARGRADE_VERSION=v0.1.0-beta.1
AARGRADE_ASSET="aargrade_${AARGRADE_VERSION#v}_darwin_arm64"
curl -fLO "https://github.com/gay00ung/aargrade/releases/download/${AARGRADE_VERSION}/${AARGRADE_ASSET}.tar.gz"
curl -fLO "https://github.com/gay00ung/aargrade/releases/download/${AARGRADE_VERSION}/checksums.txt"
grep " ${AARGRADE_ASSET}.tar.gz$" checksums.txt | shasum -a 256 -c -
tar -xzf "${AARGRADE_ASSET}.tar.gz"
sudo install -m 0755 "${AARGRADE_ASSET}/aargrade" /usr/local/bin/aargrade
aargrade version
```

`checksums.txt`는 다운로드가 손상되지 않았는지 확인합니다. Windows에서는
릴리스 페이지에서 ZIP을 내려받아 `aargrade.exe`를 PATH에 등록하면 됩니다.
각 압축에는 Apache-2.0 `LICENSE`와 함께 실행 파일에 포함된 Go 및 외부
모듈의 라이선스·고지 파일도 들어 있습니다.

### Go로 설치

Go 1.25 이상이 있다면 버전을 명시해 설치할 수도 있습니다.

```bash
go install github.com/gay00ung/aargrade/cmd/aargrade@v0.1.0-beta.1
export PATH="$(go env GOPATH)/bin:$PATH"
aargrade version
```

태그가 아직 공개되지 않았거나 개발 버전을 확인하려면 저장소에서 직접
빌드합니다.

```bash
make build VERSION=dev
./bin/aargrade version
```

## 실제 사용 순서

### 가장 먼저 써볼 명령

먼저 변경안을 미리 봅니다. 이 단계에서는 파일도 Gradle 빌드 결과도 만들지
않습니다.

```bash
aargrade upgrade --project . --target-agp 9.2.0
```

변경 내용을 확인한 뒤 전체 흐름을 실행합니다.

```bash
aargrade upgrade \
  --project . \
  --target-agp 9.2.0 \
  --library :sdk \
  --baseline-aar releases/sdk-current.aar \
  --apply
```

`--baseline-aar`는 선택 사항입니다. 적용 모드에서는 다음을 순서대로
수행합니다.

1. AGP/Gradle/JDK와 차단 항목을 진단합니다.
2. AGP·Wrapper를 바꾸고 `namespace`/옛 manifest package, custom
   `BuildConfig`, 단순 SDK setter, AGP 9 Built-in Kotlin, Java/Kotlin JVM
   target을 안전한 규칙으로 수리합니다.
3. 프로젝트 Wrapper로 `help`, `build --dry-run`, `:sdk:assembleRelease`를
   실제 실행합니다.
4. 생성된 AAR 구조·metadata·Consumer R8·JNI를 검사하고, 기준 AAR이 있으면
   JVM binary surface도 비교합니다.
5. `--matrix-config aargrade.yml`을 주면 실제 Java/Kotlin 고객 셀까지
   빌드합니다.
6. 어느 단계든 실패하면 원인을 분류하고 기본적으로 설정을 원복합니다.

성공한 경우에는 검토할 수 있도록 rollback 상태를 남깁니다. 되돌리거나,
변경을 최종 채택해 파일은 유지한 채 rollback 상태만 정리할 수 있습니다.

```bash
# 원복
aargrade migrate rollback --project . --apply

# 최종 채택(먼저 미리보기 후 적용)
aargrade migrate accept --project .
aargrade migrate accept --project . --apply
```

실패한 변경을 조사하려고 그대로 남기고 싶을 때만
`--keep-failed-changes`를 사용합니다. 이 옵션을 쓰면 자동 롤백되지 않습니다.

### 1. 현재 프로젝트 진단

`settings.gradle` 또는 `settings.gradle.kts`가 있는 프로젝트 루트에서
실행합니다.

```bash
aargrade doctor --project .
```

Gradle을 실행하거나 파일을 수정하지 않고 다음 항목을 정적으로 확인합니다.

- 앱·라이브러리 모듈, AGP, Gradle Wrapper
- AGP 9 Built-in Kotlin 전환 신호, KSP, kapt
- 오래된 Variant API와 DSL, `BuildConfig` 설정
- Consumer R8의 전역 옵션과 지나치게 넓은 keep 규칙
- NDK, CMake, JNI 설정
- Upgrade Assistant가 해석하기 어려운 동적 Gradle 구성

### 2. 마이그레이션 순서 만들기

```bash
aargrade plan --project . --target-agp 9.3.0
```

현재 프로젝트 진단 결과와 AARGrade의 버전별 정책을 합쳐 다음을 알려줍니다.

- 목표 AGP에 필요한 최소 Gradle과 JDK
- 먼저 해결해야 할 차단 항목
- 필수, 검토, 선택 단계의 실행 순서
- 마지막 `verify`와 `matrix` 검증 단계

알 수 없는 AGP 조합은 추측하지 않고 오류로 중단합니다. `plan`은 계획만
만들며 Gradle 파일을 자동 수정하지 않습니다.

### 3. 변경 엔진을 따로 사용할 때

`upgrade`가 권장 진입점입니다. 아래 `migrate`는 실제 빌드 없이 변경과
rollback만 직접 제어할 때 사용합니다.

먼저 변경될 줄을 확인합니다. 이 명령만으로는 파일을 바꾸지 않습니다.

```bash
aargrade migrate --project . --target-agp 9.2.0
```

출력된 변경과 차단 항목을 검토한 뒤 적용합니다.

```bash
aargrade migrate --project . --target-agp 9.2.0 --apply
```

현재 자동 변경 범위는 다음과 같습니다.

- Kotlin/Groovy의 literal AGP plugin과 `buildscript` classpath
- 기본 `gradle/libs.versions.toml`의 AGP version/ref
- 목표 AGP가 요구하는 최소 Gradle Wrapper URL과 공식 SHA-256
- 안전한 단일 줄 Kotlin Android plugin 제거와 AGP 9 Built-in Kotlin 전환
- 완전 전환 뒤 불필요해진 `android.builtInKotlin`, `android.newDsl` 등

다음 항목은 추측해서 바꾸지 않고 `BLOCKED`로 중단합니다.

- namespace가 없거나 `BuildConfig` 기능을 명시해야 하는 프로젝트
- 기존 Kotlin kapt(`com.android.legacy-kapt`로 검증된 경우 제외), KSP 2.3.6
  미만, `kotlinOptions`, custom Kotlin source set
- legacy Variant/internal API와 제거된 AGP 9 DSL
- `buildSrc`, custom settings catalog, RefreshVersions, 동적 convention plugin
- 다른 라이브러리와 AGP version ref를 공유하는 catalog

적용 후에는 원본 내용과 적용 직후 SHA-256이
`.aargrade/state/migration.json`에 로컬로 저장됩니다. 이후 파일을 사용자가
수정하지 않은 경우에만 정확히 되돌릴 수 있습니다.

```bash
# 되돌릴 내용 확인
aargrade migrate rollback --project .

# 확인 후 원본 복원
aargrade migrate rollback --project . --apply
```

상태 파일에는 원래 Gradle 설정 내용이 들어가므로 저장소에 커밋하지 말고
민감한 로컬 자료처럼 취급하세요. AARGrade의 `.gitignore` 예시는
`**/.aargrade/`를 제외합니다.

### 4. 새 AAR 검증

가장 신뢰도가 높은 사용법은 현재 배포 중인 기준 AAR과 새 후보 AAR을 함께
주는 것입니다.

```bash
aargrade verify \
  --baseline-aar releases/sdk-1.4.0.aar \
  --candidate-aar sdk/build/outputs/aar/sdk-release.aar
```

후보 AAR 경로를 생략하면 AARGrade가 프로젝트의 Gradle Wrapper로 `help`,
`build --dry-run`, release AAR 조립을 차례로 실행합니다.

```bash
aargrade verify \
  --project . \
  --library :sdk \
  --baseline-aar releases/sdk-1.4.0.aar
```

검사하는 내용은 다음과 같습니다.

- 안전한 AAR ZIP 구조, manifest, `classes.jar`
- AAR metadata의 `minCompileSdk`와 최소 AGP 요구사항 증가
- public/protected JVM 클래스·필드·메서드 제거와 연결 의미·sealed 계층 변경
- Consumer R8 전역 옵션과 넓은 keep 규칙
- JNI/Prefab ABI별 `.so` 제거와 바이너리 변경

기준 AAR을 주지 않으면 비교 항목은 `SKIPPED`로 표시되고 결과는
`EVIDENCE`입니다. 건너뛴 검사를 성공으로 표시하지 않습니다.

### 5. 고객 호환성 매트릭스 실행

저장소 루트의 [`aargrade.yml`](aargrade.yml)은 실제로 사용하는 설정
예제입니다.

```yaml
schemaVersion: 1
candidateAar: sdk/build/outputs/aar/sdk-release.aar
baselineAar: releases/sdk-1.4.0.aar

cells:
  - name: legacy-java
    agp: 4.2.2
    gradle: 6.7.1
    jdk: 11
    compileSdk: 30
    minSdk: 21
    language: java

  - name: current-kotlin
    agp: 9.2.0
    gradle: 9.4.1
    jdk: 17
    compileSdk: 35
    minSdk: 21
    language: kotlin
```

JDK는 셀별로 명시해야 합니다. Gradle은 직접 지정하거나, 네트워크 사용을
명시적으로 허용해 체크섬 검증 후 내려받을 수 있습니다.

```bash
export AARGRADE_JAVA_HOME_11=/path/to/jdk-11
export AARGRADE_JAVA_HOME_17=/path/to/jdk-17

aargrade matrix --config aargrade.yml --allow-downloads
```

다운로드를 원하지 않으면 실행 파일을 직접 지정합니다.

```bash
aargrade matrix \
  --config aargrade.yml \
  --gradle-bin 6.7.1=/path/to/gradle-6.7.1/bin/gradle \
  --gradle-bin 9.4.1=/path/to/gradle-9.4.1/bin/gradle
```

각 셀마다 독립된 Android 앱을 생성하고 AAR을 직접 의존성으로 넣은 뒤
release 빌드와 R8 최소화를 수행합니다. 기준 AAR은 성공하지만 후보 AAR만
실패하면 `REGRESSION`, 둘 다 실패하면 기존 미지원 환경인 `UNSUPPORTED`로
구분합니다. 생성 프로젝트와 로그는 `.aargrade/matrix/run-*`에 남습니다.

직접 파일 의존성은 Maven POM의 전이 의존성을 자동으로 가져오지 않습니다.
SDK가 다른 라이브러리를 필요로 한다면 셀의 `dependencies`에 실제 배포
의존성을 적어야 고객 환경과 같은 조건이 됩니다. 보고서는 흔한 비밀 값과
로컬 경로를 마스킹하지만, 보존된 빌드 로그는 민감한 자료처럼 취급하세요.

AARGrade는 Gradle만 선택적으로 내려받습니다. JDK와 Android SDK platform은
사용자가 로컬 또는 CI에 준비해야 합니다.

### 6. 임시 앱 모듈이 정말 필요할 때

앱 모듈이 없어서 Upgrade Assistant 문제가 발생한다는 현상을 먼저 재현한
경우에만 사용합니다. 이 우회 방식은 2026-08-14의 통제 실험에서 실제로
검증했습니다. 라이브러리 전용 상태에서는 Studio가 `application's Android
Project`를 얻지 못했고, host 추가 후에는 AGP 7.4.2 → 8.13.2 계획이
열렸으며, 제거 후 다시 실패 상태로 돌아갔습니다. 다른 Studio 버전에서도
무조건 된다는 뜻은 아닙니다.

```bash
# 변경 내용만 확인
aargrade host add --project .

# 검토 후 적용
aargrade host add --project . --apply
./gradlew :aargrade-upgrade-host:tasks --all

# 실험이 끝나면 제거도 먼저 미리보기
aargrade host remove --project .
aargrade host remove --project . --apply
```

AARGrade가 생성 당시 기록한 SHA-256과 소유 설정 블록이 그대로일 때만
제거합니다. 사용자가 수정한 파일이나 기존 앱은 삭제하지 않습니다.
직접 확인할 수 있는 fixture와 전체 A/B 증거는
[`testdata/projects/upgrade-assistant-repro`](testdata/projects/upgrade-assistant-repro)와
[한국어 실험 기록](docs/product/upgrade-assistant-experiment.ko.md)에 있습니다.

### 7. MCP 에이전트에서 사용

MCP 클라이언트에는 다음 stdio 명령을 등록합니다.

```text
aargrade mcp serve
```

일반적인 MCP 설정 형태는 다음과 같습니다. 실제 키 이름은 사용하는
클라이언트 설명서를 따르세요.

```json
{
  "mcpServers": {
    "aargrade": {
      "command": "aargrade",
      "args": ["mcp", "serve"]
    }
  }
}
```

서버가 제공하는 도구는 `aargrade_doctor`, `aargrade_plan`,
`aargrade_upgrade`, `aargrade_migrate`, `aargrade_migrate_accept`,
`aargrade_migrate_rollback`, `aargrade_verify`, `aargrade_matrix`,
`aargrade_host_add`, `aargrade_host_remove`입니다. 결과는 CLI와 같은 도메인 모델의 구조화된
JSON입니다. `aargrade_upgrade`는 기본적으로 변경 미리보기만 반환하고,
`apply=true`일 때 빌드까지 수행합니다. 모든 변경 도구의 `apply`와
`allowDownloads`는 MCP에서도 기본값이 `false`입니다.

## CI에서 사용

진단 보고서 예시입니다.

```bash
aargrade doctor \
  --project . \
  --format json \
  --fail-on warn > aargrade-doctor.json
```

호환성 비교와 매트릭스 보고서도 JSON으로 저장할 수 있습니다.

```bash
aargrade verify \
  --baseline-aar baseline.aar \
  --candidate-aar candidate.aar \
  --format json > aargrade-verify.json

aargrade matrix \
  --config aargrade.yml \
  --format json > aargrade-matrix.json
```

주요 종료 코드는 다음과 같습니다.

| 명령 | `0` | `1` | `2` |
| --- | --- | --- | --- |
| `doctor` | 기준 이상의 진단 없음 | `--fail-on` 기준 도달 | 분석/인자 오류 |
| `plan` | 실행 가능한 계획 | 차단 항목 있음 | 정책/분석/인자 오류 |
| `upgrade` | 적용 가능한 미리보기 또는 전체 검증 통과 | 차단/빌드/호환성 실패 | 미완료 환경/입력/상태 오류 |
| `migrate` | 적용 가능한 미리보기/적용/채택/롤백 | 안전하지 않아 차단 | 상태/정책/입력 오류 |
| `verify` | 증거 생성 또는 호환 | 검증 실패 | 실행/입력 오류 |
| `matrix` | 선택한 셀 통과 | 후보 실패 또는 회귀 | 미완료/환경/입력 오류 |

## 테스트와 검증 범위

```bash
make test       # 단위·fixture·공개 예제·MCP stdio 테스트
make vet        # Go 정적 분석
make test-race  # 동시성 오류 검사
make vuln       # 호출 가능한 Go 취약점 검사
make check      # test + vet + 읽기 전용 데모
make release-check # 6개 플랫폼 압축·체크섬·내장 버전 검사
make example-build
make example-verify # 예제 AAR 빌드 후 기준/후보 비교

# Upgrade Assistant 재현 fixture의 host 전/후/복원 흐름만 빠르게 검사
make test-upgrade-assistant-fixture
make test-migration-smoke # AGP 8.8 → 9.2 적용, Kotlin/AAR 빌드, 원본 복원
make test-upgrade-agent-smoke # 자동 수리 → 실제 빌드/AAR 검사 → 원본 복원
```

사용자 관점의 흐름은 [`tests/integration`](tests/integration)에 따로 두었고,
세부 단위 테스트는 Go 관례대로 각 `internal/*` 패키지의 `_test.go`에 함께
있습니다. Android SDK가 필요한 실제 소비자 빌드는 CI의 `consumer-matrix`
작업이 담당합니다.

CI는 Linux, macOS, Windows에서 Go 테스트를 수행합니다. 또한 공개 AAR을
실제로 만든 뒤 다음 네 고객 셀을 각각 release 빌드합니다.

- AGP 4.2.2 + Gradle 6.7.1 + JDK 11 + Java
- AGP 4.2.2 + Gradle 6.7.1 + JDK 11 + Kotlin
- AGP 9.2.0 + Gradle 9.4.1 + JDK 17 + Java
- AGP 9.2.0 + Gradle 9.4.1 + JDK 17 + Built-in Kotlin

별도의 Gradle smoke 작업은 AGP 4.2.2, Upgrade Assistant 재현용 AGP 7.4.2,
AGP 9.2.0에서 생성된 host가 실제로 구성되는지도 검사합니다. Android Studio
UI A/B는 headless 빌드로 대체하지 않고 위 실험 기록으로 관리합니다.

`migration-smoke` 작업은 version catalog와 Kotlin 소스가 있는 AGP 8.8
fixture를 AGP 9.2/Gradle 9.4.1로 실제 변경하고, Built-in Kotlin 컴파일과
release AAR 생성을 마친 뒤 모든 소유 설정이 byte 단위로 복원되는지 검사합니다.
같은 CI 작업의 `upgrade-agent-smoke`는 namespace가 없고 구형 SDK DSL,
implicit BuildConfig, `kotlinOptions`가 남은 fixture를 `upgrade --apply` 한 번으로
수리한 뒤 실제 release AAR을 검사하고 정확히 원복합니다.

이 결과는 **설정한 네 환경의 빌드 증거**이지 모든 고객, 모든 API 호출,
실기기 런타임을 보장한다는 뜻은 아닙니다.

## 상세 문서

- [CLI 전체 명세](docs/cli.md)
- [릴리스 방법](docs/releasing.ko.md)
- [Release guide (English)](docs/releasing.md)
- [제품 및 개발 계획](docs/product/plan.md)
- [검증 기록](docs/product/validation.md)
- [Upgrade Assistant A/B 실제 결과 (한국어)](docs/product/upgrade-assistant-experiment.ko.md)
- [Upgrade Assistant A/B evidence (English)](docs/product/upgrade-assistant-experiment.md)
- [Consumer R8 테스트 설정 분석](R8_Configuration_Analysis.md)
- [CLI 런타임 결정 기록](docs/adr/0001-cli-runtime.md)
- [자동 마이그레이션·롤백 안전성 결정](docs/adr/0003-bounded-migration-transactions.md)
- [에이전트형 업그레이드 오케스트레이션 결정](docs/adr/0004-agent-upgrade-orchestration.md)

소스와 릴리스 바이너리는 [Apache License 2.0](LICENSE)으로 배포합니다.

**AARGrade**라는 이름은 상표 검토가 완료될 때까지 임시 이름입니다.
