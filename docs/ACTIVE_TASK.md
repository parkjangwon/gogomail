# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

**STATUS: IN_PROGRESS** 🔄

- **ID**: TASK-078
- **제목**: Export/Reports (CSV, PDF)
- **배경**: Phase 8-E. Implement audit report export functionality.
  - CSV export for audit logs
  - PDF report generation
  - Filtered export (date range, user, action)
  - Report formatting and styling
  - Scheduled report generation

- **구현 대상**:
  1. `internal/admin/report_service.go` — Report generation service
  2. `internal/admin/report_service_test.go` — Unit tests
  3. CSV/PDF export functionality
  4. Report filtering and formatting
  5. Scheduled report execution

- **완료 조건**:
  - [ ] `go test ./...` 통과 (새 테스트 포함)
  - [ ] ReportService with export methods
  - [ ] CSV export implementation
  - [ ] PDF report generation
  - [ ] Filtering support
  - [ ] git status: clean

- **이전 태스크**: TASK-077 ✅ (Audit Policy Config UI) — COMPLETE

---

## 다음 단계

**Phase 8 완료 후**: Frontend 작업 시작 (TASK-079+)
- Admin Console UI (Notion Mail-inspired design)
- Dashboard, User Management, Organization, Audit Logs
- Next.js 15 + Tailwind v4 + shadcn/ui

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기 ✓
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + 테스트 + docs 전부), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
