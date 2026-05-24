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
- MCP-generated mail notice is enabled and prepends `MCP를 통해 작성된 메일입니다.`.
- Sensitive actions require explicit MCP tool confirmation strings in `basic` mode.

The webmail settings page exposes:

- MCP enablement.
- `basic` / `bypass` permission mode.
- Generated-mail notice toggle and text.
- Granular mail automation preferences.
- User MCP access key creation, listing, and revocation.

## Domain Policy

Admins can control domain policy with:

- `GET /admin/v1/domains/{id}/mcp-policy`
- `PUT /admin/v1/domains/{id}/mcp-policy`

The policy is stored in runtime config as `mcp.policy`.

Important fields:

- `enabled`: disables all user MCP keys for the domain when false.
- `allow_user_access_keys`: prevents user key issuance and key verification when false.
- `allow_bypass_mode`: prevents bypass-mode key issuance and key verification when false.
- `allowed_scopes`: constrains user key scopes.
- `force_generated_mail_notice`: reserved for enforcing the notice even if the user disables it.
- `audit_level`: controls future audit verbosity expectations.

## Tool Contract

The user MCP server only calls existing GoGoMail user APIs:

- Mail: `/api/v1/messages`, `/api/v1/drafts`, `/api/v1/search`.
- Contacts: `/api/mail/addressbooks`, `/api/mail/contacts/autocomplete`.
- Drive: `/api/v1/drive/nodes`, `/api/v1/drive/folders`, `/api/v1/drive/upload-sessions`.
- Calendar: `/api/v1/calendars`.

When a needed user workflow has no API, add the backend API first and document it in `docs/openapi.yaml` before exposing an MCP tool.
