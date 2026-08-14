# Upgrade Assistant 재현 프로젝트

이 프로젝트는 **앱 모듈이 없는 Android 라이브러리 프로젝트**에서 AGP
Upgrade Assistant가 application Android model을 얻지 못하는 현상을 재현하기
위한 최소 fixture입니다.

- Android Gradle Plugin: 7.4.2
- Gradle Wrapper: 7.5
- compileSdk: 33
- 모듈: Android 라이브러리 `:sdk` 하나
- 앱 모듈: 없음

Gradle Wrapper 배포 파일의 SHA-256은
`gradle/wrapper/gradle-wrapper.properties`에 고정되어 있습니다.

## 빠른 구조 테스트

저장소 루트에서 다음 테스트를 실행하면 fixture가 라이브러리 전용으로
진단되는지, AARGrade host 추가 후 앱 모듈로 인식되는지, 제거 후 원본
settings가 바이트 단위로 복원되는지 확인합니다.

```bash
make test-upgrade-assistant-fixture
```

이 테스트는 Android Studio UI를 흉내 내지 않습니다. 실제 Upgrade Assistant
판정은 아래 A/B 절차로 확인합니다.

## Android Studio A/B 재현

반드시 원본 fixture가 아닌 임시 복사본에서 실행하세요.

```bash
experiment_dir=$(mktemp -d)
cp -R testdata/projects/upgrade-assistant-repro/. "$experiment_dir"
chmod +x "$experiment_dir/gradlew"

git -C "$experiment_dir" init -q
git -C "$experiment_dir" add .
git -C "$experiment_dir" \
  -c user.name=AARGrade \
  -c user.email=fixture@aargrade.dev \
  commit -qm baseline

"$experiment_dir/gradlew" -p "$experiment_dir" help --no-daemon
```

1. `$experiment_dir`을 Android Studio에서 엽니다.
2. Gradle Sync가 끝나면 **Tools → AGP Upgrade Assistant**를 엽니다.
3. 업그레이드는 실행하지 말고 화면과 IDE 로그만 기록합니다.
4. 터미널에서 host를 추가합니다.

```bash
aargrade host add --project "$experiment_dir"       # 변경 미리보기
aargrade host add --project "$experiment_dir" --apply
```

5. Android Studio에서 다시 Sync한 뒤 Upgrade Assistant를 엽니다.
6. 실험이 끝나면 host를 제거합니다.

```bash
aargrade host remove --project "$experiment_dir"       # 제거 미리보기
aargrade host remove --project "$experiment_dir" --apply
```

2026-08-14에 수행한 실제 환경·로그·결과는
[`docs/product/upgrade-assistant-experiment.ko.md`](../../../docs/product/upgrade-assistant-experiment.ko.md)에
정리되어 있습니다.

> 주의: 이 fixture의 목적은 마이그레이션 계획이 열리는지 비교하는 것입니다.
> Upgrade Assistant의 **Run selected steps**는 실행하지 마세요.
