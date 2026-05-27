# ACTIVE_TASK

## ID: COMPLETE

Task 1 완료 (2026-05-27)

### 완료된 항목

1. **Internal proxy headers stripping** ✅
   - `internal/httpapi/admin_middleware.go`: StripInternalProxyHeadersMiddleware 구현
   - Headers stripped: X-Forwarded-For, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port, X-Real-IP, X-Client-IP, CF-Connecting-IP, True-Client-IP
   - Integrated into HTTP middleware chain in internal/app/run.go
   - 3개 unit 테스트 + 1105개 httpapi tests 통과

## Next Steps

`docs/NEXT_STEPS.md` 백로그에서 다음 태스크를 선택할 것. (Task 2: APNS private key file option)
