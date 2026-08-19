# AARGrade v1 외부 베타 검증 가이드

[English](beta-testing.md)

AARGrade가 `v1.0.0`으로 가려면 도구 개발자가 고른 공개 저장소뿐 아니라,
실제 Android SDK 또는 AAR 관리자가 자기 프로젝트에서 안전성을 확인해야
합니다. 비공개 프로젝트도 참여할 수 있으며 소스나 고객 이름을 공개할 필요는
없습니다.

## 어떤 결과가 도움이 되나요?

다음 중 하나를 실제 프로젝트에서 수행한 결과가 필요합니다.

1. `upgrade` 미리보기 후 `--apply`를 실행하고, 성공한 변경을 명시적으로
   rollback하거나 실패 시 자동 rollback을 확인
2. 현재 배포 AAR과 후보 AAR을 `verify`로 비교
3. Java 또는 Kotlin 고객 셀을 `matrix`로 실제 빌드

미리보기만 실행한 결과도 제품 수요와 진단 품질을 확인하는 데 유용하지만,
v1 안전성 검증에는 적용·복원 또는 실제 소비자 빌드 결과가 특히 중요합니다.

## 시작 전 안전 확인

- 본인이 관리하거나 시험 권한을 가진 프로젝트만 사용하세요.
- 커밋되지 않은 작업을 정리하고 별도 브랜치나 임시 worktree에서 실행하세요.
- 가능하면 현재 고객에게 배포 중인 기준 AAR을 준비하세요.
- AARGrade 출력과 `.aargrade` 로그를 공개하기 전에 직접 검토하세요.
- 토큰, 서명 정보, 고객 코드, 사내 저장소 주소와 로컬 경로를 이슈에 올리지
  마세요.

```bash
git status --short
aargrade version
```

권장 버전은 최신 공개 베타인 `v0.1.0-beta.2`입니다.

## 1단계: 변경 없는 진단과 미리보기

프로젝트 루트에서 실행합니다. 두 명령 모두 Gradle 설정을 바꾸지 않습니다.

```bash
aargrade doctor --project . --fail-on never
aargrade upgrade --project . --target-agp 9.3.1
```

모듈 선택이 모호하면 실제 AAR을 만드는 Gradle 경로를 지정합니다.

```bash
aargrade upgrade \
  --project . \
  --target-agp 9.3.1 \
  --library :sdk
```

미리보기의 `BLOCKED`와 종료 코드 `1`은 도구가 알 수 없는 Gradle 로직을
추측하지 않고 중단했다는 뜻일 수 있습니다. 어떤 blocker가 나왔는지 그대로
기록해 주세요.

## 2단계: 적용과 복원 검증

미리보기를 검토한 뒤 깨끗한 브랜치나 worktree에서만 실행합니다. 기준 AAR이
없다면 `--baseline-aar` 줄은 생략할 수 있습니다.

```bash
aargrade upgrade \
  --project . \
  --target-agp 9.3.1 \
  --library :sdk \
  --baseline-aar releases/sdk-current.aar \
  --apply
```

AARGrade는 적용 전에 전체 Gradle dry-run을 기록하고, 적용 후 결과와 선택한
라이브러리의 dry-run·AAR 조립을 검증합니다. 실패하면 기본적으로 AARGrade가
소유한 설정 변경을 자동 rollback합니다.

성공하면 rollback 상태가 남습니다. 변경을 유지할 생각이 없어도 먼저
미리보기한 뒤 복원하여 안전성을 확인할 수 있습니다.

```bash
aargrade migrate rollback --project .
aargrade migrate rollback --project . --apply
git status --short
```

변경을 실제로 채택하려면 rollback 대신 다음을 사용합니다.

```bash
aargrade migrate accept --project .
aargrade migrate accept --project . --apply
```

`--keep-failed-changes`는 자동 rollback을 끄므로 원인 조사가 꼭 필요한 경우가
아니면 베타 검증에서 사용하지 마세요.

## 3단계: 고객 매트릭스(선택)

저장소의 [`aargrade.yml`](../aargrade.yml)을 프로젝트에 맞게 복사해 기준·후보
AAR과 지원할 AGP/Gradle/JDK/언어 셀을 적습니다. JDK는 사용자가 준비해야
하며, `--allow-downloads`는 공식 Gradle 배포본과 체크섬 다운로드만
허용합니다.

```bash
export AARGRADE_JAVA_HOME_11=/path/to/jdk-11
export AARGRADE_JAVA_HOME_17=/path/to/jdk-17

aargrade matrix --config aargrade.yml --allow-downloads
```

직접 AAR 의존성에는 Maven POM의 전이 의존성이 따라오지 않습니다. SDK에
필요한 외부 라이브러리는 각 셀의 `dependencies`에 적어야 합니다.

## 결과 공유

[베타 실전 검증 양식](https://github.com/gay00ung/aargrade/issues/new?template=beta-validation.yml)에
다음만 공유해 주세요.

- AARGrade 버전과 OS
- 현재/목표 AGP, Gradle, JDK, DSL
- 실행한 흐름과 최종 verdict
- 잘 된 점, 안전하게 막힌 지점, 예상하지 못한 실패
- 직접 검토하고 민감정보를 제거한 핵심 finding 또는 로그

프로젝트 URL은 공개 저장소일 때만 적으면 됩니다. 비공개 프로젝트는
“비공개 멀티모듈 결제 SDK”처럼 익명으로 설명해도 충분합니다. 보안 취약점이나
실제 비밀 노출은 공개 이슈 대신 [Security Policy](../SECURITY.md)를 이용하세요.

## v1 검증으로 집계되는 기준

- 제출자가 해당 SDK/AAR의 관리자·기여자이거나 조직의 허가를 받아 평가함
- `v0.1.0-beta.2` 이상의 공개 바이너리를 사용함
- 실제 프로젝트에서 적용·복원 또는 소비자 빌드 결과를 확인함
- 해결이 필요한 안전성 blocker와 잘못된 판정을 숨기지 않고 기록함

공개 프로젝트를 AARGrade 개발자가 독립적으로 실행한 dogfood는 기술 검증에는
포함되지만 외부 사용자 수에는 포함되지 않습니다.
