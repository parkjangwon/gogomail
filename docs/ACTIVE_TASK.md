# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-016
- **제목**: Resumable Chunked Upload — Content-Range 범위 커밋
- **배경**: `attachment_upload_sessions` 테이블과 `maildb` create/cancel/expire 경로는 이미 구현됨.
  `PUT /api/v1/attachments/upload-sessions/{id}/body`는 전체 body 단일 PUT만 지원.
  ADR 0007에 정의된 "range-aware chunk commits"가 아직 미구현이고
  `resumable_chunked_uploads` capability가 false로 고정되어 있음.
- **구현 대상**:
  - `internal/maildb` — 세션에 수신 바이트 범위(offset, received_bytes) 기록
  - `internal/mailservice` — chunk append / size 검증 / finalize 경로
  - `internal/httpapi/mail.go` — `Content-Range: bytes first-last/total` 파싱, chunk PUT
  - `docs/openapi.yaml` — 청크 업로드 엔드포인트 스펙
- **완료 조건**:
  - [ ] `go test ./...` 통과
  - [ ] `Content-Range: bytes 0-N/total` 요청으로 청크 커밋 동작
  - [ ] finalize 시 staged chunks로 attachment 생성
  - [ ] 범위 겹침/갭 → HTTP 416 반환
  - [ ] `resumable_chunked_uploads: true` capability 노출
  - [ ] docs/CURRENT_STATUS.md 갱신
  - [ ] docs/openapi.yaml 갱신
- **다음 태스크**: TASK-017 — CalDAV/CardDAV 네이티브 클라이언트 호환성 픽스처

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
