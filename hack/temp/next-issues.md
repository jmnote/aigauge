# PR #43 후속 이슈

이번 리뷰에서 확인했지만 현재 변경에는 포함하지 않은 항목이다.

## 동작·성능

- Antigravity 활성 폴링마다 `agy models`를 선행 실행한다. 인증 상태를 먼저 확인하는
  현재 설계는 단순하지만, 성공 폴링에서도 서브프로세스가 하나 더 실행된다. `/usage`가
  실패했을 때만 보조 진단을 실행하는 방식과 비용·진단 품질을 비교한다.
- Claude/Codex 사용량 조회가 같은 인스턴스의 credential store를 한 주기 안에서 두 번
  읽고 복호화한다. 이미 읽은 토큰을 refresh 경로와 공유하거나 적절한 캐시를 검토한다.
- `RemoveProviderInstance`는 credential 삭제 후 설정 저장을 수행한다. 설정 저장 실패 시
  인스턴스와 credential 상태가 일시적으로 어긋날 수 있으므로, 삭제 순서와 복구 정책을
  명확히 한다. credential 보존을 우선한 현재 동작을 바꾸기 전 실패 시나리오를 검토한다.

## 구조·유지보수

- Antigravity 공개 타입이 파싱 단계에서 AI Gauge 표시용 이름으로 정규화된다. 원시 API
  타입과 `DisplayUsage` 변환의 경계를 일관되게 할지 검토한다.
- `ConnectProvider`, `SubmitAuthCode`, `ImportProvider`의 공통 오류·진단 흐름을 helper로
  통합한다.
- `diagnoseClaude`와 `diagnoseCodex`의 공통 로직을 추출할지 검토한다.
- `get-tokens.go`와 auth 저장소의 legacy credential 파싱을 공유해 두 구현이 갈라지지
  않도록 한다.
- auth/config 저장소의 임시 파일 저장 helper를 공통화한다. Windows의 기존 파일 교체,
  flush/fsync, 실패 시 임시 파일 정리 정책을 함께 확인한다.
- `ConnectProvider`의 Antigravity 분기를 provider metadata의 연결 전략으로 확장한다.
- `providerTypeCatalog`와 frontend의 반복 조회·공통 timestamp 생성 등 소규모 중복을
  정리한다.

## 진단·테스트

- credential store 복호화·읽기 오류의 UI/복구 안내를 더 세분화할지 검토한다.
- `limitedBuffer`의 short-write 불변식을 회귀 테스트로 복원한다.
- Pending cleanup, refresh singleflight, HTTP context 취소의 경계 조건을 전용 테스트로
  보강한다.
- Antigravity `Description`을 `DisplayUsage`까지 전달할 필요가 있는지 결정한다.
- `refreshTokenRaw`가 성공 상태의 비정상 응답에서 원시 response body를 보존하도록 할지
  검토한다.

## 참고

- Disconnect 기능은 중복 UX로 판단해 제거했으므로 후속 이슈로 다루지 않는다.
- Pending 메인 창 노출, 설정창 종료 cleanup race, Antigravity 단일 인스턴스 제한,
  credential 읽기 오류 분류는 이번 변경에서 반영했다.
