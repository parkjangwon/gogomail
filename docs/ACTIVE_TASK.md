# ACTIVE_TASK

## ID: COMPLETE

DM 검색 전체 히스토리 스캔 구현 완료 (2026-05-26)

- `internal/dm/dm.go`: `Service.Search` — 단일 1000개 페치를 페이지네이션 루프로 교체
  - `searchPageSize = 200` 상수 추가
  - 빈 페이지가 반환될 때까지 cursor 기반으로 전체 히스토리를 스캔
  - `limit`개 결과 달성 시 즉시 반환
- `internal/dm/dm_store.go`: `PostgresStore.ListSearchCandidates` — 1000 하드캡 제거, pageSize 파라미터 존중
- `internal/dm/dm_test.go`: 5개 새 테스트 추가
  - `TestSearchReturnsSinglePageResults` (회귀 방지)
  - `TestSearchPaginatesAcrossMultiplePages`
  - `TestSearchStopsWhenLimitReached`
  - `TestSearchExhaustsAllPagesWhenMatchesSparse`
  - `TestSearchCursorAdvancesPerPage`
- `go test -short ./...`: 6152 passed

## Next Steps

`docs/NEXT_STEPS.md` 백로그에서 다음 태스크를 선택할 것.
