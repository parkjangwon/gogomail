# gogomail next steps

Last updated: 2026-05-26

## Current focus

All planned codebase improvements are complete as of 2026-05-26. The platform
is in post-improvement steady state; immediate focus is on product feature work.

## Backlog (priority order)

1. **Attachment virus scanning** — ClamAV or external scan hook
2. **Admin audit log retention** — configurable auto-expiry with policy
3. **Mobile app** — React Native or PWA wrapper
4. **Multi-region failover** — PostgreSQL replication + S3 cross-region
5. **AI email assistant** — integrate Claude for smart compose and categorization
6. **DM message pinning** — pin important messages in a room for quick access
7. **imapgw/server.go split** — 9,654-line file → 13 focused files (in progress via agent)
8. **maildb/admin.go split** — 7,579-line file → focused domain files (in progress via agent)

## Completed backlog items

- **SMTP rate limiting per recipient domain** — `InMemoryDomainRateLimiter` (fixed-window, per-minute), config via `GOGOMAIL_DELIVERY_RATE_LIMIT_*` (2026-05-26)
- **DM search scalability** — paginated full-history scan (removed 1000-msg hard cap); iterates all room history in 200-msg pages (2026-05-26)
- **DM key rotation** — `POST /api/v1/dm/rooms/{roomID}/rotate-key`, atomic re-encryption (2026-05-26)
- **K8s deployment** — 8 manifests in `k8s/` (namespace, configmap, secret, deployment, service, HPA, PDB, ingress) (2026-05-26)
- **Frontend console.log cleanup** — 63 console.* calls removed from 29 admin pages (2026-05-26)
- **OpenSearch integration** — already complete; mail search backend is configurable (`SEARCH_INDEX_BACKEND=opensearch`) with full OpenSearch FTS and PostgreSQL fallback (2026-05-26 verified)

## Out of scope (current sprint)

See `docs/backend-roadmap.md` for items intentionally deferred.
