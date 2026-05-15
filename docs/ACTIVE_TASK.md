# ACTIVE_TASK

## TASK-084: Alerts & Notifications

### 배경

Phase 8 (Admin Console) 구현 중 핵심 운영 기능. 시스템 헬스 모니터링을 위해 임계값 기반 자동 알림 시스템 필요:
- 스토리지 사용률 > 80%
- 로그인 실패 > 10회/시간
- API 오류율 > 5%

각 알림은 메일, 웹훅, 대시보드 팝업으로 전달 가능해야 함.

### 구현 대상

Backend:
- `internal/alert/` 패키지 신규: `alert.go` (threshold evaluator, alert engine)
- `internal/maildb/alert.go` (schema + query methods for `alert_configs`, `alert_history`)
- `internal/httpapi/alerts.go` (Admin API: CRUD alert configs, list alert history)
- DB migrations: alert_configs, alert_history 테이블

Frontend:
- `apps/console/src/app/companies/[id]/alerts/page.tsx` (alerts settings page)
- `apps/console/src/hooks/useAlertConfigs.ts`
- Notification toast/modal components for alert delivery

### 완료 조건

- [x] go test ./...` 통과
- [x] Alert config CRUD API 동작
- [x] Threshold evaluation engine 정상 동작
- [x] Alert history persistence
- [x] docs/CURRENT_STATUS.md 갱신
- [x] docs/backend-roadmap.md 해당 항목 체크

### 검증

- `go test ./...`
- `go build ./...`
- `pnpm -C apps/console type-check`

### 다음 태스크

TASK-085: Admin Console Frontend (Phase 1)
