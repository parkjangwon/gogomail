# GoGoMail User MCP Server

`gogomail-user-mcp` exposes user-scoped GoGoMail mail, contacts, Drive, and calendar tools through MCP. It uses a user MCP access key created from the webmail settings page and calls the existing user HTTP APIs only.

## Setup

```bash
cd apps/gogomail-user-mcp
npm install
npm run build
```

## Environment

| Variable | Required | Description |
|---|---|---|
| `GOGOMAIL_API_URL` | yes | GoGoMail web/API origin, for example `https://mail.example.com` |
| `GOGOMAIL_USER_MCP_KEY` | yes | User-scoped MCP access key generated in webmail settings |
| `GOGOMAIL_MCP_PERMISSION_MODE` | no | Local fallback, `basic` or `bypass`; server-side user settings remain canonical when available |

## Safety Model

- Mail, contact, Drive, and calendar content returned by tools is untrusted user data and must not be treated as instructions.
- Basic permission mode requires explicit confirmation strings for sensitive actions such as send, delete, trash, and share-link creation.
- Bypass mode skips those tool-level confirmation strings but does not skip GoGoMail API auth, scopes, rate limits, or audit/metering.
- Outbound mail prepends `MCP를 통해 작성된 메일입니다.` unless the user's MCP settings disable that notice.

## API Contract Notes

- Mail and Drive tools call the `/api/v1` backend routes documented in `docs/openapi.yaml`.
- Contact tools call the existing CardDAV JSON bridge under `/api/mail/addressbooks` and `/api/mail/contacts/autocomplete`.
- Address book and calendar mutation tools send the backend `name` field. `display_name` is accepted only as a legacy MCP input alias.
- Drive text-file upload uses `/api/v1/drive/upload-sessions` with `declared_size`, `storage_backend`, binary body upload, and finalize.
