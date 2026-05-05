# ADR 0008: IMAP authentication and session semantics

Date: 2026-05-05

## Status

Accepted

## Context

gogomail now has IMAP mailbox DTOs, durable UID storage, event-broker
groundwork, an IMAP runtime scaffold, and a service-backed session-store
adapter. The remaining protocol listener work needs an authentication and
session model before any TCP server is advertised.

IMAP clients are long-lived, stateful protocol clients. They do not naturally
use the Mail API's JWT bearer flow, and mapping IMAP `LOGIN` onto HTTP session
state would couple the protocol gateway to web frontend concerns.

The project already stores local user `password_hash` values for SMTP
Submission authentication. Those hashes are created and rotated through Admin
API controls without exposing raw passwords in read models.

## Decision

The IMAP gateway will authenticate protocol sessions through a dedicated IMAP
auth adapter that reuses the local user credential boundary already used by
Submission-style authentication:

- IMAP `LOGIN` and `AUTHENTICATE PLAIN` map to a backend authenticator that
  validates local user credentials against stored `password_hash` values.
- Authenticated sessions resolve to `imapgw.Session` with stable `UserID`,
  username, domain, and display-name metadata.
- JWT bearer tokens are not accepted by the IMAP protocol listener. Webmail
  authentication remains an HTTP/Mail API concern.
- Production IMAP authentication must not allow cleartext credentials on a
  plaintext listener. The listener should support TLS policy before production
  enablement, either through implicit TLS, STARTTLS, or an explicit deployment
  decision documented in release notes.
- IMAP session state owns selected mailbox, read-only/read-write mode, and
  subscription state inside the protocol gateway. It must call
  `imapgw.Store`/`imapgw.MailboxSessionStore` instead of reaching directly into
  `maildb`.
- Persistent `SUBSCRIBE`/`UNSUBSCRIBE` state is stored as user-scoped mailbox
  names rather than folder foreign keys, so `LSUB` can retain subscribed names
  even when a mailbox no longer exists as required by RFC 3501.
- IMAP `\Deleted` is a protocol flag stored separately from gogomail's
  soft-delete message status. `EXPUNGE` may delete only messages that have
  this IMAP-specific flag in the selected mailbox, and must remove stale
  mailbox UID rows in the same transaction.
- IMAP `MOVE` is modeled as source mailbox expunge semantics plus a destination
  folder transition: the source mailbox emits sequence-number `EXPUNGE`
  responses, the message row moves folders transactionally, and source mailbox
  UID rows are removed so the destination mailbox assigns fresh local UIDs.

## Consequences

- The IMAP listener can be implemented without coupling to Mail API JWT
  middleware or browser session assumptions.
- Operators can provision IMAP-capable local users through existing Admin
  password-hash controls.
- The protocol gateway remains replaceable: future LDAP/OIDC/app-password
  adapters can implement the same authenticator boundary without changing
  mailbox storage contracts.
- TLS policy must be reviewed and wired before production IMAP enablement.
- A first TCP listener can safely start with authenticated LIST, SELECT, UID
  FETCH, UID STORE, COPY, MOVE, mailbox CRUD, and IDLE-oriented subscription
  support. UIDPLUS is advertised for implemented UID EXPUNGE and COPYUID
  response metadata, and the APPEND boundary now carries the UIDVALIDITY/UID
  metadata needed for APPENDUID once repository-backed APPEND storage is
  accepted.
