# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-021
- **제목**: WebDAV Gateway — Drive RFC 4918 지원
- **배경**: `internal/webdavgw`는 WebDAV XML 유틸만 존재. HTTP 핸들러 + Drive 연동 미구현.
  WebDAV는 OpenAPI로 정의 불가하므로 REST가 아닌 별도 프로토콜로 구현.
- **구현 대상**: `internal/httpapi/webdav.go` — RFC 4918 PROPFIND/PROPPATCH/MKCOL/GET/PUT/DELETE/COPY/MOVE 지원
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] WebDAV PROPFIND로 Drive 노드 목록 조회 가능
- **다음 태스크**: TASK-022

---

## 완료됨

- **TASK-021**: WebDAV Gateway — Drive RFC 4918 지원 ✅ (2026-05-09)
  - `internal/httpapi/webdav.go`에 RFC 4918 WebDAV 핸들러 구현
  - PROPFIND (목록), MKCOL (폴더 생성), GET (다운로드), DELETE (휴지통), MOVE/COPY (이동/복사) 지원
  - `internal/httpapi/webdav_test.go`에 11개 테스트 (모두 통과)
  - `go test ./...` 통과
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