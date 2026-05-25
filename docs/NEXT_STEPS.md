# gogomail next steps

Last updated: 2026-05-26

## Current focus

All planned codebase improvements are complete as of 2026-05-26. The platform
is in post-improvement steady state; immediate focus is on product feature work.

## Backlog (priority order)

1. **DM search scalability** — paginated full-history scan (remove 1000-msg hard cap); HMAC token index as follow-up
2. **SMTP rate limiting per recipient domain** — outbound throttle per MX
3. **Attachment virus scanning** — ClamAV or external scan hook
4. **Admin audit log retention** — configurable auto-expiry with policy
5. **Mobile app** — React Native or PWA wrapper
6. **Multi-region failover** — PostgreSQL replication + S3 cross-region
7. **AI email assistant** — integrate Claude for smart compose and categorization

## Completed backlog items

- **OpenSearch integration** — already complete; mail search backend is configurable (`SEARCH_INDEX_BACKEND=opensearch`) with full OpenSearch FTS and PostgreSQL fallback (2026-05-26 verified)

## Out of scope (current sprint)

See `docs/backend-roadmap.md` for items intentionally deferred.
