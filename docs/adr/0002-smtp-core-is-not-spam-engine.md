# ADR 0002: SMTP core is not a spam engine

Date: 2026-05-04

## Status

Accepted

## Context

gogomail must provide a polished, powerful, standards-oriented SMTP engine.
Spam filtering, malware scanning, image conversion, push notification enqueue,
indexing, audit fan-out, and tenant-specific policy actions are all important
extension points, but they should not contaminate the SMTP protocol core.

The user explicitly requested that spam filtering be developed later as a
separate module or external engine integration.

## Decision

Keep SMTP core focused on:

- RFC-compliant SMTP protocol behavior
- envelope/session state
- extension advertisement and rejection semantics
- message size and recipient guardrails
- Received/header safety
- storage and queue boundaries
- hook/event emission
- metrics boundaries

Do not hard-code built-in spam scoring, pattern filtering, quarantine decisions,
or vendor-specific spam engine behavior into SMTP core.

Optional behavior should attach through explicit hooks, adapters, or workers.

## Consequences

- SPF/DKIM/DMARC results may be carried through authentication-result
  boundaries, but enforcement must stay policy-driven and optional.
- External spam relay adapters can exist, but must remain disabled unless
  configured.
- Future modules such as Rspamd, SpamAssassin, ClamAV, OCR/image conversion,
  FCM, indexing, and audit should attach to pipeline stages without rewriting
  SMTP receive/delivery logic.
- SMTP features that are implemented must follow the relevant email RFCs.
  Features that are not fully implemented must not be advertised.
