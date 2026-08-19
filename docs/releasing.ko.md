# AARGrade 릴리스 가이드

[English](releasing.md)

AARGrade는 버전 태그를 푸시하면 GitHub Actions가 소스를 다시 검사하고,
여섯 플랫폼용 CLI와 SHA-256 체크섬을 만든 뒤 GitHub Release에 게시합니다.

## 배포되는 파일

| 운영체제 | CPU | 파일 |
| --- | --- | --- |
| macOS | Intel | `aargrade_<VERSION>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `aargrade_<VERSION>_darwin_arm64.tar.gz` |
| Linux | x86-64 | `aargrade_<VERSION>_linux_amd64.tar.gz` |
| Linux | ARM64 | `aargrade_<VERSION>_linux_arm64.tar.gz` |
| Windows | x86-64 | `aargrade_<VERSION>_windows_amd64.zip` |
| Windows | ARM64 | `aargrade_<VERSION>_windows_arm64.zip` |

각 압축 파일에는 실행 파일, 한국어·영어 README, Apache-2.0 `LICENSE`가
들어갑니다. `third_party_licenses`에는 실행 파일에 링크된 모듈과 Go
툴체인의 라이선스·고지가 들어가며, `checksums.txt`는 모든 압축 파일의
SHA-256을 기록합니다.

## 태그 전에 로컬에서 확인

```bash
make check
make vuln
make release-check
```

실제 배포할 버전 이름으로 산출물을 보고 싶다면 다음을 실행합니다.

```bash
make release RELEASE_VERSION=v0.1.0-beta.2
ls dist/v0.1.0-beta.2
```

`make release`는 GitHub에 아무것도 올리지 않습니다.

## 베타 배포

태그는 SemVer와 `v` 접두사를 사용합니다. 이미 배포된 태그는 옮기지 않고
`v0.1.0-beta.1`, `v0.1.0-beta.2`처럼 번호를 올립니다.

```bash
git switch main
git pull --ff-only
git status --short
git tag -a v0.1.0-beta.2 -m "AARGrade v0.1.0-beta.2"
git push origin v0.1.0-beta.2
```

태그가 푸시되면 `.github/workflows/release.yml`이 다음 순서로 동작합니다.

1. 태그 형식 검사
2. 태그 대상이 `main`에 합쳐진 커밋인지 확인
3. Go 테스트, vet, 취약점 검사, 공개 데모 실행
4. macOS·Linux·Windows의 amd64/arm64 바이너리 교차 빌드
5. 압축 내용, SHA-256, Linux 바이너리의 내장 버전 검사
6. 하이픈이 포함된 버전은 GitHub prerelease로 게시

진행 상황과 결과는 다음 명령으로 확인할 수 있습니다.

```bash
gh run list --workflow Release --limit 5
gh release view v0.1.0-beta.2
```

## 안정 버전 배포

베타 검증이 끝나면 하이픈이 없는 태그를 사용합니다.

```bash
git tag -a v0.1.0 -m "AARGrade v0.1.0"
git push origin v0.1.0
```

이 태그는 prerelease가 아닌 일반 GitHub Release로 게시됩니다.

## 실패했을 때

- 품질 검사나 패키징이 실패하면 Release는 생성되지 않습니다.
- 같은 워크플로를 재실행하면 기존 Release 자산을 새 산출물로 교체합니다.
- 이미 공개한 태그를 다른 커밋으로 옮기지 않습니다. 수정 후 다음
  베타 또는 패치 버전으로 새 태그를 만듭니다.
- GitHub Release 생성은 저장소의 자동 `GITHUB_TOKEN`만 사용하며 별도
  개인 토큰을 저장하지 않습니다.
