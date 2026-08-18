# AARGrade 기여 가이드

AARGrade에 기여해 주셔서 감사합니다. 이 프로젝트는 실제 고객 프로젝트의
Gradle 설정을 바꾸는 도구이므로, 기능 수보다 **미리보기·근거·안전한 중단·정확한
복원**을 우선합니다.

## 개발 환경

- Git
- Go 1.25 이상. CI와 릴리스는 Go 1.25.13을 사용합니다.
- Android 관련 smoke test를 실행할 때만 JDK 11/17과 Android SDK가
  필요합니다.

```bash
git clone https://github.com/gay00ung/aargrade.git
cd aargrade
make check
```

변경을 제출하기 전에는 최소한 다음을 실행해 주세요.

```bash
make check
make test-race
```

마이그레이션이나 AAR 빌드 경로를 바꿨다면 관련 실제 Gradle 검사도 실행합니다.

```bash
make test-migration-smoke
make test-upgrade-agent-smoke
```

네 개의 고정 공개 프로젝트를 내려받아 진단과 변경 미리보기를 반복하려면
`make test-external`을 사용합니다. 이 검사는 네트워크를 사용하며 기본 CI에는
포함되지 않습니다. 자세한 범위는
[외부 프로젝트 검증 기록](docs/product/external-validation-2026-08.ko.md)에
있습니다.

## 변경 원칙

1. 파일을 바꾸는 명령은 기본적으로 미리보기여야 합니다.
2. 자동 변환은 정적으로 증명할 수 있는 좁은 문법만 허용합니다.
3. 알 수 없는 Gradle 로직은 추측하지 않고 구체적인 blocker를 반환합니다.
4. 새 변환 규칙에는 성공·거부 사례를 모두 담은 fixture 또는 단위 테스트가
   필요합니다.
5. JSON 스키마와 종료 코드가 바뀌면 `docs/cli.md`와 README도 갱신합니다.
6. 로그·fixture·이슈에 토큰, 비밀번호, 서명 키, 고객 경로를 넣지 않습니다.

## 이슈와 Pull Request

버그를 제보할 때는 가능한 범위에서 AARGrade 버전, 운영체제, JDK, 현재/목표
AGP, 사용한 명령, 민감정보를 제거한 출력을 포함해 주세요. 재현 프로젝트는
최소화하고 공개할 수 없는 고객 코드는 올리지 마세요.

Pull Request는 한 가지 문제에 집중하고, 무엇을 자동화했는지뿐 아니라 어떤
경우에 안전하게 중단하는지도 설명해 주세요. 보안 취약점은 공개 이슈 대신
[보안 정책](SECURITY.md)의 비공개 신고 절차를 사용합니다.
