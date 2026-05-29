# ACTIVE_TASK

## ID: COMPLETE

No active task. All roadmap phases and scheduled hardening tasks are complete as of 2026-05-29.

### Completed 2026-05-29

- **IMAP graceful shutdown WaitGroup** — `imapgw.Server` now tracks in-flight connection goroutines with `sync.WaitGroup`; `Close()` waits for all active sessions to drain before returning
- **createSystemFolders batch INSERT** — 5 individual INSERTs replaced with a single `VALUES (…),(…),…` batch INSERT ON CONFLICT DO NOTHING; reduces 5 round-trips to 1 per user creation
- **DMARC quarantine → Spam folder routing** — `enforceDMARCPolicy` now returns `(quarantine bool, err error)`; receiver routes DMARC-quarantined messages to `system_type=spam` folder instead of Inbox
- **SpamFilterPolicyEditor decomposition** — 653-line monolith split into `SpamFilterRiskSection`, `SpamFilterDetectionSection`, `SpamFilterRblSection`, `SpamFilterPacksSection`; editor reduced to 373 lines with no behaviour change

See `docs/backend-roadmap.md` for the full backlog and deferred items.
See `docs/NEXT_STEPS.md` for current focus and priority order.
