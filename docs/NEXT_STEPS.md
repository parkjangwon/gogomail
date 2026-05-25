# gogomail next steps

Last updated: 2026-05-26

## Current focus

All planned codebase improvements are complete as of 2026-05-26. The platform
is in post-improvement steady state; immediate focus is on product feature work.

## Backlog (priority order)

1. **OpenSearch integration** — replace in-memory mail search with OpenSearch FTS
2. **DM search scalability** — index encrypted tokens or move to separate unencrypted index
3. **SMTP rate limiting per recipient domain** — outbound throttle per MX
4. **Attachment virus scanning** — ClamAV or external scan hook
5. **Admin audit log retention** — configurable auto-expiry with policy
6. **Mobile app** — React Native or PWA wrapper
7. **Multi-region failover** — PostgreSQL replication + S3 cross-region
8. **AI email assistant** — integrate Claude for smart compose and categorization

## Out of scope (current sprint)

See `docs/backend-roadmap.md` for items intentionally deferred.
