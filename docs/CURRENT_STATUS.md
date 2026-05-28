# gogomail current status

Last updated: 2026-05-28 (audit error logging fix + WebAuthn passkey UI)

## Audit error logging + WebAuthn passkey UI (2026-05-28)

- **maildb**: `user_mcp_access_keys.go` — replaced silent `_ = audit.Insert` with `slog.ErrorContext` in `CreateUserMCPAccessKey` and `RevokeUserMCPAccessKey`. Audit insert failures now surface in structured logs instead of being silently discarded.
- **webmail**: Added Passkeys section to Security settings (`SettingsSecuritySection.tsx`):
  - New `lib/api/webauthn.ts` — base64url ↔ ArrayBuffer helpers, coerce creation/request options, credential serializer
  - `lib/api/auth.ts` — `listPasskeyCredentials`, `registerPasskey`, `deletePasskeyCredential`
  - Passkeys card with list view, add flow (full browser WebAuthn ceremony via `navigator.credentials.create`), remove
  - Guarded by `isWebAuthnSupported()` with fallback message
  - i18n keys added to en/ko/ja/zh-CN

## Post-remediation hardening round 37 (2026-05-28)

**IDOR sweep: admin_operations.go — mail-flow-logs list/stats endpoints**

- **GET /admin/v1/mail-flow-logs**: Added `requiresCompanyAccess(req.CompanyID)` after `parseMailFlowLogListRequest`.
- **GET /admin/v1/mail-flow-logs/stats**: Added `requiresCompanyAccess(req.CompanyID)` after `parseMailFlowLogStatsRequest`.
- **GET /admin/v1/mail-flow-logs/daily-stats**: Added `requiresCompanyAccess(req.CompanyID)` after `parseMailFlowLogDailyStatsRequest`.

Company_admin could enumerate mail-flow logs belonging to other companies by supplying `company_id` query param.

## Post-remediation hardening round 36 (2026-05-28)

**IDOR sweep: admin_usage.go — api-usage aggregate list endpoints**

- **GET /admin/v1/api-usage/daily**: Added `requiresCompanyAccess(req.CompanyID)` after `parseAPIUsageAggregateListRequest`. Company_admin could enumerate any company's daily usage aggregates.
- **GET /admin/v1/api-usage/monthly**: Same fix.

**Acknowledged / out-of-scope for now:**
- api-usage/ledger, api-usage/ledger/export, ledger/retention*, api-usage/export-batches and all artifact/digest/signature sub-endpoints: These are scoped by `TenantID` (billing tenant), which is a separate concept from `CompanyID`. No direct `tenant_id → company_id` mapping exists in the handler layer. These endpoints are effectively system-admin-only in practice but lack an explicit gate. Requires architecture-level decision.

## Post-remediation hardening round 35 (2026-05-28)

**IDOR sweep: admin_domain.go (8 handlers) + admin_usage.go (2 list endpoints)**

admin_domain.go:
- **POST /admin/v1/domains**: Added `requiresCompanyAccess(req.CompanyID)` before CreateDomain — company_admin could create domains under other companies.
- **GET/PUT /admin/v1/domains/{id}/api-settings**: Added `GetDomain → requiresCompanyAccess` — no company isolation existed.
- **POST/GET /admin/v1/domains/{id}/api-keys**: Added `GetDomain → requiresCompanyAccess` — API key creation/listing for arbitrary domains.
- **DELETE /admin/v1/domains/{id}/api-keys/{keyid}**: Added `GetAPIKey → GetDomain → requiresCompanyAccess`. Previously `{id}` was ignored entirely; keyid used directly with zero isolation.
- **POST /admin/v1/domains/{id}/api-keys/{keyid}/rotate**: Same pattern — `GetAPIKey → GetDomain → requiresCompanyAccess`.
- **PATCH /admin/v1/domains/{id}/policy**: Added `GetDomain → requiresCompanyAccess` before UpdateDomainPolicy.

Added `GetAPIKey` method to `admin.Service` (wrapping `admin.Repository.GetAPIKey`), to `adminDomainService` interface, and test fake.

admin_usage.go:
- **GET /admin/v1/quota-alert-thresholds**: Added `requiresCompanyAccess(companyID)` after parsing `company_id` query param.
- **GET /admin/v1/quota-alerts**: Added `requiresCompanyAccess(companyID)` after parsing `company_id` query param.

## Post-remediation hardening round 34 (2026-05-28)

**IDOR sweep: admin_system.go, admin_governance.go, admin_company.go**

- **GET /admin/v1/roles** (`handleListRoles`): Added `requiresCompanyAccess(ctx, companyID)` after parsing `company_id` query param. Company_admin was able to enumerate roles of any company; now blocked.
- **POST /admin/v1/roles** (`handleCreateRole`): Added `requiresCompanyAccess(ctx, req.CompanyID)` before `CreateAdminRole`. Company_admin was able to create roles in any company.
- **POST /admin/v1/domains/bulk** (`handleBulkDomains`): For each domain ID, now fetches domain first with `GetDomain`, then calls `requiresCompanyAccess(ctx, domain.CompanyID)` before executing activate/suspend/delete. Company_admin could previously activate, suspend, or delete domains belonging to other companies.
- **POST /admin/v1/companies/{id}/users/bulk-import**: Added per-user domain ownership check (`GetDomain → domain.CompanyID == id`). Company_admin could previously specify `domain_id` values from other companies, injecting users into foreign domains. If domain lookup fails or CompanyID mismatch, the user record is added to the failures list.

## Post-remediation hardening round 33 (2026-05-28)

**IDOR sweep: admin_storage.go — 7 handlers (drive sessions, nodes, usage, cleanup failures)**

- **GET /drive-upload-sessions** (user_id required): Added `GetUser → GetDomain → requiresCompanyAccess`.
- **GET /drive-nodes** (list, user_id required): Added `GetUser → GetDomain → requiresCompanyAccess`.
- **GET /drive-usage** (user_id required): Added `GetUser → GetDomain → requiresCompanyAccess`.
- **POST /drive-cleanup-failures/{id}/resolve** (by ID): Added `GetDriveObjectCleanupFailure → GetUser → GetDomain → requiresCompanyAccess` before mutation. New `GetObjectCleanupFailure` method added to `drive.Repository`, `drive.ObjectCleanupFailureStore` interface, `drive.Service`, `admin_service_storage.go`, and test fakes.
- **GET /quota-usage** (domain_id optional): If `domain_id` provided, added `GetDomain → requiresCompanyAccess`.
- **GET /attachment-upload-sessions** (user_id optional): If `user_id` provided, added `GetUser → GetDomain → requiresCompanyAccess`.
- **GET /drive-cleanup-failures** (user_id optional): If `user_id` provided, added `GetUser → GetDomain → requiresCompanyAccess`.

## Post-remediation hardening round 32 (2026-05-28)

**IDOR sweep: admin_storage.go — GET /admin/v1/drive-nodes/{id}**
- Handler took `user_id` as a query parameter and fetched the user's drive node without company isolation. A company_admin could read drive nodes of users in other companies. Fixed with `GetUser → GetDomain → requiresCompanyAccess` before proceeding to `GetDriveNode`.

## Post-remediation hardening round 31 (2026-05-28)

**IDOR sweep: admin_mail.go — push notification, DKIM keys, suppression list (5 handlers)**
- **GET/PATCH /push-notification-attempts/{id}**: `PushNotificationAttemptView.CompanyID` confirmed. Added `requiresCompanyAccess(attempt.CompanyID)`. PATCH pre-fetches the attempt before updating.
- **DELETE/POST /dkim-keys/{id}**: Added `GetDKIMKey` to `maildb.Repository` (SELECT by ID). DKIM key has `DomainID` → `GetDomain → requiresCompanyAccess`. Added service interface method + test stub.
- **DELETE /suppression-list/{id}**: Added `GetSuppressionEntry` to `maildb.Repository`. Entry has `DomainID` → `GetDomain → requiresCompanyAccess`. Added service interface method + test stub.
- **System-level (no fix needed)**: `DELETE /trusted-relays/{id}`, `DELETE /delivery-routes/{id}`, `PATCH /delivery-routes/{id}/status`, `POST /outbox/{id}/retry` — these resources have no company/domain scope.

## Post-remediation hardening round 30 (2026-05-28)

**IDOR sweep: admin_ldap_sync.go + admin_rdbms_sync.go (11 handlers)**
All handlers use `{id}` as a domain ID from `/domains/{id}/ldap/...` or `/domains/{id}/rdbms/...` paths but had no company isolation.

- **admin_ldap_sync.go** (7): `handleLDAPSync`, `handleLDAPSyncHistory`, `handleLDAPSyncConflicts`, `handleResolveLDAPConflict`, `handleGetIdPConfig`, `handleSetIdPConfig`, `handleDeleteIdPConfig` — all now call `GetDomain → requiresCompanyAccess`.
- **admin_rdbms_sync.go** (4): `handleRDBMSSync`, `handleRDBMSSyncHistory`, `handleRDBMSSyncConflicts`, `handleResolveRDBMSConflict` — same fix.

## Post-remediation hardening round 29 (2026-05-28)

**IDOR sweep: directory aliases/group-memberships/delegations (7 handlers) — requires new Get methods**
- Added `GetAlias`, `GetGroupMembership`, `GetDelegation` to `internal/directory/repository_*.go`
- Added `GetDirectoryAlias`, `GetDirectoryGroupMembership`, `GetDirectoryDelegation` to admin service + interface
- **admin_directory.go**: All 7 mutation handlers now call Get → `requiresCompanyAccess(r.Context(), resource.CompanyID)` before proceeding:
  - `DELETE /admin/v1/directory/aliases/{id}`
  - `DELETE /admin/v1/directory/group-memberships/{id}`
  - `PATCH /admin/v1/directory/group-memberships/{id}/role`
  - `PATCH /admin/v1/directory/group-memberships/{id}/assignment`
  - `PATCH /admin/v1/directory/delegations/{id}/role`
  - `PATCH /admin/v1/directory/delegations/{id}/assignment`
  - `DELETE /admin/v1/directory/delegations/{id}`

## Post-remediation hardening round 28 (2026-05-28)

**IDOR sweep: admin_alerts.go — alert rule/channel by ID endpoints (5 handlers)**
- `handleGetAlertRule`: `AlertRule.CompanyID` present; no isolation. Added `requiresCompanyAccess(r.Context(), rule.CompanyID)` after `GetAlertRule`.
- `handleUpdateAlertRule`: No isolation. Added `GetAlertRule → requiresCompanyAccess` before applying updates.
- `handleDeleteAlertRule`: No isolation. Added `GetAlertRule → requiresCompanyAccess` before deletion.
- `handleUpdateAlertChannel`: `GetAlertChannel` was already called; added `requiresCompanyAccess(r.Context(), channel.CompanyID)` immediately after.
- `handleDeleteAlertChannel`: Added `GetAlertChannel → requiresCompanyAccess` before deletion.

## Post-remediation hardening round 27 (2026-05-28)

**IDOR sweep: admin_operations.go (3 endpoints)**
- **GET /admin/v1/audit-logs/{id}**: `AuditLogView.CompanyID` present; no isolation check. Fixed with `requiresCompanyAccess(r.Context(), log.CompanyID)`.
- **GET /admin/v1/mail-flow-logs/{id}**: `MailFlowLogView.CompanyID` present; no isolation check. Fixed same pattern.
- **POST /admin/v1/imap/mailboxes/{id}/uid-backfill**: `userID` query param targets a specific user's mailbox; no company isolation. Fixed with `GetUser → GetDomain → requiresCompanyAccess`.
- **GET /admin/v1/outbox-events/{id}**: `OutboxEventView` has no `CompanyID`; treated as system-level resource (no change needed).

## Post-remediation hardening round 26 (2026-05-28)

**IDOR sweep: domain stats/dns-check, security/access/usage policy handlers (21 endpoints)**
- **admin_domain.go**: `GET /admin/v1/domains/{id}/stats`, `GET /admin/v1/domains/{id}/dns-check`, `GET /admin/v1/domains/{id}/dns-checks` — missing `requiresCompanyAccess`. Fixed with `GetDomain → requiresCompanyAccess`.
- **admin_security_config.go**: 8 domain-scoped handler functions (`handleGetDomainSpamFilterPolicy`, `handlePutDomainSpamFilterPolicy`, `handleGetDomainRoutingRules`, `handlePutDomainRoutingRules`, `handleGetDomainSMTPPolicy`, `handlePutDomainSMTPPolicy`, `handleGetDomainDmarcSpfPolicy`, `handlePutDomainDmarcSpfPolicy`) — missing company isolation. Fixed with `GetDomain → requiresCompanyAccess`.
- **admin_access_policy.go**: 8 domain-scoped handler functions (`handleGetDomainRetentionPolicy`, `handlePutDomainRetentionPolicy`, `handleGetDomainIPPolicy`, `handlePutDomainIPPolicy`, `handleGetDomainAuthPolicy`, `handlePutDomainAuthPolicy`, `handleGetDomainSecurityGovernancePolicy`, `handlePutDomainSecurityGovernancePolicy`) — missing company isolation. Fixed with `GetDomain → requiresCompanyAccess`.
- **admin_usage.go**: `GET/POST /admin/v1/quota-alert-thresholds` and `GET/PATCH/DELETE /admin/v1/quota-alert-thresholds/{id}` and `GET /admin/v1/quota-alerts/{id}` — `QuotaAlertThresholdView` and `QuotaAlertView` both have `CompanyID`; GET/PATCH/DELETE by ID had no isolation. Fixed by fetching the resource first and calling `requiresCompanyAccess(r.Context(), threshold.CompanyID)` (or `alert.CompanyID`). POST (create) checks `req.CompanyID` before creating.

## Post-remediation hardening round 25 (2026-05-28)

**IDOR on POST /admin/v1/users/{id}/invite**
- **internal/httpapi/admin_user.go**: The invite handler fetched the user ID from the path and immediately created an invite token + sent an invite email without any company isolation check. A `company_admin` could trigger invite emails to any user in any company by supplying an arbitrary `{id}`. Fixed with `GetUser → GetDomain → requiresCompanyAccess`, consistent with all other user-scoped admin endpoints.

## Post-remediation hardening round 24 (2026-05-28)

**Cross-tenant IDOR in user-scoped admin endpoints (follow-up to Round 23)**
- **PATCH /admin/v1/users/{id}/recovery-email** (admin_mail.go): Missing company isolation — a `company_admin` could change another company's user's recovery email, enabling account takeover of external accounts. Fixed with `GetUser → GetDomain → requiresCompanyAccess`.
- **GET /admin/v1/users/{id}/mfa** (admin_user.go): Missing company isolation — `company_admin` could read another company's user's MFA status. Fixed same pattern.
- **DELETE /admin/v1/users/{id}/mfa** (admin_user.go): Missing company isolation — `company_admin` could disable another company's user's MFA. Fixed same pattern.
- **GET /admin/v1/users/{id}/config** and **GET /admin/v1/users/{id}/config/{key}** (admin_user.go): Missing company isolation — `company_admin` could read user configuration entries for any user. Fixed same pattern.

All five handlers now enforce the same `GetUser → GetDomain → requiresCompanyAccess` pattern as `UpdateUserStatus`, `UpdateUserQuota`, and `UpdateUserPasswordHash`.

## Post-remediation hardening round 23 (2026-05-28)

**Privilege escalation + IDOR via PATCH /admin/v1/users/{id}/role**
- **internal/httpapi/admin_user.go**: The role-update handler had two vulnerabilities:
  1. **Cross-tenant IDOR**: No `requiresCompanyAccess` check. A `company_admin` for company A could change the role of any user by supplying that user's ID, including users in company B. Fixed by adding `GetUser` → `GetDomain` → `requiresCompanyAccess` (same pattern as `UpdateUserStatus` and `UpdateUserQuota`).
  2. **Privilege escalation**: No restriction on which roles a `company_admin` can assign. A `company_admin` could call this endpoint with `{"role": "system_admin"}` to self-elevate or promote any user to `system_admin`. Fixed by checking `claims.Role != "system_admin"` when `req.Role == "system_admin"` — only a static admin token or `system_admin` JWT may assign the `system_admin` role.

## Post-remediation hardening round 22 (2026-05-28)

**WebDAV Basic auth accepts MFA-pending tokens (MFA bypass)**
- **internal/httpapi/webdav.go**: WebDAV supports Basic auth where the password field is a JWT. The handler called `VerifyFull` (signature + session-version check) but did not check `claims.TokenType`. A user who had completed password auth but not yet TOTP could use their `mfa_pending` JWT as the WebDAV password and access calendar/contacts data without completing MFA. The same bypass was fixed in Round 13 for admin API endpoints. Fixed by adding `if claims.TokenType == "mfa_pending" { 401 }` check — consistent with `claimsFromRequest` (used by the webmail API) and `adminJWTOrStaticAuth` (used by the admin API).

## Post-remediation hardening round 21 (2026-05-28)

**X-Gogomail-* internal headers not stripped at ingress (spoofable by external clients)**
- **docker/nginx-backend.conf** and **docker/nginx-single.conf**: The `X-Gogomail-Tenant-ID`, `X-Gogomail-Company-ID`, `X-Gogomail-Domain-ID`, `X-Gogomail-API-Key-ID`, and `X-Gogomail-Principal-ID` headers are consumed by the API metering layer (`apimeter`) to attribute usage to specific tenants. Nginx was forwarding client-supplied values verbatim, allowing an external client to falsify billing/usage records by sending arbitrary header values. Fixed by adding `proxy_set_header X-Gogomail-... ""` directives in all `location /` blocks — nginx overwrites whatever the client sent with an empty string before forwarding.
- **k8s/ingress.yaml** and **helm/gogomail/values.yaml**: Same fix via `nginx.ingress.kubernetes.io/configuration-snippet` annotation for Kubernetes deployments.

## Post-remediation hardening round 20 (2026-05-28)

**Admin MFA verify endpoint missing rate limit (TOTP brute-force)**
- **internal/httpapi/admin_mfa.go**: `POST /admin/v1/auth/mfa/verify` had no rate limiting. The `mfa_pending` token has a 5-minute TTL. Without a limit, an attacker who has stolen a `mfa_pending` token (e.g., via phishing the password step) could attempt all 1,000,000 six-digit codes before the token expires. The webmail equivalent (`/api/v1/auth/mfa/verify` in `mail_mfa.go`) already had `NewAdminIPRateLimiter(5, time.Minute)`. Applied the same limiter to the admin path: 5 attempts per IP per minute = at most 25 guesses in the 5-minute window, making brute-force computationally infeasible.

## Post-remediation hardening round 19 (2026-05-28)

**CSV formula injection in admin user/audit-log exports**
- **internal/httpapi/admin_company.go**: User export CSV wrote `u.Username` and `u.DisplayName` verbatim. A user whose display name starts with `=`, `+`, `-`, or `@` (e.g., `=HYPERLINK("http://evil.com","click")`) causes formula execution when an admin opens the file in Excel or LibreOffice Calc. Applied `sanitizeCSVCell` to both fields.
- **internal/httpapi/admin_governance.go**: Audit-log export CSV wrote `l.TargetID` (can be an email address) and `l.Result` without escaping. Same formula-injection risk. Applied `sanitizeCSVCell` to both fields.
- **internal/httpapi/csv_sanitize.go** (new): `sanitizeCSVCell` helper — prefixes any cell starting with `=`, `+`, `-`, `@`, `\t`, or `\r` with a tab character, which causes spreadsheet applications to treat the value as plain text rather than a formula.

## Post-remediation hardening round 18 (2026-05-28)

**Custom HMAC-SHA1 implementation in TOTP**
- **internal/authmfa/authmfa.go**: `hmacSHA1` was a 25-line hand-rolled HMAC-SHA1 (XOR pads, inner/outer hash) rather than using `crypto/hmac`. While functionally correct, the custom implementation bypasses the `hmac.Equal` constant-time comparison guarantee and carries reinvention risk (e.g., subtle length-extension or pad-length bugs if the code is ever modified). Replaced with `hmac.New(sha1.New, key)` from the standard library — fewer lines, auditable, and time-constant by default.

## Post-remediation hardening round 17 (2026-05-28)

**SCIM bearer-token brute-force (no rate limit)**
- **internal/httpapi/scim.go**: All 7 SCIM endpoints used `scimAuth` which performs a constant-time token comparison but applies no rate limiting. An attacker could probe the static bearer token at full network speed. Fixed by creating a shared `NewAdminIPRateLimiter(20, time.Minute)` inside `RegisterSCIMRoutes` and wrapping every handler with `protect(h)` — rate limit fires before the auth check so even failed attempts are throttled. 429 responses include `Retry-After: 60`.

**Legacy password hash upgrade missing from SMTP/IMAP and LDAP auth paths**
- **internal/maildb/submission.go**: `AuthenticatePlain` (SMTP submission auth) called `auth.VerifyPasswordHash` and never triggered the async PBKDF2 upgrade. Users whose sole access was via SMTP/IMAP would keep `sha256:` or `plain:` hashes indefinitely. Fixed by switching to `auth.VerifyPasswordHashResult` and spawning the same async `upgradePasswordHash` goroutine used by the web login path.
- **internal/maildb/ldap_auth.go**: `AuthenticateLDAP` had the same gap. Additionally updated the SQL query to fetch `u.id::text` (needed by the upgrader). Now uses `VerifyPasswordHashResult` and fires the async upgrade on success.

**Context: web login (`AuthenticateUser` in `user_auth.go`) already had upgrade-on-login since the prior session; SMTP/LDAP were the remaining gaps.**

## Post-remediation hardening round 16 (2026-05-28)

**SSRF via user-controlled Web Push subscription endpoint**
- **internal/httpapi/mail_push.go**: `POST /api/v1/me/push-subscriptions` stored the user-supplied `endpoint` URL without any SSRF validation. When the server later dispatches push notifications for that user, it makes an outbound HTTP request to the stored endpoint. A malicious user could register a subscription pointing at `http://169.254.169.254/...` or any RFC1918 address, causing the server to probe internal infrastructure. Fixed by calling `webhookguard.ValidateOutboundHTTPURL` before storing the endpoint, returning 400 on blocked URLs.

## Post-remediation hardening round 15 (2026-05-28)

**SSRF in SSO SAML metadata URL fetch**
- **internal/httpapi/admin_security_config.go**: `handlePostCompanySSOTest` fetched the admin-configured `metadata_url` (SAML IdP metadata endpoint) using only `url.Parse` for syntax validation but no SSRF guard. An admin could point this at `http://169.254.169.254/...` or other internal addresses to probe the internal network or cloud metadata service. Fixed by adding `webhookguard.ValidateOutboundHTTPURL` before the fetch, consistent with the pattern used in webhook channels, BIMI logo fetch, and calendar URL proxy.

## Post-remediation hardening round 14 (2026-05-28)

**Email header injection in system email and alert dispatcher**
- **internal/mailservice/systememail.go**: `buildRFC2822Message` embedded `from`, `to`, and `subject` values directly into RFC 2822 headers via string concatenation without stripping CRLF. An email address containing `\r\n` could inject additional headers (e.g., `Bcc:` lines), enabling spam relay or phishing. Added `sanitizeEmailHeader()` which removes `\r` and `\n` before all header values.
- **internal/alert/dispatcher.go**: Same issue in `sendEmail()` — alert channel config fields `from`, `to`, and `notification.AlertType` were unescaped. Fixed with the same `sanitizeEmailHeader` helper.

## Post-remediation hardening round 13 (2026-05-28)

**Admin MFA pending token bypass**
- **internal/httpapi/admin_middleware.go**: The `adminAuth` JWT wrapper checked `claims.Role` but not `claims.TokenType`. Admin MFA pending tokens carry the full `Role` field (`company_admin` or `system_admin`) alongside `TokenType = "mfa_pending"`. A user who obtained a pending token (i.e., passed password auth but not yet completed TOTP) could use that token to call any admin API endpoint without completing MFA, completely defeating the MFA enforcement. Fixed by checking `claims.TokenType == "mfa_pending"` inside the role-valid branch and returning 401 with "mfa verification required". The mail-API path already had an equivalent guard in `claimsFromRequest`.

## Post-remediation hardening round 12 (2026-05-28)

**SAML AuthnRequest XML injection**
- **internal/sso/sso.go**: `AuthnRequest.BuildXML()` assembled the SAML XML via `fmt.Sprintf`, embedding admin-configured `Destination` (SSO URL) and `Issuer` (Entity ID) fields verbatim without XML escaping. A malicious `Destination` value containing `"` or `>` could break out of the attribute context and inject arbitrary XML elements into the SAML request. Fixed by replacing the `fmt.Sprintf` construction with a proper `encoding/xml` struct marshal (`samlAuthnRequest`), which escapes all attribute and character-data values automatically.

## Post-remediation hardening round 11 (2026-05-28)

**Admin API-wide tenant isolation (IDOR) — bulk fix**
- **internal/httpapi/admin_access_policy.go** (16 handlers): ip-policy, auth-policy, audit-policy, governance, retention-policy, session-policy, sessions, rate-limit endpoints lacked `requiresCompanyAccess` checks, allowing a company_admin to read/write another company's security configuration by substituting the `{id}` path parameter.
- **internal/httpapi/admin_governance.go** (13 handlers): webhooks, notification-templates, audit-log export, health, change-history, pending-approvals endpoints had the same gap.
- **internal/httpapi/admin_security_config.go** (10 handlers): spam-filter, quota-summary, routing-rules, SSO config/test endpoints were unprotected.
- **internal/httpapi/admin_system.go** (8 handlers): security posture, global signature, legal holds, SCIM status, seat usage endpoints fixed (follow-up from round 10 discovery).
- **internal/httpapi/admin_user.go** (1 handler): `GET /admin/v1/companies/{id}/mfa/stats` fixed.
- **internal/httpapi/admin_helpers.go** (1 handler): `GET /admin/v1/companies/{id}/security/login-audits` (`handleCompanyLoginAudits`) fixed.

Pattern applied consistently: `requiresCompanyAccess(ctx, id)` immediately after `parseBoundedAdminPathValue` succeeds, returning HTTP 403 on cross-tenant access. 39 handlers total. Domain-scoped handlers were not modified.

## Post-remediation hardening round 10 (2026-05-28)

**Alert admin endpoints missing tenant isolation (IDOR)**
- **internal/httpapi/admin_alerts.go**: Five company-ID-scoped alert endpoints (`handleCreateAlertRule`, `handleListAlertRules`, `handleCreateAlertChannel`, `handleListAlertChannels`, `handleListAlertEvents`) accepted an arbitrary company ID in the URL path without verifying that the calling `company_admin` owns that company. A company_admin could enumerate or create alert rules/channels for another company. Fixed by adding `requiresCompanyAccess(r.Context(), companyID)` checks — consistent with the pattern used in `admin_company.go`, `admin_domain.go`, and `admin_user.go`.

## Post-remediation hardening round 9 (2026-05-28)

**Alert channel webhook SSRF guard**
- **internal/httpapi/admin_alerts.go**: `handleCreateAlertChannel` and `handleUpdateAlertChannel` stored webhook URLs without SSRF validation. A company_admin could configure a webhook pointing at `http://169.254.169.254/...` (or any internal service), causing the alert dispatcher to send HTTP requests there when alerts fire. Both endpoints now call `webhookguard.ValidateOutboundHTTPURL` before storing webhook-type channel configs.

## Post-remediation hardening round 8 (2026-05-28)

**Tracking endpoint X-Forwarded-For spoofing fix**
- **internal/httpapi/tracking.go**: `clientIP` used for email-open recording unconditionally trusted `X-Forwarded-For`, identical to the MFA IP spoofing bug fixed in round 5. Fixed with the same trusted-proxy check: XFF is only honoured when the TCP peer is loopback or RFC1918. Without the fix an external attacker could falsify the recorded IP address for email open events.

## Post-remediation hardening round 7 (2026-05-28)

**MFA verify endpoint rate limit**
- **internal/httpapi/mail_mfa.go**: `POST /api/v1/auth/mfa/verify` had no rate limiting, allowing a brute-force attack against the 6-digit TOTP code if an attacker had obtained a `pending_token` (which requires the user's password). Added a 5-attempts-per-minute-per-IP rate limiter (`mfaVerifyLimiter`) mirroring the same guard used on the login and password-reset endpoints. With the 5-minute pending token TTL, the old code allowed up to ~1 million attempts; the new limit caps it at 25.

## Post-remediation hardening round 6 (2026-05-28)

**Password CPU DoS cap applied to all auth paths**
- **internal/auth/password.go**: `VerifyPasswordHash` now rejects passwords longer than 1024 bytes (`MaxAuthPasswordBytes`) before starting any PBKDF2 computation. This protects all callers (HTTP login, LDAP bind, SMTP SASL PLAIN) in a single place. `MaxAuthPasswordBytes` is exported so callers can surface a user-friendly error before even calling into `maildb`.
- **internal/httpapi/mail_auth.go**: `POST /api/v1/auth/token` now returns 400 "password is too long" immediately when the password field exceeds 1024 bytes, before calling `AuthenticateUser`.
- **internal/httpapi/admin_auth.go**: Admin login endpoint now applies the same 1024-byte cap with a 400 response.
- Previously only the password-change and reset endpoints had this cap; the login, LDAP, and SMTP paths were unprotected and could be driven to 210k PBKDF2 iterations over multi-megabyte inputs.

## Post-remediation hardening round 5 (2026-05-28)

**MFA IP spoofing fix + calendar subscription SSRF**
- **internal/httpapi/mail_mfa.go**: `extractClientIP` previously trusted `X-Forwarded-For` unconditionally, letting any caller fake their IP and potentially bypass MFA CIDR exemptions. Now uses the same trusted-proxy check as `adminClientIP`: `X-Forwarded-For` is only honoured when the TCP peer is a loopback or RFC1918 address (i.e., a trusted reverse proxy).
- **internal/httpapi/calendar.go**: `POST /api/v1/calendar-subscriptions` and the fetch endpoint only validated `http/https` scheme but did not block private/loopback IPs. An authenticated user could subscribe to `http://169.254.169.254/...` or any internal service. Both endpoints now call `webhookguard.ValidateOutboundHTTPURL` and use `GuardedHTTPClient` to prevent SSRF.

## Post-remediation hardening round 4 (2026-05-28)

**SAML info leak, JMAP RFC compliance, invite password cap**
- **internal/httpapi/sso.go**: SAML signature verification failure no longer returns the raw error string to the client (prevented oracle-style information disclosure on the ACS endpoint). Error now logged via `slog.WarnContext` with domain_id; generic message returned to caller.
- **internal/jmap/handler.go**: `ServeAPI` now enforces `Content-Type: application/json` per RFC 8620 §3.3. Missing or non-JSON content type returns 400 `notJSON` immediately, before body parsing.
- **internal/httpapi/admin_user.go**: Invite-accept endpoint (`POST /admin/invite/{token}/accept`) and admin create-user endpoint (`POST /admin/v1/users`) now cap password at 1024 bytes (same limit as change-password and reset-password) to prevent CPU DoS via PBKDF2. Internal salt/hash errors are now logged via slog and return a generic 500 instead of leaking the error string.
- **internal/httpapi/admin_company.go**: Bulk-import endpoint now caps per-user password at 1024 bytes (adds to failures list for that user rather than aborting the whole batch); hash error no longer leaks internal details into the failures list.

## Post-remediation hardening round 3 (2026-05-28)

**Replace bare json.Decoder with decodeJSONBody in httpapi handlers (security/cso-audit-remediation)**
- **internal/httpapi/alerts.go** (2): `makeCreateAlertConfigHandler`, `makeUpdateAlertConfigHandler`
- **internal/httpapi/calendar.go** (3): `POST /api/v1/calendars`, `PATCH /api/v1/calendars/{id}`, `POST /api/v1/calendar-subscriptions`
- **internal/httpapi/carddav.go** (2): `POST /api/mail/addressbooks`, `PATCH /api/mail/addressbooks/{id}`
- **internal/httpapi/admin_alerts.go** (4): `handleCreateAlertRule`, `handleUpdateAlertRule`, `handleCreateAlertChannel`, `handleUpdateAlertChannel`
- **internal/httpapi/orgchart.go** (4): `POST /units`, `PUT /units/{id}`, `POST /members`, `PATCH /members/{id}`
- **internal/httpapi/scim.go** (3): SCIM uses `application/scim+json` — used inline `LimitReader+DisallowUnknownFields` instead of `decodeJSONBody`
- `decodeJSONBody` enforces `application/json` Content-Type, 1MB body cap, `DisallowUnknownFields`, and single-value rejection
- Updated `alerts_test.go` to set `Content-Type: application/json` on POST/PUT requests

## Post-remediation hardening round 2 (2026-05-27)

**Two more code-review findings fixed**
- **internal/maildb/me.go** + **internal/httpapi/password_reset.go**: Added 1024-byte cap on `new_password` in both the change-password and reset-password flows. Without the cap, a caller could submit a multi-megabyte string and force PBKDF2 210k iterations over it — a CPU DoS. IMAP already had the same cap (`maxIMAPAuthPasswordBytes = 4096`).
- **internal/httpapi/admin_ldap_sync.go**: Replaced three `writeError(w, 500, err.Error())` with generic messages + `slog.ErrorContext` log. The old code leaked LDAP/DB error strings (connection strings, hostnames, SQL details) in the HTTP response body.

## Post-remediation hardening (2026-05-27)

**Three follow-up improvements found during post-CSO code review**
- **internal/maildb/user_auth.go**: `upgradePasswordHash` goroutine now wraps context with 60-second timeout via `context.WithTimeout`; prevents goroutine leak if DB connection hangs during async hash upgrade
- **internal/bimi/bimi.go**: `FetchLogo` now calls `webhookguard.ValidateOutboundHTTPURL` before fetching, blocking SSRF attacks where an attacker-controlled `_bimi.domain` DNS record points to a private/internal IP; `LogoCache.allowPrivateNetwork` field added (false by default) for test override
- **internal/httpapi/webdav.go**: Two unbounded `io.ReadAll(r.Body)` calls (PROPFIND and PROPPATCH handlers) now use `io.LimitReader` capped at `maxJSONBodyBytes` (1 MB) to prevent large-body DoS

## Task 8: Next.js CSP nonce — webmail + console (2026-05-27)

**Replace `script-src 'unsafe-inline'` with per-request nonce-based CSP in both Next.js apps**
- **apps/webmail/src/middleware.ts** + **apps/console/src/middleware.ts**: Generate a base64 nonce (`Buffer.from(crypto.randomUUID()).toString('base64')`) on each request; set `Content-Security-Policy` header with `script-src 'self' 'nonce-<nonce>'` (no `'unsafe-inline'`); forward nonce to layout via `x-nonce` request header
- **apps/webmail/src/app/layout.tsx**: Import `headers` from `'next/headers'`; read `x-nonce` header into `nonce` variable; apply `nonce={nonce}` to the theme-init inline `<script>` tag
- **apps/webmail/next.config.ts** + **apps/console/next.config.ts**: Removed `scriptSrc` variable and `Content-Security-Policy` header block (now set dynamically by middleware); removed `isProduction` variable (was only used for scriptSrc); kept all other security headers unchanged
- **Acceptance criteria met**: `grep "'unsafe-inline'" apps/webmail/next.config.ts apps/console/next.config.ts` → 0 matches; nonce present in both middleware files and in webmail layout

## Task 7: Redis 기반 어드민 로그인 rate limiter (2026-05-27)

**Replace in-memory admin login rate limiter with Redis-backed distributed limiter**
- **internal/httpapi/admin_rate_limiter_redis.go** (new): `adminLoginRateLimiter` interface with `Middleware(next http.Handler) http.Handler`; `RedisAdminLoginLimiter` struct wrapping `ratelimit.RedisFixedWindowLimiter`; `NewRedisAdminLoginLimiter(client, limit, window)` constructor
- **internal/httpapi/admin_types.go**: Added `redisLoginClient *redis.Client` field to `adminRouteConfig`; added `WithRedisLoginLimiter(client *redis.Client) AdminRouteOption`
- **internal/httpapi/admin.go**: `registerAdminUtilityRoutes` now creates `loginLimiter` as `adminLoginRateLimiter` interface — uses `RedisAdminLoginLimiter` when `cfg.redisLoginClient != nil`, falls back to `AdminIPRateLimiter` (in-memory) otherwise
- **internal/httpapi/admin_auth.go**: `registerAuthAndAdminUserRoutes` parameter changed from `*AdminIPRateLimiter` → `adminLoginRateLimiter`
- **internal/app/run.go**: Appends `httpapi.WithRedisLoginLimiter(redisClient)` to `adminRouteOpts` when `redisClient != nil`
- **Test coverage**: 1274 tests pass across `internal/httpapi` and `internal/app`

## Task 6: 레거시 패스워드 해시 로그인 시 자동 업그레이드 (2026-05-27)

**Auto-upgrade legacy password hashes on successful login**
- **internal/auth/password.go**: Added `GenerateSalt(n int) []byte` (crypto/rand); added `VerifyPasswordHashResult(password, encoded string) (verified bool, needsUpgrade bool)` — returns `needsUpgrade=true` for `plain:` and `sha256:` formats
- **internal/maildb/user_auth.go**: `AuthenticateUser` now calls `VerifyPasswordHashResult` instead of `VerifyPasswordHash`; on `needsUpgrade=true` calls `upgradePasswordHash` (best-effort, non-blocking); `upgradePasswordHash` re-hashes with PBKDF2-SHA256 (210k rounds, 32-byte random salt) and updates `users.password_hash`
- **internal/auth/password_test.go**: Added `TestVerifyPasswordHashResult` (5 cases: plain match/mismatch, sha256 match/mismatch, pbkdf2 no upgrade)
- **Test coverage**: 602 tests pass across `internal/auth` and `internal/maildb`

## Task 5: JWT 내부 구현 → golang-jwt/jwt/v5 (2026-05-27)

**JWT library replacement**: Replaced hand-rolled HMAC-SHA256 JWT implementation with `golang-jwt/jwt/v5`
- **internal/auth/jwt.go**: Replaced manual `crypto/hmac`/`crypto/sha256`/base64 parsing with `jwt.ParseWithClaims` + `jwt.NewWithClaims`; public `Claims`/`TokenManager` interfaces unchanged
- Added `jwtInternalClaims` (embeds `jwt.RegisteredClaims`) as wire format; `Sign` maps `Claims` → `jwtInternalClaims`, `Verify` maps back
- Used `jwt.WithTimeFunc(m.now)` to inject mocked clock so tests using `manager.now` continue to pass
- Pre-validation retained: segment-size checks, manual header decode (alg+typ check), manual iat-future check — required by existing test suite
- `sign` unexported method kept for `signedTestToken` test helper
- Token format (HS256 HMAC-SHA256) unchanged — existing tokens remain valid
- **Test coverage**: All 47 `internal/auth` tests pass; all three extra test files pass

## Task 4: RDBMS identity provider SQL allowlist (2026-05-27)

**SQL injection prevention**: Admin-configured SQL queries validated to enforce SELECT-only operations
- **internal/idprovider/rdbms/provider.go**: Added `validateSourceQuery()` function with regex pattern to block SQL keywords (UNION, INSERT, UPDATE, DELETE, DROP, TRUNCATE, CREATE, ALTER, EXEC, EXECUTE, GRANT, REVOKE); enforces SELECT prefix, max 4096-byte limit, and allows trailing semicolon only
- **internal/idprovider/rdbms/provider.go** Connect(): Integrated query validation after db.Ping() for both UserQuery and GroupQuery; closes DB and returns error if validation fails
- **internal/idprovider/rdbms/provider_test.go**: Added TestValidateSourceQuery with 11 test cases covering empty query, valid SELECT statements (case-insensitive, leading space OK), injection patterns (UNION, INSERT, DROP with semicolon), length limits, and forbidden keywords
- **Test coverage**: 55 tests pass in rdbms package (added 11 new validation test cases)
- **Acceptance criteria met**: All 11 test cases pass; validation rejects empty/non-SELECT/injection queries; accepts valid SELECT with trailing semicolon; case-insensitive matching

## Task 3: Helm CHANGEME guard (2026-05-27)

**Helm deployment safety**: Added validation to prevent accidental deployment with placeholder secret values
- **helm/gogomail/templates/_helpers.tpl**: Added `gogomail.requireNotChangeme` helper function that validates secret values are not empty or contain "CHANGEME" placeholder; fails with descriptive error message if validation fails
- **helm/gogomail/templates/secret.yaml**: Added three guard calls before data block to validate GOGOMAIL_DM_MASTER_KEY, GOGOMAIL_AUTH_JWT_SECRET, and GOGOMAIL_ADMIN_TOKEN
- **Verification**: `helm template ./helm/gogomail` fails with "must be set to a non-placeholder value" error when using default values; `helm template` succeeds when all three secrets are set to real values via --set flags
- **Acceptance criteria met**: All three secrets fail helm template when containing CHANGEME, pass when set to real values

## Task 2: APNS private key file option (2026-05-27)

**Configuration enhancement**: Support for APNS private key from file path
- **internal/config/config.go**: Added APNsPrivateKeyFile field; Load() function reads file if path is specified, with file taking precedence over GOGOMAIL_APNS_PRIVATE_KEY env var
- **internal/config/config_file.go**: Added YAML support for apns_private_key_file configuration key
- **internal/config/validate.go**: Added validation to ensure file can be read and contains data
- **Test coverage**: Added 6 tests (TestLoadAPNsPrivateKeyFromEnvironmentVariable, TestLoadAPNsPrivateKeyFromFile, TestLoadAPNsPrivateKeyFilePathPrecedesEnvironmentVariable, TestValidateRejectsNonexistentAPNsPrivateKeyFile, TestValidateAcceptsAPNsPrivateKeyFromEnvironmentVariable, TestValidateRejectsEmptyAPNsPrivateKeyFile)
- **Impact**: Supports Kubernetes secret mounting patterns; backward compatible with existing GOGOMAIL_APNS_PRIVATE_KEY environment variable
- **Config test suite**: 368 passed (added 6 new tests)

## Task 1: Strip internal proxy headers (2026-05-27)

**Security hardening**: Strip inbound X-Gogomail-* request headers to prevent metering/billing spoofing
- **internal/httpapi/admin_middleware.go**: Added `StripInternalHeadersMiddleware` that removes 6 inbound X-Gogomail-* request headers before handler processing
- **internal/app/run.go**: Integrated StripInternalHeadersMiddleware right after RequestIDMiddleware
- **Test coverage**: `TestStripInternalHeadersMiddleware` (strips all 6 headers), `TestStripInternalHeadersMiddleware_PreservesOtherHeaders`
- **Headers stripped from requests**: X-Gogomail-Resolved-User-ID, X-Gogomail-Tenant-ID, X-Gogomail-Company-ID, X-Gogomail-Domain-ID, X-Gogomail-Principal-ID, X-Gogomail-API-Key-ID
- **Impact**: External callers cannot spoof internal metering or billing attribution headers

## Task 0: Secure defaults (2026-05-27)

**Security audit remediation**: Production-safe configuration defaults
- **internal/config/config.go** (line 371): Default GOGOMAIL_ENV changed from "development" to "production"
- **docker/docker-compose.scale.yml** (line 27): PostgreSQL sslmode default changed from "disable" to "require" (enforces encrypted connections)
- **Test suite hardening**: Added `setDevelopmentMode(t)` helper to ~40+ test functions in internal/config tests; fixed t.Parallel() conflicts by removing from tests that use t.Setenv()
- **cmd/gogomail/main_test.go**: Added GOGOMAIL_ENV environment setup to TestRunAcceptsStorageProfileConfigs, TestRunRejectsInvalidYAMLConfigBeforeAppStart, TestRunUsesAppModeEnvWhenModeFlagUnset, TestRunModeFlagOverridesAppModeEnv
- **Impact**: All 362 internal/config tests + 12 cmd/gogomail tests pass; production defaults now secure (no insecure auth allowed unless explicitly configured in development mode)

## Recent improvements (2026-05-27)

- **docker-compose.dev.yml**: `GOGOMAIL_DM_MASTER_KEY` → CHANGEME placeholder; Redis `requirepass dev-redis-password` + healthcheck updated; `GOGOMAIL_REDIS_PASSWORD` added to shared env anchor.
- **internal/app/run.go**: `context.Background()` → `context.WithoutCancel(ctx)` for OTel shutdown and HTTP graceful shutdown (Go 1.21+).
- **MessageList.tsx**: Filter/sort pipeline (`baseFiltered`, `afterLabelFilter`, `afterCategoryFilter`, `sortedBase`, `filteredMessages`, `threadCounts`, `pagedMessages`) wrapped in `useMemo`; `PAGE_SIZE`, `PULL_THRESHOLD`, `SKELETON_COUNT` extracted as module-level constants.
- **MessageRow.tsx**: Wrapped in `React.memo`; avatar `<img>` gets `loading="lazy" decoding="async"`.
- **useMailList.ts**: `foldersError` / `messagesError` states added; `.catch(() => {})` replaced with proper error capture.
- **useMessage.ts**: Module-level map cache replaced with TTL-aware cache (5 min); `error` state added.

## Recent refactoring

- Split `CalendarModals.tsx` (793 lines) into 6 files: `SubscriptionAddModal.tsx`, `CalendarManagementModal.tsx`, `EventCreateModal.tsx`, `EventEditModal.tsx`, `TodoCreateModal.tsx`, `calendarModalStyles.ts`; `CalendarModals.tsx` reduced to a 16-line re-export barrel with no logic changes.
- Extracted `useSettingsPrefs` hook from `SettingsView.tsx` (1098→563 lines); holds all 109 state variables and preference load/save effects.
- Extracted `useContactsBooks`, `useContactsEdit` hooks from `ContactsView.tsx` (723→~330 lines); hooks live in `apps/webmail/src/components/contacts/`.
- Extracted `useCalendarCreateForm`, `useCalendarEditForm`, and `useCalendarData` custom hooks from `CalendarView.tsx` (812→~340 lines); hooks live in `apps/webmail/src/components/calendar/`.
- Extracted `useInlineComposeSend` and `useInlineComposeAttachments` custom hooks from `InlineCompose.tsx` (477 lines); hooks live in `apps/webmail/src/components/reading-pane/`; pure code movement, no logic changes.
- Extracted `useDMModal`, `useMailLabels`, `useMailSession` hooks from `mail/page.tsx` (2302→2114 lines); hooks live in `apps/webmail/src/app/mail/`.
- Extracted `useMailSearch`, `useMailLayout` hooks from `mail/page.tsx` (2114→N lines); hooks live in `apps/webmail/src/app/mail/`.
- Extracted `useDriveUpload` (upload state, refs, pause/resume/cancel/scheduler) to `apps/webmail/src/components/drive/useDriveUpload.ts` and `useDriveSidebar` (folder tree state/loading) to `apps/webmail/src/components/drive/useDriveSidebar.ts`; DriveView.tsx reduced from 1412 → ~900 lines with no logic changes.
- Extracted `ComposeModal` (1309 lines) into three hooks: `useComposeDraft` (auto-save), `useComposeSlash` (slash command menu), `useComposeSend` (send lifecycle + countdown). No logic changes — pure code movement.
- Extracted `useMailThreads`, `useMailSettings`, `useMailToasts` hooks from `mail/page.tsx` (2085→1961 lines).
- Extracted `SpotlightSearch` pre-component helpers (`SpotlightItem`, `SYSTEM_ICONS`, `SCOPES`, `SpotlightT`, `sectionLabel`, `relativeTime`, `formatDriveSize`) to `apps/webmail/src/components/spotlight/spotlightHelpers.tsx`.
- Extracted `MessageList` helpers (`KO_KEYS`, `DateGroupKey`, `getDateGroup`) to `apps/webmail/src/components/message-list/messageListHelpers.ts`.
- Extracted `useReadingPaneAttachments`, `useReadingPaneMedia`, `useReadingPaneCalendar` hooks from `ReadingPane.tsx` (865→667 lines); hooks live in `apps/webmail/src/components/reading-pane/`.
- Extracted `useOrgTree` (org tree load, expand/collapse, search filtering), `useAddressBook` (address book load, contacts load, contacts search), and `useRecipients` (to/cc/bcc list management) from `OrgPickerModal.tsx` (620→457 lines); hooks live in `apps/webmail/src/components/org-picker/`.
- Extracted `useMessageListSelection` (bulk-select state, shift-click range, Escape/Ctrl+A/action-key shortcuts, row keyboard nav) and `useContactHoverCard` (hover card state + debounced timers) from `MessageList.tsx` (669→~520 lines); hooks live in `apps/webmail/src/components/message-list/`.
- Extracted `useMailCompose` (composeContext, openCompose/closeCompose, pendingCompose) and `useMailNav` (activeApp + localStorage/URL persist, activeFolderId, selectedMessageId) from `mail/page.tsx` (1961→1934 lines); hooks live in `apps/webmail/src/app/mail/`.
- Extracted `useComposeRecipients` (to/cc/bcc/from state, address loading, recent recipients) and `useComposeUI` (signature, emoji/org/send-dropdown toggles, confirmClose, imageResizeToolbar, trackOpens) from `ComposeModal.tsx` (1144→1115 lines); hooks live in `apps/webmail/src/components/compose/`.
- Extracted `useDriveNodes` (file/folder data loading, refresh, usage/quota) and `useDriveInteractions` (rename, menu, new-folder, drag/drop, selection, share) from `DriveView.tsx` (1197→1118 lines); hooks live in `apps/webmail/src/components/drive/`.

## Platform summary

GoGoMail is a production-grade self-hosted email platform written in Go
(single binary, 24 runtime modes). Key capabilities:

- **Protocols**: SMTP (inbound/outbound), IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP, JMAP (RFC 8620/8621 — all 20 methods implemented)
- **Mail security**: SPF, DKIM, DMARC, ARC, MTA-STS, TLS-RPT, DANE
- **Storage**: PostgreSQL (multi-tenant), Redis Streams (outbox), S3-compatible object storage
- **Auth**: JWT + TOTP MFA + refresh token rotation
- **Frontend**: Next.js 16 webmail SPA + admin console (TypeScript/TSX)
- **AI interface**: User MCP (124 tools) + Manage MCP (50 admin tools)
- **Monitoring**: Prometheus + Loki + Promtail + Grafana (provisioned dashboards)

## Completed milestones (2026-05)

| Date | Feature |
|------|---------|
| 2026-05-27 | Refactor(webmail): extracted pre-component helpers from apps/webmail/src/app/mail/page.tsx (lines 45–171) into mailPageHelpers.ts — 10 constants, 4 types, DM_RESIZE_HANDLES array, 10 functions; no logic changes |
| 2026-05-27 | Refactor(webmail): extract `RenderOrgTree`+`RenderOrgTreeProps` from `OrgPickerModal.tsx` into `OrgPickerTree.tsx`; extract `AVATAR_COLORS`, `avatarColor`, `initials`, `ParsedContact`, `ContactsSort`, `ContactsDensity`, `loadContactViewSettings`, `useContactsParsed` from `ContactsView.tsx` into `components/contacts/contactsViewHelpers.ts` |
| 2026-05-27 | Refactor(webmail): extract DriveView pre-component helpers (15 types/constants/functions, lines 22–320) into apps/webmail/src/components/drive/driveViewHelpers.ts; DriveView.tsx reduced by ~300 lines |
| 2026-05-27 | Refactor(webmail): extract SenderListTable from SettingsBlockedSection — blocked/allowed sender table+form JSX moved to shared SenderListTable.tsx (variant prop); SettingsBlockedSection.tsx reduced from 509→221 lines |
| 2026-05-27 | Refactor(webmail): extracted SettingsView blocked/vacation sections — SettingsBlockedSection.tsx (~280 lines: blocked senders, allowed senders, spam filter) and SettingsVacationSection.tsx (~100 lines) from SettingsView.tsx; SettingsView reduced from 1945→1465 lines |
| 2026-05-27 | Refactor: split 16 more large backend files — caldavgw/repository.go (2520→1107 lines, +3 files: repository_objects/sync/scheduling.go), storage/s3.go (2358→620 lines, +6 files: s3_request/sign/escape/validate/error/parse.go), carddavgw/repository.go (2325→1002 lines, +3 files: repository_vcard/objects/sync.go), maildb/messages.go (1876→trimmed, +messages_bulk.go), drive/repository.go (1854→trimmed, +repository_move/files.go), app/run.go (1820→trimmed, +run_workers/storage/metrics.go), httpapi/admin_helpers.go (1583→1120 lines, +admin_ldap_sync/rdbms_sync.go), carddavgw/xml.go (1476→1130 lines, +xml_filter/address_data.go), maildb/imap_copy_move.go (1444→1087 lines, +imap_expunge/uid_alloc.go), caldavgw/xml.go (1404→1116 lines, +xml_filter/calendar_data.go), httpapi/drive.go (1389→trimmed, +drive_share/download.go), dm/dm_store.go (1361→trimmed, +dm_store_keys/members/media.go), maildb/admin_api_usage_export.go (1230→deleted, +artifact/batch/manifest.go), maildb/attachment_upload_sessions.go (trimmed, +validate.go), maildb/admin_users.go (trimmed, +validate.go), dm/dm.go (1094→808 lines, +dm_attachments/membership.go), smtp/receiver.go (1092→737 lines, +receiver_auth/spool/defaults.go), config/validate.go (1082→777 lines, +validate_helpers.go); 6183 tests pass |
| 2026-05-27 | Refactor: split 5 large backend files — ldapgw/server.go (3287→745 lines, +5 files: server_pdu.go, server_search.go, server_attributes.go, server_controls.go, server_filter.go), maildb/imap_uid.go (3273→stub, +6 files: imap_mailboxes.go, imap_messages.go, imap_flags.go, imap_copy_move.go, imap_uid_backfill.go, imap_uid_helpers.go), caldavgw/handler.go (3178→308 lines, +5 files: handler_timezone/collection/objects/propfind/report.go), carddavgw/handler.go (2830→273 lines, +5 files: handler_collection/objects/propfind/report/sync.go), directory/repository.go (2922→~270 lines, +4 files: repository_aliases/search/memberships/delegations.go); 1974 tests pass |
| 2026-05-26 | Bug fix: internal/storage/s3.go — two pre-existing S3 signing bugs fixed: (1) escapeS3Segment now uses S3-strict unreserved-only encoding (was url.PathEscape which left @, =, ! unencoded, causing SignatureDoesNotMatch for paths with those chars); (2) s3PayloadHash computes actual SHA-256 body hash for HTTP endpoints (MinIO rejects UNSIGNED-PAYLOAD over plain HTTP); test infrastructure: pg_trgm installed in public schema via docker-entrypoint-initdb.d; webauthn test uses real UUID user |
| 2026-05-26 | HTTP client: webhook/urlguard.go *http.DefaultClient value copy replaced with explicit http.Client{} for clarity; timeout always set via DefaultOutboundTimeout |
| 2026-05-26 | Code structure: internal/imapgw/server_search.go (2170 lines) split into server_search.go (304), server_search_criteria.go (292), server_search_executor.go (901), server_search_match.go (692); server_fetch.go (2005 lines) split into server_fetch.go (478), server_fetch_body.go (549), server_fetch_envelope.go (1002); all files ≤1200 lines; 439 tests pass |
| 2026-05-26 | Code structure: internal/maildb/admin_api_usage.go split from 2361 lines into 5 domain files (admin_api_usage_quota.go, admin_api_usage_aggregate.go, admin_api_usage_ledger.go, admin_api_usage_retention.go, admin_api_usage_export.go); admin_api_usage.go reduced to 85 lines (shared utils); 549 tests pass |
| 2026-05-26 | Code structure: internal/mailservice/service.go split from 3415 lines into 10 domain files (service_folders.go, service_threads.go, service_search.go, service_imap.go, service_user.go, service_messages.go, service_drafts.go, service_attachments.go, service_delivery.go, service_helpers.go); service.go reduced to 244 lines; 305 tests pass |
| 2026-05-26 | Code structure: internal/httpapi/mail.go split from 3726 lines into 9 domain files (mail_auth.go, mail_folders.go, mail_messages.go, mail_threads.go, mail_drafts.go, mail_attachments.go, mail_push.go, mail_profile.go, mail_helpers.go); mail.go reduced to 502 lines; 1102 tests pass |
| 2026-05-26 | Code structure: internal/app/run.go split from 4048 lines into 10 subsystem files (run_imap.go, run_pop3.go, run_dav.go, run_ldap.go, run_scim.go, run_smtp.go, run_workers.go, run_search.go, run_push.go, run_delivery.go); run.go reduced to 1820 lines; 169 tests pass |
| 2026-05-26 | DM export: filename {ownerEmail}_{roomName}_{YYYYMMDDHHmmss}.txt; timestamps in user timezone (IANA via ?timezone= param); mail list reloads on web push (sw.js broadcasts mail_update to clients); recent search delete button in SpotlightSearch + SearchBar |
| 2026-05-26 | DM export filename UUID bug fixed: frontend now parses Content-Disposition header before creating blob URL; backend falls back to display_name (not UUID) when email is empty. Mail list double-poll race removed: additive setInterval in useMailList eliminated; single refresh() in page.tsx handles all polling. Mail reading pane: email dark/light toggle moved from body to MailActions toolbar (moon/sun icon); composeToAddress/blockSender buttons polished to production quality. DM export filename order: {roomName}_{ownerEmail}_{datetime}.txt. Reading pane: removed "메일 보내기" compose button from sender header; contact-save button remains |
| 2026-05-26 | Mail list refresh stale-closure fix: useMailList refresh() now uses useRef for both folderId and in-flight guard instead of useState — eliminates race where old closure (refreshing=false) could start a concurrent refresh between setRefreshing(true) and next render; folder-change guard discards in-flight results if folder switches mid-fetch; refresh() has empty deps and is created once per mount |
| 2026-05-26 | Inbox post-send refresh: ComposeModal gains onAfterSend prop; handleSuccessfulSend calls it immediately on success; page.tsx wires it to setTimeout(refresh, 1500) so backend has 1.5s to deliver before the inbox fetch fires — fixes inbox not updating after self-send when VAPID push is not configured |
| 2026-05-26 | Mail list rendering fix: handleRefresh now re-fetches threads in parallel alongside messages — threadViewEnabled is hardcoded true so getVisibleMailMessages always uses buildThreadMessages(threads); previous refresh() only updated messages/folders leaving threads stale, making refresh button and periodic poll appear non-functional; fix: listThreads() called on every non-virtual-folder refresh |
| 2026-05-26 | Config quality: TrustedProxyCIDRs changed from string to []string (loaded via splitCSV); Middleware/parseClientIP/isTrustedForwardingProxy accept []string, eliminating per-request splitting; validate.go gains GOGOMAIL_TRUSTED_PROXY_CIDRS and GOGOMAIL_SYSTEM_SMTP_ADDR validation |
| 2026-05-26 | Config centralization: os.Getenv removed from apikeys/middleware.go (trustedProxyCIDRs threaded as param), mailservice/systememail.go (NewSMTPSystemEmailSender replaces NewSMTPSystemEmailSenderFromEnv), httpapi/admin_auth.go (bootstrap creds via WithAdminBootstrap option); SystemEmailConfig + AdminBootstrapConfig named types added to config.Config |
| 2026-05-26 | HTTP client timeouts: replaced http.DefaultClient with 15s-timeout client in internal/sso (OIDC discovery + JWKS) and 30s-timeout client in internal/pushnotify (FCM/APNs/WebPush adapters); prevents goroutine leaks on unresponsive external services |
| 2026-05-26 | JMAP EventSource code quality: sseWriteEvent logs slog.Warn on marshal error, dead typesParam variable removed, fakeNotifier uses sync.Once for safe channel teardown, added TestEventSourcePingFormat + TestEventSourceDeliversStateChange; 62 tests pass |
| 2026-05-26 | JMAP Task 12: EventSource SSE push (RFC 8620 §7.3) — GET /jmap/eventsource/ with types/closeafter/ping params, initial state event, ping ticker, StateNotifier interface for live push, closeafter=state closes after first change; StateNotifier + Notifier field added to Deps; route registered in httpapi/jmap.go; 5 unit tests pass (60 total) |
| 2026-05-26 | FIDO2/WebAuthn MFA: go-webauthn/webauthn integration, 6 HTTP endpoints (register/begin, register/complete, authenticate/begin, authenticate/complete, credentials GET/DELETE), credential + challenge store, DB migration 0155 |
| 2026-05-26 | JMAP identity code quality: removed dead `wanted` map, create/destroy paths now return serverFail on SetWebmailPreferences error, destroy loop restructured to load-once/save-once eliminating read-modify-write race, update stub returns notImplemented instead of silent success |
| 2026-05-26 | JMAP auth hardening: userIDFromBearer now calls VerifyFull(ctx,token) instead of Verify(token) — session revocation enforced on every authenticated request; TestServeSessionRequiresAuth rewritten with real TokenManager asserting HTTP 401 |
| 2026-05-26 | Error wrapping: bare `return err` wrapped with fmt.Errorf context in internal/jmap (email_set.go — 5 occurrences in applyEmailPatch) and internal/mailservice/service.go exported methods (DeleteFolder, UnsubscribeIMAPMailboxName, DeleteIMAPMailbox, SetMessageFlag, MoveMessage, DeleteMessage, RestoreMessage, DeletePushDevice, DeleteDraft — 16 occurrences); 367 tests pass |
| 2026-05-26 | JMAP RFC compliance: Identity replyTo/bcc now marshal as null (not omitted), Identity/set destroy tracks and persists removals to preferences, SearchSnippet/get accepts filter param per RFC 8621 §7.1 |
| 2026-05-26 | JMAP code quality (EmailSubmission/VacationResponse): removed dead second GetMessage call post-SendDraft, json.Marshal errors handled for submission encoding, GetWebmailPreferences errors return notUpdated instead of silently using defaults, json.Marshal errors handled for vrRaw/newPrefs, proper JMAP patch semantics (RFC 8620 §3.3 key-iteration merge, leading-slash stripping); 55 tests pass |
| 2026-05-26 | JMAP RFC compliance (EmailSubmission/VacationResponse): identityId required validation, full RFC 8621 §7.2 submission fields (threadId, mailboxIds, envelope, sendAt RFC3339, undoStatus, deliveryStatus, dsnBlobIds, mdnBlobIds), newState diverges from oldState on create/update, VacationResponse/get ids filter (non-singleton → notFound), VacationResponse/set newState diverges on update; 53 tests pass |
| 2026-05-26 | JMAP Task 11: EmailSubmission/set, VacationResponse/get + /set — DraftSender interface in jmap package, EmailSubmission/set verifies draft exists then calls Sender.SendDraft (nil Sender → serverFail), VacationResponse stored in webmail preferences under `vacationResponse` key (singleton; create/destroy → forbidden), VacationResponse/set merges patch into stored struct; Sender DraftSender field added to Deps; 3 new methods registered in NewHandler; 4 unit tests pass (53 total) |
| 2026-05-26 | JMAP Task 10: Identity/get, Identity/set, SearchSnippet/get — primary identity from users table, custom identities stored in webmail preferences JSON under `identities` key, SearchSnippet/get fetches subject+TextBody preview via GetMessage; 4 unit tests pass |
| 2026-05-26 | JMAP Task 9: Email/changes, Email/copy, Email/import, Email/parse — modseq-based change tracking from imap_message_uid, MIME header parsing for Email/parse, Email/copy and Email/import stubs with proper RFC error types; 4 unit tests pass |
| 2026-05-26 | JMAP Task 8: Email/set — applyEmailPatch (keywords/$seen/$flagged/$draft, mailboxIds patch semantics), DeleteMessage for destroy, SaveDraft for create; 5 unit tests pass |
| 2026-05-26 | JMAP Task 7: Email/query, Email/queryChanges — text filter routes to SearchMessages, list filter routes to ListMessagesPage with keyword/flag mapping ($seen/$flagged); Email/queryChanges returns cannotCalculateChanges; 4 unit tests pass |
| 2026-05-26 | JMAP Task 6: Email/get real DB integration — messageDetailToJMAP property filtering, flagsToKeywords ($seen/$flagged/$draft), parseJMAPAddrs, body values/parts, requestTooLarge enforcement; 4 unit tests pass |
| 2026-05-26 | JMAP Task 5: Thread/get, Thread/changes — threadGetMethod calls ListThreadMessagesPage per threadID, notFound for missing threads; threadChangesMethod returns empty created/updated/destroyed with current modseq state; 3 unit tests pass |
| 2026-05-26 | JMAP Task 4: Mailbox/get, /query, /set, /changes — folderToMailbox conversion, JMAP role mapping (inbox/sent/drafts/trash/spam→junk/archive), CreateFolder/RenameFolder/DeleteFolder wired, conservative Mailbox/changes returning all IDs when state changes; 6 unit tests pass |
| 2026-05-26 | JMAP Task 3: State strings + blob upload/download — EmailStateFor/MailboxStateFor (modseq/session_version based), POST /jmap/upload, GET /jmap/download, jmap_blobs migration 0156; hardened error handling (DB insert failure returns 500, ErrNoRows-only fallback, Content-Disposition header per RFC 8620 §6.2) |
| 2026-05-26 | JMAP Task 2: BackReference resolution RFC 8620 §3.7 — resolveBackRefs/walkPath in internal/jmap/backref.go; wildcard (/list/*/id) and index (/list/0/id) path support; missing callID returns invalidResultReference error; wired into ServeAPI dispatch loop with prevResults map; 4 unit tests pass |
| 2026-05-26 | JMAP Task 1: Handler Deps + JWT auth + request validation — Deps{Repo,Store,Auth} struct, JWT Bearer auth on ServeSession/ServeAPI (401 when Auth set and token missing/invalid), using array validation (400 unknownCapability), maxCallsInRequest=16 (400 requestTooLarge), Method.Call now typed context.Context; 8 unit tests pass |
| 2026-05-26 | JMAP RFC 8620/8621 foundation: /.well-known/jmap session resource, POST /jmap/api dispatch, Email/get + Email/query stubs; 5 unit tests pass |
| 2026-05-26 | Refactor: split `internal/maildb/admin.go` (7579 lines) into 8 focused files — admin_users.go, admin_domains.go, admin_relay.go, admin_api_usage.go, admin_push.go, admin_outbox.go, admin_delivery.go, admin_suppression.go — admin.go reduced to 1498 lines (types/consts/validation); 549 tests pass |
| 2026-05-26 | Refactor: split `internal/imapgw/server.go` (9654 lines) into 13 focused files — server_conn.go, server_auth.go, server_capabilities.go, server_mailbox.go, server_list.go, server_idle.go, server_uid.go, server_search.go, server_fetch.go, server_store.go, server_copy_append.go, server_parse.go, server_dispatch.go — server.go reduced to 802 lines; 439 tests pass |
| 2026-05-26 | K8s deployment: 8 manifests (namespace, configmap, secret template, deployment, service, HPA, PDB, ingress) + README in `k8s/` |
| 2026-05-26 | DM room key rotation: `POST /api/v1/dm/rooms/{roomID}/rotate-key` — generates new AES-256-GCM key, atomically re-encrypts all message bodies and attachment paths |
| 2026-05-26 | Frontend cleanup: removed 63 console.log/error/warn calls from 29 console admin page files |
| 2026-05-26 | Global HTTP body limit already enforced via `MaxRequestBodyMiddleware(4MB)` applied to all routes |
| 2026-05-26 | Outbox relay horizontal scaling: WorkerCount (N goroutines per process, SKIP LOCKED prevents double-claim); ShardedPostgresStore (hashtext(partition_key) % N shard filter for partition-ordered multi-process scaling); config GOGOMAIL_OUTBOX_RELAY_{WORKER_COUNT,SHARD_TOTAL,SHARD_INDEX}; 24 tests pass |
| 2026-05-26 | Webmail unit tests: 3 new node --experimental-strip-types test scripts (check-compose-utils, check-send-result-label, check-stableid-timezone); pnpm test now runs all 6 scripts; composeUtils.ts import changed to `import type` |
| 2026-05-26 | OpenTelemetry tracing: `internal/observability/tracing.go` — TracerProvider, InitTracing (OTLP HTTP exporter), OTelHTTPMiddleware, StartSpan; config via GOGOMAIL_OTEL_{ENABLED,ENDPOINT,SERVICE_NAME,SERVICE_VERSION}; HTTP middleware wired into runHTTP + runOutboxRelay; 9 tests pass |
| 2026-05-26 | CI integration test stage: `docker/docker-compose.ci.yml` (Postgres 16, Redis 7, MinIO with tmpfs), `go-integration-test` job in CI runs full `go test ./...` against live services with `GOGOMAIL_TEST_*` env vars |
| 2026-05-26 | Delivery rate limit on by default (60/min), Redis-backed cross-process limiter (`RedisDomainRateLimiter`); backend selectable via `GOGOMAIL_DELIVERY_RATE_LIMIT_BACKEND` |
| 2026-05-26 | SMTP rate limiting per recipient domain: `InMemoryDomainRateLimiter` (fixed-window, per-minute), handler integration via `WithRateLimiter`, config via `GOGOMAIL_DELIVERY_RATE_LIMIT_*` env vars |
| 2026-05-26 | DM search scalability: paginated full-history scan (removed 1000-msg hard cap); search now iterates all room history in 200-msg pages until results found or history exhausted |
| 2026-05-26 | DM room export: TXT download from room header ⋯ menu (any participant; includes deleted/system messages); User MCP `gogomail_dm_export_room` tool (124 total) |
| 2026-05-26 | Doc optimization: deleted 12 completed/stale docs, trimmed backend-roadmap.md 7057→110 lines, removed 18 stale worktrees (1.4GB), freed 174MB from .git via gc |
| 2026-05-26 | JMAP session URL fix: APIUrl/DownloadUrl/UploadUrl/EventSourceUrl now point to /jmap/* (was /.well-known/jmap/* — 404 on all client operations); README.md and README.ko.md updated with JMAP client section |
| 2026-05-26 | JMAP integration: nil-safe DraftSender guard in jmapHandler (svc==nil → Sender=nil → graceful serverFail instead of panic) |
| 2026-05-26 | JMAP Task 13: Integration complete — DraftSender adapter wired (mailservice.Service→jmap.DraftSender), submission/vacationresponse capabilities in session resource; all 20 JMAP methods live |
| 2026-05-26 | JMAP hardening: replaced panic in mustRawString with safe strconv.AppendQuote fallback (rawString function); prevents HTTP handler goroutine death on encoding edge case; 62 internal/jmap tests pass |
| 2026-05-26 | VitePress AI Automation MCP guide (12 pages × 4 locales), expanded User MCP READMEs 147→1024 lines |
| 2026-05-26 | Codebase improvements (10 tasks): doc cleanup, security hardening, TypeScript domain splits (MCP tools, webmail API, UI components), Go package refactoring (httpapi/admin.go 8901→420 lines + 12 files, app/admin_service.go 1759→93 lines + 5 files) |
| 2026-05-25 | DM search candidate limit 10000→1000; system messages injectable |
| 2026-05-25 | Grafana default password removed; metrics interface{} replaced with typed interfaces |
| 2026-05-24 | User MCP DM tools (18 tools): rooms, messages, attachments, reactions, search |
| 2026-05-23 | Web push notifications + push device management |
| 2026-05-23 | Monitoring stack: Prometheus, Loki, Promtail, Grafana with provisioned dashboards |
| 2026-05-27 | Refactor: SettingsModalContent.tsx (568 lines) split into 9 sub-components under settings-modal/: MailboxSection, ComposeSection, ThemeSection, NotificationsSection, AccountSection, SecuritySection, ShortcutsSection, AdvancedSection, FiltersSection + shared sharedStyles.ts; root file reduced to 68 lines |
| 2026-05-27 | Refactor: SettingsView account/inbox/reading/compose/appearance/accessibility/contacts/drive/shortcuts extracted to separate components (1465 → 1098 lines) |
| 2026-05-22 | Admin console MFA enforcement |
| 2026-05-21 | Multiple query sargability improvements, LDAP batching, i18n cleanup |
| 2026-05-14 | DM instant messaging: rooms, encrypted messages, attachments, reactions, invites, Drive messages |
| 2026-05-10 | Web Push service worker, calendar edit/delete, password reset UI, server-synced signatures |
| 2026-05-07 | User MCP server: 123 tools across mail, DM, contacts, Drive, calendar, preferences |
| 2026-05-05 | Manage MCP server: 50 admin tools for Admin API, queue/health, org/security/spam policies |

## Architecture

See `docs/ARCHITECTURE.md` for the architecture overview.
See `docs/backend-roadmap.md` for the full feature roadmap.
See `docs/openapi.yaml` for the REST API spec.
See `PROJECT_HARNESS.md` for development workflow.

## Post-remediation hardening (2026-05-27)

**Three residual defects found in security branch and fixed:**
- **internal/httpapi/admin_rate_limiter_redis.go**: Redis rate limiter now fail-open on Redis error (warn log + allow) instead of fail-closed (429 for all logins)
- **internal/config/config.go**: APNS key-file loading now clears env-var value before file read — missing/unreadable file can no longer silently fall back to inline env var
- **internal/maildb/user_auth.go**: Legacy password hash upgrade now runs in a goroutine (`context.WithoutCancel`) — no longer blocks login response with 210k PBKDF2 iterations

## Post-remediation hardening round 4 (2026-05-28)

**Two code-quality / security refactors**
- **internal/auth/jwt.go**: Removed redundant manual `alg == "HS256"` check before `ParseWithClaims`. golang-jwt's key function already rejects non-HMAC algorithms; the duplicate check added noise without additional security. The `typ` check (golang-jwt doesn't verify `typ`) and `iat`-future check (golang-jwt v5 doesn't enforce iat > now) are retained.
- **44 files across maildb, drive, admin, apikeys, orgchart, idprovider, alert**: Replaced 94 occurrences of `err == sql.ErrNoRows` with `errors.Is(err, sql.ErrNoRows)`. Idiomatic Go 1.13+ style; future-proofs against driver changes that might wrap the error.

## Post-remediation hardening round 5 (2026-05-28)

**Password TrimSpace consistency fix**
- **internal/httpapi/mail_profile.go**: Added `strings.TrimSpace` for both `current_password` and `new_password` in the change-password handler. The login endpoint (`mail_auth.go`) and reset-confirm endpoint (`password_reset.go`) already trimmed; the change-password path did not, causing a user who set a password with leading/trailing whitespace to be unable to log in (hash of `"pass "` ≠ hash of `"pass"` after login trims).
