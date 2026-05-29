# ACTIVE_TASK

## ID: COMPLETE

No active task. All roadmap phases and scheduled hardening tasks are complete as of 2026-05-29.

### Completed 2026-05-29

- **IMAP graceful shutdown WaitGroup** — `imapgw.Server` now tracks in-flight connection goroutines with `sync.WaitGroup`; `Close()` waits for all active sessions to drain before returning
- **createSystemFolders batch INSERT** — 5 individual INSERTs replaced with a single `VALUES (…),(…),…` batch INSERT ON CONFLICT DO NOTHING; reduces 5 round-trips to 1 per user creation
- **DMARC quarantine → Spam folder routing** — `enforceDMARCPolicy` now returns `(quarantine bool, err error)`; receiver routes DMARC-quarantined messages to `system_type=spam` folder instead of Inbox

### Console refactor 2026-05-29

- **Extract RuleModal/ChannelModal** — `alerts/page.tsx` (660 lines) split: two `<Modal>` blocks extracted into `RuleModal.tsx` and `ChannelModal.tsx` as pure presentational components; page reduced to ~340 lines with state/handlers only
- **Fix `as unknown as` type casts** — `alert_type` and `channel_type` enum casts simplified to direct casts (enums share identical string values); `payload as unknown as AlertRuleUpdateRequest` replaced with explicit field spread matching the `UpdateAlertRule` schema (which has no `alert_type` field)
- **Hook audit** — all hooks in `/hooks/` follow the standard React Query pattern; `useLocale` correctly uses `useState`/`useEffect` for browser-local state (not server state); `useReportCsvExport` intentionally uses raw `fetch` for binary blob download

See `docs/backend-roadmap.md` for the full backlog and deferred items.
See `docs/NEXT_STEPS.md` for current focus and priority order.
