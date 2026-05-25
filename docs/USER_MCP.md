# User MCP Automation

GoGoMail now separates the operational management MCP from the user-facing MCP.

- `apps/gogomail-manage-mcp` is for support and operations.
- `apps/gogomail-user-mcp` is for an individual webmail user's mail, contacts, Drive, and calendar automation.

The user MCP server authenticates with user-scoped `gmu_` access keys. Raw tokens are returned only once at creation time; the backend stores only a hash and token suffix.

## User Settings

Webmail stores MCP preferences under the existing user webmail preferences document at `mcp`.

Defaults:

- MCP access is disabled until the user enables it.
- Permission mode is `basic`.
- Domain MCP policy is fail-closed by default. An admin must enable MCP, user access keys, and allowed scopes before users can issue keys.
- Bypass mode is not allowed by default; a user and domain admin must opt in.
- MCP-generated mail notice is enabled and prepends `MCP를 통해 작성된 메일입니다.`.
- Sensitive actions require explicit MCP tool confirmation strings in `basic` mode.
- `mail.send_enabled`, external-recipient confirmation, attachment confirmation, and daily send limits are enforced server-side for user MCP keys.

The webmail settings page exposes:

- MCP enablement.
- `basic` / `bypass` permission mode.
- Generated-mail notice toggle and text.
- Granular mail automation preferences.
- User MCP access key creation, listing, and revocation with per-key scopes, optional CIDR allowlists, optional expiry, and per-key permission mode.

## Domain Policy

Admins can control domain policy with:

- `GET /admin/v1/domains/{id}/mcp-policy`
- `PUT /admin/v1/domains/{id}/mcp-policy`

The policy is stored in runtime config as `mcp.policy`.

Important fields:

- `enabled`: enables user MCP keys for the domain when true. Missing policy defaults to false.
- `allow_user_access_keys`: allows user key issuance and key verification when true. Missing policy defaults to false.
- `allow_bypass_mode`: prevents bypass-mode key issuance and key verification when false.
- `allowed_scopes`: constrains user key scopes. Missing policy defaults to an empty scope list.
- `force_generated_mail_notice`: forces the generated-mail notice in effective user MCP settings even if the user disables it.
- `audit_level`: controls future audit verbosity expectations.

In `basic` mode, sensitive user-key API calls must include `X-Gogomail-MCP-Confirm` with the same confirmation string required by the MCP tool. Bypass-mode user keys skip this backend confirmation gate. If the user enables external-recipient or attachment confirmation, send calls must also include `X-Gogomail-MCP-External-Confirm: external recipients` or `X-Gogomail-MCP-Attachment-Confirm: send attachments`.

## Tool Contract

The user MCP server only calls existing GoGoMail user APIs:

- Account/context: webmail capabilities, mailbox overview, profile, sender addresses, user MCP settings, and read-only webmail preferences.
- Mail: messages, drafts, folders, threads, attachments, delivery status, bulk routes, and search.
- DM: participant-only rooms, public room discovery, group membership/owner/invites, messages, attachments, read marks, search/media read models, and reactions.
- Contacts and directory: address books, contacts, autocomplete, directory users, and organization tree.
- Drive: nodes, folders, downloads, upload sessions, usage, and share links.
- Calendar: calendars, calendar objects, calendar subscriptions, and subscription events.

For agent-native coverage, `gogomail_api_request` can call the documented user API surface through an exact method/path manifest. It blocks admin, auth, password, session, and MCP key-management routes and denies undocumented future routes until the manifest is intentionally updated. Full-object preference writes stay behind this generic bridge so agents must intentionally preserve unknown preference fields instead of clobbering them through a broad typed setter.

Convenience tools are layered on top of those real contracts:

- Bulk mail actions accept capped unique message/thread ID lists and use the documented PATCH/POST bulk routes.
- `gogomail_contacts_upsert_simple` builds a vCard from structured fields; raw vCard upsert remains available.
- `gogomail_calendar_upsert_event_simple` builds a single VEVENT ICS object; raw ICS upsert remains available.
- Drive downloads return text plus base64 and may save to a local path only when basic mode receives `confirm="save download <path>"`.

Drive text-file creation uses the real upload-session API. If `storage_backend` is omitted, the backend selects the configured storage backend; clients should only pass an explicit backend label when they know it is enabled for that environment. Permanent Drive delete applies to already-trashed nodes, so agents should call `gogomail_drive_trash` before `gogomail_drive_delete` for active files.

When a needed user workflow has no API, add the backend API first and document it in `docs/openapi.yaml` before exposing an MCP tool.
