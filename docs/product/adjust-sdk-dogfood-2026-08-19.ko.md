# Adjust Android SDK 실전 검증 기록 — 2026-08-19

이 검증은 AARGrade 저장소의 샘플이 아니라 GitHub에서 직접 고른 실제 배포
SDK를 대상으로 수행했다. 대상은 [Adjust Android SDK](https://github.com/adjust/android_sdk)의
릴리스 태그 [`v5.8.0`](https://github.com/adjust/android_sdk/releases/tag/v5.8.0)이며,
태그가 가리키는 소스 커밋은
`b31ee274a2d189c4ff705c2ee6c47ad09ca5eb62`이다.

## 검증 조건

| 항목 | 값 |
| --- | --- |
| 기준 AAR | `com.adjust.sdk:adjust-android:5.8.0` Maven Central 배포본 |
| 시작 AGP | 9.2.1 |
| 목표 AGP | 9.3.1 |
| 프로젝트 Wrapper | Gradle 9.6.0 |
| 마이그레이션·AAR JDK | Microsoft OpenJDK 17.0.16 |
| 구형 고객 JDK | Microsoft OpenJDK 11.0.29 |
| 대상 모듈 | `:sdk-core` |

기준 AAR은 [Maven Central](https://repo.maven.apache.org/maven2/com/adjust/sdk/adjust-android/5.8.0/adjust-android-5.8.0.aar)에서
직접 받았다. 공개 POM에 선언된
`com.adjust.signature:adjust-android-signature:5.0.0`도 소비자 매트릭스에
전이 의존성으로 넣었다.

## 첫 실행에서 발견한 제품 문제

Adjust는 다음처럼 AGP 버전을 Groovy `buildscript.ext` 변수로 관리한다.

```groovy
buildscript {
    ext {
        agp_version = '9.2.1'
    }
    dependencies {
        classpath "com.android.tools.build:gradle:$agp_version"
    }
}
```

기존 AARGrade는 literal classpath와 version catalog만 읽어서 이를
`AGP: unknown`으로 판정하고 안전하게 중단했다. 실제 저장소에서 확인된 이
빈틈을 다음 조건으로 제한해 수정했다.

- 변수가 literal 버전으로 정확히 한 번 선언돼야 한다.
- 그 변수의 모든 문자열 interpolation이 AGP classpath에서만 사용돼야 한다.
- 재할당, 동적 값, 다른 의존성과 공유된 변수는 계속 `BLOCKED`로 남긴다.

회귀 테스트에는 정상 인식·적용뿐 아니라 공유 변수와 재할당 변수를 거부하는
경우도 포함했다. 수정 후 `doctor`는 AGP 9.2.1, Gradle 9.6.0, Android 모듈
21개를 읽었고 경고와 오류는 0개였다. 변경 미리보기는 다음 한 줄뿐이었다.

```diff
- agp_version = '9.2.1'
+ agp_version = '9.3.1'
```

## Gradle과 AAR 결과

AGP 9.3.1 적용 뒤 JDK 17에서 다음 결과를 확인했다.

- `./gradlew help --no-daemon`: 통과
- `./gradlew :sdk-core:assembleRelease --dry-run --no-daemon`: 통과
- `./gradlew :sdk-core:assembleRelease --no-daemon`: 통과
- `aargrade verify --baseline-aar ... --candidate-aar ...`: 통과

기준과 후보 AAR은 모두 약 287 KiB였으며 해시는 서로 달랐다.

| AAR | SHA-256 |
| --- | --- |
| Maven Central 기준 | `d00ddde91b0bfe46fb7c5675fa5a52fe3393e990a334f14804b01c2e4f0affea` |
| AGP 9.3.1 후보 | `69457f6f2b09c8d9daeda7a3432ef7c78ef3ad2576764d9e84e3b6f6d75fca61` |

해시가 다르다는 사실만으로 호환성 실패는 아니다. AARGrade의 비교 결과는
다음과 같았다.

- AAR 필수 구조: 통과
- JVM public/protected binary surface: 호환
- AAR metadata의 고객 요구사항 증가: 없음
- JNI/Prefab 제거: 없음
- Consumer R8: 실패 없음, 기존의 넓은 keep 규칙 네 개를 검토 경고로 보고

두 AAR의 metadata는 `minCompileSdk=1`,
`minAndroidGradlePluginVersion=1.0.0`으로 같았다.

## 실제 고객 매트릭스

각 셀에서 기준과 후보를 별도의 앱에 직접 넣고 release/R8 빌드를 수행했다.

| 고객 환경 | 기준 | 후보 | 판정 |
| --- | --- | --- | --- |
| AGP 4.2.2 / Gradle 6.7.1 / JDK 11 / API 30 / Java | 통과 | 통과 | PASS |
| AGP 4.2.2 / Gradle 6.7.1 / JDK 11 / API 30 / Kotlin 1.6.21 | 통과 | 통과 | PASS |
| AGP 9.3.1 / Gradle 9.5.0 / JDK 17 / API 37 / Java | 통과 | 통과 | PASS |
| AGP 9.3.1 / Gradle 9.5.0 / JDK 17 / API 37 / Built-in Kotlin | 통과 | 통과 | PASS |

따라서 이번 대상에서는 AGP 9.3.1로 다시 만든 AAR이 선언한 구형·최신
Java/Kotlin 고객 환경에서 새 회귀를 만들지 않았다.

## 저장소 전체 dry-run의 기존 실패

`./gradlew build --dry-run`은 AGP 9.3.1 후보에서 실패했다. AARGrade의 자동
`upgrade --apply`는 이 실패를 감지하고 소유한 변경을 즉시 원복했다.
원본 AGP 9.2.1을 별도 worktree에서 실행해 보니 같은 오류가 재현됐다.

```text
Could not determine the dependencies of task
':plugins:sdk-plugin-google-lvl:adjustLvlPluginJar'.
Task with name 'packageReleaseAssets' not found
```

즉 이 오류는 마이그레이션 회귀가 아니라 릴리스 태그에 이미 존재하는 다른
플러그인 모듈의 전체 `build` task 문제다. 그래서 이 기록은 **선택한 배포
모듈과 AAR 고객 호환성이 통과했다**고만 결론 내리며, Adjust 저장소 전체
빌드가 깨끗하다고 주장하지 않는다.

이 사례는 다음 제품 개선 후보도 드러냈다. 전체 dry-run의 전체 오류를 보고서에
보존하고, 마이그레이션 전에도 같은 실패가 있었는지 비교해 “기존 실패”와
“후보 회귀”를 구분할 필요가 있다.

## 재현 범위

`make test-external`은 위 고정 커밋을 새 임시 디렉터리에 받아 `doctor`와
AGP 9.3.1 변경 미리보기를 다시 확인한다. 전체 Maven AAR 비교와 네 소비자
빌드는 JDK 11/17, Android SDK platform 30/37, 네트워크가 필요한 opt-in
검증이므로 기본 CI에는 넣지 않았다.

외부 저장소에는 어떤 commit이나 push도 하지 않았다.
