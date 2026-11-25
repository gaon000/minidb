# Development

## 환경과 도구

데이터베이스 구현 언어는 Go다. [go.mod](../go.mod)에 최소 Go 버전 `1.27.4`와 모듈 경로 `github.com/gaon000/minidb`를 기록한다.

Node.js 기반 도구가 필요한 경우 Node.js 22 이상과 pnpm을 우선 사용한다. 필요한 Node.js 버전이 로컬에 없으면 nvm 설치 목록을 확인한다.

## 시작하기

[Go 공식 설치 안내](https://go.dev/doc/install)에 따라 Go를 설치한 뒤 프로젝트 루트에서 `go version`으로 버전을 확인한다. 현재 외부 모듈 의존성이 없어 별도 설치 단계가 없다.

환경변수는 이름, 용도, 필수 여부와 비밀값 없는 예제로 설명한다. 실제 credential은 안전한 별도 경로로 관리한다.

## 명령

아래 명령은 프로젝트 루트에서 실행한다.

| 작업 | 실제 명령 |
| --- | --- |
| 의존성 설치 | 해당 없음 |
| 로컬 실행 | 아래 1강 예제 |
| 정적 분석 | `go vet ./...` |
| 포맷 검사 | `gofmt -l .` 출력이 비어 있어야 함 |
| 타입 검사 | `go build ./...`에 포함 |
| 기본 테스트 | `go test ./...` |
| 통합·주요 흐름 테스트 | `go test -run '^TestStoragePersistsRecordsAcrossReopen$' ./...` |
| 빌드 | `go build ./...` |

### 1강 예제

새 임시 디렉터리에서 레코드 세 개를 저장하고, 별도 실행에서 RID `2`로 Charlie를 조회한다. `write`를 다시 실행하면 샘플 레코드가 파일 끝에 추가된다.

```sh
demo_dir=$(mktemp -d)
go run ./cmd/lesson1 write "$demo_dir/users.db"
go run ./cmd/lesson1 read "$demo_dir/users.db"
```

## 코드 기준

Go 파일은 `gofmt`로 정리한다.

JavaScript나 TypeScript를 사용할 경우 다음 기준을 적용한다.

- 새 TypeScript 파일에는 strict 모드를 적용한다.
- 문자열에는 작은따옴표를 사용한다.
- Promise 체인보다 `async`와 `await`를 우선 사용한다.
- 내보내는 함수에는 JSDoc 주석을 추가한다.

언어별 규칙은 formatter, linter, 타입 설정으로 가능한 범위에서 검증한다. 테스트에서 만든 전역 상태, mock, 타이머와 외부 자원은 종료 시 정리한다.

## 테스트 환경

Go의 표준 `testing` 패키지를 사용한다. 저장소 테스트에서 파일이 필요하면 `t.TempDir()`로 테스트별 경로를 만든다.

테스트 데이터와 외부 연동을 격리한다. 파괴적인 fixture나 migration 검증에는 폐기 가능한 전용 환경을 사용한다. 테스트 수집 실패, 전부 skip, 예상보다 줄어든 실행 범위를 통과로 해석하지 않는다.

## CI와 릴리스

CI와 릴리스 절차는 미구성이다. 첫 버전의 제공 형태를 정하고 실제 설정을 추가할 때 검사 경로와 릴리스 절차를 기록한다.
