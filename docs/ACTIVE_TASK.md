# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-001
- **제목**: iMIP RFC 6047 wire format 테스트 추가
- **배경**: `internal/scheduling/handler.go`의 `buildMultipartMessage`가 이전에
  `Content-Type: message/rfc822`를 사용했다 (RFC 6047 위반). 수정은 완료됐지만
  이 동작을 보장하는 단위 테스트가 없다.
- **구현 대상**: `internal/scheduling/handler_test.go`

### 완료 조건

- [ ] `TestBuildMultipartMessageContentType`: 반환된 메시지 바이트에
  `Content-Type: text/calendar; method=REQUEST` 파트가 있고
  `message/rfc822`가 없음을 assert
- [ ] `TestBuildMultipartMessageCANCEL`: method=CANCEL이면
  `Content-Type: text/calendar; method=CANCEL` 파트가 있음을 assert
- [ ] `TestBuildMultipartMessageHeaderOrder`: 헤더가 From/To/Subject/Date/Message-ID/MIME-Version/Content-Type 순서로 나타남을 assert
- [ ] `go test ./internal/scheduling/...` 통과
- [ ] docs/CURRENT_STATUS.md 갱신 (iMIP RFC 6047 테스트 커버리지 추가 반영)

### 커밋 후 다음 태스크 후보 (백로그 참고)

`docs/NEXT_STEPS.md` §4 Pipeline extension hooks:
"Add first-party FCM/APNs/Web Push sink adapters behind `internal/pushnotify`"

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
