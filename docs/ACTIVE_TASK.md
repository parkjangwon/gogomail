# ACTIVE_TASK

## ✅ TASK-186: CalDAV sync-collection delta duplicate href coalescing

### 배경

CalDAV `sync-collection` delta 경로는 같은 object href가 토큰 이후 여러 번 변경되어도
raw change 개수를 기준으로 먼저 truncation을 판단했다.
클라이언트 응답에는 최신 상태의 href 하나만 필요하므로, object별 최신 변경으로 coalescing한 뒤
`nresults` limit을 판단해 불필요한 truncation을 줄인다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] joined change+object fast path가 object href별 최신 변경만 응답 후보로 남긴다.
- [x] fallback change-list path가 object href별 최신 변경만 배치 조회 및 응답 후보로 남긴다.
- [x] collection-only changes는 sync-token 갱신에는 반영하되 object response count에서는 제외한다.
- [x] raw change stream이 WebDAV report 최대치를 넘는 경우에는 기존 truncation precondition을 유지한다.
- [x] 회귀 테스트가 duplicate object changes + `nresults=1` 조합에서 단일 응답과 최신 sync-token을 검증한다.
- [x] `go test ./internal/caldavgw` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-187: CalDAV calendar-timezone VTIMEZONE RFC 정합성
