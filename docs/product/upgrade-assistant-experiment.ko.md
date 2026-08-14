# Upgrade Assistant host A/B 실험 결과

[English](upgrade-assistant-experiment.md)

**결론: 2026-08-14의 아래 환경에서 검증 성공.** 앱 모듈이 없는 상태에서는
Upgrade Assistant가 실패했고, AARGrade host를 추가하자 실제 업그레이드 계획이
열렸습니다. 다만 Android Studio 모든 버전과 모든 Gradle 구조에서 똑같이
동작한다고 보장하는 결과는 아닙니다.

## 무엇을 확인했나

확인할 질문은 하나였습니다.

> 같은 라이브러리 전용 프로젝트에 AARGrade가 소유한 임시 앱만 추가했을 때,
> 열리지 않던 AGP Upgrade Assistant 계획이 실제로 열리는가?

Gradle 빌드 성공만으로는 성공 처리하지 않았습니다. Upgrade Assistant가 현재
AGP를 읽고 구체적인 마이그레이션 단계를 보여주는지를 기준으로 삼았고, 실제
업그레이드는 실행하지 않았습니다.

## 실험 환경

| 항목 | 값 |
| --- | --- |
| Android Studio | Panda 2, 2025.3.2 RC 1; bundle version `2025.3`; build `AI-253.30387.90.2532.14901460` |
| 운영체제 | macOS 26.3.1 (`25D2128`), arm64 |
| Gradle JDK | Microsoft OpenJDK 17.0.16 (`17.0.16+8-LTS`) |
| 프로젝트 구조 | `com.android.library`를 쓰는 `:sdk` 하나, 앱 모듈 없음 |
| 빌드 도구 | AGP 7.4.2, Gradle 7.5, compileSdk 33 |
| 원본 settings digest | `4d52544df9939fa68da4e905b0c316b5ff5eb3faa71f63f1d8b49250e7c3dd4c` |
| Studio가 선택한 목표 | AGP 8.13.2; IDE가 아는 최신 버전은 9.1.0-rc01 |

처음 Sync에서는 Android Studio가 JDK 21을 선택해 Gradle 7.5와 충돌했습니다.
프로젝트 전용 **Apply compatible Gradle JDK configuration and sync**를 적용하자
JDK 17로 정상 Sync됐습니다. 이 문제는 host 기능과 무관한 환경 설정이므로 제품
판정에서 제외했습니다.

## A — 앱 모듈이 없는 기준 상태

`testdata/projects/upgrade-assistant-repro`를 임시 폴더로 복사하고 Git 기준점을
만든 뒤 Gradle Wrapper의 `help`와 Android Studio Sync가 모두 성공했습니다.

그 상태에서 **Tools → AGP Upgrade Assistant**를 실행했지만 사용할 수 있는
계획이 열리지 않았고, IDE 로그에 다음 이유가 두 번 남았습니다.

```text
WARN - Upgrade Assistant - Unable to obtain application's Android Project
```

즉, AARGrade를 적용하기 전에 라이브러리 전용 프로젝트의 문제부터 독립적으로
재현했습니다.

## B — AARGrade host를 추가한 상태

같은 임시 프로젝트에서 다음 명령을 실행했습니다.

```bash
aargrade host add --project /temporary/fixture --apply
```

AARGrade는 settings의 소유 블록과 `.aargrade/upgrade-host`만 만들었고, 임시 앱이
`:sdk`를 의존하도록 연결했습니다. 다시 Sync하자 Android Studio가 곧바로 다음
알림을 표시했습니다.

```text
Android Gradle plugin version 7.4.2 has an upgrade available.
Start the AGP Upgrade Assistant to update this project's AGP version.
```

Upgrade Assistant에는 다음 실제 결과가 나타났습니다.

- 제목: `Upgrade project from AGP 7.4.2`
- 선택된 목표: AGP 8.13.2
- 표시된 단계: Gradle 버전, R8 기본값, BuildConfig, R 클래스, AGP 의존성 변경

IDE 로그도 실패 경고 대신 다음 정상 상태를 기록했습니다.

```text
INFO - Upgrade Assistant - Starting AGP Upgrade Assistant
INFO - Upgrade Assistant - Gradle model version: 7.4.2, latest known version for IDE: 9.1.0-rc01
INFO - Upgrade Assistant - Gradle upgrade state: GradlePluginUpgradeState(importance=RECOMMEND, target=8.13.2)
```

**Run selected steps는 누르지 않았습니다.** 이번 실험의 목적은 업그레이드 계획이
열리는지 확인하는 것이기 때문입니다.

## 제거 후 원상 복구 확인

```bash
aargrade host remove --project /temporary/fixture --apply
```

AARGrade가 만든 파일만 삭제됐고 settings의 SHA-256은 원본 값
`4d52544df9939fa68da4e905b0c316b5ff5eb3faa71f63f1d8b49250e7c3dd4c`로 정확히
돌아왔습니다. 다시 Sync하자 열려 있던 화면도 `Upgrade project from unknown
AGP`, `Android Gradle Plugin not present` 상태로 돌아갔습니다.

즉, 결과가 **실패 → host 추가 후 성공 → host 제거 후 다시 실패**로 바뀌어
host가 원인이라는 근거가 더 강해졌습니다.

## 제품에 반영할 결론

- Gate A(문제 재현): 통과
- Gate C(host 필요성): 통과
- `host add/remove`: Upgrade Assistant 우회 기능으로 유지
- 안전 규칙: 미리보기 기본, `--apply` 명시, 소유 파일 digest 검사, 수정 파일 삭제 거부 유지
- 표현 범위: “검증된 환경에서 해결”이라고 말하되 모든 Studio 버전에서 된다고 과장하지 않음

직접 다시 확인하려면
[`testdata/projects/upgrade-assistant-repro/README.md`](../../testdata/projects/upgrade-assistant-repro/README.md)의
순서를 따르면 됩니다. CI는 fixture와 생성된 host가 AGP 7.4.2에서 계속
구성되는지 검사합니다. Android Studio 화면 자체는 headless Gradle 빌드로
대체할 수 없으므로 수동 A/B 증거로 관리합니다.
