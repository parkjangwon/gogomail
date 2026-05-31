# ACTIVE_TASK

## ID: COMPLETE

No active task. All roadmap phases, scheduled hardening tasks, and the
2026-05-31 backend observability pass are complete.

### Completed 2026-05-31

- **Protocol/RFC hardening** — protocol gateways now expose more consistent
  metrics/logging; POP3 enforces the 512-octet command-line limit; LDAP rejects
  malformed or oversized BER PDUs; SMTP streaming/spool readers reject
  overlong lines with regression coverage
- **Cleanup/rollback observability** — attachment, Drive, SMTP, IMAP APPEND,
  outbound-send, DSN enqueue, API usage export, and storage readiness cleanup
  failures now warn or persist retryable failure records instead of silently
  dropping orphan/cost signals
- **SCIM sync tracking** — soft-delete/deactivate/active PATCH paths now warn
  when external IdP `UpdateUserStatus` synchronization fails
- **Remote signer hardening** — `cmd/remote-signer` now has structured JSON
  logs, config validation, graceful SIGINT/SIGTERM shutdown, request timeouts,
  max-header limits, and lifecycle tests
- **Frontend promise policy** — webmail fire-and-forget promises now use
  `ignoreNonCritical()` with contextual warning logs; webmail/console server
  proxy fallback helpers are explicit and tested

### Completed 2026-05-29

- **IMAP graceful shutdown WaitGroup** — `imapgw.Server` now tracks in-flight connection goroutines with `sync.WaitGroup`; `Close()` waits for all active sessions to drain before returning
- **createSystemFolders batch INSERT** — 5 individual INSERTs replaced with a single `VALUES (…),(…),…` batch INSERT ON CONFLICT DO NOTHING; reduces 5 round-trips to 1 per user creation
- **DMARC quarantine → Spam folder routing** — `enforceDMARCPolicy` now returns `(quarantine bool, err error)`; receiver routes DMARC-quarantined messages to `system_type=spam` folder instead of Inbox
- **webmail frontend refactor** — standardized loading state names (`loading`→`isLoading`, `foldersLoading`→`isFoldersLoading`, `messagesLoading`→`isMessagesLoading`); replaced `as unknown[]` and `Record<string, any>` type casts; later hardening converted intentional fire-and-forget paths to `ignoreNonCritical()` with warning logs
- **console alerts page decomposition** — `alerts/page.tsx` (660 lines) split into `RuleModal.tsx` and `ChannelModal.tsx` as pure presentational components; page reduced to ~340 lines; `as unknown as` enum casts simplified
- **SpamFilterPolicyEditor decomposition** — 653-line monolith split into `SpamFilterRiskSection`, `SpamFilterDetectionSection`, `SpamFilterRblSection`, `SpamFilterPacksSection`; editor reduced to 373 lines; hook audit confirmed all 30 console hooks follow React Query pattern

See `docs/backend-roadmap.md` for the full backlog and deferred items.
See `docs/NEXT_STEPS.md` for current focus and priority order.
