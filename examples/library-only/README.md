# 앱 모듈 없는 Android 라이브러리 예제

한국어 | [English](README.en.md)

이 프로젝트는 앱 모듈 없이 `:sdk` 라이브러리 모듈 하나만 가진 정상적인
Android 프로젝트입니다. 실제 업무 프로젝트를 건드리지 않고 AARGrade의
전체 흐름을 시험할 수 있도록 만들었습니다.

AARGrade 저장소 루트에서 실행합니다.

```bash
make demo
```

이 명령은 다음 작업만 수행합니다.

1. 프로젝트를 읽기 전용으로 진단합니다.
2. AGP 9.3 업그레이드 계획을 만듭니다.
3. 임시 앱 모듈을 만든다면 변경될 내용을 미리 보여줍니다.
4. 실제 파일은 변경하지 않습니다.

임시 복사본에서 전체 생성·제거 과정을 직접 확인하려면 다음과 같이
실행합니다.

```bash
cp -R examples/library-only /tmp/aargrade-library-example
go run ./cmd/aargrade host add \
  --project /tmp/aargrade-library-example \
  --apply
go run ./cmd/aargrade host remove \
  --project /tmp/aargrade-library-example \
  --apply
```

실제 AAR을 빌드하려면 JDK 17과 Android SDK platform 35가 필요합니다.

```bash
make example-build
```

체크섬이 고정된 Gradle Wrapper가 Gradle 9.4.1을 처음 한 번 내려받습니다.
결과 파일은 다음 위치에 생성됩니다.

```text
examples/library-only/sdk/build/outputs/aar/sdk-release.aar
```

같은 AAR을 기준/후보로 넣어 정적 검증 흐름을 확인할 수 있습니다.

```bash
go run ./cmd/aargrade verify \
  --baseline-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar \
  --candidate-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar
```

저장소 루트의 `aargrade.yml`에는 CI가 실제로 실행하는 AGP 4.2/9.2 ×
Java/Kotlin 네 고객 셀이 들어 있습니다. 실행에 필요한 JDK와 Android SDK
준비 방법은 루트 `README.md`의 `matrix` 설명을 참고하세요.
