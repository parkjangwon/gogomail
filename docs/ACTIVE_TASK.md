# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-020
- **제목**: OpenAPI → TypeScript 클라이언트 생성
- **배경**: `docs/openapi.yaml`이 완성되어 있음. `openapi-typescript` 또는 `oapi-codegen`으로
  TS 타입/클라이언트 생성 파이프라인 추가. 프론트엔드 게이트와 무관한 백엔드 계약 작업.
- **구현 대상**: `Makefile` 또는 `scripts/gen-ts-client.sh`, `clients/typescript/` 생성물
- **완료 조건**:
  - [x] `make gen-ts-client` 실행 시 `clients/typescript/` 아래 타입 파일 생성 ✅
  - [x] `go test ./...` 통과 ✅
- **다음 태스크**: NEXT_STEPS.md 백로그 테이블 항목 모두 완료. "Next:" 섹션에서 다음 작업 선택.

---

## 완료됨

- **TASK-020**: OpenAPI → TypeScript 클라이언트 생성 ✅ (2026-05-09)
  - `Makefile`에 `gen-ts-client` 타겟 추가
  - `openapi-typescript` v7.13.0으로 `docs/openapi.yaml` → `clients/typescript/index.ts` 생성 (383KB, 11986줄)
  - `go test ./...` 통과
- **TASK-019**: Drive 파일 공유 — Directory delegation 통합 ✅ (2026-05-09)
  - `internal/httpapi/drive.go`에 `DelegatedAccessAuthorizer` 적용
  - `checkDriveDelegatedAccess` 헬퍼로 owner/actor/role 기반 권한 체크
  - 역할별 권한: read(List/Get/download), write(Trash/Restore/Rename/Move/Copy/Share), manage(PermanentDelete)
  - `go test ./...` 통과

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + docs), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```