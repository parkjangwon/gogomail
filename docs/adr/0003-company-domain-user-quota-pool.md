# ADR 0003: Company-domain-user quota pool

Date: 2026-05-04

## Status

Accepted

## Context

gogomail targets SaaS-like enterprise/public-sector deployments. Storage is a
billable resource and must be modeled as a product-level quota pool, not as
isolated per-feature limits.

Future product modules such as Drive, large attachments, archive storage, and
other user-owned storage features should share the same capacity model.

## Decision

Storage quota is hierarchical:

1. Company owns the contracted total storage pool.
2. Domains receive allocations within the company pool.
3. Users receive allocations within the domain pool.

User quota is a unified personal storage allowance. A user may spend that
allowance freely across mailbox, attachments, future Drive, and other
user-owned storage features. Mailbox and Drive are not separate hard buckets by
default.

Domain policy may define a default user/mailbox storage allowance. When that
default changes:

- New users receive the new default.
- Existing users that still follow the domain default should inherit the new
  value.
- Users with explicit custom quota overrides keep their custom value.

The intended persisted model is:

- `companies.quota_limit`, `companies.quota_used`
- `domains.quota_limit`, `domains.quota_used`
- `domains.settings.policy.default_user_quota`
- `users.quota_limit`, `users.quota_used`
- user quota source: `default` or `custom`

Quota checks for storage-increasing operations must evaluate:

1. User quota
2. Domain quota
3. Company quota

Any exceeded level rejects the write. SMTP receive/submission should map mailbox
quota failures to RFC-correct temporary/permanent enhanced status semantics as
the relevant boundary requires.

## Consequences

- Storage enforcement must move toward a shared quota ledger that can be used by
  mail, attachment, and future Drive modules.
- Admin APIs should distinguish changing a domain default from overriding an
  individual user quota.
- Existing mailbox quota enforcement is a first slice and should be expanded to
  domain/company aggregate enforcement plus user quota source tracking.
- Product pricing can sell company-level storage and let administrators
  distribute it across domains and users.
