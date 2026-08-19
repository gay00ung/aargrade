# 외부 Android 프로젝트 검증 기록 — 2026-08-18

이 검증은 AARGrade가 자체 fixture에서만 동작하는지 확인하기 위해 공개 Android
라이브러리·SDK 다섯 개의 고정 커밋을 사용했다. 수요 검증이나 모든 Gradle 구조에
대한 호환성 보장이 아니라, 실제 저장소에서의 기술 검증이다.

## 고정 검증 대상

| 프로젝트 | 커밋 | 시작 환경 | 실행 범위 | 결과 |
| --- | --- | --- | --- | --- |
| [Timber](https://github.com/JakeWharton/timber) | `c51c0cad85bfd4c4c452222dae5ec49e8988303a` | AGP 9.2.1, Gradle 9.6.1, KMP Android library | 9.3.1 실제 적용·Gradle·AAR 검사 | 통과 |
| [Picasso](https://github.com/square/picasso) | `e94d6116e3fff99b3932103e0ab22ff5b44a1273` | AGP 8.7.2, Gradle 8.10.2, Groovy DSL | 9.3.1 변경 미리보기 | 준비 완료 |
| [Lottie Android](https://github.com/airbnb/lottie-android) | `05ea92e90381eb8a8ae06855ea2b74f322bebbec` | RefreshVersions가 AGP 관리 | 진단·변경 미리보기 | 안전하게 중단 |
| [Glide](https://github.com/bumptech/glide) | `bf4901de78c206ed21ce73f5d25aa2bbf35ebf3c` | AGP 9.2.0, Gradle 9.4.1, 대형 멀티모듈 | 9.3.1 실제 적용·Gradle·AAR 검사 | 통과 |
| [Adjust Android SDK](https://github.com/adjust/android_sdk) | `b31ee274a2d189c4ff705c2ee6c47ad09ca5eb62` | AGP 9.2.1, Gradle 9.6.0, Groovy `buildscript.ext` | 9.3.1 미리보기와 별도 실전 AAR·매트릭스 검증 | 통과 |

## 실제 적용 결과

### Timber

- version catalog의 AGP만 9.2.1에서 9.3.1로 변경했다.
- 기존 KMP, Kotlin source set, JVM lint 모듈의 kapt 설정은 유지했다.
- `help`, 전체 `build --dry-run`, `:timber:bundleAndroidMainAar`가 JDK 17에서
  통과했다.
- 생성된 `timber.aar`은 32,705바이트이며 SHA-256은
  `3270214ff2784195326b0a67da50404f7cb36291e30b50690e6cb0dba38d19f1`이다.
- AAR 필수 구조, metadata, Consumer R8 검사가 통과했다. 기준 AAR을 주지
  않았으므로 ABI·JNI 비교는 명시적으로 건너뛰었다.

### Glide

- version catalog의 AGP를 9.2.0에서 9.3.1로, Wrapper를 9.4.1에서 9.5.0으로
  공식 SHA-256과 함께 변경했다.
- `help`, 대형 멀티모듈 전체 `build --dry-run`,
  `:library:assembleRelease`가 JDK 17에서 통과했다.
- 생성된 `library-release.aar`은 719,394바이트이며 SHA-256은
  `59e1c99532535b4ce15973539a9359af0efbe903c8215dad415eb74b64cc52c7`이다.
- AAR 구조와 metadata 검사는 통과했다. Glide가 의도적으로 제공하는 넓은
  Consumer R8 keep 규칙 세 개는 실패가 아닌 검토 경고로 남았다.

두 성공 케이스 모두 검증 후 `migrate accept --apply`가 적용 파일의 hash를
확인하고 rollback 상태만 안전하게 제거했다.

## 미리보기와 중단 결과

- Picasso에서는 여섯 Android 모듈의 AGP/Wrapper, 연속 Groovy `apply plugin`,
  `compileSdkVersion`/`minSdkVersion`, `kotlinOptions` 변경안이 생성됐다.
  `libs.versions.*.get()` 값은 Java/Kotlin target이 정확히 같을 때만 자동
  변환된다. 이 검증에서는 AGP 9 미만의 제3자 저장소를 실제 수정하지 않았다.
- Lottie에서는 RefreshVersions가 AGP 선언을 동적으로 관리한다는 사실을
  감지했다. AARGrade는 버전을 추측하지 않고 blocker와 다음 조치를 반환했다.

## 이 검증으로 고친 문제

1. version catalog의 `{ group, name, version.ref }` 라이브러리 좌표 인식
2. AGP 9.x 사이의 업그레이드에서 기존 Kotlin/KMP 구성을 불필요하게 바꾸던
   동작
3. 연속 Groovy `apply plugin`과 `pluginManager.withPlugin` callback 오인식
4. 제한된 `libs.versions.*.get()` SDK/JVM target 변환
5. KMP Android library의 `bundleAndroidMainAar` 태스크와 접미사 없는 AAR
   탐색
6. 단일 literal Groovy `buildscript.ext` AGP 변수의 보수적 인식과 변경

모든 수정에는 회귀 테스트가 추가됐다. 임의 함수 호출, 서로 다른 Java/Kotlin
provider, RefreshVersions와 복잡한 convention logic은 계속 자동 변경하지
않는다.

## 재현 방법

다음 명령은 고정 커밋을 새 임시 디렉터리에 받아 `doctor`와 `upgrade`
미리보기를 실행한다. 네트워크와 Git, Go가 필요하며 기본 CI에는 포함되지
않는다.

```bash
make test-external
```

Timber와 Glide에 실제 적용·Gradle·AAR 검증까지 실행하려면 JDK 17,
Android SDK platform 36과 37을 준비한 뒤 명시적으로 opt-in 한다.

```bash
AARGRADE_EXTERNAL_APPLY=1 AARGRADE_KEEP_EXTERNAL=1 make test-external
```

`AARGRADE_KEEP_EXTERNAL=1`은 종료 후 임시 작업 경로를 출력하고 보존한다.
Adjust의 Maven 기준 AAR 비교와 네 고객 환경 결과는 별도의
[실전 검증 기록](adjust-sdk-dogfood-2026-08-19.ko.md)에 정리했다.
