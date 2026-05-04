# gogomail agent instructions

## AGENTS.md and CLAUDE.md must stay synchronized

`AGENTS.md` and `CLAUDE.md` are equivalent agent handoff instruction files for
different coding assistants. Whenever either file is changed, update the other
file in the same commit so both files carry the same guidance, philosophy,
roadmap guardrails, and operational rules.

## Mail standards are mandatory

gogomail is a mail server project. Correct RFC/international-standard compliance is a core requirement, not an optional enhancement.

When implementing SMTP, message parsing, storage, headers, MIME, authentication, DNS, delivery, bounce handling, or IMAP-related behavior, prefer standards-compliant behavior over shortcuts.

Important standards to keep in mind include, but are not limited to:

- RFC 5321: SMTP
- RFC 5322: Internet Message Format, successor to RFC 822
- RFC 2045/2046/2047/2049: MIME
- RFC 6531/6532: SMTPUTF8 and internationalized email headers
- RFC 6376: DKIM
- RFC 7208: SPF
- RFC 7489: DMARC
- RFC 3461/3464: DSN and delivery status notifications
- RFC 3501 and successors: IMAP, when IMAP is implemented

Use well-maintained libraries for protocol parsing/serialization where possible instead of ad-hoc string parsing.

Tests for mail behavior should include protocol edge cases and RFC-shaped examples whenever practical.

## EML parser performance matters

The shared EML parser is on the hot path for SMTP receive, Mail API, future IMAP, future POP3, search indexing, and attachment handling.

Parser work must be designed for high throughput and low memory use:

- Prefer streaming readers over loading whole messages into memory.
- Avoid unnecessary body copies.
- Enforce maximum message/part/header sizes.
- Keep attachment handling streaming-first.
- Benchmark parser changes when behavior or allocation patterns change.
- Use `go test -bench` and allocation checks for parser-sensitive changes.
- Treat RFC correctness and performance as joint requirements; do not trade away standards compliance for speed unless explicitly documented and approved.

## SMTP pipeline extensibility

The SMTP engine must keep mail processing stages clearly separated.

Spam processing will be built as a separate module later, so the SMTP engine should expose stable internal stages/hooks rather than hard-code spam-specific behavior.

Do not turn the SMTP engine into a spam engine. SPF/DKIM/DMARC, spam scoring, pattern filtering, quarantine decisions, and vendor-specific spam behavior must stay optional and pluggable. The SMTP core may define interfaces, hook stages, envelopes, trace headers, and result carriers, but actual spam filtering modules or external spam relay integrations should live outside the protocol core and remain disabled by default unless explicitly wired by configuration.

Design SMTP receive/delivery changes so extra behavior can be attached at specific stages, such as:

- image conversion
- FCM/push notification enqueue
- audit logging
- spam relay handoff
- attachment scanning
- indexing enqueue
- custom tenant policy

Prefer explicit pipeline events over hidden side effects.

## Architecture style

gogomail should feel polished, high-performance, and customization-friendly.

When designing or changing architecture:

- Prefer small composable interfaces over monolithic components.
- Keep hot paths efficient and allocation-aware.
- Keep protocol engines decoupled from optional product features.
- Make behavior replaceable through interfaces, hooks, or adapters.
- Avoid hard-coding vendor-specific services into core SMTP/message logic.
- Keep defaults simple, but allow advanced deployments to swap storage, queues, spam engines, auth providers, rate limiters, and notification handlers.
- Favor clear boundaries that let future modules plug in without invasive rewrites.

The intended style is: elegant core, high-throughput internals, flexible extension points.

## Product feel

gogomail should feel like a polished, developer-friendly, modern mail server from the code outward.

- Keep APIs intuitive and pleasant to use.
- Prefer clean names, crisp boundaries, and readable flow over clever obscurity.
- Make extension points feel intentional, not bolted on.
- Leave the codebase feeling stylish, composed, and hackable.

## Scale ambition

gogomail is not a toy mail server. It should be designed to grow from a clean single-node deployment into a powerful national-scale platform.

- Assume large public-sector and enterprise-scale workloads are an explicit long-term target.
- Keep hot paths streaming, allocation-aware, and horizontally scalable.
- Prefer backpressure, queues, retries, idempotency, and observability over best-effort shortcuts.
- Design components so a small deployment stays simple while a large deployment can split, shard, replicate, and scale independently.
- Treat operational resilience, clear failure modes, and graceful degradation as core product features.

## Scheduled/autonomous work continuity

When resuming work from a scheduled or autonomous run, first rebuild context before editing:

- Work only in `/Users/pjw/dev/project/gogomail` unless the user explicitly changes the project path.
- Read this `AGENTS.md`/`CLAUDE.md` guidance and keep the product feel, scale ambition, RFC correctness, parser performance, and extension-point philosophy in mind.
- Review `docs/backend-roadmap.md` for current roadmap state.
- Inspect recent git history with `git log --oneline` to understand what was just changed.
- Check `git status --short` before editing.
- Prefer improvements that move gogomail toward a releasable, powerful mail server rather than low-value churn.
- Commit each autonomous improvement as a meaningful, reviewable unit.
- After successful verification, push completed feature commits to `origin/main` unless the user explicitly says not to push.

## Context continuity protocol

The repository must carry enough project memory for future coding agents to keep
the same philosophy, roadmap, and direction even when the active agent changes.

Before starting meaningful work, read:

- `AGENTS.md`
- `CLAUDE.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/backend-roadmap.md`
- `docs/backend-api-contracts.md`
- `docs/backend-release-readiness.md`
- relevant files under `docs/adr/`
- recent `git log --oneline`

After finishing meaningful work, update documentation when applicable:

- Update `docs/CURRENT_STATUS.md` when project phase, completed scope, or active direction changes.
- Update `docs/NEXT_STEPS.md` when priorities change.
- Update `docs/backend-roadmap.md` when a meaningful backend capability is completed.
- Update `docs/backend-api-contracts.md` and `docs/openapi.yaml` when HTTP API behavior changes.
- Add or update an ADR under `docs/adr/` when an architectural direction or boundary changes.
- Use `docs/CHANGE_CHECKLIST.md` before the final commit/push.

ADR-worthy changes include tenant/domain boundaries, SMTP core boundaries,
queue/storage/auth/policy architecture, delivery routing, frontend start gate,
spam integration strategy, API contract versioning, and release/deployment
architecture.

## Frontend start gate

Frontend implementation is planned, not forbidden. Continue backend contracts, API readiness, and frontend planning autonomously. However, before creating or substantially implementing actual frontend apps (`apps/shell`, `apps/webmail`, `apps/admin`, shared UI packages, or real Next.js screens), explicitly tell the user that frontend work is about to begin and wait for the user's frontend-specific guidance.
