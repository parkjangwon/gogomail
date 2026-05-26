# Quality Fixes — 2026-05-26

## HTTP Client Timeout Code Quality Issues

### Fix 1: Named Constant for SSO Timeout ✅

**File**: `internal/sso/verify.go`

**Issue**: 15-second timeout defined in two places (oidcHTTPClient AND hardcoded in context.WithTimeout calls), creating drift risk.

**Solution**: 
- Added named constant: `const oidcRequestTimeout = 15 * time.Second`
- Updated `oidcHTTPClient.Timeout` to use the constant
- Replaced hardcoded `15*time.Second` in `fetchJWKSURI()` (line 195) with `oidcRequestTimeout`
- Replaced hardcoded `15*time.Second` in `getJWKSKeys()` (line 245) with `oidcRequestTimeout`

**Verification**: All SSO tests pass

### Fix 2: Better Comment for Push Notification Client ✅

**File**: `internal/pushnotify/pushnotify.go`

**Issue**: Comment for `defaultPushHTTPClient` lacked context about the 30-second timeout choice.

**Solution**:
- Enhanced comment to explain the timeout rationale:
  - "30s covers FCM/APNs/WebPush handshake + typical response latency while still bounding goroutines."

**Verification**: All pushnotify tests pass (51 tests)

## Test Results

```
go test ./internal/sso/... ./internal/pushnotify/... -v
Go test: 74 passed in 2 packages
```

Both fixes ensure consistency and clarity without changing behavior.
