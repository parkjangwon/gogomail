# ACTIVE_TASK

## ID: COMPLETE

이슈 6개 조치 완료 (2026-05-26)

### 완료된 항목

1. **프론트 console.log 제거** ✅
   - 29개 console 앱 페이지 파일에서 63개 console.log/error/warn 제거
   - requestLog.ts, ErrorBoundary.tsx, notifications/store.ts (의도적 로깅) 보존

2. **글로벌 HTTP 바디 제한** ✅ (이미 구현됨)
   - `internal/httpapi/admin_middleware.go`: `MaxRequestBodyMiddleware(4MB)` 이미 모든 라우트에 적용 중
   - 별도 작업 불필요

3. **DM 키 로테이션 경로** ✅ 신규 구현
   - `POST /api/v1/dm/rooms/{roomID}/rotate-key`
   - `Service.RotateRoomKey`: 기존 키로 복호화 → 새 키 생성 → 재암호화 → 단일 TX로 저장
   - `Store.RotateRoomKey`: dm_room_keys + 모든 dm_messages를 단일 TX로 원자적 업데이트
   - 3개 단위 테스트, OpenAPI 스펙 추가

4. **K8s 배포 지원** ✅ 신규 구현
   - `k8s/` 디렉터리: namespace, configmap, secret 템플릿, deployment, service, hpa, pdb, ingress + README
   - Non-root 보안 컨텍스트, readOnlyRootFilesystem, HPA(2-10 replica), PDB

5. **imapgw/server.go 분리** 🔄 진행 중 (백그라운드 에이전트)
   - 9,654줄 → server_conn.go, server_auth.go, server_capabilities.go, server_mailbox.go, server_list.go, server_idle.go, server_uid.go, server_search.go, server_fetch.go, server_store.go, server_copy_append.go, server_parse.go, server_dispatch.go

6. **maildb/admin.go 분리** 🔄 진행 중 (백그라운드 에이전트)
   - 7,579줄 → admin_users.go, admin_domains.go, admin_delivery.go, admin_relay.go, admin_audit.go, admin_quota.go, admin_security.go, admin_spam.go, admin_api_keys.go, admin_validation.go

## Next Steps

`docs/NEXT_STEPS.md` 백로그에서 다음 태스크를 선택할 것.
