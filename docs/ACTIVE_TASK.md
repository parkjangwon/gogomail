# ACTIVE_TASK

## Current Task

**TASK-CODEBASE-IMPROVEMENTS — Phase 3 Code Quality Improvements**

## Background

Following completion of the DM instant messaging and monitoring stack implementation (May 2026), the current focus is on codebase quality improvements identified in the Phase 3 evaluation. This includes TypeScript file splits, Go package refactoring, and documentation hygiene.

## Scope

### TypeScript file splits
- Extract MCP tool implementations into separate modules (`apps/gogomail-user-mcp/src/tools/`)
- Refactor webmail API layer (`apps/webmail/src/api/`)
- Organize UI components by domain (`apps/webmail/src/components/`)

### Go package refactoring
- Split `internal/httpapi/admin.go` into focused modules
- Refactor `internal/app/admin_service.go` for better maintainability
- Ensure consistent error handling and logging across packages

### Documentation hygiene
- Reset accumulated AI-agent logs (`docs/CURRENT_STATUS.md`, `docs/NEXT_STEPS.md`)
- Update `PROJECT_HARNESS.md` with current workflow
- Ensure `docs/openapi.yaml` reflects all recent additions

## Completion Checklist

- [ ] TypeScript file splits completed
- [ ] Go package refactoring completed
- [ ] Documentation updated and verified
- [ ] `go test ./...` passes
- [ ] `npm test` + `npm run type-check` pass in webmail and MCP apps
- [ ] Commit and push to `origin/main`

## Next Task

After completion, refer to `docs/NEXT_STEPS.md` backlog (priority order):
1. OpenSearch integration
2. DM search scalability
3. SMTP rate limiting per recipient domain
4. Attachment virus scanning
5. And more...
