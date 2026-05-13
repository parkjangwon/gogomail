# ACTIVE_TASK

## TASK-258: CardDAV addressbook-query filter semantics audit

### 배경

CardDAV `addressbook-query`의 `param-filter`는 파라미터 값이 여러 개일 때도
`text-match negate-condition="yes"`를 정확히 해석해야 한다. 현재 구현은 값 중
하나라도 금지 텍스트와 다르면 통과할 수 있어, `TYPE=home,work` 같은 주소가
`home` 부정 필터를 잘못 만족할 수 있다.

### 구현 대상

- `internal/carddavgw/handler.go`
- `internal/carddavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 부정 `param-filter`가 모든 파라미터 값을 검사해 금지 값이 하나라도 있으면 거절한다.
- [x] 회귀 테스트가 다중 파라미터 값의 negated text-match 의미를 커버한다.
- [x] `go test ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-259: CardDAV addressbook-query candidate optimization audit
