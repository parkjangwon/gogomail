# ADR 0006: IMAP UID storage

Date: 2026-05-04

## Status

Accepted

## Context

IMAP clients depend on stable per-mailbox identity and message numbering:
`UIDVALIDITY`, `UIDNEXT`, message `UID`, and change `MODSEQ`. These values must
survive process restarts and must not be derived from mutable list ordering,
timestamps, or HTTP pagination cursors.

The project already has an `internal/imapgw` boundary, but no durable storage
for these IMAP-specific values.

## Decision

Store IMAP state in tables separate from the existing `folders` and `messages`
rows:

- `imap_mailbox_state` stores one row per folder with `uidvalidity`, `uidnext`,
  and `highest_modseq`.
- `imap_message_uid` stores one row per message with the mailbox-local `uid` and
  message `modseq`.

The `maildb` repository exposes small methods for ensuring mailbox state and
assigning a message UID. Message UID assignment locks the mailbox state row,
uses current `uidnext`, increments `uidnext`, and advances `highest_modseq` in
one transaction.

## Consequences

- Existing Mail API and SMTP persistence remain unchanged.
- IMAP UID behavior can be implemented without overloading HTTP pagination,
  message timestamps, or message UUIDs.
- Future IMAP adapters can map `maildb` rows into `internal/imapgw` DTOs through
  an explicit persistence boundary.
- `MODSEQ` storage exists, but full CONDSTORE/QRESYNC semantics remain future
  protocol work.
