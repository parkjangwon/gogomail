# ADR 0001: Domain is the primary tenant boundary

Date: 2026-05-04

## Status

Accepted

## Context

gogomail is a multi-tenant webmail and mail-server platform. The design includes
platform, company, domain, organization, user, mailbox, and message concepts.

Company is important for contract, billing, global policies, and grouped
administration. However, actual mail routing, mailbox ownership, message
isolation, DKIM/DNS configuration, SMTP recipient validation, and most operator
actions are domain-centered.

## Decision

Treat `domain_id` as the primary tenant isolation key for mail data and runtime
mail operations.

Company remains the higher-level commercial/administrative container. A company
can own multiple domains. Domain remains the practical operational unit for:

- SMTP recipient routing
- user address ownership
- mailbox and message isolation
- DKIM/DNS setup
- quota and policy application
- domain-admin scope
- future row-level security

## Consequences

- Message records must carry `domain_id`/tenant identity.
- APIs should avoid crossing domain boundaries unless explicitly scoped for
  company/platform administrators.
- User-facing delivery status must filter by authenticated user and domain.
- Admin APIs should be clear about whether they operate at domain, company, or
  platform scope.
- Future frontend and generated clients should preserve this conceptual model
  instead of flattening all administration into a single global namespace.
