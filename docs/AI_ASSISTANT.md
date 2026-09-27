# AI Email Assistant

GoGoMail ships a provider-pluggable, privacy-preserving AI email assistant with
three features:

1. **Thread summarization** — structured summaries (participants, key points,
   action items, suggested reply draft).
2. **Auto-categorization** — rule-based classification of incoming mail into
   user-defined categories, applied at delivery time and stored as message
   labels + extracted features.
3. **Smart compose assist** — subject suggestions, tone/length variants, and
   pre-send checks (e.g. "mentions an attachment but none attached").

The assistant is **off by default** and every feature is gated behind both a
per-domain policy kill switch and a per-user opt-in.

---

## Privacy guarantee

**In the default configuration, no mail content leaves the server.**

- The default summarization provider (`local`) is a deterministic, in-process
  extractive summarizer. It performs **no network I/O** and requires **no
  external API key**.
- Auto-categorization and smart compose assist are **pure local heuristics** —
  they never call an external service.
- Audit log entries for assistant usage record **metadata only** (user, domain,
  action, provider, result). They never contain subject lines, bodies, or
  recipients.

An operator may later wire an external LLM provider through the `Summarizer`
interface. That is an explicit, opt-in operator decision; until then the local
provider is authoritative and the guarantee above holds.

---

## HTTP API

All endpoints are under the Mail API base (`/api/v1`) and are user-scoped via
the shared JWT / API-key binding helpers (`internal/httpapi/mail_helpers.go`).
Requests are rate-limited per user (30 req/min).

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/me/assistant/settings` | Read assistant settings (opt-in state, provider). |
| PUT | `/api/v1/me/assistant/settings` | Update assistant settings. |
| GET | `/api/v1/me/assistant/categorization-rules` | List categorization rules. |
| POST | `/api/v1/me/assistant/categorization-rules` | Create a rule. |
| PUT | `/api/v1/me/assistant/categorization-rules/{id}` | Update a rule. |
| DELETE | `/api/v1/me/assistant/categorization-rules/{id}` | Delete a rule. |
| POST | `/api/v1/threads/{id}/summary` | Summarize a thread. |
| POST | `/api/v1/compose/assist` | Smart compose assistance for a draft. |

Full request/response schemas are documented in
[`docs/openapi.yaml`](openapi.yaml).

### Policy & opt-in enforcement

`POST /threads/{id}/summary` and `POST /compose/assist` require, in order:

1. **Per-domain kill switch** — the MCP domain policy `enabled` flag
   (`runtime_config` key `mcp.policy`). When disabled the endpoints return
   `403 assistant is disabled for this domain`.
2. **Per-user opt-in** — `assistant_settings.enabled` must be true (default
   false). Otherwise `403 assistant is not enabled; opt in via settings`.
3. **Per-feature opt-in** — `summarization_enabled` / `compose_assist_enabled`.

`PUT /me/assistant/settings` refuses to set `enabled: true` when the domain kill
switch is off.

### Example: summarize a thread

```bash
curl -X POST https://mail.example.com/api/v1/threads/THREAD_ID/summary \
  -H "Authorization: Bearer $JWT"
```

```json
{
  "subject": "Q1 Planning",
  "participants": ["Alice Kim <alice@example.com>", "bob@example.com"],
  "key_points": ["Let's align on the Q1 roadmap.", "..."],
  "action_items": [{ "text": "send me the budget draft by Friday", "owner": "bob@example.com" }],
  "suggested_reply": "Hi Alice,\n\nThanks for your message...",
  "message_count": 2,
  "provider": "local",
  "truncated": false,
  "generated_offline": true
}
```

---

## Provider interface

Summarization is provider-pluggable via the `Summarizer` interface in
`internal/aiassistant/assistant.go`:

```go
type Summarizer interface {
    Summarize(ctx context.Context, thread Thread) (Summary, error)
    Name() string    // stable provider id, e.g. "local"
    Offline() bool   // true when all content stays on the server
}
```

- **Default:** `LocalSummarizer` (`summarizer_local.go`) — extractive,
  deterministic, `Offline() == true`.
- **Extension:** an operator may implement an external LLM-backed `Summarizer`
  and inject it via `AssistantRouteOptions.Summarizer`. Such a provider must
  honor the `ctx` deadline and should document whether it transmits content
  off-box (`Offline()` must return `false`).

Auto-categorization and compose assist are intentionally **not** provider-
pluggable in this foundation; they are pure local engines
(`categorizer.go`, `compose.go`).

---

## Auto-categorization

### Rules

Users define keyword rules per mailbox. Each rule has:

- `category` — the label to assign.
- `field` — one of `from`, `subject`, `body`, `to`, `any`.
- `match_type` — one of `contains`, `prefix`, `suffix`, `exact`.
- `keyword` — the term to match (case-insensitive).
- `priority` — higher-priority rules contribute more weight; ties break
  deterministically.
- `weight` — base score contribution (default 1).

The winning category is the one with the highest accumulated score;
`confidence` is `winner_score / total_score`.

### Apply point

Categorization is applied by the service method
`(*mailservice.Service).ApplyCategorization(ctx, userID, messageID, hasAttachment, listHeaders)`.
This is the designated integration point for the delivery pipeline (or a
post-delivery worker). It:

1. Loads the user's assistant settings and is a **no-op** when the assistant or
   categorization is disabled.
2. Loads the message and the user's rules.
3. Runs the `Categorizer` and persists the result to
   `assistant_message_labels` via `UpsertAssistantMessageLabel`.

It runs **off the hot SMTP receive path** by design, per the project's
architecture rules (the SMTP core must not embed product logic).

### Features, not just verdicts

`ApplyCategorization` always persists an extracted `MessageFeatures` blob
(`from_domain`, subject tokens, body token count, recipient count, bulk/list
signals, top body keywords, etc.) even when no rule matches. Persisting
features — not just the winning category — keeps the design open to a **trained
classifier** later without re-plumbing delivery.

---

## Smart compose assist

`POST /compose/assist` accepts a draft (`subject`, `body`, `recipients`,
`attachment_count`) and returns:

- `subject_suggestions` — reworded alternatives, or derived candidates when the
  subject is empty.
- `variants` — `concise`, `formal`, and `friendly` rewrites of the body.
- `checks` — pre-send warnings, including:
  - `missing_attachment` — body/subject mentions an attachment but
    `attachment_count == 0`.
  - `empty_subject`, `empty_body`, `no_recipients`, `many_recipients`.
- `word_count`.

All checks are deterministic local heuristics.

---

## MCP tools

The user MCP server (`apps/gogomail-user-mcp`) exposes the assistant through
these tools (see `src/tools/assistant.ts`):

- `gogomail_assistant_get_settings` / `gogomail_assistant_update_settings`
- `gogomail_assistant_summarize_thread`
- `gogomail_assistant_compose_assist`
- `gogomail_assistant_list_categorization_rules`
- `gogomail_assistant_create_categorization_rule`
- `gogomail_assistant_update_categorization_rule`
- `gogomail_assistant_delete_categorization_rule`

These call the HTTP endpoints above and inherit the same opt-in and kill-switch
enforcement.

---

## Data model

Migration `migrations/0159_ai_assistant.sql` adds:

- `assistant_settings` — per-user opt-in (`enabled` defaults to false),
  per-feature toggles, and provider selection.
- `assistant_categorization_rules` — user-managed rules.
- `assistant_message_labels` — applied category, confidence, matched rule IDs,
  and the extracted `features` JSON per message.

All tables are keyed on `user_id` with `ON DELETE CASCADE` for tenant-safe
cleanup.
