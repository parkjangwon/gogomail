# ADR 0005: IMAP gateway boundary

Date: 2026-05-04

## Status

Accepted

## Context

gogomail needs IMAP compatibility for users who rely on standard mail clients.
The current mailbox, folder, and flag models are close to IMAP-friendly, but
RFC 3501 clients require stable mailbox state such as UID, UIDVALIDITY, UIDNEXT,
flag semantics, and change notifications.

Building a TCP IMAP server before those backend contracts are explicit would
risk coupling protocol behavior directly to HTTP-oriented repository shapes.

## Decision

IMAP is modeled as a separate gateway boundary over gogomail-native mailbox and
message interfaces. The first package, `internal/imapgw`, defines DTOs,
interfaces, mailbox helpers, and RFC-shaped flag mapping. It intentionally does
not include a TCP listener, command parser, `go-imap` adapter, or database
implementation yet.

The gateway boundary treats these as first-class future storage contracts:

- durable per-mailbox UID state
- UIDVALIDITY and UIDNEXT
- system flag mapping for `\Seen`, `\Flagged`, `\Answered`, and `\Draft`
- explicit deferral of `\Deleted`/EXPUNGE until gogomail has an IMAP-safe delete
  model separate from the current soft-delete status
- mailbox change subscriptions for future IDLE support

## Consequences

- Future IMAP protocol code can adapt to `internal/imapgw` instead of reaching
  into Mail API or SMTP internals.
- Current HTTP mailbox behavior remains unchanged.
- Storage migrations for UID/MODSEQ/change tracking should be designed before
  enabling a real IMAP listener.
- `\Deleted` must not be mapped onto the existing soft-delete status without a
  deliberate EXPUNGE design.
