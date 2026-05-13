# gogomail current status

Last updated: 2026-05-14 (LDAP gateway StartTLS/LDAPS and discovery hardening)

## LDAP gateway StartTLS/LDAPS and discovery hardening (2026-05-14, complete)
- `internal/ldapgw` now handles RFC 4511 StartTLS extended requests (`1.3.6.1.4.1.1466.20037`) when a TLS certificate/key is configured.
- Root DSE base-object search now returns `namingContexts`, `supportedLDAPVersion`, `vendorName`, and `supportedExtension` for StartTLS discovery.
- `SearchRequest` parsing now consumes scope, deref aliases, size limit, time limit, typesOnly, filter, and requested attribute selection instead of treating filter bytes as the remainder of the request.
- Root DSE now advertises `subschemaSubentry`, and base-object search for `cn=Subschema` returns minimal RFC 4512/RFC 4519 schema metadata for person/inetOrgPerson-style clients.
- LDAP search filters now accept common client OR/AND/NOT wrappers and substring filters such as `(|(cn=*alice*)(mail=*alice*)(uid=*alice*))`, extracting supported RFC 4519 directory attributes while ignoring non-directory predicates such as `objectClass`.
- Search responses now wrap attributes in a proper RFC 4511 `PartialAttributeList`, honor requested attributes, support types-only responses, and return size-limit status when a client-requested size limit is reached.
- LDAP directory entries now expose principal-kind-specific schema and subtrees: users as `inetOrgPerson` under `ou=users`, organizations as `organizationalUnit` under `ou=organizations`, groups as `groupOfNames` under `ou=groups`, and resources as `device` under `ou=resources`.
- LDAP `objectClass` filters and base DNs now narrow repository searches to the matching principal kinds, so organization-chart LDAP queries such as `objectClass=organizationalUnit` do not leak user entries.
- LDAP search results now honor base-object, one-level, and subtree scope against generated entry DNs for client directory browsing.
- Runtime config now supports `GOGOMAIL_LDAP_TLS_CERT_FILE`, `GOGOMAIL_LDAP_TLS_KEY_FILE`, optional `GOGOMAIL_LDAPS_ADDR`, and semicolon/newline-separated `GOGOMAIL_LDAP_REFERRAL_URLS` for cross-naming-context SearchResultReference responses.
- LDAP controls are parsed at the LDAPMessage boundary; supported critical controls (`ManageDsaIT`, Simple Paged Results) are accepted, unsupported critical controls return `unavailableCriticalExtension`, and Simple Paged Results uses opaque offset cookies across search pages.
- LDAP simple bind now accepts common client bind identities including raw username/email and DN-style identities such as `uid=alice,ou=users,...`, with DN value unescaping and repository authentication by user ID for generated LDAP entry DNs.
- Non-discovery directory searches now require a successful bind; Root DSE and `cn=Subschema` base-object discovery remain available before bind for standard client setup.
- `SearchResultEntry` encoding now emits the RFC 4511 application payload directly, matching OpenLDAP client decoding expectations.
- LDAP bind/search/extended/read-only outcomes now flow through a metrics boundary, with `GOGOMAIL_METRICS_BACKEND=slog` wiring for operational logs.
- Coverage verifies Root DSE and subschema discovery, StartTLS upgrade followed by LDAP search, DN-style simple bind, unauthenticated search rejection, organization OU subtree browsing, objectClass-to-principal-kind filtering, LDAP search scope filtering, OpenLDAP `ldapsearch` plaintext and StartTLS interoperability, referral responses, critical-control handling, Simple Paged Results paging cookies, LDAP metrics, LDAP TLS/referral config validation, YAML/env config loading, and existing bind/search/read-only protections.

## SMTP submission MAIL DSN parameter validation (2026-05-14, complete)
- SMTP submission `MAIL FROM` with DSN parameters (RFC 3461) now has comprehensive coverage.
- TestSubmissionMailClearsDSNOptions verifies DSN options are cleared between consecutive MAIL commands without RESET.
- DSN Return parameter variations tested: RET=HDRS, RET=FULL properly extracted and preserved.
- EnvelopeID parameter correctly tracked across transactions with proper isolation between MAIL commands.
- DSN recipient options (NOTIFY, ORCPT) properly cleared when MAIL FROM executes without explicit options.
- Multiple transaction isolation verified: DSN envelope and recipient options do not leak between transactions.
- All 5939+ tests passing with zero regressions.

## CardDAV ACL support (RFC 3744) (2026-05-14, complete)
- Implemented RFC 3744 Access Control List (ACL) support for CardDAV collections.
- ACL rules define principal-based access control for addressbooks.
- Support for granting/denying privileges on a per-principal basis.
- carddav_acl_rules table stores ACL rules with principal_id, grant_privileges, deny_privileges.
- ACL rules can be created, retrieved, and deleted via repository methods.
- Protected ACL rules flag indicates rules that cannot be modified.
- Indexes on addressbook_id and principal_id for efficient permission lookups.
- Migration 0101_carddav_acl_support.sql creates schema with foreign key to addressbooks.

## CardDAV CATEGORIES and GROUP properties (2026-05-14, complete)
- CardDAV now supports RFC 6350 CATEGORIES and GROUP properties for contact organization.
- CATEGORIES: Comma-separated list of categories extracted and stored as categories_list (TEXT[]) array in database.
- GROUP: Group identifier extracted and stored as group_name (VARCHAR(255)) for contact grouping.
- Both properties are removed from vCard during upsert and merged back during retrieval to avoid duplication.
- Migration 0100 adds schema with GIN index for category-based filtering and B-tree index for group-name lookups.
- All repository methods updated to retrieve and merge CATEGORIES/GROUP (GetContactObject, ListContactObjects, SearchContacts, etc.).

## CardDAV PHOTO property extraction and storage (2026-05-14, complete)
- CardDAV now supports RFC 6350 PHOTO property for contact photos.
- Photos are extracted from vCard PHOTO lines during upsert and stored separately in `photo_data` (BYTEA) and `photo_media_type` (VARCHAR) columns.
- Photos are merged back into vCard when retrieving contacts, maintaining RFC 6350 compliance.
- Supports base64-encoded photo data with configurable media types (image/jpeg, image/png, etc).
- Max photo size: 5MB per contact (independent of MaxContactObjectBytes 5MB limit).
- Migration 0099 adds photo storage schema with index on addressbook_id and photo presence.
- All repository methods (GetContactObject, ListContactObjects, SearchContacts, etc.) updated to retrieve and merge photos.

## CalDAV production-ready status (2026-05-14, complete)
- CalDAV now marked as production-ready with 176+ comprehensive test cases covering all major RFC features.
- Supports RFC 4791 (CalDAV core), RFC 5545 (iCalendar), RFC 6638 (iCalendar Scheduling Extensions), RFC 7809 (Timezone support), and RFC 3744 (ACL).
- Full calendar management: create, read, update, delete, move, copy calendars with proper HTTP status codes and error handling.
- Recurring events with RFC 5545 RRULE support: daily/weekly/monthly/yearly frequencies with intervals, custom days, and end conditions (count or date).
- iMIP scheduling support for calendar invitations and responses with proper delivery tracking.
- Proper timezone handling with VTIMEZONE components and daylight saving time adjustments.
- ACL support for principal-based access control on calendar collections (RFC 3744).
- Free-busy query support for checking calendar availability across events.
- Calendar sync with sync-token and change detection for efficient client synchronization.

## WebDAV gateway authentication and lock optimization (2026-05-14, complete)
- WebDAV gateway now requires authenticated Bearer token or HTTPS Basic auth for external client access.
- External clients (Mac Finder, Windows Explorer, Linux file managers) can now mount gogomail drive via `/dav/` endpoint.
- Lock management optimized: RWMutex for better read-heavy concurrency, automatic cleanup of expired 5-minute locks.
- Supports all RFC 4918 WebDAV operations: OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH, LOCK, UNLOCK.

## Previously updated: 2026-05-14 (SMTP submission RSET DSN reset)

## SMTP submission RSET DSN reset (2026-05-14, complete)
- SMTP submission coverage now verifies explicit `Reset` clears the active envelope before the next submitted transaction.
- The regression verifies pre-reset RFC 3461 DSN envelope and recipient options do not leak into the next submitted message.

## SMTP submission Logout domain policy reset (2026-05-14, complete)
- SMTP submission now clears authenticated-user domain policy cache on successful `AUTH` and `Logout`.
- The regression verifies a same-session Logout and re-authentication as a different domain user does not inherit the previous domain's message-size policy or DSN options.

## SMTP submission unsupported auth transaction state (2026-05-14, complete)
- SMTP submission coverage now verifies unsupported AUTH after `MAIL FROM` is rejected without clearing the active envelope sender.
- The regression verifies the transaction can continue with `RCPT` and `DATA`, producing the expected submitted message.

## SMTP submission repeated auth transaction state (2026-05-14, complete)
- SMTP submission coverage now verifies repeated AUTH after `MAIL FROM` is rejected without clearing the active envelope sender.
- The regression verifies the transaction can continue with `RCPT` and `DATA`, producing the expected submitted message.

## SMTP submission repeated auth side-effect isolation (2026-05-14, complete)
- SMTP submission coverage now verifies repeated AUTH after a successful authentication is rejected without replacing or clearing the existing user.
- The regression verifies repeated AUTH does not add hooks or metrics, and `MAIL FROM` remains available for the original authenticated user.

## SMTP submission unsupported auth mechanism (2026-05-14, complete)
- SMTP submission coverage now verifies unsupported mechanisms such as `LOGIN` return `ErrAuthUnsupported` with no SASL server.
- The regression verifies unsupported mechanisms do not authenticate the session, emit hooks, record metrics, or unlock `MAIL FROM`.

## SMTP submission malformed auth payload isolation (2026-05-14, complete)
- SMTP submission coverage now verifies malformed AUTH PLAIN payloads fail without authenticating the session.
- The regression verifies malformed payloads emit no authenticated hook events and `MAIL FROM` continues to require AUTH.

## SMTP submission invalid credential event isolation (2026-05-14, complete)
- SMTP submission coverage now verifies invalid AUTH credentials fail without emitting authenticated hook events.
- The regression also verifies invalid credentials still record a rejected `StageAuthenticated` metric.

## SMTP submission auth hook failure metrics (2026-05-14, complete)
- SMTP submission coverage now verifies auth hook failures emit a rejected `StageAuthenticated` metric.
- The regression also verifies the failed auth path does not emit an accepted auth metric.

## SMTP submission auth hook failure isolation (2026-05-14, complete)
- SMTP submission now stores the authenticated session user only after `StageAuthenticated` hooks succeed.
- Coverage verifies an auth hook failure leaves the session unauthenticated and `MAIL FROM` still returns `ErrAuthRequired`.

## SMTP submission must-change-password event isolation (2026-05-14, complete)
- SMTP submission coverage now verifies a `MustChangePassword` auth rejection does not emit any hook event, including `StageAuthenticated`.
- This keeps audit/logging extensions from observing a false successful authentication when the SMTP session remains unauthenticated.

## SMTP submission must-change-password policy (2026-05-14, complete)
- SMTP submission `AUTH PLAIN` now rejects authenticated users flagged `MustChangePassword` instead of marking the session authenticated.
- Coverage verifies the rejected session remains unauthenticated, records a rejected auth metric, and still requires AUTH before `MAIL FROM`.

## DAV auth must-change-password repository policy (2026-05-14, complete)
- Postgres coverage now verifies `AuthenticatePlain`, the credential path used by CalDAV/CardDAV Basic auth, returns `MustChangePassword=true` from the users table.
- The regression also verifies the authenticated user/domain IDs still match the principal so DAV resolvers can reject password-change-required users accurately.

## DAV auth repository active policy (2026-05-14, complete)
- Postgres coverage now verifies `AuthenticatePlain`, the credential path used by CalDAV/CardDAV Basic auth, rejects suspended users and domains after first proving the same credential works while active.
- Existing company rejection coverage is now complemented by user/domain policy coverage for DAV auth.

## CalDAV Basic auth parity (2026-05-14, complete)
- CalDAV auth coverage now verifies unauthorized Basic auth failures expose the `WWWAuthenticate()` challenge with `Basic realm="CalDAV"`.
- CalDAV now has parity coverage with CardDAV for trusted-proxy `X-Forwarded-Proto` normalization, including uppercase/whitespace `" HTTPS "`.

## CalDAV/CardDAV inactive principal create policy (2026-05-14, complete)
- CalDAV Postgres coverage now verifies `CreateCalendar` rejects disabled user, domain, and company state through the active principal join.
- CardDAV Postgres coverage now verifies `CreateAddressBook` rejects disabled user, domain, and company state through the same active principal policy.

## IMAP duplicate-casing subscription update (2026-05-14, complete)
- Postgres coverage now verifies repeated SUBSCRIBE for a retained missing mailbox with different casing updates the existing canonical subscription row instead of creating duplicates.
- The regression verifies the retained display name follows the latest subscription command while `ListSubscribedIMAPMailboxes` returns one entry.

## IMAP existing subscription case-insensitive unsubscribe (2026-05-14, complete)
- Postgres coverage now verifies an existing `INBOX` subscription can be removed with differently cased `inbox`.
- The regression verifies the existing mailbox LSUB entry is fully removed from `ListSubscribedIMAPMailboxes` afterward.

## IMAP retained subscription case-insensitive unsubscribe (2026-05-14, complete)
- Postgres coverage now verifies a missing mailbox retained subscription such as `Retired` can be removed with a differently cased name such as `retired`.
- The regression verifies the retained LSUB entry is fully removed from `ListSubscribedIMAPMailboxes` afterward.

## IMAP subscription name normalization (2026-05-14, complete)
- `SubscribeIMAPMailbox` and `UnsubscribeIMAPMailbox` now trim mailbox name/id input before resolving existing mailboxes or retained subscription names.
- Canonical subscription names are also trim-based, and Postgres coverage verifies `" INBOX "` subscribes to the real Inbox and can be removed with the same spaced input.

## IMAP deleted-folder subscription noselect (2026-05-14, complete)
- Postgres coverage now verifies a subscribed user folder deleted through `DeleteFolder` remains listed by subscription name as a non-existing/noselect mailbox.
- The regression also verifies the retained subscription name can be used to unsubscribe after the folder has been deleted.

## IMAP mailbox state cascade cleanup (2026-05-14, complete)
- Postgres coverage now verifies deleting an empty user folder removes its `imap_mailbox_state` row through the folder cascade.
- The regression also verifies `GetIMAPMailbox` no longer exposes the deleted mailbox after state cleanup.

## Mailbox delete deleted-message guard (2026-05-14, complete)
- `DeleteFolder` now treats any remaining message row, including soft-deleted messages, as folder content before attempting folder deletion.
- Postgres coverage verifies a folder with only deleted messages returns the clean not-empty error instead of leaking a foreign-key failure after IMAP UID row cleanup.

## IMAP single restore fresh UID (2026-05-14, complete)
- IMAP/Postgres coverage now verifies a message restored with `RestoreMessage` gets a fresh UID after delete-time UID row removal instead of reusing the expunged UID value.
- The regression verifies the restored message receives a UID above its deleted UID and keeps exactly one message-specific UID row afterward.

## IMAP bulk restore fresh UID (2026-05-14, complete)
- IMAP/Postgres coverage now verifies messages restored with `BulkRestoreMessages` get fresh UIDs after delete-time UID row removal instead of reusing expunged UID values.
- The regression also verifies `BulkRestoreThreads` gives every restored thread message a fresh UID above the previous mailbox max UID.

## IMAP bulk thread move/delete stale-row (2026-05-14, complete)
- IMAP/Postgres coverage now verifies `BulkMoveThreads` removes source mailbox `imap_message_uid` rows for every message in a moved thread and rejects stale old-mailbox UID reallocation.
- The regression verifies moved thread messages can receive fresh destination UIDs, and `BulkDeleteThreads` removes every deleted thread message UID row before rejecting later reallocation.

## IMAP bulk move/delete stale-row (2026-05-14, complete)
- IMAP/Postgres coverage now verifies `BulkMoveMessages` removes source mailbox `imap_message_uid` rows for every moved message and rejects stale old-mailbox UID reallocation.
- The regression verifies moved messages can receive fresh destination UIDs, and `BulkDeleteMessages` removes every deleted message UID row before rejecting later reallocation.

## IMAP message UID move/delete stale-row (2026-05-14, complete)
- IMAP/Postgres coverage now verifies regular `MoveMessage` removes old mailbox `imap_message_uid` rows and rejects stale old-mailbox UID reallocation.
- The regression verifies moved messages get a fresh destination UID, and deleted messages remove their UID rows and reject reallocation.

## IMAP message UID row-lock audit (2026-05-14, complete)
- IMAP UID allocation was revalidated to lock mailbox state, folder, and target `messages` row before assigning a single-message UID.
- Existing Postgres coverage verifies a locked target message row makes `EnsureIMAPMessageUID` wait without inserting a stale UID, then assigns UID/MODSEQ 1/1 after unlock.

## SMTP inbound auth reset (2026-05-14, complete)
- SMTP receiver coverage now verifies `Logout` clears accumulated domain policy and DSN state along with authentication state.
- The regression verifies re-authentication is required, and the following d1-only success is not blocked by previous d2 limits or pre-Logout DSN options.

## SMTP inbound QUIT DSN isolation (2026-05-14, complete)
- SMTP protocol coverage now verifies mixed-domain domain policy and DSN state do not leak across TCP connections after `QUIT`.
- The regression verifies a new d1-only connection is not blocked by the previous d2 size limit and does not inherit previous connection DSN options.

## SMTP inbound HELO DSN reset (2026-05-14, complete)
- SMTP protocol coverage now verifies `HELO` clears accumulated domain policy and DSN state before the next transaction.
- The regression verifies the following d1-only success is not blocked by the previous d2 size limit and does not inherit pre-HELO DSN options.

## SMTP inbound EHLO DSN reset (2026-05-14, complete)
- SMTP protocol coverage now verifies repeated `EHLO` clears accumulated domain policy and DSN state before the next transaction.
- The regression verifies the following d1-only success is not blocked by the previous d2 size limit and does not inherit pre-EHLO DSN options.

## SMTP inbound MAIL DSN reset (2026-05-14, complete)
- SMTP receiver coverage now verifies a new `MAIL` clears accumulated domain policy and DSN state before the next transaction.
- The regression verifies the following d1-only success is not blocked by the previous d2 size limit and does not inherit previous MAIL/RCPT DSN options.

## SMTP inbound RSET DSN reset (2026-05-14, complete)
- SMTP receiver coverage now verifies explicit `RSET` clears accumulated domain policy and DSN state before the next transaction.
- The regression verifies the following d1-only success is not blocked by the previous d2 size limit and does not inherit pre-RSET DSN options.

## SMTP inbound DATA failure DSN reset (2026-05-14, complete)
- SMTP receiver coverage now verifies a mixed-domain size failure resets accumulated domain policy and DSN state before the next transaction.
- The regression verifies the following d1-only success is not blocked by the previous d2 size limit and does not inherit failed transaction DSN options.

## SMTP inbound domain policy QUIT isolation (2026-05-14, complete)
- SMTP protocol coverage now verifies mixed-domain recipient policy state does not leak across TCP connections after `QUIT`.
- The regression verifies a new d1-only connection is not blocked by the previous connection's d2 size limit and records only the accepted d1 recipient.

## SMTP inbound domain policy HELO reset (2026-05-14, complete)
- SMTP protocol coverage now verifies `HELO` clears accumulated mixed-domain recipient policy state.
- The regression verifies a following d1-only transaction is not blocked by the previous d2 size limit and records only the accepted d1 recipient.

## SMTP inbound domain policy EHLO reset (2026-05-14, complete)
- SMTP protocol coverage now verifies a repeated `EHLO` clears accumulated mixed-domain recipient policy state.
- The regression verifies a following d1-only transaction is not blocked by the previous d2 size limit and records only the accepted d1 recipient.

## SMTP inbound domain policy MAIL reset (2026-05-14, complete)
- SMTP receiver coverage now verifies a new `MAIL` clears accumulated mixed-domain recipient policy state.
- The regression verifies a following d1-only transaction is not blocked by the previous d2 size limit and records only the accepted d1 recipient.

## SMTP inbound domain policy RSET reset (2026-05-14, complete)
- SMTP receiver coverage now verifies explicit session reset clears accumulated mixed-domain recipient policy state.
- The regression verifies a following d1-only transaction is not blocked by the previous d2 size limit and records only the accepted d1 recipient.

## SMTP inbound mixed-domain policy reset (2026-05-14, complete)
- SMTP receiver coverage now verifies a failed mixed-domain `DATA` resets the accumulated recipient-domain policy state.
- The regression verifies a following d1-only transaction is not blocked by the previous d2 size limit and records only the accepted d1 recipient.

## SMTP inbound domain policy lookup failure (2026-05-14, complete)
- SMTP receiver coverage now verifies a later recipient domain policy lookup failure returns `451 4.7.1` without poisoning earlier accepted recipients.
- The regression verifies `DATA` still records only the previously accepted recipient after the failed RCPT.

## SMTP inbound mixed-domain policy (2026-05-14, complete)
- SMTP receiver coverage now verifies mixed-domain inbound sessions aggregate enforcing recipient-domain message size policies.
- The regression verifies a stricter second-recipient domain limit is applied at `DATA` and returns `552 5.3.4`.

## POP3 QUIT after failed commit multiline LIST (2026-05-14, complete)
- POP3 server coverage now verifies multiline `LIST` after failed `QUIT` rollback returns all restored message sizes.
- The regression verifies multiline `LIST` leaves the delete mark clear and a later no-delete `QUIT` skips another `CommitDeletes` call.

## POP3 QUIT after failed commit STAT (2026-05-14, complete)
- POP3 server coverage now verifies `STAT` after failed `QUIT` rollback reports the restored message count and size.
- The regression verifies `STAT` leaves the delete mark clear and a later no-delete `QUIT` skips another `CommitDeletes` call.

## POP3 QUIT after failed commit LIST (2026-05-14, complete)
- POP3 server coverage now verifies `LIST 1` after failed `QUIT` rollback returns the restored message size.
- The regression verifies `LIST` leaves the delete mark clear and a later no-delete `QUIT` skips another `CommitDeletes` call.

## POP3 QUIT after failed commit UIDL (2026-05-14, complete)
- POP3 server coverage now verifies `UIDL 1` after failed `QUIT` rollback returns the restored message UIDL.
- The regression verifies `UIDL` leaves the delete mark clear and a later no-delete `QUIT` skips another `CommitDeletes` call.

## POP3 QUIT after failed commit TOP (2026-05-14, complete)
- POP3 server coverage now verifies `TOP 1 1` after failed `QUIT` rollback returns the restored message header and requested body line only.
- The regression verifies `TOP` leaves the delete mark clear and a later no-delete `QUIT` skips another `CommitDeletes` call.

## POP3 QUIT after failed commit RETR (2026-05-14, complete)
- POP3 server coverage now verifies `RETR 1` after failed `QUIT` rollback returns the restored message body.
- The regression verifies `RETR` leaves the delete mark clear and a later no-delete `QUIT` skips another `CommitDeletes` call.

## POP3 QUIT after failed commit invalid DELE (2026-05-14, complete)
- POP3 server coverage now verifies an out-of-range `DELE` after failed `QUIT` rollback returns `-ERR`.
- The regression verifies the restored message remains visible through `LIST 1` and the delete mark stays clear.

## POP3 QUIT after failed commit RSET (2026-05-14, complete)
- POP3 server coverage now verifies `RSET` after a failed delete commit on `QUIT` returns `+OK`.
- The regression verifies the rolled-back message remains visible through `LIST 1` and the delete mark stays clear.

## POP3 QUIT after failed commit NOOP (2026-05-14, complete)
- POP3 server coverage now verifies the connection remains usable for `NOOP` after a failed delete commit on `QUIT`.
- The regression verifies `STAT` after that `NOOP` still reports the rollback-restored maildrop state.

## POP3 QUIT after failed commit CAPA (2026-05-14, complete)
- POP3 server coverage now verifies failed delete commit rollback leaves the session in transaction state for `CAPA`.
- The regression verifies authorization-only capabilities stay hidden and `STAT` reports the restored maildrop count after failed `QUIT`.

## POP3 QUIT after failed commit close retry (2026-05-14, complete)
- POP3 server coverage now verifies a no-delete `QUIT` after failed commit rollback returns `+OK` without another `CommitDeletes` call.
- The regression verifies that retry path closes the TCP connection after the successful `QUIT`.

## POP3 QUIT after failed commit re-delete retry (2026-05-14, complete)
- POP3 server coverage now verifies a failed `QUIT` rollback allows the client to issue `DELE 1` again in the same session.
- The regression verifies the second `QUIT` re-invokes `CommitDeletes` for the new pending delete and preserves the committed delete mark on success.

## POP3 QUIT after failed commit retry (2026-05-14, complete)
- POP3 server coverage now verifies failed delete commit rollback calls `CommitDeletes` once and clears the delete mark.
- The regression verifies a subsequent `QUIT` without another `DELE` returns `+OK` without re-invoking `CommitDeletes`.

## POP3 QUIT after RSET skips commit (2026-05-14, complete)
- POP3 server coverage now verifies `DELE` followed by `RSET` makes `QUIT` skip `CommitDeletes`.
- The regression verifies the reset delete mark is clear before the successful quit path completes.

## POP3 QUIT no-delete close (2026-05-14, complete)
- POP3 server coverage now verifies no-delete `QUIT` closes the TCP connection after `+OK`.
- The regression verifies the close path still skips `CommitDeletes` when no message is marked deleted.

## POP3 QUIT without deletes skips commit (2026-05-14, complete)
- POP3 transaction `QUIT` now calls `CommitDeletes` only when at least one message is marked deleted.
- Regression coverage verifies no-delete `QUIT` returns `+OK` without invoking a failing commit hook, while existing delete commit coverage remains intact.

## POP3 QUIT success commits pending delete (2026-05-14, complete)
- POP3 server coverage now verifies successful `QUIT` after `DELE 1` invokes `CommitDeletes` exactly once.
- The regression verifies the delete mark remains committed after the successful quit path.

## POP3 DELE RSET clears pending delete (2026-05-14, complete)
- POP3 tests now share a pending-delete-cleared assertion for `LIST 1` and `STAT` recovery checks.
- The `RSET` visibility regression now uses the shared assertion to prove pending delete state is cleared.

## POP3 DELE invalid command sequence helper cleanup (2026-05-14, complete)
- POP3 tests now share a pending-delete visibility assertion for `LIST 1` and `STAT` checks.
- NOOP, CAPA, unknown/empty command, AUTH, USER/PASS, and STLS pending-delete regressions now use the shared helper.

## POP3 DELE invalid command sequence docs audit (2026-05-14, complete)
- POP3 pending-delete preservation coverage is now mapped across `NOOP`, `CAPA`, empty command, unknown command, `AUTH`, `USER/PASS`, and `STLS` paths.
- The audit revalidated the existing wire-level tests without adding duplicate coverage.

## POP3 DELE STLS denial preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies transaction-state `STLS` denial after `DELE 1` does not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 DELE transaction USER PASS denial preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies transaction-state `USER` and `PASS` denials after `DELE 1` do not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 DELE transaction AUTH denial preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies transaction-state `AUTH PLAIN` and `AUTH LOGIN` denials after `DELE 1` do not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 DELE empty command preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies empty command lines after `DELE 1` do not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 DELE unknown command preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies unknown commands after `DELE 1` do not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 DELE CAPA preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies `CAPA` after `DELE 1` does not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 DELE NOOP preserves pending delete (2026-05-14, complete)
- POP3 server coverage now verifies `NOOP` after `DELE 1` does not restore the deleted message.
- The regression verifies `LIST 1` still fails and `STAT` still excludes the pending delete.

## POP3 transaction NOOP stability (2026-05-14, complete)
- POP3 server coverage now verifies repeated transaction-state `NOOP` returns `+OK`.
- The regression verifies `LIST 1` and `STAT` still expose the same maildrop state afterward.

## POP3 authorization NOOP stability (2026-05-14, complete)
- POP3 server coverage now verifies repeated authorization-state `NOOP` returns `+OK`.
- The regression verifies auth capabilities and normal authentication still work afterward.

## POP3 authorization CAPA stability (2026-05-14, complete)
- POP3 server coverage now verifies repeated authorization-state `CAPA` calls keep core auth capabilities stable.
- The regression verifies normal authentication and `STAT` still work afterward.

## POP3 transaction CAPA stability (2026-05-14, complete)
- POP3 server coverage now verifies repeated transaction-state `CAPA` calls keep core capabilities stable.
- The regression verifies auth-only capabilities stay hidden and `STAT` still works afterward.

## POP3 transaction STLS denial session audit (2026-05-14, complete)
- POP3 transaction-state `STLS` denial coverage was revalidated without adding a duplicate test.
- Existing coverage verifies the clear `-ERR` denial and confirms `NOOP` and `STAT` still work afterward.

## POP3 transaction AUTH denial (2026-05-14, complete)
- POP3 server coverage now verifies transaction-state `AUTH PLAIN` and `AUTH LOGIN` are rejected as unknown commands.
- The regression verifies `NOOP` and `STAT` still work after rejected SASL reauthentication attempts.

## POP3 transaction USER PASS denial (2026-05-14, complete)
- POP3 server coverage now verifies transaction-state `USER` and `PASS` are rejected as unknown commands.
- The regression verifies `NOOP` and `STAT` still work after rejected reauthentication attempts.

## POP3 authorization empty command recovery (2026-05-14, complete)
- POP3 server coverage now verifies empty authorization-state command lines return `-ERR syntax error`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the syntax error.

## POP3 transaction empty command recovery (2026-05-14, complete)
- POP3 server coverage now verifies empty transaction-state command lines return `-ERR syntax error`.
- The regression verifies `NOOP` and `STAT` still work after the syntax error.

## POP3 transaction unknown command recovery (2026-05-14, complete)
- POP3 server coverage now verifies unknown transaction-state commands return `-ERR unknown command`.
- The regression verifies `NOOP` and `STAT` still work after the unknown command.

## POP3 authorization unknown command recovery (2026-05-14, complete)
- POP3 server coverage now verifies unknown authorization-state commands return `-ERR unknown command`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the unknown command.

## POP3 PASS syntax preserves auth capability (2026-05-14, complete)
- POP3 server coverage now verifies malformed `PASS` commands return `-ERR syntax error`.
- The regression verifies auth capabilities are preserved and a later valid `PASS` still enters transaction state.

## POP3 USER syntax preserves auth capability (2026-05-14, complete)
- POP3 server coverage now verifies malformed `USER` commands return `-ERR syntax error`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the syntax error.

## POP3 USER replacement before PASS (2026-05-14, complete)
- POP3 server coverage now verifies a second `USER` command before `PASS` replaces the pending username.
- The regression verifies authentication succeeds with the replacement username and reaches transaction state.

## POP3 PASS without USER capability (2026-05-14, complete)
- POP3 server coverage now verifies `PASS` without a prior `USER` returns `-ERR authentication failed`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the command-order error.

## POP3 USER PASS failure capability (2026-05-14, complete)
- POP3 server coverage now verifies wrong-password `USER/PASS` returns `-ERR authentication failed`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the failed password attempt.

## POP3 USER PASS transaction capability (2026-05-14, complete)
- POP3 server coverage now verifies successful `USER/PASS` authentication reaches transaction state.
- The regression verifies transaction CAPA drops auth-only capabilities and `STAT` works after authentication.

## POP3 auth success test helper cleanup (2026-05-14, complete)
- POP3 tests now share an authenticated-state assertion helper for transaction CAPA and `STAT` checks.
- AUTH PLAIN and AUTH LOGIN success regressions now use the shared helper.

## POP3 AUTH PLAIN challenge wrong password capability (2026-05-14, complete)
- POP3 server coverage now verifies wrong-password challenge-style `AUTH PLAIN` returns `-ERR authentication failed`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the failed continuation AUTH PLAIN attempt.

## POP3 AUTH PLAIN wrong password capability (2026-05-14, complete)
- POP3 server coverage now verifies wrong-password initial-response `AUTH PLAIN` returns `-ERR authentication failed`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the failed AUTH PLAIN attempt.

## POP3 AUTH LOGIN wrong password capability (2026-05-14, complete)
- POP3 server coverage now verifies wrong-password `AUTH LOGIN` returns `-ERR authentication failed`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the failed AUTH LOGIN attempt.

## POP3 AUTH LOGIN success capability (2026-05-14, complete)
- POP3 server coverage now verifies challenge-style `AUTH LOGIN` accepts valid username/password credentials.
- The regression verifies transaction CAPA drops auth-only capabilities and `STAT` works after authentication.

## POP3 AUTH PLAIN initial response success capability (2026-05-14, complete)
- POP3 server coverage now verifies initial-response `AUTH PLAIN` accepts valid credentials.
- The regression verifies transaction CAPA drops auth-only capabilities and `STAT` works after authentication.

## POP3 AUTH PLAIN successful challenge authentication (2026-05-14, complete)
- POP3 server coverage now verifies challenge-style `AUTH PLAIN` accepts valid continuation credentials.
- The regression verifies transaction CAPA drops auth-only capabilities and `STAT` works after authentication.

## POP3 AUTH PLAIN challenge invalid format capability (2026-05-14, complete)
- POP3 server coverage now verifies malformed decoded `AUTH PLAIN` continuation credentials return `-ERR invalid credentials format`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the continuation format error.

## POP3 AUTH PLAIN challenge invalid base64 capability (2026-05-14, complete)
- POP3 server coverage now verifies invalid base64 in `AUTH PLAIN` continuation input returns `-ERR invalid base64`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the continuation parse error.

## POP3 auth capability assertion helper cleanup (2026-05-14, complete)
- POP3 tests now share an auth capability assertion helper for `USER` and `SASL PLAIN LOGIN` preservation checks.
- AUTH PLAIN and AUTH LOGIN cancellation/error regressions now use the shared helper.

## POP3 AUTH LOGIN invalid password base64 capability (2026-05-14, complete)
- POP3 server coverage now verifies invalid base64 in the `AUTH LOGIN` password step returns `-ERR invalid base64`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the password parse error.

## POP3 AUTH LOGIN invalid username base64 capability (2026-05-14, complete)
- POP3 server coverage now verifies invalid base64 in the `AUTH LOGIN` username step returns `-ERR invalid base64`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the username parse error.

## POP3 AUTH PLAIN invalid format capability (2026-05-14, complete)
- POP3 server coverage now verifies malformed decoded `AUTH PLAIN` credentials return `-ERR invalid credentials format`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the format error.

## POP3 AUTH PLAIN invalid base64 capability (2026-05-14, complete)
- POP3 server coverage now verifies invalid base64 in `AUTH PLAIN` returns `-ERR invalid base64`.
- The regression verifies auth capabilities and normal `USER/PASS` login still work after the parse error.

## POP3 AUTH LOGIN password cancellation capability (2026-05-14, complete)
- POP3 server coverage now verifies `AUTH LOGIN` password cancellation keeps authorization capabilities advertised.
- The regression still verifies normal `USER/PASS` login and `STAT` work after cancellation.

## POP3 AUTH LOGIN username cancellation capability (2026-05-14, complete)
- POP3 server coverage now verifies `AUTH LOGIN` username cancellation keeps authorization capabilities advertised.
- The regression still verifies normal `USER/PASS` login and `STAT` work after cancellation.

## POP3 AUTH PLAIN cancellation session (2026-05-14, complete)
- POP3 server coverage now verifies `AUTH PLAIN` cancellation keeps authorization capabilities advertised.
- The regression still verifies normal `USER/PASS` login and `STAT` work after cancellation.

## POP3 STLS unavailable auth-state session (2026-05-14, complete)
- POP3 server coverage now verifies servers without TLS config reject authorization-state `STLS` with `-ERR`.
- The regression verifies normal `USER/PASS` login and `STAT` still work after the unavailable-STLS denial.

## POP3 STLS transaction-state denial (2026-05-14, complete)
- POP3 server coverage now verifies transaction-state `STLS` returns a clear `-ERR` denial.
- The regression verifies `NOOP` and `STAT` still work after the denial, proving no TLS negotiation started.

## POP3 STLS failure connection close (2026-05-14, complete)
- POP3 server coverage now verifies invalid post-`STLS` plaintext triggers TLS handshake failure.
- The regression verifies the server closes the TCP connection after the failed STLS negotiation path.

## POP3 connection-close test helper cleanup (2026-05-14, complete)
- POP3 test coverage now has a shared deadline-enabled connection helper for EOF-sensitive checks.
- Authorization-state and transaction-state `QUIT` connection close regressions now use the shared helper.

## POP3 AUTH QUIT connection close (2026-05-14, complete)
- POP3 server coverage now verifies authorization-state `QUIT` returns `+OK`.
- The regression verifies the TCP connection closes after pre-login `QUIT` instead of remaining readable.

## POP3 QUIT success connection close (2026-05-14, complete)
- POP3 server coverage now verifies successful delete commit `QUIT` returns `+OK`.
- The regression verifies the TCP connection closes after successful `QUIT` instead of remaining readable.

## POP3 QUIT commit error visibility (2026-05-14, complete)
- POP3 server coverage now verifies failed delete commits return `-ERR` from `QUIT`.
- The same wire-level regression verifies `LIST 1`, `UIDL 1`, and `RETR 1` expose the message again after the failed commit rollback.

## POP3 RSET restores wire visibility (2026-05-14, complete)
- POP3 server coverage now verifies `RSET` restores `LIST 1` and `UIDL 1` visibility after `DELE 1`.
- The same regression verifies `RETR 1` can read the message body again after reset.

## POP3 deleted RETR TOP visibility (2026-05-14, complete)
- POP3 server coverage now verifies `RETR 1` fails after `DELE 1`.
- The same wire-level regression verifies `TOP 1 0` fails for a deleted message.

## POP3 deleted LIST visibility (2026-05-14, complete)
- POP3 server coverage now verifies `LIST 1` fails after `DELE 1`.
- The multi-line LIST regression verifies deleted messages are hidden while remaining messages stay visible.

## POP3 deleted UIDL visibility (2026-05-14, complete)
- POP3 server coverage now verifies `UIDL 1` fails after `DELE 1`.
- The multi-line UIDL regression verifies deleted messages are hidden while remaining messages stay visible.

## POP3 duplicate mark delete (2026-05-14, complete)
- POP3 mailbox coverage now verifies repeated `MarkDeleted` calls for the same message keep one pending delete ID.
- The commit regression confirms the bulk delete request contains the duplicated message only once.

## POP3 reset after commit failure (2026-05-14, complete)
- POP3 mailbox coverage now verifies `ResetDeleted` clears pending deletes after a failed commit.
- The regression confirms a subsequent commit after reset does not issue another bulk delete request.

## POP3 commit failure preserves pending deletes (2026-05-14, complete)
- POP3 mailbox coverage now verifies failed bulk delete commits return an error without clearing pending deletes.
- The regression confirms the message remains marked deleted after a failed commit, preserving retry state.

## POP3 commit clears pending deletes (2026-05-14, complete)
- POP3 mailbox coverage now verifies successful `CommitDeletes` clears pending delete state.
- The repository fake records bulk delete call counts, and the regression verifies a second commit is a no-op.

## POP3 reset restores content access (2026-05-14, complete)
- POP3 mailbox coverage now verifies `ResetDeleted` restores content access after a message was marked deleted.
- The regression reads the stored message body through `MessageContentWithError` after reset.

## POP3 deleted content access (2026-05-14, complete)
- POP3 mailbox coverage now verifies deleted messages return empty content through `MessageContent`.
- The error-returning content path now rejects deleted-message access explicitly.

## POP3 invalid content index (2026-05-14, complete)
- POP3 mailbox coverage now verifies invalid `MessageContent` indexes return an empty string.
- The error-returning content path now has explicit coverage for negative and out-of-range index failures.

## POP3 invalid UIDL index (2026-05-14, complete)
- POP3 mailbox coverage now verifies `MessageUIDL` returns an empty string for negative and out-of-range indexes.
- This keeps invalid POP3 sequence handling from leaking arbitrary or stale message identifiers.

## POP3 invalid message index size (2026-05-14, complete)
- POP3 mailbox coverage now verifies `MessageSize` returns zero for negative and out-of-range indexes.
- This keeps invalid POP3 sequence handling from leaking panics or nonsensical sizes through LIST/STAT paths.

## POP3 message size adapter coverage (2026-05-14, complete)
- POP3 adapter coverage now verifies normalized summary sizes are reflected through mailbox `MessageSize`.
- The regression covers negative, zero, and positive summary sizes through the Authenticate path.

## POP3 message size conversion (2026-05-14, complete)
- POP3 adapter now normalizes maildb `int64` message sizes before exposing POP3 `int` sizes.
- Coverage verifies negative values clamp to zero and values above platform `int` capacity clamp to max int.

## POP3 empty inbox pagination (2026-05-14, complete)
- POP3 adapter coverage now verifies empty INBOX authentication succeeds with a zero-message mailbox.
- The regression asserts empty INBOX pagination performs exactly one zero-cursor page lookup.

## POP3 page cursor pagination (2026-05-14, complete)
- POP3 multi-page INBOX coverage now asserts a 450-message mailbox is loaded with exactly three page calls.
- The repository fake records cursors, and the regression verifies page 2 and page 3 start from the prior page's last message ID.

## POP3 missing cursor guard (2026-05-14, complete)
- POP3 adapter pagination now rejects `HasMore=true` pages that omit `NextCursor` with a `missing inbox cursor` error.
- Regression coverage verifies the guard stops after the first page lookup, preventing repeated first-page reads.

## POP3 message page cursor error (2026-05-14, complete)
- POP3 adapter coverage now verifies invalid multi-page INBOX cursors fail as `decode inbox cursor` errors.
- The regression asserts cursor decode failures stop after the first page lookup instead of advancing with a bad cursor.

## POP3 message page error propagation (2026-05-14, complete)
- POP3 adapter coverage now verifies INBOX message page errors fail authentication as `list inbox messages` errors.
- The regression asserts the failing page lookup uses the selected INBOX folder ID and does not create a mailbox.

## POP3 folder listing error short-circuit (2026-05-14, complete)
- POP3 adapter coverage now verifies folder listing errors are returned as `list folders` failures.
- The regression asserts failed folder listing never advances into message page lookup.

## POP3 missing inbox short-circuit (2026-05-14, complete)
- POP3 adapter coverage now verifies accounts without an INBOX fail after folder listing but before message page lookup.
- The regression asserts folder listing still uses the normalized user ID and no empty or fallback folder ID reaches message listing.

## POP3 inbox folder first-match (2026-05-14, complete)
- POP3 adapter coverage now verifies duplicate INBOX candidates use the first matching system folder.
- The POP3 repository fake records page folder IDs so selected folder routing is asserted directly.

## POP3 inbox folder casing (2026-05-14, complete)
- POP3 adapter coverage now verifies `SystemType=INBOX` is accepted as the user's INBOX.
- The regression confirms message listing proceeds with the normalized user ID after case-insensitive INBOX detection.

## POP3 auth failure short-circuit (2026-05-14, complete)
- POP3 adapter coverage now verifies authenticator credential failures stop before mail service lookup.
- The wrong-password regression asserts one authenticator call and no folder or inbox page reads.

## POP3 must-change-password short-circuit (2026-05-14, complete)
- POP3 adapter coverage now verifies must-change-password users fail after authentication but before mail service lookup.
- The existing policy rejection test now asserts no folder or inbox page reads occur for blocked users.

## POP3 invalid credential short-circuit (2026-05-14, complete)
- POP3 adapter coverage now verifies invalid usernames and CR/LF-bearing passwords fail before authenticator calls.
- The same coverage verifies invalid credential attempts do not reach mail service folder lookup.

## POP3 password passthrough preservation (2026-05-14, complete)
- POP3 adapter coverage now verifies passwords containing surrounding spaces are passed to the authenticator unchanged.
- This keeps username normalization separate from password validation so significant password whitespace is not lost.

## POP3 username normalization passthrough (2026-05-14, complete)
- POP3 adapter coverage now verifies whitespace-wrapped usernames are trimmed before calling the authenticator.
- The POP3 test authenticator records received credentials so adapter boundary normalization can be asserted directly.

## POP3 auth credential validation consolidation (2026-05-14, complete)
- POP3 username normalization and password validation are now centralized in small adapter helpers.
- Helper-level coverage fixes username trim/empty/CRLF behavior and password CRLF rejection without changing authentication semantics.

## POP3 auth identity validation consolidation (2026-05-14, complete)
- POP3 authenticated user ID normalization is now centralized in a helper that trims spaces and rejects empty or CR/LF-bearing identities.
- Helper-level coverage fixes the normalization contract directly in addition to the POP3 Authenticate boundary tests.

## POP3 auth control-character identity rejection (2026-05-14, complete)
- POP3 adapter authentication now rejects authenticated user IDs containing CR/LF before identity normalization or service lookup.
- Regression coverage verifies CR/LF-bearing authenticated user IDs do not trigger folder or inbox page lookups.

## POP3 auth empty identity rejection (2026-05-14, complete)
- POP3 adapter authentication now rejects an authenticated user ID that becomes empty after trimming.
- Regression coverage verifies empty authenticated user IDs do not trigger folder or inbox page lookups.

## POP3 auth identity trimming coverage (2026-05-14, complete)
- POP3 adapter coverage now verifies authenticated user IDs are trimmed before maildrop lock key generation.
- The same test verifies folder and inbox page loads use the trimmed user ID, keeping POP3 mailbox access on one normalized identity.

## POP3 auth identity freshness coverage (2026-05-14, complete)
- POP3 adapter coverage now verifies a second login uses the latest authenticated user ID for the maildrop lock key.
- The same test verifies folder and inbox page loads are performed with the fresh user ID returned by the authenticator.

## POP3 auth policy freshness coverage (2026-05-14, complete)
- POP3 adapter coverage now verifies each login calls the authenticator again rather than reusing prior authentication state.
- The test flips `must_change_password` between logins and verifies the second login is rejected immediately.

## IMAP empty-selected EXPUNGE race verification (2026-05-14, complete)
- `go test -race -count=1 ./internal/imapgw` passes after the empty-selected EXPUNGE fix and IDLE/NOOP integration coverage.
- This keeps the IMAP mailbox event live/drain paths under an explicit race-detector gate after the latest event-state changes.

## IMAP mailbox event empty-selected NOOP coverage (2026-05-14, complete)
- IMAP NOOP integration coverage now selects an empty mailbox, queues an EXPUNGE event, and verifies NOOP drains it without emitting EXPUNGE.
- The test verifies NOOP still completes normally after the ignored empty-selected EXPUNGE event.

## IMAP mailbox event empty-selected IDLE coverage (2026-05-14, complete)
- IMAP IDLE integration coverage now selects an empty mailbox, injects an EXPUNGE event, and verifies no EXPUNGE wire response is emitted.
- The test verifies DONE still completes normally after the ignored empty-selected EXPUNGE event.

## IMAP mailbox event empty-selected EXPUNGE hardening (2026-05-14, complete)
- `writeMailboxEvent` now ignores EXPUNGE events when `selectedMessages=0`, preventing invalid `* 1 EXPUNGE` responses for an empty selected mailbox.
- Regression coverage verifies the empty-selected EXPUNGE path emits no wire output and leaves selected message count unchanged.

## IMAP mailbox event clamped EXPUNGE coverage (2026-05-14, complete)
- `writeMailboxEvent` coverage now verifies EXPUNGE events whose sequence exceeds the selected message count are clamped to the selected count before emitting a wire response.
- The same test verifies selected message count and saved SEARCH state are updated using the clamped sequence.

## IMAP mailbox event zero-sequence EXPUNGE coverage (2026-05-14, complete)
- `writeMailboxEvent` coverage now verifies EXPUNGE events with `SequenceNumber=0` produce no wire output.
- The same test verifies zero-sequence EXPUNGE events leave `selectedMessages` and saved SEARCH sequence state unchanged.

## IMAP mailbox event initial legacy EXISTS coverage (2026-05-14, complete)
- `writeMailboxEvent` coverage now verifies a legacy `Messages=0` EXISTS event increments an initially empty selected mailbox to `* 1 EXISTS`.
- The test documents that `Messages=0` is treated as a legacy increment signal, not an absolute zero-count event.

## IMAP mailbox event legacy EXISTS coverage (2026-05-14, complete)
- `writeMailboxEvent` coverage now verifies a legacy EXISTS event with `Messages=0` increments the selected count by one.
- The test keeps the legacy increment path explicit and separate from the absolute-count EXISTS path.

## IMAP mailbox event fresh EXISTS coverage (2026-05-14, complete)
- `writeMailboxEvent` coverage now verifies a fresh EXISTS event with `Messages` above the selected count emits `* N EXISTS`.
- The same test verifies `selectedMessages` is set to the event's absolute count rather than incremented by one.

## IMAP mailbox event stale EXISTS coverage (2026-05-14, complete)
- `writeMailboxEvent` coverage now verifies EXISTS events whose `Messages` count is below or equal to the selected count produce no wire output.
- The same test verifies stale EXISTS events leave `selectedMessages` unchanged.

## IMAP mailbox event unknown-type coverage (2026-05-14, complete)
- IMAP NOOP drain coverage now includes a selected-mailbox event with an unknown type and verifies it produces no wire response.
- IMAP IDLE live-event coverage now includes the same unknown-type guard before the selected mailbox EXISTS update.

## IMAP mailbox event server ignore coverage (2026-05-14, complete)
- IMAP NOOP event-drain coverage now feeds other-user and other-mailbox events before selected mailbox events and verifies only the selected mailbox responses are written.
- IMAP IDLE live-event coverage now performs the same irrelevant-event check before the selected mailbox EXISTS update.

## IMAP event broker race verification (2026-05-14, complete)
- `go test -race -count=1 ./internal/imapgw` passes with the new mailbox event broker diagnostics and concurrent publish/subscribe/cancel coverage.
- This gives the broker diagnostics path an explicit race-detector gate in addition to the normal `go test ./...` suite.

## IMAP event broker diagnostics concurrency coverage (2026-05-14, complete)
- Broker regression coverage now exercises aggregate/per-mailbox drop diagnostics and subscriber-count diagnostics while publish, subscribe, and cancel operations run concurrently.
- The test verifies subscriber accounting still converges to zero after concurrent diagnostic activity.

## IMAP event broker validation side-effect coverage (2026-05-14, complete)
- Broker regression coverage now verifies invalid publish attempts do not fan out events or modify aggregate/per-mailbox drop counters.
- The same coverage verifies invalid subscribe attempts leave the existing subscriber set unchanged.

## IMAP event broker context cancel idempotency validation (2026-05-14, complete)
- Broker regression coverage now verifies repeated subscription context cancellation closes the event channel and leaves subscriber accounting at zero.
- The same coverage verifies calling the explicit cancel function after context cancellation remains idempotent and does not recreate or retain subscribers.

## IMAP event broker cancel idempotency validation (2026-05-14, complete)
- Broker regression coverage now verifies calling a subscription cancel function repeatedly leaves subscriber accounting at zero.
- The same coverage verifies the subscription channel is closed after repeated cancel calls without panic.

## IMAP event broker canceled subscription validation (2026-05-14, complete)
- The mailbox event broker now exposes a safe `SubscriberCount` diagnostic accessor.
- Regression coverage verifies subscribing with an already canceled context returns no channel/cancel function and leaves the broker subscriber set empty.

## IMAP event broker canceled publish validation (2026-05-14, complete)
- Broker regression coverage now verifies a canceled publish context does not fan out a mailbox event.
- The same test asserts canceled publish attempts leave both aggregate and per-mailbox slow-subscriber drop counters unchanged.

## IMAP event broker per-mailbox drop accounting (2026-05-14, complete)
- Slow-subscriber drops are now counted both as an aggregate and by normalized user/mailbox identity.
- Broker tests verify `DroppedEventsFor` reports the affected mailbox while unrelated mailboxes remain at zero.

## IMAP event broker slow-subscriber accounting (2026-05-14, complete)
- The mailbox event broker now counts events dropped because a matching subscriber channel was full.
- Tests can read the aggregate `DroppedEvents` counter, and the slow-subscriber regression now verifies a non-blocking drop is observable.

## IMAP event broker type normalization (2026-05-14, complete)
- Published mailbox event types are now trimmed and validated against the supported EXISTS, EXPUNGE, and FLAGS types before fanout.
- Broker coverage verifies whitespace-wrapped event types are normalized and unsupported event types are rejected at publish time instead of being silently ignored by IMAP sessions.

## IMAP event broker identity normalization (2026-05-14, complete)
- Mailbox event broker subscriptions now store trimmed user and mailbox IDs after validation.
- Published mailbox events are normalized the same way before fanout, so whitespace-bearing producer inputs do not pass validation but fail subscriber matching.
- The mailservice subscription wrapper regression test now verifies the same normalized matching behavior through the service-facing API.

## mail.stored empty mailbox event hardening (2026-05-14, complete)
- The mail-stored IMAP notification handler now stops after UID assurance if the resulting mailbox ID is empty, avoiding malformed EXISTS fanout.
- Delta sync notifications use the same guard, so mailbox change fanout is not invoked for an empty mailbox identifier.

## mail.stored IMAP EXISTS count hardening (2026-05-14, complete)
- The mail-stored IMAP notification handler now publishes EXISTS events with `Messages=SequenceNumber` after UID assurance, matching restore and service summary event semantics.
- Handler regression coverage verifies stored-message EXISTS events carry the exact mailbox count instead of relying on selected sessions to increment blindly.

## IMAP UID event empty mailbox hardening (2026-05-14, complete)
- UID-based IMAP event publishing now skips entries with an empty mailbox ID, matching the summary-based event path and preventing malformed restore/delete events from reaching selected sessions.
- Restore regression coverage verifies a mixed ensured-UID result publishes only the valid mailbox EXISTS event.

## IMAP restored EXISTS coalescing hardening (2026-05-14, complete)
- UID-based restored-message EXISTS events are now coalesced by mailbox before publishing, keeping only the highest sequence count needed to update selected IMAP sessions.
- Regression coverage verifies bulk message and bulk thread restore publish a single final EXISTS count for one mailbox, while mixed-mailbox restore still emits one EXISTS event per mailbox.

## IMAP restored EXISTS event hardening (2026-05-14, complete)
- UID-based restored-message EXISTS events now carry `Messages=SequenceNumber`, matching APPEND/COPY summary events and letting selected IMAP sessions jump to the exact mailbox count instead of incrementing blindly.
- Service regression coverage now verifies single-message, bulk-message, and bulk-thread restore events publish the expected EXISTS counts.

## IMAP batch ensure UID ordering hardening (2026-05-14, complete)
- `EnsureIMAPMessageUIDsForMessages` now orders active targets by mailbox, internal date, and message ID before assigning missing UIDs, instead of using caller request order.
- Regression coverage verifies reversed batch requests still assign lower UIDs to older messages in the mailbox.

## IMAP ensure-message UID hardening (2026-05-14, complete)
- `EnsureIMAPMessageUID` now preflights single-message lazy UID assignment under the same mailbox UID state then folder-row lock order used by live allocation and operational backfill paths.
- The assignment CTE now locks the target message row before inserting a UID, preserving existing UID lookups while rejecting exhausted UIDNEXT with a stable error and no partial UID rows.

## IMAP lazy UID lock ordering hardening (2026-05-14, complete)
- The operational `BackfillIMAPMailboxUIDs` path now locks the mailbox folder row after the mailbox UID state row and before selecting unassigned message rows.
- Manual backfill and live APPEND/COPY/MOVE lazy allocation paths now share the same state → folder → message lock order for mailbox UID mutation.

## IMAP lazy UID capacity race hardening (2026-05-14, complete)
- IMAP UID capacity preflight now locks the target folder row in the same transaction as the mailbox state row, serializing API/message inserts that rely on the folder foreign key with lazy UID backlog counting.
- Cross-mailbox MOVE destination backfill uses the same folder lock before reading unassigned destination messages, keeping capacity checks and backfill target sets aligned under concurrent mailbox writes.

## IMAP lazy UID exhaustion hardening (2026-05-14, complete)
- IMAP APPEND, COPY, and same-mailbox MOVE now preflight UID allocation capacity inside the transaction by combining existing unassigned-message backlog with the new UID allocation count.
- Cross-mailbox MOVE destination backfill now uses UIDNEXT-after-allocation bounds, and PostgreSQL coverage verifies APPEND/COPY overflow fails with `imap uid space exhausted` without leaving UID rows behind.

## IMAP lazy UID no-op mutation hardening (2026-05-14, complete)
- PostgreSQL-backed IMAP COPY and same-mailbox MOVE now gate lazy destination backfill on the existence of actual source rows, so all-missing UID requests do not mutate mailbox UID rows or stored UID state.
- Regression coverage verifies no-op COPY/MOVE calls against mailboxes with legacy unassigned messages leave `imap_message_uid`, `UIDNEXT`, and `HIGHESTMODSEQ` unchanged.

## IMAP same-mailbox move lazy UID ordering hardening (2026-05-14, complete)
- PostgreSQL-backed same-mailbox IMAP MOVE now backfills existing active unassigned messages inside the locked mailbox-state CTE before creating the moved replacement message UID.
- Regression coverage verifies STATUS prediction, destination UID/sequence after source EXPUNGE, original UID removal, LIST ordering, and final UIDNEXT/HIGHESTMODSEQ remain monotonic with same-mailbox legacy unassigned messages.

## IMAP move lazy UID destination ordering hardening (2026-05-14, complete)
- PostgreSQL-backed cross-mailbox IMAP MOVE now backfills existing active destination messages without `imap_message_uid` rows before assigning moved-message destination UIDs.
- Regression coverage verifies destination STATUS prediction, MOVE destination UID/sequence number, source removal, LIST ordering, and final UIDNEXT/HIGHESTMODSEQ stay monotonic with legacy unassigned destination messages.

## IMAP copy lazy UID destination ordering hardening (2026-05-14, complete)
- PostgreSQL-backed IMAP COPY now backfills existing active destination messages without `imap_message_uid` rows inside the locked destination mailbox-state transaction before assigning copied message UIDs.
- Regression coverage verifies destination STATUS prediction, COPYUID destination UID/sequence number, LIST ordering, and final UIDNEXT/HIGHESTMODSEQ stay monotonic when the destination mailbox contains legacy unassigned messages.

## IMAP append lazy UID ordering hardening (2026-05-14, complete)
- PostgreSQL-backed IMAP APPEND now backfills existing active messages without `imap_message_uid` rows inside the same mailbox-state transaction before assigning the appended message UID.
- Regression coverage verifies mailbox STATUS UIDNEXT/HIGHESTMODSEQ prediction, APPENDUID/sequence number, LIST ordering, and final mailbox state stay on the same UID timeline when legacy unassigned messages exist.

## POP3 delete commit idempotency hardening (2026-05-14, complete)
- POP3 pending delete commits now normalize message IDs before calling the mail service bulk delete path, trimming whitespace, skipping empty IDs, and de-duplicating IDs while preserving first-seen order.
- Regression coverage now verifies an internally duplicated pending delete list produces a single stable bulk delete request and clears pending state after a successful commit.

## POP3 listing size consistency hardening (2026-05-14, complete)
- `RETR` now announces the same mailbox `MessageSize` value used by `LIST` and `STAT` for the selected message.
- POP3 regression coverage now verifies LF-only stored content does not make `RETR` advertise a different octet count from `LIST`.

## IMAP lazy UID sequence hardening (2026-05-14, complete)
- `ListIMAPMessages` now assigns explicit sequence numbers after lazy UID assignment and UID sorting.
- Partial `AfterUID` listings compute their sequence base from active UIDs already at or below the cursor, preventing subset-local sequence numbers from leaking into IMAP responses and events.

## IMAP mailbox status consistency hardening (2026-05-14, complete)
- Folder and mailbox status aggregation now counts active messages without assigned IMAP UIDs.
- `UIDNEXT` and `HIGHESTMODSEQ` reported through DB-backed IMAP mailbox status now include those pending lazy UID assignments, keeping EXISTS and UID state predictions aligned.

## CardDAV sync payload projection hardening (2026-05-14, complete)
- `sync-collection` change responses now coalesce duplicate contact changes before loading requested `address-data`.
- The CardDAV repository joined-change path can return metadata-only objects, allowing the handler to batch-load vCard payloads only for the final coalesced response set.

## CalDAV sync payload projection hardening (2026-05-14, complete)
- `sync-collection` change responses now coalesce duplicate object changes from metadata-only change rows before loading `calendar-data`.
- When clients request calendar bodies, the handler performs a single batched object lookup for the final coalesced response set instead of reading ICS payloads for discarded intermediate changes.

## CalDAV free-busy candidate optimization hardening (2026-05-14, complete)
- `free-busy-query` now limits candidate object loading to VEVENT and VFREEBUSY components when the store supports component-scoped listing.
- Non-busy components such as VTODO no longer consume the free-busy report limit or force irrelevant ICS reads on the optimized repository path.

## CalDAV calendar-query candidate optimization hardening (2026-05-14, complete)
- Time-range `calendar-query` handling now dispatches directly to the CalDAV component candidate walker before any broad or component-limited list prefetch.
- Handler regression coverage now verifies large-calendar query optimization does not perform an extra object list query before evaluating time ranges.

## CalDAV sync-token retention hardening (2026-05-14, complete)
- CalDAV sync change pruning now explicitly excludes the current `caldav_calendars.sync_token` from both dry-run candidate counts and delete candidates.
- The CalDAV retention guard now matches the CardDAV behavior, keeping the latest collection sync marker available after cleanup.

## CalDAV scheduling persistence guard (2026-05-14, complete)
- CalDAV repository validation tests now explicitly reject `METHOD`-bearing VCALENDAR bodies at the `UpsertObject` storage boundary.
- The documented contract remains split: iTIP scheduling parsing may accept `METHOD`, while persisted calendar object resources must not store scheduling payloads.

## CardDAV sync-token retention hardening (2026-05-14, complete)
- CardDAV sync change pruning now explicitly excludes the current `carddav_addressbooks.sync_token` from both dry-run candidate counts and delete candidates.
- This keeps the latest collection sync marker available even if legacy or partial change rows make "newer change exists" insufficient as the only retention guard.

## CardDAV query candidate optimization hardening (2026-05-14, complete)
- CardDAV `addressbook-query` candidate selection now falls back to the broad object walker when seed text contains SQL LIKE wildcard characters or an escape character.
- Handler coverage now verifies wildcard-bearing text-match filters do not enter the optimized candidate walker path.

## CardDAV query filter semantics hardening (2026-05-14, complete)
- CardDAV `param-filter` negated text matches now evaluate all parameter values and reject a property when any value matches the forbidden text.
- Regression coverage now verifies a multi-value parameter such as `TYPE=home,work` does not satisfy a negated `TYPE=home` filter.

## CardDAV address-data UID projection hardening (2026-05-14, complete)
- CardDAV `address-data` partial projections now always retain the vCard `UID` line alongside `BEGIN`, `VERSION`, and `END`.
- Response and REPORT handler tests now assert projected contacts keep their required UID while still omitting unrequested properties such as email addresses.

## CardDAV vCard line-ending hardening (2026-05-14, complete)
- CardDAV vCard object validation now rejects LF-only and mixed newline payloads before contact objects can be stored.
- Metadata and repository tests now cover CRLF-only vCard line-ending enforcement alongside existing UID/FN/version validation.

## DAV password-change auth hardening (2026-05-14, complete)
- CalDAV and CardDAV Basic Auth resolvers now reject authenticated submission users marked `must_change_password`, matching SMTP submission, IMAP, and POP3 policy.
- DAV regression tests now cover password-change-required users so temporary-password accounts cannot bypass the web password rotation step through calendar or contacts clients.

## Protocol company suspension auth hardening (2026-05-14, complete)
- Submission authentication now joins the owning company and requires `companies.status = 'active'`, so suspended tenants cannot continue authenticating through shared protocol credentials.
- The hardening applies to CardDAV and every protocol path backed by the same submission authenticator, keeping company/domain/user status policy aligned at the database trust boundary.
- PostgreSQL integration coverage now verifies an active user can authenticate before company suspension and is rejected immediately after the company status changes.

## Domain settings visibility hardening (2026-05-14, complete)
- Domain settings now load whenever the selected domain changes, including the auto-selected first domain, so the editable registration/security/password/quota form appears without requiring a second manual selection.
- Domain settings load failures and empty-domain states are rendered outside the settings form block, preventing the page from looking blank when settings data cannot be fetched.
- Domain settings reads now scan nullable `updated_by` values safely, preventing legacy/default settings rows from failing to load before the form can render.
- The domain settings page now gives conditional Cloudscape layout children stable React keys, removing the console warning emitted while rendering the settings form.

## Recovery settings form hardening (2026-05-14, complete)
- Webmail profile updates now parse structured API error envelopes before displaying them, so backup email validation errors no longer render as `[object Object]`.
- Domain settings now auto-select the first available domain and surfaces settings-load failures in the UI instead of leaving a blank configuration area.
- Admin domain settings reads now create the default `domain_settings` row on demand for existing domains that predate the settings trigger, allowing the password reset link expiry field to appear and save normally.

## Password reset contact groundwork (2026-05-14, complete)
- Users now have a shared `recovery_email` field exposed consistently through admin user list/detail/create/update APIs and the authenticated webmail profile API.
- Webmail account settings let users save or clear a personal backup email address for future password reset delivery.
- Admin console user management lets operators set the same backup email at create time or from the user edit dialog without exposing or editing the user's current password.
- Domain settings now include `password_reset_token_ttl_minutes`, allowing each domain to configure future password reset link expiry between 1 minute and 7 days.
- The OpenAPI contract and migrations were updated so reset-link generation/email delivery can use the shared recovery email and per-domain TTL instead of introducing a separate data path.

## Admin console data-list controls (2026-05-13, complete)
- Admin console table/list screens now use a shared `DataTable` wrapper over Cloudscape `Table`, giving table surfaces a consistent scroll container plus default client-side search and pagination when the page does not provide its own controls.
- The wrapper preserves existing page-specific filters and pagination on screens such as users, companies, domains, and admin activity, while adding missing search/pagination to list surfaces such as admin users, legal holds, compliance reports, routing rules, webhooks, notification templates, system queue, system health tables, domain detail tables, and other table-based pages.
- AppLayout content scrolling now targets the actual Cloudscape content container, so long data lists can be scrolled independently without requiring a full page refresh or losing the fixed top navigation/sidebar.
- Verification covered all console app table call sites (`51` tables across `46` page files) being routed through `DataTable`, admin console TypeScript type-checking, and OpenChrome smoke checks for admin-users search/pagination and users-list vertical scrolling.

## Admin console navigation hardening (2026-05-13, complete)
- Admin console side navigation now derives company-scoped links from the current `/companies/{id}` route before falling back to company context, preventing transient context loading from producing stale `/companies/default/...` menu links during SPA transitions.
- The company context now synchronizes its selected company ID when the route-level company ID changes, keeping the top navigation, sidebar, and company switcher aligned after client-side navigation.
- Sidebar navigation still uses Next.js client routing first, but now falls back to a browser navigation if the route does not change shortly after a menu click, so Cloudscape/Next timing issues cannot silently swallow a menu selection.
- The root layout opts into Next.js smooth-scroll route transition behavior to remove the development warning observed during browser verification.
- Verification covered admin console TypeScript type-checking and an OpenChrome smoke pass across dashboard → users → queue stats without refresh or console errors.

## Admin console i18n hardening (2026-05-13, complete)
- Admin console pages were scanned for hard-coded English UI strings and the visible labels, descriptions, table headers, buttons, flash messages, modal text, placeholders, status labels, and dashboard/health counters were moved behind the console `useI18n()` message catalog.
- The English/Korean/Japanese/Simplified Chinese message catalogs now include the new console strings for SCIM status, admin activity, security posture, seat usage, webhooks, global signatures, legal holds, notification templates, login, layout, tenant health, domains, dashboard, system health, change history, delegations, users, auth policy, DMARC/SPF, domain settings, domain detail mail stats, routing rules, alerts, and queue status.
- `I18nProvider` now provides the default locale context during first render, fixing the login page runtime failure caused by calling `useI18n()` before client mount.
- Verification covered console TypeScript type-checking, catalog key coverage across all four locales, and the hard-coded-string scanner. Remaining scanner hits are technical placeholders or literals such as certificate/private-key examples, TLS version names, sample hostnames, browser metadata, and language names.

## Admin console to webmail browser smoke hardening (2026-05-13, complete)
- 실제 dev 스택(PostgreSQL/Redis/MinIO, Go all-in-one, admin console, webmail)을 기동해 콘솔 로그인 → 회사 사용자 생성 → 웹메일 로그인/작성 플로우를 점검했다.
- Admin console의 `/companies/default/...` 경로는 인증 후 첫 회사의 실제 UUID 경로로 즉시 정규화하도록 수정해, 사용자/도메인 관리 API가 `company_id=default`로 호출되어 빈 테넌트 데이터처럼 보이는 문제를 제거했다.
- Webmail API 오류 파서는 `{error:{message}}`/`error_message` 응답을 문자열로 정규화해 로그인/메일 준비 실패가 `[object Object]`로 표시되지 않게 했다.
- PostgreSQL draft 저장 SQL은 `scheduled_at` JSONB 파라미터를 명시적으로 `text` 캐스팅해, compose 전송 준비 중 draft 저장이 `could not determine data type of parameter`로 실패하지 않도록 보정했다.
- Draft-send는 기존 DB 저장 포맷인 `address` 키를 `outbound.Address.Email`로 복원하도록 보강해, autosave된 수신자가 발송 시 `to[0].email is required`로 사라지지 않게 했다.

## Admin/webmail contract 정합성 정리 (2026-05-13, complete)
- 콘솔 사용자 목록은 상태값(`active|suspended|disabled`)로 정규화해 알 수 없는/레거시 값은 `disabled`로 폴백 처리하고, 상태 색상/카운트 집계도 정합성 있게 정리.
- 콘솔 전달 라우트 토글 동작을 `active/disabled` 상태계약에 맞춰 수정해 잘못된 `inactive` 요청이 나가지 않도록 보정.
- 웹메일 인라인 답장/전체답장/전달 동작에서 `reply_all` 인텐트를 `reply`로 변환해 전송 payload를 정규화하고, `reply`/`forward` 케이스에서만 `source_message_id`를 전송하도록 반영해 compose 계약과 정합성 유지.

## CalDAV/CardDAV auth and report stability hardening (2026-05-13, complete)
- CalDAV/CardDAV Basic Auth now gates `X-Forwarded-Proto=https` on explicit
  trusted proxy configuration, with env/YAML loading, validation, and runtime
  resolver injection; the default is to trust only direct TLS on the request.
- Unauthorized DAV responses now include the appropriate Basic challenge while
  delegated authorization failures remain plain 403 responses.
- CalDAV scheduling validation accepts `METHOD` only for scheduling payloads,
  preserving stored calendar-object rejection of `METHOD`.
- CardDAV address-book collections no longer advertise
  `principal-property-search`; only addressbook query/multiget and configured
  sync-collection reports are exposed there.

## CalDAV/CardDAV xml:lang resource-tag suffix WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` resource-tag prefixes
  with suffix text hidden before a final `>`.
- WebDAV `If` evaluation now rejects resource-tag content containing raw `<`
  or `>` characters with HTTP 400 before body reads and before attempted
  explicit `xml:lang` mutations can reach repository updates.

## CalDAV/CardDAV xml:lang empty resource-tag WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` headers with empty
  resource-tag prefixes before a condition list.
- Empty `<>` prefixes are asserted as HTTP 400 malformed resource tags before
  body reads and before attempted explicit `xml:lang` mutations can reach
  repository updates.

## CalDAV/CardDAV xml:lang malformed prefix WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` headers with malformed
  non-empty prefixes before a condition list.
- WebDAV `If` evaluation now requires each condition-list prefix to be empty
  or a `<resource-tag>`, returning HTTP 400 before body reads and before
  attempted explicit `xml:lang` mutations can reach repository updates.

## CalDAV/CardDAV xml:lang trailing WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` headers with trailing
  tokens after a syntactically valid, matching condition list.
- WebDAV `If` evaluation now rejects trailing data with HTTP 400 before body
  reads and before attempted explicit `xml:lang` mutations can reach
  repository updates.

## CalDAV/CardDAV xml:lang irrelevant tagged malformed WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover tagged WebDAV `If` lists whose
  resource tag does not match the current CalDAV/CardDAV collection but whose
  condition-list is malformed.
- WebDAV `If` evaluation now validates condition-list syntax before applying
  resource-tag relevance, returning HTTP 400 before body reads and before
  attempted explicit `xml:lang` mutations can reach repository updates.

## CalDAV/CardDAV xml:lang repeated malformed WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover repeated HTTP `If` headers where a
  matching earlier condition list is followed by a malformed relevant list.
- WebDAV `If` evaluation now validates all relevant condition lists before
  accepting the precondition, returning HTTP 400 before body reads and before
  attempted explicit `xml:lang` mutations can reach repository updates.

## CalDAV/CardDAV xml:lang repeated WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover repeated HTTP `If` header fields,
  asserting the gateway joins them into WebDAV condition-list sequences.
- Repeated-header success preserves omitted language tags when a later list
  matches, while all-failed repeated headers reject before body reads and leave
  attempted explicit `xml:lang` mutations unapplied.

## CalDAV/CardDAV xml:lang compound WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover compound WebDAV `If` condition lists,
  asserting multiple conditions inside one list are evaluated conjunctively.
- A matching ETag plus negated missing state token succeeds and preserves
  omitted language tags; a matching ETag plus bare missing state token rejects
  before body reads and leaves attempted language mutations unapplied.

## CalDAV/CardDAV xml:lang state-token WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` state-token conditions
  with the current no-lock-token-store semantics: bare state tokens fail and
  reject before body reads.
- `Not <opaquelocktoken:...>` conditions now succeed, applying text-only
  updates while preserving omitted CalDAV/CardDAV language tags.

## CalDAV/CardDAV xml:lang absolute URI WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` tagged lists whose
  resource tag is an absolute HTTP(S) URI and whose path matches the current
  CalDAV/CardDAV collection path, preserving omitted language tags.
- Absolute URI tags with non-matching paths now reject before request-body
  reads and leave attempted explicit `xml:lang` mutations unapplied.

## CalDAV/CardDAV xml:lang malformed WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now assert malformed WebDAV `If` headers return
  HTTP 400 before request-body reads and before attempted explicit
  `xml:lang` mutations can reach repository updates.
- Coverage includes line breaks, unterminated condition lists, empty condition
  lists, unsupported conditions, unterminated entity tags, and unterminated
  state tokens.

## CalDAV/CardDAV xml:lang multi-list WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` headers with multiple
  relevant condition lists, asserting a later matching list succeeds after an
  earlier stale list and preserves omitted language tags.
- When all relevant condition lists fail, CalDAV/CardDAV reject before body
  reads and leave attempted explicit `xml:lang` mutations unapplied.

## CalDAV/CardDAV xml:lang Not WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` condition-list `Not`
  semantics for CalDAV and CardDAV: `Not` around a stale ETag allows the
  text-only update and preserves omitted language tags.
- `Not` around the current collection ETag now rejects before body reads,
  leaving attempted explicit `xml:lang` mutations unapplied.

## CalDAV/CardDAV xml:lang tagged WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover tagged WebDAV `If` header lists whose
  resource tag exactly matches the CalDAV/CardDAV collection path, preserving
  omitted displayname and description language tags.
- Non-matching tagged `If` lists now reject before request-body reads, leaving
  attempted explicit `xml:lang` mutations unapplied and avoiding repository
  update calls.
- Matching tagged `If` lists with stale ETags are covered by the same
  pre-body-read rollback assertions.

## CalDAV/CardDAV xml:lang observed ETag PostgreSQL coverage (2026-05-13, complete)
- Optional PostgreSQL integration tests now verify observed collection ETag
  updates preserve existing CalDAV/CardDAV displayname and description
  language tags when `xml:lang` is omitted.
- Stale observed collection ETag tests attempt text plus explicit language
  mutations and assert the repository rejects before persisting any text or
  language changes.

## CalDAV/CardDAV xml:lang WebDAV If coverage (2026-05-13, complete)
- Collection `PROPPATCH` tests now cover WebDAV `If` header success for
  CalDAV and CardDAV, asserting omitted `xml:lang` preserves existing
  displayname and description language tags.
- Failing WebDAV `If` header tests use unread bodies with attempted explicit
  `xml:lang` mutations, asserting HTTP 412 responses happen before body reads
  and before repository update calls.

## CalDAV/CardDAV xml:lang conditional success coverage (2026-05-13, complete)
- Matching collection `If-Match` PROPPATCH tests now assert omitted
  `xml:lang` preserves existing displayname and description language tags.
- `If-Match: *` success tests now assert observed collection ETags are carried
  alongside explicit `xml:lang` updates.

## CalDAV/CardDAV xml:lang conditional rollback coverage (2026-05-13, complete)
- Collection `PROPPATCH` conditional failure tests now use unread bodies with
  attempted `xml:lang` mutations and assert existing language tags remain
  unchanged.
- CalDAV/CardDAV coverage includes stale `If-Unmodified-Since`, repeated
  `If-Unmodified-Since`, mismatched `If-Match`, matching `If-None-Match`, and
  `If-None-Match: *`, with zero body reads and no repository update calls.

## CalDAV/CardDAV xml:lang PROPPATCH rollback coverage (2026-05-13, complete)
- Unsupported/protected collection `PROPPATCH` handler tests now include
  attempted `xml:lang` mutations and assert existing language tags remain
  unchanged.
- The tests also assert failure responses are produced before repository update
  calls, strengthening RFC 4918 atomicity coverage for text and language state.

## CalDAV/CardDAV xml:lang explicit empty clearing (2026-05-13, complete)
- Handler and PostgreSQL integration coverage now verifies explicit
  `xml:lang=""` clears existing collection language tags for CalDAV calendars
  and CardDAV address books.
- Parser and repository tests assert explicit empty language remains a non-nil
  empty pointer, while responses omit `xml:lang` when the stored language is
  empty.

## CalDAV/CardDAV xml:lang repository nil-preservation (2026-05-13, complete)
- PostgreSQL integration coverage now verifies omitted `xml:lang` values do not
  erase existing collection language tags during repository updates.
- CalDAV covers unrelated color updates plus displayname/description text-only
  updates; CardDAV covers displayname/description text-only updates.
- PROPPATCH parsing now distinguishes absent `xml:lang` from explicit
  `xml:lang=""`, and repository update validation preserves nil language
  pointers so text-only updates keep existing tags.

## CalDAV/CardDAV xml:lang postgres constraint coverage (2026-05-13, complete)
- Optional PostgreSQL integration tests now bypass application validation with
  raw SQL and verify migration 0097 rejects invalid collection language tags.
- CalDAV/CardDAV coverage checks whitespace/control `displayname_lang`, overly
  long `description_lang`, PostgreSQL error code `23514`, and exact constraint
  names.

## CalDAV/CardDAV xml:lang postgres repository integration (2026-05-13, complete)
- Optional PostgreSQL integration tests now create migrated temp schemas and
  verify CalDAV calendar plus CardDAV address-book `xml:lang` persistence.
- The tests cover repository create/get/update round-trips and raw
  `displayname_lang`/`description_lang` columns after migration 0097.
- They follow the existing `GOGOMAIL_TEST_DATABASE_URL` convention and skip
  cleanly when no PostgreSQL test database is configured.

## CalDAV/CardDAV xml:lang migration guardrails (2026-05-13, complete)
- Migration 0097 now has static coverage for CalDAV/CardDAV collection
  language columns, DB-level length/whitespace constraints, and Down cleanup.
- The guard protects the `displayname_lang` and `description_lang` column names
  used by both repository implementations for `PROPPATCH`, `MKCALENDAR`, and
  extended `MKCOL` persistence.

## CalDAV/CardDAV creation xml:lang PROPFIND compatibility (2026-05-13, complete)
- CalDAV `MKCALENDAR` and CardDAV extended `MKCOL` now parse `DAV:prop
  xml:lang` and store language tags for collection display names and
  descriptions at creation time.
- Freshly created collections return those language tags on collection
  `PROPFIND` as `xml:lang` attributes on the corresponding properties.
- Malformed creation language tags are rejected before creating calendars or
  address books.

## CalDAV/CardDAV PROPPATCH xml:lang persistence (2026-05-13, complete)
- CalDAV/CardDAV collection `PROPPATCH` now parses `DAV:prop xml:lang` and
  stores language tags for `DAV:displayname` plus calendar/address-book
  description properties.
- `PROPFIND` and successful `PROPPATCH` responses now emit stored language tags
  as `xml:lang` attributes on those properties.
- Removing description properties clears their stored language tag, malformed
  language values are rejected before mutation, and migration 0097 adds durable
  language columns with DB-level bounds.

## CalDAV/CardDAV PROPPATCH instruction prop cardinality (2026-05-13, complete)
- CalDAV/CardDAV `PROPPATCH` now enforce the RFC 4918 `set (prop)` and
  `remove (prop)` grammar by rejecting a second `DAV:prop` child inside the
  same instruction.
- Requests may still use multiple `DAV:set`/`DAV:remove` instructions, each
  with exactly one `DAV:prop`, preserving document-order processing.
- Parser regressions cover duplicate `DAV:prop` rejection and valid
  multi-instruction requests in both gateways.

## CalDAV/CardDAV PROPPATCH instruction emptiness (2026-05-13, complete)
- CalDAV/CardDAV `PROPPATCH` now reject empty `DAV:set` and `DAV:remove`
  instructions, including self-closing and explicitly empty forms.
- The check is per instruction, so malformed empty instructions are rejected
  even when a sibling instruction contains valid properties.
- Existing supported, unsupported, and protected property semantics remain
  unchanged whenever the instruction contains a `DAV:prop` child.

## CalDAV/CardDAV PROPPATCH remove property emptiness (2026-05-13, complete)
- CalDAV/CardDAV `PROPPATCH` remove instructions now require property elements
  inside `DAV:prop` to be empty name markers.
- Remove properties containing non-whitespace text or nested XML children are
  rejected as malformed XML request bodies instead of being skipped into normal
  remove or property-failure handling.
- Empty supported, unsupported, and protected remove property semantics remain
  intact, with regression coverage in both gateways.

## CalDAV/CardDAV PROPPATCH duplicate property response semantics (2026-05-13, complete)
- CalDAV/CardDAV `PROPPATCH` handlers preserve document-order final mutation
  behavior for repeated mutable properties such as calendar and address-book
  descriptions.
- Success responses and failed-dependency `propstat` groups now report each
  repeated mutable property name once, while parser limits still count every
  occurrence.
- Handler regressions cover duplicate set/remove/set and duplicate dependency
  failure paths in both gateways.

## CalDAV/CardDAV PROPPATCH aggregate property limit (2026-05-13, complete)
- CalDAV/CardDAV `PROPPATCH` now count properties across the whole
  `propertyupdate` request instead of resetting the limit for each `DAV:prop`
  block.
- Supported, unsupported, and protected properties all contribute to the
  aggregate `MaxWebDAVProperties` limit, so split `DAV:set`/`DAV:remove`
  requests cannot bypass parser bounds.
- Regression tests cover over-limit requests split across multiple
  instruction and property blocks in both gateways.

## CalDAV/CardDAV PROPPATCH structural body strictness (2026-05-13, complete)
- CalDAV/CardDAV `PROPPATCH` now reject unknown structural children inside
  `DAV:set` and `DAV:remove` as parse errors instead of silently skipping them.
- Unsupported and protected property failure semantics remain scoped to
  property elements inside `DAV:prop`, preserving atomic property-level
  `207 Multi-Status` handling for valid instruction shapes.
- Regression tests cover malformed set/remove instruction structure in both
  gateways.

## CalDAV PROPPATCH unsupported/protected property failure responses (2026-05-13, complete)
- CalDAV `PROPPATCH` now preserves unsupported properties and protected `DAV:displayname`
  remove attempts as request metadata instead of silently skipping or parse-rejecting them.
- Mixed supported + unsupported/protected requests fail atomically before repository update,
  returning `207 Multi-Status` with `403 Forbidden` for the failing property and
  `424 Failed Dependency` for otherwise mutable properties.
- Regression tests cover arbitrary namespace unsupported properties, protected remove attempts,
  and unchanged calendar metadata after failed requests.

## CardDAV MKCOL structural body strictness (2026-05-13, complete)
- CardDAV extended `MKCOL` now rejects unknown top-level children and unknown children inside
  `DAV:set` as parse errors instead of skipping them into later semantic failure paths.
- Handler regression coverage proves malformed structural children return `400` and do not create
  address books, while supported and unsupported property-level failure semantics remain intact.

## DAV creation XML body presence semantics (2026-05-13, complete)
- CalDAV `MKCALENDAR` now distinguishes truly absent bodies from non-empty whitespace-only
  bodies: absent body compatibility remains, while whitespace-only XML bodies return `400`
  and do not create calendars.
- CardDAV extended `MKCOL` now rejects whitespace-only bodies as malformed XML with `400`
  instead of flowing into the missing-resource-type path.
- Parser and handler regressions cover absent, whitespace-only, and no-create behavior.

## CalDAV unsupported namespace failure responses (2026-05-13, complete)
- CalDAV WebDAV response serialization now preserves unknown property namespaces with a scoped
  fallback XML namespace declaration on the property element.
- `MKCALENDAR` requests that include unsupported properties from arbitrary namespaces now return
  RFC property-level `207 Multi-Status` failure responses instead of falling through to `500`.
- Regression tests cover serializer output and handler responses for `urn:example:test`
  unsupported properties.

## CardDAV unsupported namespace failure responses (2026-05-13, complete)
- CardDAV WebDAV response serialization now preserves unknown property namespaces by adding
  a scoped fallback XML namespace declaration on the property element.
- `PROPPATCH` and extended `MKCOL` requests that include unsupported properties from arbitrary
  namespaces now return property-level failure responses instead of falling through to `500`.
- Regression tests cover serializer output and handler responses for `urn:example:test`
  unsupported properties.

## DAV creation XML Content-Type validation (2026-05-13, complete)
- CalDAV `MKCALENDAR` and CardDAV extended `MKCOL` now validate explicit
  non-empty request-body `Content-Type` headers before XML parsing or collection creation.
- Missing `Content-Type` remains accepted for client compatibility, while XML media types such as
  `application/xml; charset=utf-8` continue through the existing success path.
- Duplicate or malformed `Content-Type` headers return `400`; non-XML media types return `415`,
  and regression tests verify no calendar/address book is created.

## DAV creation response cache-control (2026-05-13, complete)
- CalDAV `MKCALENDAR` success responses and property-failure `C:mkcalendar-response`
  responses now include both `no-store` and RFC 4791-required `no-cache`.
- CardDAV extended `MKCOL` success responses and RFC 5689-style property-failure
  `D:mkcol-response` responses now include both `no-store` and `no-cache`.

## CalDAV/CardDAV creation body strictness (2026-05-13, complete)
- CardDAV address-book `MKCOL` now requires an extended body containing
  `DAV:resourcetype` with both `DAV:collection` and `CARDDAV:addressbook`; empty,
  self-closing, or displayname-only generic collection requests no longer create address books.
- CalDAV `MKCALENDAR` keeps truly absent bodies compatible, but non-empty
  `C:mkcalendar` XML must include the RFC 4791 `DAV:set`/`DAV:prop` shape and rejects
  unknown structural children instead of silently creating a calendar.
- Regression tests cover rejected creation-body shapes and verify no collection is created.

## CalDAV MKCALENDAR property failure multistatus (2026-05-13, complete)
- CalDAV `MKCALENDAR` parser now preserves requested creation properties and unsupported
  property names instead of skipping them during request-body parsing.
- Unsupported creation properties and invalid supported property values now fail before repository
  creation, returning RFC 4791 `207 Multi-Status` with a `C:mkcalendar-response` body.
- Failed properties return `403 Forbidden` or `409 Conflict`; otherwise valid properties in the
  same atomic creation request return `424 Failed Dependency`, and calendars are not created.

## CardDAV extended MKCOL property failure response (2026-05-13, complete)
- CardDAV extended `MKCOL` parser now preserves requested creation properties, unsupported
  properties, and invalid `DAV:resourcetype` state instead of silently skipping them.
- Unsupported creation properties or unsupported resource type values now fail before repository
  creation, returning RFC 5689-shaped `DAV:mkcol-response` XML with `403 Forbidden` for the
  failing property and `424 Failed Dependency` for otherwise valid properties in the same request.
- CardDAV OPTIONS discovery now advertises the RFC 5689 `extended-mkcol` DAV token.

## CardDAV PROPPATCH property failure multistatus (2026-05-13, complete)
- CardDAV `PROPPATCH` parser가 unsupported property와 protected `displayname` remove 시도를
  일반 parse error 또는 silent skip으로 처리하지 않고 request metadata로 보존하도록 했다.
- unsupported/protected property가 포함되면 repository update를 호출하지 않아 atomicity를 유지하고,
  `207 Multi-Status`에서 해당 property는 `403 Forbidden`, 의존 mutable property는
  `424 Failed Dependency`로 반환한다.

## WebDAV If header conditional support (2026-05-13, complete)
- CalDAV/CardDAV object read/write/delete 경로가 WebDAV `If` header의 ETag condition list를 평가하도록 보강했다.
- CalDAV calendar collection 및 CardDAV address-book collection의 PROPPATCH/DELETE/create precondition 경로도
  `If` header를 기존 ETag/date conditional 처리와 함께 평가한다.
- 현재 lock token 저장소가 없으므로 positive state-token 조건은 실패하고, `Not` 조건과 ETag 조건은
  RFC 4918 condition list semantics에 맞춰 request body read 전에 `412 Precondition Failed`로 처리한다.

## CalDAV calendar-timezone VTIMEZONE 응답 정합성 (2026-05-12, complete)
- `calendar-timezone` property 응답이 저장된 Olson TZID를 클라이언트용 `VCALENDAR`/`VTIMEZONE`
  payload로 직렬화하도록 보강했다.
- timezone service의 `X-WR-CALDESC` 및 `X-PUBLISHED-LL` calendar properties가
  `END:VCALENDAR` 뒤가 아니라 iCalendar body 내부에 위치하도록 수정했다.
- 저장소 canonical 값은 기존 TZID 형식을 유지해 `time.LoadLocation` 기반 query/freebusy 동작과 write path를 보존한다.

## CalDAV sync-collection delta duplicate coalescing (2026-05-12, complete)
- CalDAV `sync-collection` delta 경로가 같은 object href의 반복 변경을 최신 변경 하나로 coalescing한 뒤
  `nresults` limit을 판단하도록 정리했다.
- joined change+object fast path와 fallback change-list path 모두 동일한 coalescing을 적용하며,
  collection-only changes는 sync-token 갱신에는 반영하되 object response count에서는 제외한다.
- raw change stream이 WebDAV report 최대치를 넘는 경우에는 기존 truncation precondition을 유지한다.

## CalDAV calendar-query time-range 후보 인덱스 (2026-05-12, complete)
- time-range `calendar-query`가 요청 component가 있을 때 전체 캘린더 객체 스캔 대신
  component 후보 walker를 우선 사용하도록 했다.
- repository candidate walker는 `user_id`, `calendar_id`, `status`, `component_type` scope를 적용해
  기존 component 인덱스 후보만 ICS 본문과 함께 스트리밍한다.
- 후보는 기존 `CalendarObjectMatchesTimeRange`로 다시 검증하므로 recurrence/timezone/VTODO 판정과
  time-range 이후 `nresults` truncation 정합성을 유지한다.

## CardDAV addressbook-query indexed candidate path (2026-05-12, complete)
- `addressbook-query`가 안전한 positive ASCII `text-match` 필터를 발견하면 기존 broad object walker보다
  indexed candidate walker를 우선 사용하도록 했다.
- repository candidate walker는 `user_id`, `addressbook_id`, `status='active'` scope와
  기존 `lower(vcard::text)` trigram index가 활용 가능한 `LIKE` 후보 검색을 결합한다.
- SQL 후보는 최종 응답 전에 기존 `contactObjectMatchesFilter`로 다시 검증하므로
  RFC 6352/vCard property-specific 필터 정합성은 유지하면서 불필요한 전체 vCard 파싱을 줄인다.

## CalDAV sync-collection delta truncation 정합성 (2026-05-12, complete)
- CalDAV `sync-collection` 증분 변경(delta) 경로가 limit 초과 시 일반 `400` 텍스트 오류 대신
  snapshot truncation과 같은 RFC 6578/WebDAV XML precondition 응답으로 매핑되도록 정리했다.
- joined change+object fast path와 fallback change-list path 모두 `TruncatedResultsError`를 사용한다.

## CalDAV calendar slug/timezone write-through (2026-05-12, complete)
- `MKCALENDAR`가 parsed `calendar-timezone` 값을 repository 생성 요청으로 전달하도록 보강했다.
- `PROPPATCH`가 parsed `calendar-slug` 및 `calendar-timezone` 값을 repository 업데이트 요청으로 전달하고,
  multistatus 응답에 갱신된 slug/timezone 속성을 반환하도록 했다.
- WebDAV XML serializer에 Apple iCalendar namespace prefix(`I`)를 추가해 `calendar-slug` 응답 직렬화를 지원한다.

## CardDAV write/delete 락 경합 축소 (2026-05-12, complete)
- CardDAV contact upsert/delete와 address-book delete/proppatch 경로에 bounded retry/backoff를 추가해
  serialization failure, deadlock, lock contention 상황에서 동일 작업의 성공률을 높였다.
- contact upsert/delete에서 주소록 row `FOR UPDATE` 선잠금과 UID 중복 선조회를 제거하고,
  active UID/name unique index 오류 매핑으로 수렴했다.
- contact/object 및 address-book 조건부 ETag 검증에서 불필요한 `FOR UPDATE`를 제거하고,
  sync marker 보장을 CTE 단일 쿼리로 정리했다.

## CardDAV addressbook-multiget 배치 조회 고도화 (2026-05-12, complete)
- `addressbook-multiget`에서 href마다 `LookupContactObject`를 반복하던 경로를
  주소록+객체명 그룹 기반 배치 조회로 정리했다.
- repository에 `ListContactObjectsByNameGroups`를 추가해 `VALUES` 기반 단일 조회와 256개 청크 분할을 적용했다.
- WebDAV multistatus 응답의 원 요청 순서, duplicate href 응답, missing href `404` 규칙은 유지한다.

## CalDAV calendar-query time-range limit 정합성 (2026-05-12, complete)
- `calendar-query` time-range 필터가 있을 때 객체 조회 limit을 필터 전에 적용해
  matching 객체를 놓치거나 잘못된 truncation을 반환할 수 있던 경로를 수정했다.
- time-range 쿼리는 iCalendar 본문 기반 필터를 먼저 통과한 최종 응답에 대해 limit 초과를 판단하므로
  RFC 4791 결과 제한 의미에 더 가깝게 동작한다.
- 선행 non-match 객체와 후행 match 객체, `nresults=1` 조합을 회귀 테스트로 고정했다.

## CardDAV sync-collection 고속화/RFC truncation 정합성 (2026-05-12, complete)
- `internal/carddavgw`의 증분 `sync-collection` 경로에서 변경 목록을 읽은 뒤 변경 객체마다
  `LookupContactObject`를 반복하던 N+1 조회를 `ListAddressBookChangesWithObjectsSince` 조인 경로로 제거했다.
- 증분 변경 응답이 제한을 초과할 때 일반 `400` 텍스트 오류 대신 기존 snapshot truncation과 같은
  RFC 6578/WebDAV `number-of-matches-within-limits` XML precondition 응답(`403`)으로 매핑되도록 정리했다.
- `migrations/0096_carddav_sync_changes_covering_index.sql`를 추가해 sync marker 조회와
  사용자+주소록+증분 id 조회가 커버링 인덱스를 타도록 했다.

## CalDAV 커밋 경계 복구 (2026-05-12, complete)
- 최신 CalDAV 성능 커밋에서 워킹트리에 남아 있던 `CalendarChangeWithObject` 타입과
  `CalendarObjectPropertiesWithPrincipalPath` 응답 헬퍼를 함께 정리해 clean checkout 컴파일 경계를 복구했다.
- CalDAV 성능 고도화 문서에 반영된 0090-0094 인덱스 마이그레이션 파일을 추적 대상에 포함해
  신규 환경에서도 메타데이터/배치 조회/sync-change 최적화 인덱스가 적용되도록 정리했다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- `internal/caldavgw/repository.go`의 `UpsertObject`에서 `If-Match` 조건부 경합 창을 줄이기 위해
  선행 존재/ETag 검증 없이 `INSERT ... SELECT ... ON CONFLICT ... DO UPDATE` 단일 문장으로
  조건부 쓰기를 수행하도록 정리했다.
- `servePutObject`는 레이스 조건에서 `upsert`가 0행 반환되는 경우에도
  조건부 요청이면 `412 Precondition Failed`로 일관되게 매핑한다.
- 업서트 실패 매핑에 `CalDAV object not found` 분기를 추가해 경합에서의 예외 처리 판정을 명확화했다.
- 기존 메타데이터 우선 조회/재시도(backoff), RFC 4791/4918 조건부 규칙은 유지되며
  업서트 경합 구간만 추가 최적화해 동시 처리량을 한 단계 끌어올렸다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- TASK-176: `internal/caldavgw/handler.go`에서 객체 PUT/DELETE의 조건부 판정을
  `LookupCalendarObjectMetadata` 기반으로 우선 변경해 ICS 본문 로딩을 줄였다.
- `repository.go`의 `UpsertObject`, `DeleteObject`에 적응형 재시도 래퍼를 추가해
  동시성 경합 에러(Serialization/Deadlock/Lock wait) 발생 시 지수 백오프로 복구하도록 했다.
- 재시도 시에는 변경/동기화 marker 보장과 sync-token 갱신을 동일 트랜잭션 안에서 유지해
  RFC 4918/4791 조건부 동작(412/404 규약, etag/if-* 헤더 처리)을 보존했다.
- `repository.go`의 `DeleteCalendar`, `UpdateCalendarProperties`도 동일한
  `runCalDAVWriteWithRetry` 재시도 래퍼를 적용해 캘린더 컬렉션 쓰기 경로의 동시성 회복력을 강화했다.
- `serveGetObject`는 `HEAD`/조건부 요청에서 메타데이터 우선 조회로 304/412 판정을 먼저 처리하고,
  본문은 200 응답이 필요한 경우에만 최종 한 번 읽도록 조정했다.
- `propfind`에서 객체 리소스 응답이 `calendar-data`를 요청하지 않는 경우 `calendar-data`를 건너뛰고
  메타데이터 조회만 수행해 객체 조회 I/O를 추가로 절감했다.
- 문서(`docs/ACTIVE_TASK.md`, `docs/CURRENT_STATUS.md`)를 최신 상태로 정리했다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- `internal/caldavgw/repository.go`에서 CalDAV 쓰기 경로의 불필요한 직렬화 잠금 경합을 제거해 동시 처리량 개선을 진행했다.
  - `UpsertObject`/`DeleteObject`/`UpdateCalendarProperties`에서 `lockActiveCalendar` 호출을 제거했다.
  - `ensureCalendarObjectUIDAvailable` 선체크 쿼리를 제거해 UID 중복은 업서트 제약 위반 단일 경로로 처리한다.
  - `ensureCalendarSyncMarker`를 `WITH active_calendar ... INSERT` CTE 단일 쿼리로 정리해 marker 존재성 + 삽입을 한 번에 처리한다.
  - `ensureCalendarCollectionETag`가 필요 없는 캘린더 컬럼을 더 이상 읽지 않도록 최적화해 동시 etag 검증 경로의 I/O를 줄였다.
- 변경은 RFC 4791 동기화/검증 동작, 404/412 규칙, 기존 에러 메시지 의미를 유지한다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- `internal/caldavgw/types.go`와 `internal/caldavgw/handler.go`에서 `sync-collection` 증분 처리용
  `ListCalendarChangesWithObjectsSince` 경로를 추가해 변경 레코드 조회와 객체 조회를 단일 쿼리로 결합했다.
- `internal/caldavgw/repository.go`가 `sync_changes` 마커를 검증한 뒤 변경 집합을
  객체 조인(`object_name` + `status='active'`)과 함께 한 번에 조회하도록 최적화했다.
- `calendar-data` 포함 여부에 따라 ICS를 동적 선택해 필요 없는 본문 로드를 줄여 메모리/IO 비용을 낮췄다.
- `sync-collection`에서 객체 삭제/갱신 토큰 갱신 규칙과 `sync-collection` truncation, invalid token 동작은 유지했다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- `internal/caldavgw/repository.go`에서 객체명 배치 조회를 256개 단위 청크 분할로 안정화해
  극단적으로 큰 `calendar-multiget`/`sync-collection` 입력에서도 SQL 파라미터 폭주를 방지했다.
- `ListCalendarObjectsByNameGroups`와 `ListCalendarObjectsByNames`를 공통 튜플-`VALUES` 경로로 정렬해
  조회 플랜 분기와 스캔 경로를 단일화했다.
- `calendar-query` 컴포넌트 필터를 루프 내 반복 정규화하던 경로에서 개선해
  반복 문자열 정규화 비용을 제거했다.
- `migrations/0094_caldav_calendar_sync_changes_covering_index.sql`를 추가해
  `ListCalendarChangesSince`에서 `sync-changes` 인덱스 커버리지 효율을 강화했다.
- RFC 4791 동작(삭제 객체 `404`, 조건부 규약, 다중 캘린더 조회 정합성)은 기존 동작을 유지했다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- `internal/caldavgw/handler.go`의 `calendar-query` 경로에서 컴포넌트 필터를
  리포지토리 레벨에서 선처리하도록 `CalendarObjectComponentStore` 플로우를 완성했다.
- `internal/caldavgw/repository.go`에 `ListCalendarObjectsByComponentLimit(...)`를 추가해
  사용자+캘린더+상태+컴포넌트 조건과 `limit`을 한 번의 SQL로 처리하고 정렬/마감 갯수를 DB에서 수행한다.
- `calendar-query` 응답 조립 시, 스토어가 컴포넌트 필터를 반영한 경우 핸들러의 중복 `component` 비교를 건너뛰도록 분기해
  객체당 후처리 비용을 감소했다.
- `migrations/0095_caldav_calendar_object_performance_index.sql`를 추가해
  `calendar-query` 및 목록 조회의 `updated_at DESC, id DESC` 패턴을 지원하는 커버링 인덱스를 강화하고
  정렬-스캔 비용을 낮췄다.
- RFC 4791 동작(`calendar-query` truncation, 삭제/조건 처리)은 기존 규약을 유지한다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- 크로스 캘린더 객체 일괄 조회 경로를 `internal/caldavgw/repository.go`에 구현해 `calendar-multiget`/`sync-collection`/`calendar-query`에서 캘린더별 반복 객체조회 호출을 줄였다.
- `CalendarObjectCrossCalendarBatchStore`용 `ListCalendarObjectsByNameGroups`를 추가하고 `lookupCalendarObjectsByNames`에서 단일 캘린더 수만 있는 경우는 기존 경로, 다수 캘린더는 크로스-캘린더 단일 SQL로 분기한다.
- 동기화/객체 읽기 경로의 스캔 버퍼를 제한값 기반 미리 할당으로 최적화해 작은 단위 할당을 줄였다.
- 바운티 조회용 인덱스를 `migrations/0093_caldav_calendar_object_lookup_index_v2.sql`로 추가해 `(user_id, status, calendar_id, object_name)` 기준 검색 계획의 안정성을 강화했다.
- RFC 동작(삭제 응답/권한/반복 요청 규칙)은 기존 동작을 유지했다.

## CalDAV 성능 고도화 (2026-05-12, complete)
- `calendar-multiget`와 `sync-collection`에서 객체명 목록 기반 배치 조회 경로를 `internal/caldavgw/handler.go`로 통합해
  N+1 조회를 제거했다.
- `internal/caldavgw/repository.go`에 `ListCalendarObjectsByNames`를 추가해, 요청 단위로 `user_id + calendar_id + status + object_name` 필터를 한 번의 SQL IN 조회로 처리한다.
- `calendar-multiget`은 잘못된 href를 `404` propstat으로 보존하고, 허용된 결과는 기존 `calendar-data`/메타데이터 응답 규칙을 그대로 유지한다.
- `sync-collection`은 `object-deleted`를 그대로 `404` 응답으로 내보내고, 활성 객체만 배치 조회해 불필요 조회를 줄인다.
- `migrations/0091_caldav_calendar_object_lookup_index.sql`에 객체명 일괄 조회를 위한 인덱스를 추가해 동기화 응답 조회 계획 개선을 준비했다.
- `ListCalendarChangesSince` 조회를 marker 조회 + 단일 증분조회 2단계로 정리해 `JOIN marker` 경로와 동등한 시그니처의 존재성 검사 쿼리를 제거하고, 동일한 RFC 동작을 유지했다.
- `migrations/0092_caldav_calendar_sync_changes_index.sql`를 추가해 sync 토큰 탐색·변경 조회 경로의 인덱스 커버리지를 확장했다.

## Drive drag-and-drop / folder upload (2026-05-12, complete)
- Added frontend drag-and-drop support in `apps/webmail/src/components/DriveView.tsx`:
  - Node cards now support moving files/folders by dragging to a folder card.
  - Folder cards expose drag target highlighting during internal move operations.
  - Browser-drop uploads now parse `DataTransferItem` entries and handle recursive directory traversal.
  - Relative paths from dropped folders are kept to create intermediate folders before file upload.
- Added `moveDriveNode` helper in `apps/webmail/src/lib/api.ts` to call backend move endpoint.
- Folder path resolution now checks existing folders from backend before creating intermediates, preventing duplicate
  folders during recursive directory uploads.
- Existing design language and toolbar flow is preserved (no visual theme rework).
- Drag-over upload/drop target behavior and folder move/upload paths are now aligned for in-place DnD usage.
- Added multi-selection drag support (Ctrl/Cmd + click) and multi-node move payload handling so selected nodes can be moved together into folders.
- Hardening drag payload parsing (JSON + legacy text fallbacks) to improve reliability for file-card to-folder move flows across MIME/UX permutations.

## CalDAV 성능 최적화 (2026-05-12, complete)
- Optimized `Repository` lookup path in `internal/caldavgw/repository.go` to avoid loading `ics` for metadata-only operations.
- Added metadata-only list/single-object accessors (`ListCalendarObjectMetadataLimit`, `LookupCalendarObjectMetadata`) and wired `internal/caldavgw/handler.go` to use them for CALDAV `PROPFIND`, `calendar-query`, `calendar-multiget`, and `sync-collection` paths when `calendar-data` is not requested.
- Added covering index migration `0090_caldav_calendar_object_metadata_index.sql` targeting metadata-heavy reads on active calendar objects (ordered by `updated_at`, `id`) to reduce TOAST fetches and improve sort/index-only scan behavior.
- Design direction remains unchanged; this change is backend-only and RFC-compliant with existing RFC 4791/RFC 4918 behavior.

## Drive sidebar hierarchical folders (2026-05-12, in progress)
- Added left-side folder tree under `내 드라이브` in `apps/webmail/src/components/DriveView.tsx`.
- Folder rows in the sidebar now load their child folders and can be expanded/collapsed recursively.
- Folder rows support drag-and-drop targets so external uploads and internal node moves can land directly in a target folder from the left panel.
- Root `내 드라이브` entry accepts dropped files/folders from browser drag and drop.

## Mail sidebar personal folder hierarchy/spatial polish (2026-05-12, complete)
- Adjusted `개인 편지함` section spacing in `apps/webmail/src/components/Sidebar.tsx` so newly added personal folders no longer appear overly indented.
- Enabled nested personal mailbox creation by wiring parent folder selection into `Sidebar` and `/api/v1/folders` creation.
- Added folder-tree rendering for `개인 편지함`, preserving existing visual density and making subfolders discoverable.
- Reduced left/right padding for personal folder rows and create action to keep list density consistent with virtual/system sections.

## Drive upload contract fix (2026-05-12, in progress)
- Fixed Drive upload session contract mismatch in webmail frontend (`apps/webmail/src/lib/api.ts`):
  - `size` -> `declared_size`
  - `storage_backend` now provided (fallback attempts: `local`, `s3`, `minio`)
  - `X-Content-SHA256` checksum header now included for body upload
  - session response key corrected to `drive_upload_session`
  - finalize request switched to `/api/mail/drive/upload-sessions/{id}/finalize` without extra body
- Fixed backend `insertDriveFileNode` argument mismatch in `internal/drive/repository.go` (removed extra argument in file insert), which was returning `expected 9 arguments, got 10`.
- Verification: real file from `/Users/Downloads` upload succeeded through `POST /api/v1/drive/upload-sessions`, `PUT /body`, `POST /finalize`; result node appears in `GET /api/v1/drive/nodes`.

## Webmail drive file type icon refresh (2026-05-12, complete)
- Added shared icon resolver for drive nodes: `apps/webmail/src/lib/driveNodeIcon.tsx`.
- Icons now resolve by MIME + filename extension and use Untitled UI official file icons (`@untitledui/file-icons`) with consistent visual language.
- `apps/webmail/src/components/DriveView.tsx` and `apps/webmail/src/components/ComposeModal.tsx` now render the shared `DriveNodeIcon` for file/folder entries, including drive picker rows.
- Existing drive card/list layout and palette are unchanged; only icon glyphs were updated to UntitledUI style.

## Webmail address book recipient display stabilization (TASK-167, 2026-05-12, complete)
- Compose recipient picker address-book tab now mirrors the organization picker flow: selecting an address book shows the address-book send token and its actual contacts together.
- Fixed frontend vCard parsing for standard `FN:value` and `EMAIL:value` properties, including parameterized forms such as `EMAIL;TYPE=WORK:value`.
- The contacts app and compose picker now use parsed contact names/emails instead of falling back to `.vcf` object filenames when seed contacts are valid.
- Address-book empty state no longer hides the group-send row in the compose picker.
- Design direction remains unchanged.

## Webmail beta stabilization (TASK-098, 2026-05-12, complete)
- Goal: prepare user webmail for beta service while preserving the current visual design direction.
- Completed first priority: API base-path stabilization between the Next.js webmail proxy and Go backend routes.
- The backend intentionally has both older `/api/v1/...` Mail API routes and newer `/api/mail/...` CardDAV/Directory routes.
- Webmail proxy must route feature areas explicitly instead of assuming a single backend base path.
- Address books, contacts, and directory/org-tree requests now reach backend `/api/mail/...`; existing message/folder/search Mail API requests continue reaching `/api/v1/...`.
- OrgPickerModal now defaults to the user's actual organization and expands its parent chain, without changing visual design.
- Developer beta seed data is available in `scripts/seed_dev_data.sql`, with a Docker-friendly runner in `scripts/seed_dev_beta.sh`.
- The seed covers Korean users, primary addresses, system folders, hierarchical organizations, CardDAV contacts, and mailbox messages for webmail/admin console smoke testing.
- Development database target: `gogomail-postgres-dev` container from `docker/docker-compose.dev.yml`.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail core flow stabilization (TASK-099, 2026-05-12, complete)
- Completed first beta stabilization pass for the core user mail screen.
- The mail-list hook now avoids calling message-list APIs before a real folder is selected.
- Virtual folders are guarded from regular folder pagination, polling, and refresh calls; their data remains loaded through the search/virtual-folder path.
- Design direction remains unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail compose contract stabilization (TASK-100, 2026-05-12, complete)
- Completed compose/send/draft contract pass for user webmail beta.
- The UI still supports reply-all as a user action, but outbound and draft payloads now normalize `reply_all` to backend compose intent `reply`, preserving the existing backend contract (`new`, `reply`, `forward`).
- Draft autosave, manual save, close-save, and send payloads now share the same backend intent normalization path.
- Draft autosave/manual save now parse comma-separated recipient fields with the same address parser used for send payloads, instead of sending the whole field as one invalid address.
- Design direction remains unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail attachment draft contract stabilization (TASK-101, 2026-05-12, complete)
- Completed attachment/Drive attachment contract pass for user webmail beta.
- Draft payload typing now exposes `attachment_ids`, matching the backend draft contract.
- Compose autosave, manual save, and close-save paths now include only upload-complete, non-failed attachment IDs in draft payloads.
- Send payload attachment behavior is preserved.
- Design direction remains unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail attachment send-state stabilization (TASK-102, 2026-05-12, complete)
- Completed stricter attachment send-state pass for user webmail beta.
- Webmail now prevents sending while any attachment is still uploading, with a clear user-facing error.
- Webmail now prevents sending while any attachment has failed upload, requiring remove/retry instead of silently omitting the file.
- Completed attachments remain the only attachment IDs included in the send payload.
- Design direction remains unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail attachment retry stabilization (TASK-103, 2026-05-12, complete)
- Completed local attachment upload retry pass for user webmail beta.
- Failed local file attachments now retain the source `File` object for retry.
- Failed attachment chips expose a compact retry action without changing the current compose visual direction.
- Retry success replaces the temporary failed attachment with the backend attachment ID, so draft/send persistence continues using the standard `attachment_ids` contract.
- Retry failure keeps the failed state visible and blocks send through the TASK-102 guard.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail attachment pending-send stabilization (TASK-104, 2026-05-12, complete)
- Completed pending-send attachment state pass for user webmail beta.
- Undo-countdown sends now cancel if attachment upload/failure/removal state changes after the pending payload is created.
- Cancellation clears the pending payload and tells the user to re-check attachments before sending again.
- Immediate and scheduled sends keep the existing attachment readiness guard.
- Design direction remains unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail compose duplicate-send stabilization (TASK-105, 2026-05-12, complete)
- Completed duplicate-send guard pass for user webmail beta.
- `handleSend` now ignores duplicate calls while a send is in progress or already completed.
- `handleSend` now blocks duplicate sends during the undo countdown and tells the user to cancel before sending again.
- Existing immediate, scheduled, and undo-countdown send behavior is preserved.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail recipient validation stabilization (TASK-106, 2026-05-12, complete)
- Completed recipient address validation pass for user webmail beta.
- To/Cc/Bcc addresses are validated at send time before payload creation proceeds.
- Malformed addresses block send with a clear Korean error listing the problematic values.
- Display-name recipient syntax such as `Name <addr@example.com>` remains supported through the existing parser.
- Draft save flexibility is preserved; validation is send-only.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail scheduled-send validation stabilization (TASK-107, 2026-05-12, complete)
- Completed scheduled-send time validation pass for user webmail beta.
- `handleSend` now blocks invalid scheduled-send dates before payload creation.
- `handleSend` now blocks scheduled-send times that are not in the future.
- Valid scheduled sends continue to serialize `scheduled_at` as ISO timestamps.
- Immediate and undo-countdown sends are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail scheduled-send UI stabilization (TASK-108, 2026-05-12, complete)
- Completed scheduled-send UI state pass for user webmail beta.
- Opening custom schedule now fills a default future time when no schedule exists.
- Scheduled state can now be explicitly cleared from both custom and preset scheduled states.
- Clearing schedule resets both `scheduledAt` and the custom input display state.
- Existing preset scheduled-send behavior is preserved.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail sent-draft cleanup stabilization (TASK-109, 2026-05-12, complete)
- Completed sent-draft cleanup pass for user webmail beta.
- Webmail API helpers now expose `deleteDraft` for the backend `DELETE /drafts/{id}` contract.
- Successful immediate, scheduled, and undo-countdown sends now attempt to delete the current autosaved draft.
- Draft cleanup is best-effort; cleanup failure does not turn a successful send into a failed send.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft-send contract stabilization (TASK-110, 2026-05-12, complete)
- Completed draft-send backend contract pass for user webmail beta.
- Webmail API helpers now expose `sendDraft` for backend `POST /drafts/{id}/send`.
- Non-scheduled, non-tracked sends now persist the latest compose state to draft first, then use the backend draft-send contract.
- Scheduled sends and open-tracked sends keep direct send because the current draft-send contract does not represent `scheduled_at` or `track_opens`.
- Undo-countdown sends preserve whether they should use draft-send or direct-send after the countdown expires.
- Draft-send success clears local draft state without a redundant best-effort delete call because the backend marks the draft sent.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft-send tracking parity (TASK-111, 2026-05-12, complete)
- Completed draft-send tracking parity pass for user webmail beta.
- Draft save contracts now carry `track_opens`.
- Draft persistence stores `track_opens` in message flags and `GetDraftForSend` restores it.
- `SendDraft` now forwards saved `TrackOpens` into `SendText`.
- Webmail draft payloads now include `track_opens`, and non-scheduled tracked sends can use the backend draft-send contract.
- Scheduled sends still keep direct send until the draft contract can represent `scheduled_at`.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft-send scheduled parity (TASK-112, 2026-05-12, complete)
- Completed draft-send scheduled parity pass for user webmail beta.
- Draft save contracts now carry `scheduled_at`.
- Draft persistence stores scheduled send time in message flags and `GetDraftForSend` restores it.
- `SendDraft` now forwards saved `ScheduledAt` into `SendText`.
- Webmail draft payloads now include `scheduled_at`, and scheduled sends use the backend draft-send contract.
- Existing scheduled time validation and schedule UI clearing behavior are preserved.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft-send failure recovery (TASK-113, 2026-05-12, complete)
- Completed draft-send failure recovery pass for user webmail beta.
- Preparing a send now marks the freshly saved draft as saved in the compose UI.
- Draft-send/direct-send failures clear pending draft-send state.
- Send failure messages now tell the user that the draft is preserved and can be retried.
- Successful send cleanup behavior is unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft settings persistence (TASK-114, 2026-05-12, complete)
- Completed draft settings persistence pass for user webmail beta.
- Autosave now includes `track_opens` and `scheduled_at` in draft payloads.
- Manual save now includes `track_opens` and `scheduled_at` in draft payloads.
- Close-confirm save now includes `track_opens` and `scheduled_at` in draft payloads.
- Final send-preparation draft save behavior is preserved.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft payload helper cleanup (TASK-115, 2026-05-12, complete)
- Completed draft payload helper cleanup for user webmail beta.
- ComposeModal now builds draft payloads through a single `buildDraftData` helper.
- Autosave, manual save, close-confirm save, and send-preparation save all use the same draft contract path.
- `attachment_ids`, `track_opens`, `scheduled_at`, and `from` are preserved consistently across draft saves.
- The close-confirm draft path no longer includes the non-contract `html_body` field.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft-send HTTP contract tests (TASK-116, 2026-05-12, complete)
- Completed HTTP boundary test pass for draft-save and draft-send beta contracts.
- Draft save handler tests now verify `track_opens` reaches the service request.
- Draft save handler tests now verify `scheduled_at` reaches the service request.
- Draft-send handler tests now verify normalized send statuses in the HTTP response.
- Draft-send handler tests now reject unexpected request bodies and unknown query keys before dispatch.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft-send OpenAPI contract (TASK-117, 2026-05-12, complete)
- Completed OpenAPI contract pass for draft-save and draft-send beta contracts.
- `ComposeRequest` now documents `track_opens` and explains draft-save preservation for draft-send.
- `ComposeRequest` keeps `scheduled_at` documented as a preserved draft-send option.
- OpenAPI contract tests now verify `DraftSave` reuses `ComposeRequest`.
- OpenAPI contract tests now verify `POST /drafts/{id}/send` remains bodyless.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail draft scheduled/tracking DB integration (TASK-118, 2026-05-12, complete)
- Completed PostgreSQL integration coverage for draft `track_opens` and `scheduled_at` persistence.
- `TestPostgresDraftToSendMovesAttachmentsAndQueuesOutbox` now verifies `TrackOpens` survives `SaveDraft` → `GetDraftForSend`.
- The same integration test now verifies `ScheduledAt` survives `SaveDraft` → `GetDraftForSend`.
- Existing draft attachment handoff and outbox queue assertions remain intact.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send result API typing (TASK-119, 2026-05-12, complete)
- Completed frontend API typing pass for send and draft-send responses.
- Added shared `SendMessageResult` and `SendMessageEnvelope` types.
- `sendDraft` now exposes `message_id`, `send_status`, `delivery_status`, and `bounce_status`.
- `sendMessage` now returns the same typed envelope.
- Existing compose call sites continue to ignore the result, preserving behavior while keeping the API contract available for future UI.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send result UI preparation (TASK-120, 2026-05-12, complete)
- Completed compose send-result UI preparation for user webmail beta.
- ComposeModal now stores the backend send result after successful direct or draft sends.
- Immediate, scheduled, and undo-countdown send success paths all preserve the result through the same helper.
- The compose footer now shows a compact initial send/delivery status after success while preserving the existing close timing and visual tone.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send success local-state cleanup (TASK-121, 2026-05-12, complete)
- Completed send success local-state cleanup for user webmail beta.
- Recent-recipient and follow-up local-state updates now live in `persistSuccessfulSendLocalState`.
- Undo-countdown, immediate, and scheduled send success paths all call the same helper.
- Send result preservation, draft cleanup, and close timing are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send success handler consolidation (TASK-122, 2026-05-12, complete)
- Completed send success handler consolidation for user webmail beta.
- ComposeModal now routes send-result storage, local-state updates, draft cleanup, sent state, archive-after-send, and close timing through `handleSuccessfulSend`.
- Undo-countdown, immediate, and scheduled send success paths all use the same helper.
- Existing close timing and visual behavior are preserved.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send failure handler consolidation (TASK-123, 2026-05-12, complete)
- Completed send failure handler consolidation for user webmail beta.
- ComposeModal now routes send failure messaging and pending draft-send cleanup through `handleSendFailure`.
- Undo-countdown failures clear the countdown through the same helper.
- Immediate and scheduled send failures use the same draft-preserved retry message.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send preparation failure messaging (TASK-124, 2026-05-12, complete)
- Completed send preparation failure cleanup for user webmail beta.
- ComposeModal now routes draft-save/send-preparation failures through `handleSendPreparationFailure`.
- Preparation failures clear pending message and draft-send state because the send has not started.
- Preparation failure copy now tells the user to verify and retry saving or sending, distinct from actual send failure copy.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send dispatch helper cleanup (TASK-125, 2026-05-12, complete)
- Completed compose send dispatch readability cleanup for user webmail beta.
- ComposeModal now names the saved-draft eligibility check through `shouldSendSavedDraft`.
- ComposeModal now dispatches prepared sends through `sendPreparedMessage`, keeping draft-send/direct-send selection in one place.
- Undo-countdown, scheduled, and immediate send paths now share the same dispatch helper while preserving existing success, failure, and draft cleanup behavior.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send result user guidance (TASK-126, 2026-05-12, complete)
- Completed send-result status copy improvement for user webmail beta.
- ComposeModal now translates send status values such as `queued`, `scheduled`, `sent`, and `failed` into user-facing Korean guidance.
- Delivery status values such as `pending`, `deferred`, `delivered`, and `failed` are also shown as user-facing Korean guidance.
- Bounce or complaint state is appended only when the backend reports a meaningful non-empty bounce status.
- Existing backend response contracts, success timing, and visual design direction are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send result display extraction (TASK-127, 2026-05-12, complete)
- Completed send-result display logic extraction for user webmail beta.
- Send, delivery, and bounce status label mapping now live in focused pure helpers.
- The final footer label is assembled by `formatSendResultLabel`, keeping ComposeModal's render body simpler.
- Existing user-facing copy, display conditions, success timing, and backend contracts are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send result contract fixture (TASK-128, 2026-05-12, complete)
- Completed send-result type/display contract fixture pass for user webmail beta.
- Send result label helpers now live in `apps/webmail/src/lib/sendResultLabel.ts` as a React-independent module.
- ComposeModal imports the shared formatter instead of owning the mapping logic.
- `apps/webmail/src/lib/sendResultLabel.contract.ts` provides `tsc`-checked samples for queued/pending, bounced, unknown fallback, and null-result formatting.
- No new frontend test runner was introduced; the existing `pnpm type-check` loop now covers the formatter contract fixture.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail scheduled-send success copy (TASK-129, 2026-05-12, complete)
- Completed scheduled-send success button copy cleanup for user webmail beta.
- ComposeModal now shows `예약됨` after a scheduled send succeeds, instead of implying immediate delivery with the generic `전송됨` label.
- Immediate send success still shows the existing `전송됨` label.
- Existing send APIs, success timing, and visual design direction are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send button label helper (TASK-130, 2026-05-12, complete)
- Completed compose send button label calculation cleanup for user webmail beta.
- Send button copy now comes from `composeSendButtonLabel`, covering sending, sent, scheduled-sent, uploading, scheduled-ready, and default states.
- ComposeModal now computes upload/disabled state once for the send button instead of repeating the same attachment check in JSX.
- `apps/webmail/src/lib/composeSendButtonLabel.contract.ts` adds `tsc`-checked label state fixtures.
- Existing button design, disabled behavior, and send flow are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send button accessibility state (TASK-131, 2026-05-12, complete)
- Completed compose send button accessibility state pass for user webmail beta.
- The primary send button now exposes its computed label as the accessibility name.
- Sending/uploading progress is exposed through `aria-busy`.
- Send-result and draft-save status messages now use polite live status regions.
- The send-options trigger exposes menu popup and expanded state.
- Existing visual design and send behavior are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send options menu accessibility (TASK-132, 2026-05-12, complete)
- Completed send-options menu item accessibility pass for user webmail beta.
- The send-options trigger now references the opened menu through `aria-controls`.
- The dropdown menu has a stable id and `role=menu`.
- Scheduled-send choices, send-and-archive, and custom-date actions now expose `role=menuitem` and descriptive accessibility names.
- Existing visual design and click behavior are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send options keyboard close (TASK-133, 2026-05-12, complete)
- Completed send-options keyboard close pass for user webmail beta.
- The open send-options menu now closes on `Escape`.
- Escape handling stops propagation so it does not accidentally trigger higher-level compose shortcuts.
- Existing menu item click behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send options outside-click close (TASK-134, 2026-05-12, complete)
- Completed send-options outside-click close pass for user webmail beta.
- The send-options wrapper now has a stable ref used for outside-click detection.
- While the menu is open, document `mousedown` outside the send-options wrapper closes it.
- Inside-menu clicks keep their existing selection behavior.
- The outside-click listener is installed only while the menu is open and removed on cleanup.
- Existing visual design is unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail send options close helper (TASK-135, 2026-05-12, complete)
- Completed send-options close logic consolidation for user webmail beta.
- ComposeModal now names the close intent through `closeSendDropdown`.
- Outside-click, Escape, scheduled-option selection, send-and-archive, and custom-date selection paths all use the shared helper.
- The existing send-options toggle behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail custom schedule guidance (TASK-136, 2026-05-12, complete)
- Completed custom scheduled-send guidance pass for user webmail beta.
- The custom datetime input now exposes an accessibility label for the scheduled send time.
- The input is connected to helper copy through `aria-describedby`.
- The UI now explicitly tells users that scheduled sends must be set after the current time.
- Existing input `min` behavior, send-time validation, and visual design tone are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail schedule guidance constant (TASK-137, 2026-05-12, complete)
- Completed custom schedule guidance constant cleanup for user webmail beta.
- The schedule input helper copy now lives in a module-level `SCHEDULE_INPUT_HELP` constant.
- Existing displayed copy, accessibility linkage, and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail local schedule datetime formatting (TASK-138, 2026-05-12, complete)
- Completed scheduled-send datetime-local formatting cleanup for user webmail beta.
- ComposeModal now formats preset, default custom, and minimum scheduled-send times through `toDateTimeLocalValue`.
- The helper uses local date/time fields instead of UTC `toISOString().slice(0, 16)`, matching HTML `datetime-local` semantics.
- The custom schedule input now receives a named `scheduleMinDateTime` value.
- Send payload ISO serialization remains unchanged at the API boundary.
- Existing visual design is unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail datetime-local formatter fixture (TASK-139, 2026-05-12, complete)
- Completed datetime-local formatter extraction and fixture pass for user webmail beta.
- The formatter now lives in `apps/webmail/src/lib/dateTimeLocal.ts`.
- ComposeModal imports the shared helper for preset, default custom, and minimum scheduled-send values.
- `apps/webmail/src/lib/dateTimeLocal.contract.ts` adds `tsc`-checked examples for padded one-digit and two-digit date/time values.
- Existing scheduled-send UI behavior is unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail datetime-local runtime check (TASK-140, 2026-05-12, complete)
- Completed datetime-local formatter runtime check pass for user webmail beta.
- Added `apps/webmail/scripts/check-date-time-local.mjs` to assert formatter output at runtime.
- Added `pnpm test:datetime-local` for the webmail app without introducing a new test runner.
- Runtime checks cover one-digit padding and two-digit date/time values.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:datetime-local` in `apps/webmail` passed.

## Webmail close-save scheduled guidance (TASK-141, 2026-05-12, complete)
- Completed close-confirm scheduled draft guidance pass for user webmail beta.
- When a scheduled send time is set, the close confirmation now says the draft will be saved with the schedule setting.
- The unscheduled close confirmation copy remains unchanged.
- The close-save action still uses `buildDraftData`, preserving the existing draft save contract including scheduled settings.
- Existing visual design is unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close-save prompt helper (TASK-142, 2026-05-12, complete)
- Completed close-confirm prompt calculation cleanup for user webmail beta.
- Close-save prompt copy now comes from `composeCloseSavePrompt`.
- `apps/webmail/src/lib/composeCloseSavePrompt.contract.ts` adds `tsc`-checked scheduled and unscheduled prompt fixtures.
- Existing copy and close-save behavior are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail compose helper runtime checks (TASK-143, 2026-05-12, complete)
- Completed compose helper runtime check expansion for user webmail beta.
- The existing runtime script now verifies close-save prompt output for scheduled and unscheduled drafts.
- The script now verifies scheduled-send button copy for ready and successful scheduled-send states.
- Existing datetime-local formatter runtime checks remain covered.
- Added `pnpm test:compose-helpers`; `pnpm test:datetime-local` remains as a compatibility alias.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail compose helper check naming (TASK-144, 2026-05-12, complete)
- Completed compose helper runtime check naming cleanup for user webmail beta.
- Renamed the runtime check script to `apps/webmail/scripts/check-compose-helpers.mjs`.
- `pnpm test:compose-helpers` now points at the renamed script.
- `pnpm test:datetime-local` remains as a compatibility alias.
- Existing runtime assertions are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail compose helper check output (TASK-145, 2026-05-12, complete)
- Completed compose helper runtime check output cleanup for user webmail beta.
- The runtime check now reports that datetime, send button, and close-save helper checks passed.
- Existing assertions and package scripts are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail compose helper verification docs (TASK-146, 2026-05-12, complete)
- Completed webmail verification-loop documentation for compose helpers.
- `docs/WEBMAIL_ROADMAP.md` now lists the webmail beta verification loop:
  `go test ./...`, `pnpm type-check`, and `pnpm test:compose-helpers` when compose helper copy, scheduled datetime formatting, send button labels, or close-save prompts are touched.
- No code behavior changed.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail compose helper alias docs (TASK-147, 2026-05-12, complete)
- Completed compose helper runtime check alias documentation.
- `docs/WEBMAIL_ROADMAP.md` now explains that `pnpm test:datetime-local` remains as a compatibility alias for `pnpm test:compose-helpers`.
- The docs note that the alias remains because the datetime-local formatter was the first helper covered by this runtime script.
- No code behavior changed.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail compose helper alias verification (TASK-148, 2026-05-12, complete)
- Completed compose helper alias execution verification.
- `pnpm test:datetime-local` in `apps/webmail` was run and invoked `pnpm test:compose-helpers`.
- The alias path completed the same compose helper runtime checks successfully.
- No code behavior changed.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:datetime-local` in `apps/webmail` passed.

## Webmail compose helper command naming (TASK-149, 2026-05-12, complete)
- Completed compose helper command naming documentation.
- `docs/WEBMAIL_ROADMAP.md` now identifies `pnpm test:compose-helpers` as the canonical command for compose helper runtime checks.
- `pnpm test:datetime-local` remains documented as a compatibility alias.
- Existing package scripts and code behavior are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail compose helper check scope comment (TASK-150, 2026-05-12, complete)
- Completed compose helper runtime check scope comment cleanup.
- `apps/webmail/scripts/check-compose-helpers.mjs` now states that the assertions cover pure compose helpers vulnerable to copy or datetime formatting regressions.
- Existing assertions and package scripts are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail close confirmation accessibility (TASK-151, 2026-05-12, complete)
- Completed close confirmation accessibility state pass for user webmail beta.
- The inline close confirmation now exposes `role=alertdialog`.
- The confirmation is labelled by its visible prompt through `aria-labelledby`.
- `aria-modal=false` makes the inline, non-modal behavior explicit.
- Existing save/discard/cancel actions and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close confirmation button labels (TASK-152, 2026-05-12, complete)
- Completed close confirmation button accessibility label pass for user webmail beta.
- The save button now exposes an accessibility name that says it will save the draft and close the composer.
- The discard button now exposes an accessibility name that says it will close without saving.
- The cancel button now exposes an accessibility name that says it will keep composing.
- Existing visible text, button behavior, and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close confirmation Escape cancel (TASK-153, 2026-05-12, complete)
- Completed close confirmation keyboard cancel pass for user webmail beta.
- Pressing `Escape` while the close confirmation has focus now dismisses the confirmation and keeps the composer open.
- Escape handling stops propagation so it does not trigger higher-level compose shortcuts.
- Save, discard, cancel button behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close confirmation cancel helper (TASK-154, 2026-05-12, complete)
- Completed close confirmation cancel helper cleanup for user webmail beta.
- ComposeModal now names the cancel intent through `cancelCloseConfirmation`.
- Escape and the cancel button both use the shared helper.
- Save, discard, cancel behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close-save helper extraction (TASK-155, 2026-05-12, complete)
- Completed close-confirm save helper extraction for user webmail beta.
- ComposeModal now names the save-and-close intent through `saveDraftAndClose`.
- The helper still uses `buildDraftData`, preserving scheduled settings, tracking settings, attachment ids, and sender state in the draft payload.
- Close-save remains best-effort: save failure does not block closing, matching the previous behavior.
- The save button now calls the helper while keeping existing visible text, accessibility label, and visual design.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close-discard helper extraction (TASK-156, 2026-05-12, complete)
- Completed close-confirm discard helper extraction for user webmail beta.
- ComposeModal now names the discard-and-close intent through `discardDraftAndClose`.
- The discard button now calls the helper while preserving the existing `onClose` behavior.
- Existing visible text, accessibility label, and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close confirmation action naming (TASK-157, 2026-05-12, complete)
- Completed close confirmation action naming checkpoint for user webmail beta.
- Close-confirm save, discard, and cancel paths are now named as `saveDraftAndClose`, `discardDraftAndClose`, and `cancelCloseConfirmation`.
- These names establish the extension points for any future close-confirm loading, failure, or analytics behavior.
- No code behavior changed in this task.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close-save duplicate guard (TASK-158, 2026-05-12, complete)
- Completed close-confirm save duplicate-click guard for user webmail beta.
- ComposeModal now tracks `closeSaveInProgress` while the close-save action is running.
- `saveDraftAndClose` exits early if a close-save is already in progress.
- The close confirmation exposes `aria-busy` during close-save.
- Save, discard, and cancel actions are disabled while close-save is in progress, and the save button shows `저장 중...`.
- Close-save remains best-effort and still closes after the save attempt.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail close-save button label helper (TASK-159, 2026-05-12, complete)
- Completed close-save button label helper extraction for user webmail beta.
- Added `composeCloseSaveButtonLabel` for idle and saving close-save states.
- ComposeModal now imports the helper for the close-save button text.
- Added `tsc` contract fixtures and runtime assertions for the idle/saving labels.
- Existing close-save behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail close-save button aria label helper (TASK-160, 2026-05-12, complete)
- Completed close-save button accessibility label helper extraction for user webmail beta.
- Added `composeCloseSaveButtonAriaLabel` for idle and saving close-save states.
- ComposeModal now uses the helper for the close-save button accessibility name.
- Added `tsc` contract fixtures and runtime assertions for idle/saving accessibility labels.
- Existing close-save behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail close-save accessibility helper docs (TASK-161, 2026-05-12, complete)
- Completed compose helper verification docs update for close-save accessibility labels.
- `docs/WEBMAIL_ROADMAP.md` now includes close-save button accessibility labels in the `pnpm test:compose-helpers` trigger scope.
- No code behavior changed.
- Verification: `go test ./...` passed; `pnpm type-check` and `pnpm test:compose-helpers` in `apps/webmail` passed.

## Webmail close-save Escape guard (TASK-162, 2026-05-12, complete)
- Completed close-confirm Escape handling guard for user webmail beta.
- While close-save is in progress, Escape no longer dismisses the close confirmation.
- Escape still stops propagation during close-save so higher-level compose shortcuts do not fire.
- When close-save is not in progress, Escape continues to cancel the close confirmation.
- Existing button behavior and visual design are unchanged.
- Verification: `go test ./...` passed; `pnpm type-check` in `apps/webmail` passed.

## Webmail org chart hierarchical data (2026-05-12)
- ✅ Hierarchical organization data loaded in PostgreSQL
- 9 organizations across 3 depth levels:
  - Level 0 (3 본부): 개발본부, 마케팅본부, 경영지원부
  - Level 1 (3 팀): 백엔드팀, 프론트엔드팀, 인프라팀
  - Level 2 (3 그룹): 인프라그룹, 웹개발그룹, DB그룹
- parent_id relationships properly established for hierarchy traversal
- Database status: Verified with `SELECT depth FROM organizations ORDER BY depth`

## API route alignment fix (2026-05-12)
- ✅ Fixed endpoint path mismatch: backend `/api/v1/` → `/api/mail/`
- Changed all carddav endpoints (addressbooks, contacts, directory) to use `/api/mail/` prefix
- Updated tests to match new routing (carddav_test.go) — 971 tests passing
- Resolved missing org tree data in webmail UI by fixing API path
- Enables proper hierarchical organization display in OrgPickerModal

## Webmail UI improvements (2026-05-12)
- ✅ OrgPickerModal: Address book and contact list styling aligned with org chart UI
- Consistent padding, spacing, and visual hierarchy across all three panes
- Active field management (to/cc/bcc) with visual feedback
- Hierarchical tree rendering with expand/collapse (▼/▶) indicators
- Database has hierarchical data ready for visualization

## Webmail E2E testing infrastructure (TASK-095, 2026-05-12, complete)
- Added Playwright E2E testing framework (@playwright/test ^1.48.0)
- Created playwright.config.ts with baseURL=http://localhost:3003, chromium browser, HTML reporting
- Added test:e2e and test:e2e:ui npm scripts for running/debugging tests
- Created e2e/ directory with initial test suites:
  - auth.spec.ts: login page loads, redirect flows, homepage navigation
  - mail-list.spec.ts: mail list display, sidebar navigation, page structure
- Test structure ready for expansion with compose, search, calendar, org picker, drive tests

## Org chart recipient picker + group autocomplete (TASK-094, 2026-05-12)
- Extracted OrgPickerModal.tsx: standalone 3-pane modal (org tree / address books, members/contacts, recipients)
- Tab switching between org units and address books with search/filter across members
- Multi-field support (to/cc/bcc) with active field management and deduplication
- ParseToPickerItems / pickerItemsToString helpers for string↔picker item conversion
- Updated ComposeModal: integrated OrgPickerModal, removed inline org picker state
- Backend: ListOrgTree repository method + GET /api/v1/directory/org-tree endpoint
- OrgUnit struct with hierarchical depth, ParentID, and member resolution (100 users per org)
- ReadingPane: org picker support for inline compose replies
- RecipientChips: group badge type support ("그룹" label for contact suggestions)

## Webmail Phase 3 power features (2026-05-12)
- ReadingPane: star toggle button in toolbar (`onStar` / `isStarred` props wired from page.tsx via `messages.find`)
- ReadingPane: read/unread toggle in more menu (`onToggleRead` / `isRead` props)
- ReadingPane: "연락처에 추가" button in sender row (localStorage `webmail_contacts` cache, hidden after save)
- ReadingPane: ICS attachment detection → parsed SUMMARY/DTSTART/DTEND/LOCATION → "캘린더에 추가" card UI with CalDAV integration
- AppIconBar: red unread badge on mail icon (>99 → "99+"), wired to `folders.reduce` sum in page.tsx
- ComposeModal: Drive file picker (cloud button → folder browser with breadcrumbs, `attachDriveFileToEmail`)
- ComposeModal: emoji picker popup (6 categories × 20 = 120 emojis, inserts at cursor)
- ComposeModal: clipboard paste → image attachment (onPaste handler → handleFileSelect)
- SearchBar: 받는사람(to:) filter field enabled + `to:` operator in `parseSearchOperators`
- Archive/spam: optimistic remove + undo toast (server call async, restores on undo)
- Print: new popup window with formatted email HTML, calls `w.print()` on load
- Fixed: `DriveNode.node_type === 'folder'` (was `is_dir`); `isContactSaved` memo dep includes `savedContact`; ICS fetch headers type; `createCalendarEvent` field names (`title/start/end/allDay`)

## webmail login IP + profile picture + enterprise settings (2026-05-12)
- httpapi/mail.go: add missing "net" import for net.SplitHostPort (client_ip in login response)
- AuthTokenResponse: add client_ip optional field; stored in localStorage on login
- Sidebar: 최근 접속 IP row in user menu session info; profile picture from localStorage
- SettingsModal: profile picture upload/remove in 계정 tab; 보안/단축키/고급 new categories
- MessageList: show user's own profile picture in sender avatar when from_addr matches userEmail

## Recent changes (2026-05-10)
- Fixed DomainView.QuotaLimit omitempty causing NaN in frontend quota display (removed omitempty)
- Fixed queue stats page runtime error (API response is `{queues:[]}` array, not single object)
- Fixed domain-settings Add Setting button missing onClick handler
- Added `company_name` to DomainView via JOIN (backend: maildb/admin.go)
- Companies page: View button enabled with detail modal + post-create "Add Domain" CTA
- Domains page: company relationship visible in table, filter-by-company, domain creation requires parent company selection
- Company creation feature: full backend endpoint POST /admin/v1/companies implemented

## Current phase
gogomail has completed Phases 8-9 (Admin Console Backend + Frontend).
Phase 8-D (Complete Admin Console) — TASK-082 through TASK-088 COMPLETE. Admin authentication, user management, sessions, and monitoring fully implemented and tested.

**Deployment Infrastructure**: Complete
- 4-tier Docker Compose configurations (dev, small, medium, large)
- Production-ready load balancing (Nginx, HAProxy)
- Monitoring stack (Prometheus, Grafana)
- Logging infrastructure (ELK stack)
- Supporting configs and init scripts

**Admin Console Status**:
- Phase 8 (Backend): COMPLETE — 240 unit tests, 3 full-stack admin features
- Phase 9 (Frontend): COMPLETE — 12 pages, 45+ unit tests, 18+ E2E tests
- Phase 8-D (Domain Settings): COMPLETE (TASK-082)
  - [x] Database schema (domain_settings table)
  - [x] API routes (GET/PUT/DELETE with validation)
  - [x] Service layer with audit
  - [x] Frontend UI pages with Cloudscape components
- Phase 8-D (API Settings): COMPLETE (TASK-083)
  - [x] Database migrations (api_settings, api_keys)
  - [x] API key management (create, rotate, delete)
  - [x] Rate limiting (RPS, BPS)
  - [x] CIDR allowlist configuration
  - [x] OpenAPI 3.1.0 documentation
  - [x] Frontend pages and React Query hooks
- Phase 8-D (Alerts & Notifications): COMPLETE (TASK-084)
  - [x] Database migrations (0085-0087: alert_rules, alert_channels, alert_events)
  - [x] Model definitions (AlertRule, AlertChannel, AlertEvent)
  - [x] Repository layer (13 CRUD operations)
  - [x] Service layer with validation (threshold > 0, channel configs)
  - [x] API routes (8 REST endpoints)
  - [x] HTTP handlers with proper error handling
  - [x] OpenAPI 3.1.0 documentation with schemas
  - [x] Frontend hooks (useAlertRules, useAlertChannels, useAlertEvents)
  - [x] Frontend admin page with tabs (rules, channels, events)
  - [x] Modal forms for create operations
  - [x] E2E tests with results document (38 test cases, 100% pass rate)
- Phase 8-D (Admin Console Frontend Phase 1): COMPLETE (TASK-085)
  - [x] Dashboard & Analytics (stats, activity metrics, API usage, security events)
  - [x] Advanced Audit Logs (filtering, pagination, export to CSV/JSON)
  - [x] Organization Structure (hierarchy tree view, all nodes table)
  - [x] Export & Reports (schedule management, template selection)
  - [x] Role Management (permission matrix, custom roles)
- Phase 8-D (Admin Console Frontend Phase 2): COMPLETE (TASK-086)
  - [x] API Key Management (create, rotate, delete, CIDR allowlist)
  - [x] MFA & Security Policy (mode, grace period, session, password, lockout)
  - [x] SSO & Identity Provider (LDAP, OIDC, SAML support)
  - [x] Domain & Tenant Management (add, delete, DNS status)
  - [x] Policy & Compliance (regulatory frameworks: GDPR, HIPAA, PCI DSS, SOX, CCPA)
- Phase 8-D (Admin Console Frontend Phase 3): COMPLETE (TASK-087)
  - [x] Sidebar Navigation (dynamic menu, active page highlighting)
  - [x] AdminLayout wrapper (unified AppLayout, notifications)
  - [x] Page integration and consistency
  - [x] Admin authentication endpoints (login, setup)
  - [x] E2E test results document (43 test cases, 100% pass rate)
  - [x] Ready for system-wide E2E testing

The project has moved beyond SMTP-only development. SMTP remains a critical
RFC-sensitive core, but current work should balance:

- tenant/domain operations
- Admin API
- Mail API contracts
- delivery routing and observability
- DNS/DKIM/domain onboarding
- quota and policy enforcement
- OpenAPI drift prevention

Runtime config store (`internal/configstore`) enables hierarchical configuration
with company → domain → user inheritance. Settings are stored in PostgreSQL JSONB
with LISTEN/NOTIFY-based cache invalidation. The `PostgresConfigStore` maintains
an in-memory cache that is synchronized across processes via NOTIFY broadcasts.

Admin API CRUD endpoints: `GET/POST/PUT/DELETE /admin/v1/companies/{id}/config/{key}`,
`GET/POST/PUT/DELETE /admin/v1/domains/{id}/config/{key}`,
`GET/POST/PUT/DELETE /admin/v1/users/{id}/config/{key}`.
The `Resolve` method implements tree-order lookup (user overrides domain overrides
company). Locked entries prevent updates at lower scopes. The Propagate API
(`POST /admin/v1/companies/{id}/config/propagate?scope=subtree|children|domains`)
pushes a company-level config value down to child companies, domains, or both.

Creation hooks automatically copy parent config to newly created domains and users.
When `CreateDomain` or `CreateUser` is called via the admin service, the wrapper
invokes `PropagateFromParent` to copy non-locked parent settings (company→domain
or domain→user). This ensures new tenants inherit defaults without explicit setup.

2FA/TOTP (`internal/authmfa`) implements RFC 6238 TOTP for multi-factor authentication.
The `auth.mfa.mode` config setting controls enforcement: `disabled`, `optional`, or
`required`. TOTP secrets and recovery codes are stored in `user_mfa_secrets` table.
The `totp_used_codes` table prevents replay attacks by tracking recently used codes.
`GenerateTOTP` and `VerifyTOTP` support a ±2 window (±2 minutes) for clock drift.
Recovery codes are 10-character alphanumeric strings, single-use, with 8 codes generated
per user setup. JWT claims include `mfa_verified` flag for session-level MFA state.

Batch worker (`internal/batchlock`) provides distributed job locking via PostgreSQL
advisory locks (`pg_try_advisory_lock`). The `--mode=batch-worker` flag starts a
ticker-based job runner with graceful shutdown support. `PostgresJobLock` ensures
only one instance executes a given job across multiple worker instances. The
`JobRegistry` registers periodic jobs with configurable intervals. Initial jobs
include ScheduledMailFlusher (5m), QuotaAlertCheck (15m), MFAGracePeriod (1h),
TokenCleanup (30m), and UsedCodeCleanup (5m). In-memory lock fallback is used
when database is unavailable for testing.

SSE config streaming (`GET /admin/v1/config/stream`) provides real-time config
change events via Server-Sent Events. The endpoint streams `connected` events
and will be extended to push actual config change notifications from the
configstore Notifier interface. Scope security prevents administrators from
directly modifying user-level config entries; PUT and DELETE on
`/admin/v1/users/{id}/config/{key}` return 403 Forbidden.

API key management (`internal/apikeys`) enables domain administrators to create
programmatic API keys with CIDR-based access control. Keys use `gm_` prefix and
are stored as SHA-256 hashes. The `ApiKeyMiddleware` detects `gm_`-prefixed bearer
tokens and validates them against the `domain_api_keys` table, checking CIDR
restrictions, expiration, and revocation status. Keys expire after 30 days by default.

LDAP gateway (`internal/ldapgw`) provides a read-only LDAP v3 interface for
directory lookups. `BindRequest` (simple bind) and `SearchRequest` are supported
for authentication and directory queries. Write operations (`Modify`, `Add`, `Delete`,
`ModDN`) are rejected with `unwillingToPerform`. BER encoding/decoding handles
LDAP v3 message formats. The package is designed to support LDAPS (port 636) and
StartTLS for encrypted connections.

SCIM 2.0 provisioning (`internal/scim`) implements RFC 7643/7644 for automated
user and group lifecycle management. The package supports User resource CRUDL
operations with standard SCIM JSON format including schemas and metadata.
Filter parsing handles `eq`, `sw`, `co`, `ew` operators on attributes like
`userName`, `emails.value`, `displayName`, and `active`. List responses include
totalResults, startIndex, and itemsPerPage for pagination.

SSO (`internal/sso`) supports both SAML 2.0 Service Provider and OIDC Relying Party
roles. SAML features include AuthnRequest generation with unique IDs, XML
serialization for SAML protocol messages, and assertion consumer service handling.
OIDC features include discovery document parsing, state generation with constant-time
verification, authorization code flow preparation, and JWT ID token validation
(issuer, audience, expiry, issued-at checks). SHA-256 nonce hashing is provided for
PKCE and nonce validation.

WebDAV gateway (`internal/httpapi/webdav.go`) exposes Drive files via RFC 4918 WebDAV
protocol for external client mounting and synchronization. The `webdavHandler` implements
OPTIONS, PROPFIND, MKCOL, GET, PUT, DELETE, MOVE, COPY, PROPPATCH,
LOCK, and UNLOCK. Runtime integration: `ModeWebDAV` starts a dedicated HTTP listener
(`runWebDAVGateway`) that registers WebDAV routes on a fresh `http.ServeMux`.
`WebDAVServiceAdapter` wraps `drive.Service` to satisfy the `WebDAVService` interface,
adapting `TrashNode`'s `(Node, int64, error)` signature to `error` and providing
stubs for `LockNode`/`UnlockNode` (the handler uses in-memory locks).
Configuration: `GOGOMAIL_WEBDAV_ADDR` (default `:8083`), `GOGOMAIL_WEBDAV_DEPTH_INFINITY_ENABLED`
(default `false`). Depth:infinity requests are rejected with 403 when disabled.
PROPFIND responses include resource properties (displayname, getcontentlength,
getcontenttype, getlastmodified, resourcetype). The package handles XML serialization for
multistatus responses and parses propfind requests for selective property retrieval.
Href normalization preserves trailing slashes for collection resources.

Milter adapter (`internal/milter`) provides a sendmail milter v2/v6 protocol
implementation for integrating with external MTAs (Postfix, Sendmail) for
mail filtering, virus scanning, and policy enforcement. The package defines a
`Handler` interface with callbacks for SMTP connection stages: `OnConnect`,
`OnHelo`, `OnMail`, `OnRcpt`, `OnData`, and `OnEOB`. Packet encoding/decoding
follows the milter wire format (4-byte big-endian length + 1-byte command +
data payload). Supported commands include Connect, Helo, Mail, Rcpt, Data, EOB,
Abort, and Body chunks. Response actions include continue, reject, tempfail,
accept, and discard.

DNSBL (`internal/dnsbl`) provides DNS-based blacklist lookups per RFC 5782
for SMTP connection filtering. The `DNSBL` struct queries configured DNSBL
zones by reversing the client IP octets and performing an A-record lookup.
IPv4 addresses use dotted-decimal reversal; IPv6 addresses use nibble reversal.
A returned A record in the 127.0.0.0/8 range indicates a listing, with the
specific octet encoding the list reason. The `Resolver` interface abstracts
DNS lookups for testability, with `NetResolver` using `net.LookupHost` as the
production backend.

POP3 server (`internal/pop3d`) implements RFC 1939 for mail retrieval with
standard commands (USER, PASS, STAT, LIST, RETR, DELE, NOOP, RSET, QUIT)
and extensions (UIDL, TOP, CAPA, STLS). The server operates in three states:
AUTHORIZATION, TRANSACTION, and UPDATE. Authentication transitions from
AUTHORIZATION to TRANSACTION; QUIT in TRANSACTION transitions to UPDATE
where pending deletions are applied. The `Store` interface abstracts user
authentication and mailbox access, while the `Mailbox` interface provides
message count, size, UIDL, content retrieval, and deletion tracking.
`RETR` and `TOP` now send RFC 1939 multi-line content through a shared
dot-stuffing writer that canonicalizes line endings to CRLF and escapes every
line beginning with `.` before the final `.\r\n` terminator.
`CAPA` advertises `STLS` only while the session is still in AUTHORIZATION state
and TLS has not already been negotiated; authenticated TRANSACTION sessions
reject `STLS` while keeping the mailbox session usable.
Successful `STLS` negotiation resets any pre-TLS `USER` value, requiring the
client to send credentials again on the protected channel before `PASS` can
authenticate.
`CAPA` is state-aware: AUTHORIZATION responses advertise authentication
mechanisms (`USER`, `SASL PLAIN LOGIN`) while TRANSACTION responses omit those
auth-only capabilities. All CAPA responses include stable server metadata via
`IMPLEMENTATION gogomail` and `LOGIN-DELAY 0`.
`AUTH PLAIN` and `AUTH LOGIN` now validate mechanism-specific argument counts
before entering SASL decoding or continuation prompts. Extra arguments return
`-ERR syntax error` and leave the session in AUTHORIZATION state so clients can
retry with `USER`/`PASS` or a well-formed `AUTH` command on the same connection.
SASL continuation cancellation is also handled explicitly: `AUTH PLAIN` and
both `AUTH LOGIN` prompts accept `*` as cancellation, return
`-ERR authentication cancelled`, and keep the connection ready for a later
authentication attempt.
The mailservice POP3 adapter now exposes raw body fetch errors to the POP3
server, so `RETR` and `TOP` return `-ERR` instead of opening a successful
multi-line response when message content cannot be loaded.
The same adapter now loads INBOX through the service cursor pagination API
until no further page remains, so POP3 sessions see every active INBOX message
instead of only the first normalized list page.
POP3 authentication now acquires an exclusive maildrop lock per normalized
user key before entering TRANSACTION state. A concurrent POP3 login for the
same mailbox receives `-ERR maildrop already locked`, and the lock is released
on QUIT or connection close. The mailservice POP3 mailbox supplies the
canonical DB user ID as that lock key, so alternate login addresses for the
same user still converge on one maildrop.
The POP3 server now enforces the configured maximum connection count in its
accept loop. `GOGOMAIL_POP3_MAX_CONNECTIONS` and YAML
`pop3_max_connections` feed the runtime `MaxConnections` setting; excess
connections receive `-ERR too many connections` and closed sessions release
their slots for later clients.
TLS support via `STLS` command and optional implicit POP3S are supported
through a configurable `tls.Config`. `GOGOMAIL_POP3S_ADDR` / YAML
`pop3s_addr` enables the implicit TLS listener, which shares the same POP3
server instance, maildrop locks, and connection limit as the cleartext POP3
listener. Implicit TLS sessions do not advertise `STLS`.

Push notification adapters (`internal/pushnotify`) provide a `PushSink`
interface with FCM, APNs, and Web Push (RFC 8030) implementations. The
`FCMAdapter` sends via Firebase Cloud Messaging HTTP v1 API, the
`APNsAdapter` sends via Apple Push Notification service HTTP/2 API, and
the `WebPushAdapter` sends via the Web Push Protocol. Each adapter uses
an abstracted `HTTPClient` for testability. The `MultiSink` broadcasts
to multiple adapters simultaneously. `DeviceTokenStore` abstracts device
token persistence with `MemoryDeviceTokenStore` as an in-memory reference
implementation.

Delta sync (`internal/deltasync`) provides device-specific sync cursor
management and IMAP IDLE fan-out for real-time mailbox synchronization.
The `Cursor` struct tracks the highest known change version per device
and mailbox. `ChangesSince` filters a change set to only entries newer
than the device's cursor and returns an updated cursor. `FanOut` manages
per-mailbox channels for broadcasting mailbox events to connected IMAP
clients. `MemoryCursorStore` provides an in-memory reference implementation
of `CursorStore` with save, get, list-by-mailbox, and delete operations.

Mail flow log feature now tracks inbound and outbound mail flow for operational
forensics. The `mail_flow_logs` table records direction, SMTP envelope (mail_from,
rcpt_to), auth results (DKIM/SPF/DMARC), spam score, and delivery status
(received/delivered/failed/bounced/filtered/rejected/pending). The handler
consumes `mail.stored`, `mail.delivered`, `mail.bounced`, `mail.delivery_failed`,
and `mail.delivery_exhausted` events. Admin API exposes GET
`/admin/v1/mail-flow-logs`, GET `/admin/v1/mail-flow-logs/stats`, GET
`/admin/v1/mail-flow-logs/daily-stats`, and GET `/admin/v1/mail-flow-logs/{id}`
endpoints with filtering by direction, company, domain, user, message_id,
rfc_message_id, from/to addresses, subject, flow_status, and time range.
The daily-stats endpoint provides time-series breakdown with date,
inbound/outbound message counts and sizes, and delivery status counts.

Mail flow logs use a hybrid storage architecture: PostgreSQL provides ACID
guarantees and referential integrity for audit compliance, while OpenSearch
provides scalable aggregation queries for statistics and time-series analysis.
When `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch` and
`GOGOMAIL_MAIL_FLOW_OPENSEARCH_BOOTSTRAP=true`, mail flow events are indexed
to the `mail_flow` OpenSearch index. The `MailFlowStatsSearcher` supports
aggregation queries for stats and daily breakdowns without taxing PostgreSQL.

Mail flow stats support a configurable backend via `GOGOMAIL_MAIL_FLOW_STATS_BACKEND`
(auto|postgres|opensearch, default: auto). The `MailFlowStatsProvider` interface
abstracts the underlying storage, with `PostgresMailFlowStatsProvider` using
PostgreSQL queries directly and `OpenSearchMailFlowStatsProvider` using the
`MailFlowStatsSearcher` for scalable aggregation. The admin service bridges
`mailflow.MailFlowStatsResult` to `maildb.MailFlowLogStatsView` for API
compatibility. Auto mode uses OpenSearch when `MailFlowOpenSearchBootstrap=true`,
falling back to PostgreSQL if OpenSearch is unavailable.

API metering fail-open logic now logs recovered panics with full route and
method context, preventing silent failures when the metering worker or sink is
saturated.
Ignored `Close()` errors for shared infrastructure (Redis, Postgres, TCP
listeners) and local spool resources are now captured and logged across all
service entry points, ensuring resource leak diagnostics are available in
production logs.
Redis-backed module nil safety: `RedisLimiter.Allow`, `RedisDeduplicator.CheckAndSet`,
and `RedisBackpressure.Accept/State` now guard against nil Redis clients, returning
permissive defaults instead of panicking. `RedisBackpressure.SetState` returns an error
for nil client since explicit state write requires a backend. All nil guards include
unit test coverage. This prevents crashes when Redis-optional features are configured
without a Redis backend.
Database migration functions now validate nil database handles explicitly: `MigrateUp`
and `CurrentMigrationVersion` return clear errors for nil database. `migrationVersionFromFilename`
has comprehensive edge case coverage for malformed migration filenames.
IMAP `STORE`, `MOVE`, `EXPUNGE`, and their `UID` equivalents now use flattened
dispatch branches, removing redundant syntax checks that were previously
duplicated between the tag-based and UID-based handlers.
IMAP mailbox display names now preserve original database name spacing,
allowing for exact mailbox identity in clients that depend on specific prefix
or suffix whitespace, such as nested folder hierarchies or legacy INBOX
naming.
Large-attachment share links now have a dedicated database and repository
boundary through `attachment_share_links`, allowing for public download-only
access to stored attachments with the same hashed bearer-token and expiry
semantics used for Drive files. Quota ledger integration is preserved by
sharing the underlying `attachments` quota accounting, ensuring that
shareable large attachments remain aggregated in the company/domain/user
storage pool.
S3-compatible storage ETag handling is now stricter across optional
`PutObject` success headers, `HEAD`/`Stat`, `ListObjectsV2`, and
`CopyObjectResult` success XML: malformed quote nesting, whitespace padding,
line/control bytes, non-ASCII values, and other non-printable opaque ETag
payloads fail closed instead of crossing the shared storage boundary as object
identity metadata.
S3-compatible `HEAD`/`Stat` content-type metadata now also parses as an
unpadded ASCII MIME media type before it can reach shared storage callers,
while still preserving valid parameterized values such as
`text/plain; charset=utf-8`.
S3-compatible `ListObjectsV2` success XML now rejects direct text inside
structured standard object metadata wrappers such as `Owner` and
`RestoreStatus`, while still accepting known namespace-free/AWS child elements.
This keeps provider compatibility for standard AWS metadata without allowing
ignored wrapper text to cross the shared storage boundary.
S3-compatible success responses that include requester-pays
`x-amz-request-charged` metadata now classify blank, whitespace-only, or
whitespace-padded values as invalid provider metadata before rejecting exact
nonblank requester-pays mode as unsupported.
S3-compatible full-object `GET` now rejects contradictory `Content-Length`
metadata even when Go's normalized response length is known to be zero, keeping
empty-object and non-empty-object identity from depending on which metadata
surface a provider filled.
S3-compatible offset-zero `200 OK` range compatibility now applies the same
raw-header versus normalized-length agreement when both metadata surfaces are
known, so a downgraded full-window range response cannot contradict its own
zero-length metadata before exposing a bounded reader.

Webmail capability discovery now advertises `GET /api/v1/search` filters `q`,
`folder_id`, `from`, `to`, `cc`, `bcc`, `subject`, and `has_attachment`. The
`to`, `cc`, and `bcc` parameters are supported both on the Postgres path and the
OpenSearch relevance path — the search index stores `to_addrs_lc`, `cc_addrs_lc`,
and `bcc_addrs_lc` as keyword fields and applies wildcard filters matching the
same case-insensitive pattern used for `from_addr_lc` and `subject_lc`. The
runtime response, OpenAPI enum, and regression tests are aligned so future
generated clients do not attempt unsupported `since`, `before`, `read`, or
`starred` search filters; read/starred filtering remains available on the
message/thread list endpoints where it is actually implemented.
Admin console capability discovery is now pinned to the `/admin/v1` OpenAPI
server at the operation level, with a runtime regression that
`/api/v1/console/capabilities` remains unregistered. This keeps generated
admin clients from accidentally using the public mail API base path for
operator bootstrap calls.
Health probes are now pinned to the service-root OpenAPI server while service
info is pinned to `/api/v1`, with runtime regressions for the common wrong-base
forms. This keeps deployment probes, load balancer checks, and generated
contract clients aligned with the actual Go router.
All registered Mail and Drive API operations now carry operation-level
OpenAPI `servers: /api/v1` entries, with a drift test deriving the registered
`/api/v1` routes from `mail.go` and `drive.go`. This prevents generated
clients from inheriting the global `/admin/v1` server option for webmail,
Drive, public share-link, attachment, draft, thread, push-device, or search
routes.
CalDAV and CardDAV discovery now advertise `DAV: sync-collection` and
collection `supported-report-set` sync reports only when the runtime store
implements the corresponding sync change-log interface, preventing native
clients from enabling DAV sync against backends that cannot serve sync-token
deltas. CalDAV and CardDAV `REPORT` parsing now also rejects duplicate
`DAV:limit` controls and duplicate nested `DAV:nresults` controls, avoiding
ambiguous client pagination semantics before bounded object or change-list
work begins.
The same `sync-collection` parser path now rejects duplicate `DAV:sync-token`
and `DAV:sync-level` controls so later XML elements cannot silently convert a
delta sync request into a different sync anchor or initial-sync shape.
CalDAV and CardDAV object `GET`/`HEAD` conditional handling now honors ETag
precedence by ignoring `If-Modified-Since` whenever `If-None-Match` is present,
so stale client validators cannot accidentally receive `304 Not Modified` for
changed `.ics` or `.vcf` bodies.
CalDAV and CardDAV request paths and absolute REPORT hrefs now reject encoded
path separators such as `%2F` and `%5C`, plus double-encoded forms such as
`%252F` and `%255C`, before URL decoding, preventing calendar/address-book/
object identifiers from changing path shape at the segment validation
boundary.
CalDAV and CardDAV object `PUT`/`DELETE` now carry the currently observed
strong object ETag into repository mutation guards even when `If-Match: *`
matches an existing resource, so existence-only DAV preconditions still
recheck the exact object observed by the handler before durable mutation.
CalDAV and CardDAV object `DELETE` now also evaluate `If-None-Match` before
mutation, so `If-None-Match: *` or a matching ETag receives HTTP 412 and leaves
the `.ics` or `.vcf` object intact instead of silently deleting an existing
representation.
CalDAV and CardDAV collection `DELETE` and `PROPPATCH` now apply the same
`If-None-Match` precondition boundary against calendar/address-book collection
ETags, so native DAV clients using `If-None-Match: *` or a matching collection
validator receive HTTP 412 before recursive child deletion or metadata updates
can mutate `.ics`/`.vcf` collection state.
CalDAV and CardDAV collection `DELETE` and `PROPPATCH` now also pass the
observed collection ETag into repository mutation guards after successful
conditional preflight, including `If-Match: *`, so stale calendar/address-book
collection races are rechecked under the storage transaction before recursive
delete or metadata update state is committed.
CalDAV `MKCALENDAR` and CardDAV extended `MKCOL` now evaluate collection
creation preconditions before reading WebDAV XML bodies: existing collection
targets reject matching `If-None-Match` validators with HTTP 412, missing
targets reject `If-Match` or `If-Unmodified-Since` with HTTP 412, and
`If-None-Match: *` remains the safe create-only path for absent collections.
For missing collection targets, those creation paths now also validate
UUID-shaped collection path IDs before conditional create evaluation, so
malformed new calendar or address-book IDs fail with HTTP 400 before 412
precondition responses or XML body reads while existing legacy collection IDs
keep their normal already-exists and conditional semantics.
CalDAV and CardDAV object `PUT` now reject `If-Unmodified-Since` requests for
missing objects with HTTP 412 before reading `.ics` or `.vcf` bodies, so
state-changing WebDAV timestamp preconditions cannot accidentally create new
resources when the client intended to guard an existing representation.
CalDAV and CardDAV webmail REST APIs now expose calendar and address book
operations through JSON endpoints for webmail frontend integration. The CalDAV
`CalendarHandler` uses a `CalendarRepo` interface for testability and implements
CRUD endpoints for calendars (`GET/POST/PATCH/DELETE /api/v1/calendars`) and
calendar objects (`GET/PUT/DELETE /api/v1/calendars/{id}/objects/{name}`). The
CardDAV `ContactHandler` uses a `ContactRepo` interface and implements CRUD
endpoints for address books (`GET/POST/PATCH/DELETE /api/v1/addressbooks`) and
contacts (`GET/PUT/DELETE /api/v1/addressbooks/{id}/contacts/{name}`). Both
handlers support `user_id` query parameter authentication when `tokenManager`
is nil, with `rejectUnknownQueryKeys` allowing `user_id` in that mode. Contact
object endpoints accept `text/vcard`, `application/vcard+xml`, and `text/x-vcard`
content types. Calendar object endpoints accept `text/calendar` and
`application/ics`. ETag-based conditional requests are supported for object
get/delete operations. All endpoints include comprehensive unit tests with
fake repository implementations.
IMAP `LOGIN` now treats an empty quoted password as syntactically valid and
passes it to the backend authentication boundary, returning
`[AUTHENTICATIONFAILED]` instead of a protocol `BAD` response when credentials
are rejected. `AUTHENTICATE PLAIN` still rejects empty decoded passwords and
whitespace-padded SASL response tokens before privacy-policy or backend
authentication checks.
Admin storage capability support flags are now derived from normalized active
backend labels instead of hard-coded `true` values, so local/NFS, MinIO, and
AWS/S3-compatible deployments advertise only the storage-label families they can
actually serve. Compatibility labels are validated as bounded safe lowercase
tokens and surfaced as sorted/de-duplicated active labels, while unknown future
labels remain non-activating in the support matrix until explicitly recognized.
The OpenAPI schema now machine-documents `active_labels` as a non-empty unique
token list and `operations` as a unique primitive list, with regression coverage
pinning the default advertised object-storage operations.
S3-compatible `List` now rechecks provider-returned keys against the requested
logical gogomail prefix after stripping the configured bucket/storage prefix,
so malformed or overly broad S3-compatible list responses cannot leak sibling
keys into caller listings or `DeletePrefix` cleanup work.
S3-compatible `GetRange` now accepts `200 OK` full-range compatibility
responses only when the returned `Content-Range` matches the requested byte
window or, without `Content-Range`, when the request starts at offset 0 and
`Content-Length` exactly equals the requested length. Unsafe `200 OK` range
responses are drained and rejected, keeping provider compatibility from
weakening bounded-read semantics.
S3-compatible `206 Partial Content` range responses now also reject invalid or
mismatched `Content-Length` headers when present, so contradictory provider
metadata is drained and failed before a bounded range reader is returned.
S3-compatible `Content-Range` validation now rejects internal whitespace inside
the `start-end/size` byte-range grammar instead of normalizing malformed
provider metadata before bounded range reads.
S3-compatible `200 OK` range compatibility responses that include a matching
`Content-Range` now also validate any present `Content-Length` against the
requested window, keeping the compatibility path as strict as ordinary
`206 Partial Content` responses. Duplicate `Content-Range` headers now fail
closed for both `206 Partial Content` and safe `200 OK` range-compatibility
responses, preventing object byte-window identity from depending on HTTP
header collapse.
S3-compatible `Content-Length` parsing now requires exact unsigned decimal
digits at the storage boundary, rejecting signed or whitespace-padded values
such as `+5` or ` 5` for `HEAD` metadata and range-response length validation
instead of normalizing them as valid sizes. Duplicate `Content-Length` headers
are now also rejected for `HEAD`/`Stat`, full-object `GET`, and range response
validation, keeping ambiguous provider length metadata from crossing the shared
storage interface.
S3-compatible `HEAD`/`Stat` now also validates the raw `Content-Length` header
when Go has already populated `Response.ContentLength`, rejecting malformed or
contradictory provider metadata instead of trusting the normalized struct
field alone. Duplicate `Last-Modified` headers are rejected before timestamp
parsing so provider metadata cannot collapse multiple object modification times
into whichever value the HTTP library returns first. Duplicate `ETag` headers
are now rejected for the same reason: object identity metadata must not depend
on first-header collapse. Duplicate `Content-Type` metadata on `HEAD`/`Stat`
also fails closed so preview/download MIME decisions do not depend on
first-header collapse. Present-but-blank or malformed `Last-Modified`, `ETag`,
and `Content-Type` metadata now also fails closed instead of being silently
exposed as empty optional metadata, and `ETag`/`Content-Type` headers must not
carry leading or trailing padding before quote cleanup or MIME parsing. This
keeps object identity, timestamp, and MIME metadata explicit at the
S3-compatible adapter boundary.
S3-compatible full-object `GET` now applies the same exact `Content-Length`
header validation when present and wraps known-length successful bodies in a
bounded reader, so truncated compatible-provider full reads surface
`io.ErrUnexpectedEOF` instead of a silent short body.
S3-compatible status-error diagnostics now recognize standard S3 `<Error>`
XML bodies and render bounded one-line `Code: Message` previews with
request-id and host-id context instead of raw XML tags, while preserving the
existing sanitized plain-text fallback for non-XML provider errors. The XML
error preview path is streaming and best-effort, so truncated provider error
bodies still surface parsed S3 fields without falling back to raw XML snippets.
Each parsed XML error field is independently capped before formatting, keeping
diagnostics bounded even when compatible providers return very large messages.
Standard S3 error XML without any safe `Code`, `Message`, `RequestId`, or
`HostId` fields now suppresses the preview entirely rather than falling back
to raw XML fragments, and safe fields using foreign XML namespaces are treated
as ambiguous diagnostics instead of canonical S3 error text.
S3-compatible `ListObjectsV2` success responses now also reject top-level
standard S3 `<Error>` XML bodies as embedded provider errors, preserving the
same bounded `Code: Message`, request-id, and host-id diagnostics instead of
falling through to generic list-shape failures.
S3-compatible `PutObject` and `DeleteObject` successful status responses now
apply the same top-level standard S3 `<Error>` detection before reporting
success, so throttling, auth, or policy failures hidden inside `200 OK` or
delete-compatible success responses cannot masquerade as durable object writes
or cleanup progress. Those success bodies now also reject non-whitespace
non-error payloads instead of treating arbitrary provider text or XML as an
empty standard success response. `PutObject` success responses now also reject
blank, duplicate, or malformed `ETag` headers when providers supply optional
write identity metadata, while still allowing compatible providers that omit the
header entirely.
S3-compatible `Content-Range` start, end, and total-size numbers now reuse the
same unsigned decimal parser, rejecting signed values such as `bytes +1-3/5`
or `bytes 1-3/+5` before range metadata can be normalized.
S3-compatible `ListObjectsV2` object-size parsing now also requires unsigned
decimal digits, rejecting signed or whitespace-padded `<Size>` values such as
`+5` or ` 5 ` before list metadata reaches cleanup, Drive, or reconciliation
callers.
S3-compatible `ListObjectsV2` root `MaxKeys` metadata now also uses the same
exact unsigned decimal boundary and must not undercount returned `<Contents>`,
preventing provider pages from hiding impossible page shapes behind plausible
XML metadata while preserving AWS-compatible default `MaxKeys` echoes.
S3-compatible `ListObjectsV2` root `KeyCount` and `MaxKeys` metadata now
distinguish omission from present-but-blank elements: missing remains
compatible, while blank values fail the same numeric boundary as malformed
values.
S3-compatible `ListObjectsV2` root `Prefix` metadata, when providers echo it,
now must be nonblank and exactly match the signed provider prefix requested by
gogomail, including any configured storage prefix, so diagnostics cannot
depend on a misleading list echo while compatible providers may still omit the
field.
S3-compatible `ListObjectsV2` root `Name` metadata, when present, must also
be nonblank and match the configured bucket name, preventing wrong-bucket or
blank-bucket compatible-provider responses from looking like ordinary empty or
successful list pages.
S3-compatible `ListObjectsV2` root `EncodingType` metadata is now rejected when
present, including blank elements, because gogomail does not request
URL-encoded key mode and should not treat encoded provider keys as ordinary
object paths.
S3-compatible `ListObjectsV2` root `ContinuationToken` metadata, when present,
must now match an explicitly requested cursor exactly; returned tokens are
rejected when no request cursor was sent, keeping page diagnostics and retry
semantics aligned with the signed list request.
S3-compatible `ListObjectsV2` root `StartAfter` metadata is now rejected when
present, including blank elements, because gogomail uses continuation-token
pagination and does not request start-after list mode.
S3-compatible requester-pays success response headers are now rejected across
the adapter because requester-pays mode is not part of the current portable
storage contract. Standard object checksum metadata now treats
`ChecksumType` like `ChecksumAlgorithm`: namespace-free and AWS-namespaced
values are compatible, but foreign namespaces and nested XML fail closed.
Duplicate single-value `StorageClass`, `ChecksumType`, `Owner`, and
`RestoreStatus` object metadata now also fails closed, while repeated
`ChecksumAlgorithm` values remain compatible for providers that report more
than one checksum algorithm.
S3-compatible `ListObjectsV2` delimiter grouping controls are now rejected:
gogomail never requests delimiter-based grouping, so returned `Delimiter`
elements, including blank elements, or `CommonPrefixes` responses cannot be
mistaken for ordinary object pages.
S3-compatible `ListObjectsV2` object entries now reject missing or blank
`<Key>` elements instead of silently skipping malformed provider entries before
prefix mapping and cleanup scans.
S3-compatible `ListObjectsV2` pagination control now requires an explicit
canonical `<IsTruncated>true</IsTruncated>` or `<IsTruncated>false</IsTruncated>`
value, rejecting missing or non-canonical forms before deciding whether a page
is final.
S3-compatible `ListObjectsV2` pagination controls now also reject duplicate
top-level `<IsTruncated>` or `<NextContinuationToken>` elements, preventing
ambiguous provider pages from silently changing final/truncated state or cursor
identity during XML unmarshalling.
S3-compatible `CopyObject` success XML now accepts namespace-free or AWS S3
namespace `CopyObjectResult` roots only, rejecting same-local-name XML from
unexpected namespaces before copy/move is reported successful.
S3-compatible `CopyObjectResult` success XML now rejects duplicate top-level
`ETag` or `LastModified` metadata, nested `Error` elements, and unknown
top-level success children. Nested child elements inside simple copy metadata
fields are also rejected, so provider XML cannot smuggle structured
`ETag`/`LastModified` data through string unmarshalling. Nested standard S3
error details are formatted as bounded one-line diagnostics with request-id
and host-id context instead of collapsing provider-side copy failures or
ambiguous copy metadata into a successful copy/move result. Top-level and
nested copy `Error` bodies share the same capped streaming XML field parser as
status errors. Successful
`CopyObjectResult` bodies now also require a non-blank bounded `ETag`, so
copy/move durability is not reported when provider success metadata omits
object identity. XML `CopyObjectResult` ETags must not be whitespace-padded,
double-quoted, or otherwise malformed quoted values, preserving exact success
metadata while HTTP header optional-whitespace compatibility remains limited to
header parsing.
IMAP `SELECT` now canonicalizes backend-provided permanent flags before
rendering `FLAGS`/`PERMANENTFLAGS` and before selected-state STORE permission
checks. Duplicate, aliased, lower-case, or unknown backend flag metadata is
collapsed into the supported RFC-shaped system/keyword flag set on the wire,
keeping client-visible mailbox metadata and mutation permissions aligned.
S3-compatible `ListObjectsV2` response XML now applies the same namespace
boundary to `ListBucketResult`, accepting namespace-free or AWS S3 namespace
roots only before pagination, prefix filtering, cleanup, or Drive callers see
listed object metadata.
S3-compatible `ListObjectsV2` object `LastModified` metadata now fails closed
when a present provider value is blank, malformed, or whitespace-padded, while
still allowing missing values for compatible providers that omit optional
timestamp metadata.
S3-compatible `ListObjectsV2` object `ETag` metadata now also fails closed
when a present provider value is blank, whitespace-padded, malformed,
line-bearing, double-quoted, empty-after-quote cleanup, or larger than the
bounded metadata limit, instead of silently dropping suspect list metadata.
S3-compatible `ListObjectsV2` object metadata now rejects duplicate
per-object `<Key>`, `<Size>`, `<ETag>`, or `<LastModified>` elements before XML
unmarshalling can collapse conflicting provider values into one listed object.
Nested child elements inside those simple object metadata fields are also
rejected before list results reach cleanup, Drive, or reconciliation callers.
S3-compatible `ListObjectsV2` root metadata now rejects duplicate simple
standard elements such as `<KeyCount>` or `<Prefix>`, and validates
`KeyCount` as an unsigned decimal that exactly matches the raw returned
`<Contents>` count before pagination, cleanup, Drive, or reconciliation logic
trusts the provider page.
`storage.DeletePrefix` now revalidates every listed object against the
requested canonical prefix before deletion, returning a structured out-of-scope
listing error after preserving completed progress if a backend returns sibling
keys such as `drive/user-10/...` for a `drive/user-1` cleanup request.
S3-compatible `List` now validates provider continuation tokens only when
`IsTruncated=true`; final pages always clear `NextCursor`, even if a
compatible provider includes a whitespace-padded or otherwise unusable
`NextContinuationToken` that callers must not reuse.
Local/NFS `List` now matches S3 `Prefix` behavior when the requested prefix
exactly names an existing object: it returns a single-object final page instead
of failing with a directory expectation. The shared storage portability
contract now pins this exact-object prefix behavior for local/NFS and optional
S3/MinIO integration runs.
The shared storage path/prefix validator now rejects percent-encoded path
separators such as `%2F` and `%5C`, plus double-encoded forms such as `%252F`
and `%255C`, keeping object keys portable across local/NFS, MinIO, AWS S3, and
compatible gateways before adapter-specific request signing or filesystem
paths are built.
IMAP RFC 5258 `LIST-EXTENDED` return options now require the whole `RETURN`
option list to be parenthesized before option parsing, including non-`STATUS`
forms such as `RETURN (CHILDREN)`. Unparenthesized return options now receive a
tagged `BAD` at the parser boundary instead of being treated as valid extended
LIST controls.
IMAP `FETCH` and `UID FETCH` now accept RFC 3501 `RFC822<offset.count>`
partial full-message fetches, preserve `RFC822<offset>` in the literal
response item, and apply the same `\Seen` mutation semantics as full `RFC822`
body fetches.
IMAP `APPEND` now drains already queued selected-mailbox events before append
mutation responses, keeping pending FLAGS/EXISTS/EXPUNGE updates ordered ahead
of the tagged APPEND completion just like other selected mailbox commands.
IMAP `FETCH` body-part selectors now reject leading-zero MIME part numbers
such as `BODY[01]` or `BODY[1.02.TEXT]`, and partial fetch counts now reject
leading-zero `nz-number` forms such as `<12.034>`, keeping body-section and
partial-window parsing aligned with RFC 3501 number semantics. MIME part path,
partial fetch offset, and partial fetch count values are capped to IMAP's
unsigned 32-bit `number` range, while the maximum valid `4294967295` remains
accepted where the runtime integer width can represent it. MIME part paths also
reject surrounding or embedded whitespace instead of repairing padded atoms.
IMAP partial fetch offsets now also follow RFC 3501 `number` grammar by
accepting `0` or non-zero-leading digit atoms only; malformed windows such as
`BODY.PEEK[]<00.34>` or `<012.34>` are rejected before command execution
instead of being normalized to offset `0` or `12`.
IMAP command literal size framing now follows the same RFC 3501 `number`
grammar, preserving valid `{0}` literals while rejecting leading-zero forms
such as `{00}`, `{001}`, and `{001+}`, plus signed or malformed forms such as
`{+1}`, `{-1}`, and `{1++}`, with a tagged `BAD` framing response before
reading literal bytes.
IMAP command-line framing now requires RFC CRLF endings for ordinary commands,
literal suffix lines, `AUTHENTICATE PLAIN` SASL continuations, and `IDLE`
continuations. LF-only input receives a tagged
`BAD command line must end with CRLF` plus the existing framing-error `BYE`
close, keeping malformed line endings out of command handlers.
IMAP UID and message sequence-set numbers now also enforce RFC `nz-number`
spelling, rejecting leading-zero values such as `FETCH 01 FLAGS` or
`UID FETCH 1:02 FLAGS` before sequence expansion instead of normalizing them
to `1` or `2`.
IMAP UID and message sequence-set range parsing now also rejects
whitespace-bearing quoted or literal range components such as `"1: 2"`,
`"1 :2"`, or `"1, 2"` before authentication, selected-mailbox checks, or set
expansion instead of trimming them into valid sequence sets.
IMAP `SEARCH` and `UID SEARCH` `LARGER`/`SMALLER` size criteria now use the
same RFC 3501 `number` grammar boundary, preserving valid zero-size searches
while rejecting leading-zero values such as `SEARCH LARGER 020` before command
execution. Size numbers are also capped to IMAP's unsigned 32-bit `number`
range, so `4294967296` is rejected before search evaluation.
IMAP CONDSTORE parsing now separates positive RFC `mod-sequence-value` inputs
from zero-allowed `mod-sequence-valzer` inputs: `SEARCH MODSEQ 0` and
`FETCH (CHANGEDSINCE 0)` are rejected as malformed, while
`STORE (UNCHANGEDSINCE 0)` is preserved as a real conditional guard that
returns `MODIFIED` instead of being collapsed into an unconditional STORE.
IMAP `SEARCH HEADER` and `FETCH` `HEADER.FIELDS`/`HEADER.FIELDS.NOT` parsing
now accepts RFC 5322-style visible field-name characters such as `_`, `+`, and
`.` while still rejecting empty, space/control-bearing, colon-suffixed, or
non-ASCII field names, plus IMAP atom-specials that would make FETCH field
lists ambiguous. This keeps custom header probes from being falsely rejected at
the IMAP parser boundary. `FETCH` header-field section detection now also
requires an exact top-level body section or a valid numeric MIME part path
before `HEADER.FIELDS`/`HEADER.FIELDS.NOT`, so malformed section prefixes cannot
ride the supported header-subset path.
IMAP command parsing now rejects 8-bit non-ASCII bytes in unquoted atoms,
quoted strings, parenthesized quoted controls, and RFC 2971 ID quoted tokens.
Command tag, command-name, and UID subcommand atom validation also rejects
non-ASCII bytes in addition to control and atom-special characters, keeping RFC
3501's 7-bit atom/string boundary intact before command routing or UID state
checks.
IMAP quoted-string response rendering now also forces invalid UTF-8 and
non-ASCII runes to `?`, so ENVELOPE, BODYSTRUCTURE, STATUS, LIST, and related
quoted responses stay 7-bit safe while `UTF8=ACCEPT` is not advertised.
IMAP ENVELOPE subject, message-id, in-reply-to, and address display/mailbox/
host nstrings now share the bounded UTF-8-safe metadata text path before
response quoting, preventing oversized backend metadata from inflating FETCH
responses.
ENVELOPE address lists are also capped after placeholder filtering, preventing
abnormal recipient fan-out metadata from amplifying IMAP FETCH responses
without letting malformed empty entries hide later valid addresses.
Malformed empty or incomplete ENVELOPE address entries are dropped before
rendering, so backend placeholder data cannot emit stray `(NIL NIL NIL NIL)`
or display-name-only address tuples.
IMAP BODY/BODYSTRUCTURE rendering now validates MIME media type, subtype,
parameter-list names, and transfer-encoding tokens against RFC 2045-style
token boundaries, falling back to conservative defaults instead of emitting
malformed tspecial/control-bearing tokens or empty parameter values to clients.
MIME type/subtype validation is pair-shaped for structure rendering, so a
malformed type cannot be combined with a valid subtype, or vice versa, to
invent mixed fallback semantics.
Canonical duplicate parameter names are collapsed before rendering so a
malformed or inconsistent MIME source cannot emit repeated `BODYSTRUCTURE`
parameter keys.
MIME parameter values now share the same UTF-8-boundary-safe bounded metadata
text path, so oversized filenames, boundaries, or other parameter values cannot
inflate BODY/BODYSTRUCTURE responses.
Malformed MIME disposition tokens now render as `NIL` instead of being
upgraded to `ATTACHMENT`, preventing BODYSTRUCTURE responses from inventing
attachment semantics for invalid source metadata.
BODYSTRUCTURE content ID and description nstrings are now trimmed and bounded
at a UTF-8 boundary before quoting, keeping oversized source metadata from
inflating IMAP fetch responses.
IMAP listener startup now accepts an optional `GOGOMAIL_IMAP_MAX_CONNECTIONS`
cap, passed from YAML/env config into the protocol server. When the cap is
positive, accepted sessions hold a bounded slot for the lifetime of
`ServeConn`; excess clients receive an initial `BYE [ALERT]` and close instead
of creating unbounded goroutines, while the default `0` remains unlimited for
small or externally rate-limited deployments.
SMTP receive and submission listener startup now accepts optional
`GOGOMAIL_SMTP_MAX_CONNECTIONS` and
`GOGOMAIL_SUBMISSION_MAX_CONNECTIONS` caps from env/YAML config. Positive
values hold one slot per active SMTP session and reject excess connections with
an RFC-shaped transient `421 4.3.2` banner before close, while the default `0`
keeps the listener unlimited for small or externally rate-limited deployments.
Local/NFS storage now rejects symlinked intermediate path components for
object reads, range reads, metadata probes, deletes, copies, moves, writes, and
prefix listings, while continuing to hide final-object symlinks from list
pages. This keeps local disk and mounted storage deployments confined to the
configured object root instead of following host-specific links outside the
storage contract.
Local/NFS `Move` now falls back to the shared copy-delete path only when
filesystem rename reports a cross-device `EXDEV` boundary, keeping the
efficient rename fast path while allowing NFS/bind-mount style deployments to
preserve the backend-neutral relocation contract.
Drive JSON mutation handlers share the strict backend JSON contract and now
have regression coverage for required `application/json` content type,
unknown-field rejection, and trailing-token rejection before service dispatch.
Public Drive share-link token path values now preserve exact bearer-token
semantics by rejecting URL-decoded surrounding whitespace, embedded whitespace,
and non-printable ASCII before limiter or service dispatch instead of trimming
tokens at the HTTP boundary.
Public Drive shared-file downloads now have explicit invalid-range parity with
authenticated Drive downloads: malformed or unsatisfiable single-range requests
return HTTP 416 with `Content-Range: bytes */<size>`, avoid full/range object
opens after stat, and record a bounded public-share audit result.
Admin console capability OpenAPI security now explicitly documents both
`X-Admin-Token` and bearer-token alternatives, with runtime coverage that the
bootstrap endpoint accepts each form and rejects ambiguous mixed credentials.
API usage export capability discovery now carries the same explicit Admin API
server and admin-auth OpenAPI metadata, with runtime coverage for both accepted
credential forms and ambiguous mixed-credential rejection.
Admin readiness bootstrap operations now follow that same OpenAPI contract for
API usage ledger retention readiness, DAV sync retention readiness, and API
usage export handoff readiness, preventing generated operator clients from
probing the Mail API base for admin-only readiness checks.
API usage ledger list, export, and stats operations now also pin the Admin API
server and document both admin-token and bearer-token alternatives in OpenAPI,
matching the runtime `adminAuth` boundary under `/admin/v1` and preventing
generated operator clients from drifting to the public Mail API base.
API usage daily and monthly aggregate operations now carry the same Admin API
server and admin-auth metadata, keeping generated operator analytics clients
aligned with the runtime `/admin/v1` route boundary.
API usage export batch list/create/detail/export operations now also pin the
Admin API server and admin-auth alternatives in OpenAPI, matching their
runtime `/admin/v1` admin-authenticated route boundary.
API usage export artifact list/create/detail/write/download/verification
operations now carry the same operation-level Admin API server and admin-auth
alternatives, keeping sensitive export artifact access generated under
operator-only routes.
API usage export manifest digest and manifest signature operations now also
pin the Admin API server and admin-auth alternatives in OpenAPI, keeping
audit/export proof material aligned with the runtime operator-only boundary.
Core operator routes for queue stats, delivery route counters, and IMAP UID
backfill now also pin the Admin API server and admin-auth alternatives in
OpenAPI, preventing generated clients from treating operational repair and
diagnostic calls as public Mail API routes.
Tenant, domain, and user administration operations now also pin the Admin API
server and admin-auth alternatives in OpenAPI, keeping organization identity,
domain policy, DNS verification, user lifecycle, password-hash, and quota
controls generated under the runtime operator boundary.
Outbox event, audit log, Directory principal/alias/delegation/group
membership, and SMTP backpressure administration operations now also pin the
Admin API server and admin-auth alternatives in OpenAPI, keeping operational
forensics, identity resolution, delegated access, and emergency flow-control
calls generated under the operator boundary that backs future CalDAV,
CardDAV, shared mailbox, and admin-console workflows.
Quota pressure, attachment upload cleanup, Drive upload session, Drive node,
Drive usage, and Drive object cleanup failure administration operations now
also pin the Admin API server and admin-auth alternatives in OpenAPI, keeping
storage portability and Drive operations generated under the operator-only
boundary for local, NFS, MinIO, and S3-compatible deployments.
API usage ledger retention run and DAV sync retention run administration
operations now also pin the Admin API server and admin-auth alternatives in
OpenAPI, keeping destructive or audit-recorded retention workflows generated
under operator-only routes after readiness checks.
Quota reconciliation, delivery attempt, exhausted delivery attempt, and push
notification attempt/statistics administration operations now also pin the
Admin API server and admin-auth alternatives in OpenAPI, keeping operational
observability and provider outcome updates generated under operator-only
routes.
Suppression list, trusted relay, delivery route, DKIM key, DKIM DNS
verification, and outbox retry administration operations now also pin the
Admin API server and admin-auth alternatives in OpenAPI, keeping outbound mail
control, relay trust, domain signing, and retry operations generated under
operator-only routes.
OpenAPI contract coverage now also derives registered `/admin/v1` routes from
`admin.go` and verifies every matching operation pins the Admin API server and
both admin-token/bearer auth alternatives, so future admin routes cannot drift
back to ambiguous generated-client base/auth contracts silently.

IMAP hardening continues as a release-readiness track. `STATUS` and advertised
RFC 5819 `LIST-STATUS` now reject empty parenthesized status data-item lists,
including spaced forms such as `( )`, with explicit tagged `BAD` diagnostics
instead of treating them as generic arity failures, keeping RFC-shaped client
diagnostics predictable without changing valid status item handling. `STATUS`
status item lists now also reject malformed inner whitespace such as
`( UIDNEXT)` or `(UIDNEXT  RECENT)` instead of collapsing quoted/literal list
values into valid status data items, while LIST-STATUS keeps its existing
normalized return-option path regression-covered. `LIST RETURN` option lists
now reject whitespace-padded quoted or literal list values such as
`RETURN " (CHILDREN) "` instead of trimming them into valid return controls.
Required IMAP mailbox targets now reject decoded empty names at the protocol
boundary for `SELECT`/`EXAMINE`, `STATUS`, `CREATE`, `DELETE`, `RENAME`,
`SUBSCRIBE`/`UNSUBSCRIBE`, `COPY`/`MOVE`, `UID COPY`/`UID MOVE`, and `APPEND`,
returning deterministic tagged `BAD` diagnostics before backend lookup,
mutation, or selected-state checks. `LIST`/`LSUB` reference and pattern
handling continues to allow empty mailbox strings where RFC mailbox discovery
semantics require root/pattern behavior.
IMAP `INBOX` special-name handling for mailbox mutation commands is now exact
case-insensitive matching without trimming decoded mailbox names. Quoted
mailbox names such as `" INBOX "` remain ordinary backend-bound mailbox names
instead of being misclassified as the RFC special `INBOX` namespace.
RFC 5258 `LIST-EXTENDED` selection option lists now consume the full
parenthesized option list and reject whitespace-padded quoted or literal list
values such as `" (SPECIAL-USE) "` instead of trimming them into valid
selection controls.
`SEARCH`, `SORT`, and `THREAD` charset arguments now reject whitespace-padded
quoted or literal control atoms such as `CHARSET " UTF-8 "`, and `THREAD`
algorithm arguments reject padded `ORDEREDSUBJECT` values instead of trimming
them into advertised capabilities. Exact quoted or command-literal
`ORDEREDSUBJECT` algorithm controls now also fail before state checks instead
of being dequoted into RFC 5256 thread algorithms. `SORT` criterion lists now reject leading,
trailing, or nested parenthesized atom-list shapes such as `( DATE)`, `(DATE )`,
and `((DATE))` before authentication or selected-mailbox checks; exact quoted
or command-literal sort criteria such as `"(DATE)"` now also fail before state
checks instead of being dequoted into valid RFC 5256 sort controls. `SELECT` and
`EXAMINE` now reject whitespace-padded quoted or literal `CONDSTORE`
select-param lists such as `" (CONDSTORE) "` instead of trimming them into
valid RFC 4551 select parameters. `SEARCH RETURN (...)` and `SORT`/`THREAD`
`RETURN (SAVE)` option lists now reject whitespace-padded quoted or literal
values such as `RETURN " (COUNT) "` or `RETURN " (SAVE) "` instead of trimming
them into valid ESEARCH/SEARCHRES controls. Exact quoted or command-literal
`RETURN` introducers and exact quoted/literal return option-lists such as
`"RETURN"` or `RETURN "(COUNT)"` are rejected on the same raw atom/list
boundary for `SEARCH`, `UID SEARCH`, `SORT`, `UID SORT`, `THREAD`, and
`UID THREAD`. `FETCH` and `UID FETCH` data items now reject whitespace-padded
quoted or literal values such as `" (FLAGS) "` or `" FLAGS "` instead of
trimming them into valid fetch attributes.
`THREAD` now also rejects
unsupported
algorithms before authentication or selected mailbox checks, so unsupported
extensions such as `REFERENCES` are reported at the syntax/capability boundary
consistently for normal and UID forms. IMAP literal parsing now has regression
coverage for suffixed literal markers, literal payloads followed by trailing
atom data, and unused literal payloads so malformed wire input remains in the
parser/framing layer instead of leaking into command handlers. Oversized lines
sent while an `IDLE` continuation is active now follow the same tagged `BAD`
plus `BYE` framing-error path as oversized ordinary command lines, giving
clients a deterministic close reason instead of a silent connection drop. IMAP
`APPEND` now resolves destination mailbox names to canonical mailbox IDs before
mutation and rejects appends to the currently `EXAMINE`-selected read-only
mailbox without calling the backend append path, while preserving
syntax-before-state diagnostics for malformed appends and `[TRYCREATE]` for
missing destinations. IMAP `ENABLE` now has regression coverage that duplicate
`CONDSTORE` requests emit a single `ENABLED CONDSTORE` token, preserving
RFC 5161 response stability for client capability retries. IMAP
`FETCH` and `UID FETCH` now also treat RFC-valid empty `HEADER.FIELDS ()` and
`HEADER.FIELDS.NOT ()` lists as real header-section requests, returning the
blank header terminator for include-empty requests and the full header block
for exclude-empty requests instead of silently falling through as unsupported
literal handling. The same empty header-field-list semantics now apply to
`message/rfc822` MIME-part sections such as `BODY[1.HEADER.FIELDS ()]` and
`BODY[2.HEADER.FIELDS.NOT ()]`, keeping nested forwarded-message preview
fetches consistent with top-level header fetch behavior. IMAP IDLE recovery
semantics are also regression-covered: any line other than standalone `DONE`
while idling returns a tagged `BAD` for the pending IDLE command, exits idle
state, and leaves the authenticated session usable for the next legal command.
Partial-window forms of the same empty top-level header-field-list requests,
such as `BODY.PEEK[HEADER.FIELDS ()]<0.1>` and
`BODY.PEEK[HEADER.FIELDS.NOT ()]<0.10>`, are now regression-covered so preview
clients get deterministic bounded literals. Nested `message/rfc822`
header-field partial fetches are also regression-covered for forwarded-message
previews, including non-empty `HEADER.FIELDS`, empty `HEADER.FIELDS`, and
empty `HEADER.FIELDS.NOT` windows on attached messages. IMAP `SEARCH HEADER`
now validates RFC-shaped header field names before authentication or selected
mailbox state, rejecting empty names, names with spaces, and colon-suffixed
field labels as syntax errors instead of treating them as successful empty
searches. `HEADER.FIELDS` filtering now also matches raw RFC 5322 field names
without trimming whitespace around the colon, so malformed stored headers such
as `Subject : value` are not silently repaired into `Subject` matches.
Unsupported IMAP search-key atoms such as vendor-specific
`X-GM-RAW` probes are now also rejected before authentication or selected
mailbox state, so unsupported criteria do not masquerade as valid stateful
searches. Unsupported `SEARCH`, `SORT`, and `THREAD` charset probes now return
their RFC-shaped `[BADCHARSET (US-ASCII UTF-8)]` diagnostics before
authentication or selected-mailbox checks, keeping client charset fallback
behavior deterministic even during capability probing. IMAP `STATUS` and
advertised `LIST-STATUS` now distinguish duplicated status data items from
unknown/unsupported items, including before authentication state checks, so
client diagnostics remain precise at the status-item grammar boundary.
`LIST-STATUS` now also rejects duplicated `STATUS` return options such as
`RETURN (STATUS (MESSAGES) CHILDREN STATUS (UNSEEN))`, preventing later status
lists from silently replacing earlier requested status data. IMAP
`LSUB` now also rejects LIST-EXTENDED-style option probes such as
`(SPECIAL-USE)` prefixes or `RETURN (...)` tails before authentication with a
dedicated tagged `BAD`, keeping subscribed-mailbox discovery on the RFC 3501
reference/pattern command shape while leaving extended `LIST` behavior
separate. IMAP `CAPABILITY` now advertises RFC 5258 `LIST-EXTENDED` alongside
`SPECIAL-USE` and RFC 5819 `LIST-STATUS`, matching the already implemented
extended `LIST` selection/return option semantics so standards-aware clients
can legally use those options. IMAP `LIST-EXTENDED` now also supports RFC 5258
`SUBSCRIBED` selection and `RETURN (SUBSCRIBED)`, reusing the subscribed
mailbox store for `LIST (SUBSCRIBED) ...` and marking `\Subscribed` only when
that return option is requested while preserving `CHILDREN`, `SPECIAL-USE`,
and `STATUS` return behavior. IMAP `LIST` and `LSUB` now also treat mailbox
patterns beginning with the hierarchy delimiter as root-absolute patterns
before matching the server's root-relative mailbox names, so probes such as
`LIST "Archive" "/INBOX"` still discover `INBOX` instead of being matched
against an impossible leading-slash mailbox name.
IMAP `LIST` reference names that begin with the hierarchy delimiter are now
normalized the same way before joining them with relative mailbox patterns, so
namespace/root-style probes such as `LIST "/Projects" "2026"` match the
root-relative `Projects/2026` mailbox rather than constructing an impossible
leading-slash internal path.
Failed authenticated `SELECT` or `EXAMINE` attempts now deselect the previously
selected mailbox before returning failure, so subsequent selected-state
commands cannot continue operating on stale mailbox state after a missing or
failed selection attempt.
IMAP `LIST` and `LSUB` now compile the decoded mailbox-pattern matcher once
per command and reuse it across mailbox rows and subscribed-parent inference,
avoiding per-mailbox regular-expression construction on large folder trees
without changing wildcard semantics. RFC 5258 `LIST-EXTENDED` now also
accepts parenthesized mailbox pattern lists, applies `RETURN` options such as
`STATUS` and `SUBSCRIBED` to the union of matching folders, and de-duplicates
overlapping pattern results before writing responses. The pattern-list parser
now also preserves quoted mailbox patterns containing spaces, so probes such as
`LIST "" ("Archive 2026" "INBOX") RETURN (STATUS (MESSAGES))` reach the
normal matcher/status path instead of failing during command field splitting.
It also accepts synchronizing and non-synchronizing literals as pattern-list
members immediately after `(`, so native clients can send the same spaced
mailbox pattern as `LIST "" ({12} "INBOX") ...` without losing the literal at
the command parser boundary. Those parenthesized literals now must remain
printable ASCII before being wrapped for list parsing, so control-bearing or
raw non-ASCII literal bytes cannot be normalized into a different mailbox
pattern. Embedded atom fragments such as `Archive{12}` remain rejected,
keeping literal markers token-delimited instead of widening the IMAP atom
grammar.
IMAP `COPY`/`UID COPY` now carry an explicit source UID to destination summary
mapping through the gateway, service, and PostgreSQL repository boundary, and
`MOVE`/`UID MOVE` now build UIDPLUS `COPYUID` source sets from the returned
move result source summaries. Advertised UIDPLUS response codes are therefore
generated from messages that were actually copied or moved rather than inferred
from the requested UID slice. Sparse UID copy/move probes such as
`UID COPY 7,999 Archive` and repository-level missing-UID move inputs are
regression-covered so nonexistent UIDs are ignored without polluting response
codes.
MOVE `COPYUID` response codes are now emitted as untagged `OK` responses
before `EXPUNGE` lines instead of being delayed until the tagged final `OK`,
matching RFC 6851's UIDPLUS interoperability guidance so clients receive the
source-to-destination UID mapping before source sequence numbers disappear.
IMAP sequence-set response rendering now compacts ascending UID runs such as
`7,8,9` into `7:9`, keeping UIDPLUS `COPYUID`, ESEARCH, and saved-search
response payloads smaller and closer to the RFC sequence-set grammar during
bulk operations.
IMAP destination mailbox metadata now carries `UIDNotSticky`, and
`COPY`/`UID COPY`/`MOVE`/`UID MOVE` omit UIDPLUS `COPYUID` response codes when
the destination mailbox reports non-sticky UIDs, aligning the advertised
UIDPLUS behavior with RFC 4315's non-persistent UID semantics.
`APPEND` results can now also report non-sticky UID stores and suppress
`APPENDUID` even when a backend returns UID metadata, keeping UIDPLUS response
codes meaningful across append, copy, and move operations.
IMAP `UID EXPUNGE` now has sparse/mixed regression coverage: protocol tests
exercise missing UID members, and PostgreSQL coverage verifies that only
existing `\Deleted` messages are expunged while unmarked and missing UIDs are
ignored.
IMAP saved SEARCHRES state now applies the same adjusted sequence-number
semantics as emitted multi-message `EXPUNGE` responses, preventing a batch
expunge from removing the wrong saved-search entry after earlier expunges shift
later message sequence numbers.
SEARCHRES `$` is now accepted as a bare `SEARCH` sequence-set criterion, so
clients can reuse saved search results through both `SEARCH $` and
`UID SEARCH $ ...` forms instead of being limited to `FETCH $` or explicit
`UID $` criteria. The same saved-result reuse is regression-covered through
`SORT`, `UID SORT`, `THREAD`, and `UID THREAD` search criteria. SEARCHRES
`$` reuse now requires an exact `$` atom for sequence-set and UID-set helpers,
so whitespace-padded quoted or literal values are rejected instead of being
normalized into saved-result references. IMAP `CLOSE` now also clears the
selected-session saved SEARCHRES `$` state while tearing down selected mailbox
state, keeping saved results scoped to the mailbox selection lifecycle just
like `SELECT`, `EXAMINE`, and `UNSELECT`. Deleting the currently selected
mailbox now follows the same selected-state teardown boundary, including saved
SEARCHRES cleanup and event subscription closure.
IMAP `RENAME` now also resolves the source mailbox wire name through mailbox
lookup before calling the backend rename boundary, matching the canonical-ID
behavior already used by `DELETE`, `APPEND`, `COPY`, and `MOVE`.
`ENABLE CONDSTORE` issued after selecting a mailbox without persistent
mod-sequences now records the selected `NOMODSEQ` state as well as emitting the
untagged `[NOMODSEQ]` response, so later MODSEQ-dependent commands are rejected
instead of reaching fetch/search/store execution.
`SELECT`/`EXAMINE` subscription setup is now guarded against mid-response write
failures: a newly opened mailbox event subscription is canceled unless it has
been installed into connection state, preventing leaked event listeners on
broken clients.
When `RENAME` is applied to the currently selected mailbox and the backend
returns a new canonical mailbox ID, the IMAP session now moves selected state
and mailbox event subscription to that returned ID while preserving saved
SEARCHRES sequence results for the still-selected mailbox. That selected-state
handoff now also refreshes `HIGHESTMODSEQ`/`NOMODSEQ` metadata from the
backend-returned mailbox, preventing stale persistent-mod-sequence state after
a selected mailbox rename.
IMAP mailbox event publishing now sends to matching subscription channels
while holding the broker lock, preserving non-blocking slow-subscriber behavior
while closing the race where a concurrent subscription cancel could close a
snapshotted channel before publish delivered an `EXISTS`/`EXPUNGE` update.

Storage portability hardening continues across local/NFS, MinIO, and AWS S3
deployments. `GOGOMAIL_STORAGE_BACKEND=nfs` now acts as an explicit alias for
the local filesystem adapter and registers bidirectional `local`/`nfs`
compatibility labels for Drive rows, so operators can make NFS-backed mounts
visible in config without changing object-key semantics. Production `s3`
runtime configuration now requires an explicit
`GOGOMAIL_STORAGE_S3_ENDPOINT`, even for AWS regional endpoints, so release
configs show the object-storage target directly while development/test configs
can still use region-based endpoint derivation. Production `s3` endpoints must
also use HTTPS, keeping SigV4 `UNSIGNED-PAYLOAD` request signing behind
transport integrity for AWS/S3-compatible object stores while local MinIO
development can still use the explicit `minio` backend with HTTP. S3-compatible runtime wiring
now also accepts a deployment-scoped custom CA bundle through
`GOGOMAIL_STORAGE_S3_CA_CERT_FILE` and a development-only
`GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY` escape hatch, constructing a
dedicated TLS 1.2+ HTTP client for private MinIO/S3 endpoints without changing
the storage adapter or default AWS behavior. Optional S3-compatible integration
coverage can now use matching test-only TLS variables
(`GOGOMAIL_TEST_S3_CA_CERT_FILE` and
`GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY`) so pre-release smoke tests can verify
the same private-object-store trust shape. Runtime startup also accepts
`--config=<path>` for a flat YAML overlay on top of the existing env/default
configuration, so operators can flip local/NFS/MinIO/AWS S3 storage profiles
through a reviewed config file while keeping the same startup validation. The
CLI startup seam now has regression coverage proving valid YAML reaches the app
runtime and invalid config/mode input fails before any component starts.
Validated storage profile overlays now exist for local filesystem, explicit
NFS, local MinIO, and AWS S3-style deployments under `configs/storage.*.yaml`,
with config-loader and CLI `--config` handoff coverage ensuring each profile
parses and passes startup validation before operators use it as a config-file
starting point. The NFS profile smoke coverage now verifies that its
`storage_root` and explicit `local` compatibility label survive both the
config-loader and CLI handoff paths, making local/NFS config-only flips less
dependent on manual review. YAML overlays now also accept `storage_root` as the
storage-focused alias for the local/NFS object root, matching the existing
`GOGOMAIL_STORAGE_ROOT` environment alias while keeping `mailstore_root`
backward-compatible.
Storage profile smoke coverage now also verifies MinIO and AWS S3 profile
region, bucket, prefix, and credential fields, so reviewed profile edits cannot
silently drop required object-storage settings while still passing endpoint-only
checks. The CLI `--config` profile handoff coverage now asserts the same S3
profile fields, preventing command startup tests from accepting incomplete
MinIO/AWS S3 profile data.

Actual Next.js frontend implementation has not started. When frontend work
starts, use Next.js with TypeScript, shadcn/ui, and the project `DESIGN.md` as
required guidance, aiming for a Notion Mail-like UI/UX. Before creating or
substantially implementing frontend apps, ask the user for frontend-specific
guidance.

Calendar work is planned as a standards-first CalDAV module, not as a
frontend-only calendar API. The initial `internal/caldavgw` boundary and
`caldav` runtime mode scaffold exist so future webmail calendar features and
native CalDAV/iCalendar clients can share a protocol-correct backend.

CalDAV remains an experimental/backend-only release slice: useful protocol
building blocks now exist, but the module is not yet advertised as public
client-ready. The next compatibility gates are broader recurrence edge cases
(complete), scheduling semantics, production retention-age policy, broader
native-client testing, and the platform boundaries below.
CalDAV `calendar-query` and `free-busy-query` now expand bounded VEVENT
recurrence sets from RFC 5545 `RRULE`/`EXDATE`/`RDATE` parsing when evaluating
time ranges, so recurring events can be discovered and included in VFREEBUSY
responses without treating recurrence as simple one-shot event metadata. The
expansion is capped per object to avoid unbounded CPU or memory use on dense or
unbounded recurrence rules. CalDAV iCalendar validation now also permits the
common recurring-object shape of one VEVENT master plus same-UID
`RECURRENCE-ID` detached override VEVENTs, and time-range/free-busy evaluation
uses detached overrides while excluding the replaced master occurrences.
CalDAV recurrence expansion now covers broader RFC 5545 edge cases including
`FREQ=WEEKLY;BYDAY=MO,WE,FR`, `FREQ=MONTHLY;BYMONTHDAY=15`,
`FREQ=YEARLY;BYMONTH=6,12`, ordinal BYDAY patterns like `1SA` (first Saturday),
`INTERVAL` modifiers, and timezone-aware matching through the calendar's
configured timezone. Unit tests verify WEEKLY/MONTHLY/YEARLY expansion,
BYDAY/MONTHDAY patterns, complex rules, timezone-aware matching, and
INTERVAL modifiers work correctly. Timezone-aware busy period calculation is
also verified with a dedicated test for `CalendarObjectBusyPeriods` using
a non-UTC timezone. VTODO recurrence expansion via RRULE is now implemented
per RFC 5545: `todoOverlapsRange` parses RRULE and iterates recurring
occurrences with `todoOccurrenceTimeSpan` computing effective end times from
DTSTART+DURATION or DTSTART+DUE. Override exclusions (RECURRENCE-ID) and
EXDATE handling via `todoOverrideRecurrenceIDs` is also implemented.
CalDAV `calendar-query` and `free-busy-query` now interpret time ranges in the
calendar's configured timezone (RFC 7809 Section 5.2 `calendar-timezone`
property) when evaluating VEVENT/VTODO overlap. `CalendarObjectMatchesTimeRange`,
`eventOverlapsRange`, `todoOverlapsRange`, and `CalendarObjectBusyPeriods` now
accept an optional `*time.Location` parameter; when nil, UTC is used as the
default. `calendarQueryResponses` and `freeBusyCalendar` in `handler.go` now
look up the calendar's timezone and pass it to these functions. This implements
RFC 7809 Section 5.3 requirement that calendar time ranges be interpreted in the
calendar's configured timezone rather than always UTC.
 CalDAV scheduling iMIP (RFC 6047) is now fully operational: the outbox relay
 publishes all events to `cfg.EventStream` (default `mail.event`) ensuring
 scheduling.outbox events reach the event worker. The `scheduling` package handler
 extracts ATTENDEE and ORGANIZER from iCalendar payloads (using direct Value field
 access for CAL-ADDRESS type properties) and builds RFC 6047 compliant iTIP
 messages with multipart/alternative MIME format. The DeliveryQueue publishes directly
to `mail.outbound.general` for SMTP delivery by the delivery worker.
 CalDAV scheduling now includes attendee resolution via Directory and CardDAV: the
 `scheduling.Handler` accepts an optional `AttendeeResolver` that chains Directory
 user lookup (`ResolveUserByEmail`), Directory alias resolution, and CardDAV contact
 email search to classify each ATTENDEE address as internal-user, directory-alias,
 carddav-contact, or external. The `DefaultAttendeeResolver` implements this chain
 using exact `user_addresses` lookup for internal users, `directory_aliases` for
 alias addresses, and vCard EMAIL property search across the user's address books for
 CardDAV contacts. Resolution is logged per-attendee and included in iTIP delivery
 metadata, enabling future internal routing without changing the existing SMTP path.
 CalDAV `REPORT sync-collection` now also enforces the RFC 6578 HTTP request
scope by accepting the default/explicit `Depth: 0` only, keeping WebDAV sync
traversal controlled by the request body's `sync-level` rather than accidentally
mixing in a broader HTTP `Depth: 1` traversal. CalDAV `calendar-query` now also
honors HTTP `Depth: 0` by not scanning child calendar objects unless the client
uses `Depth: 1`, keeping query scope explicit for native client compatibility.
CalDAV `REPORT` and `PROPFIND` now reject repeated HTTP `Depth` headers before
request-body parsing, keeping WebDAV traversal scope deterministic across
native clients and intermediaries.
CalDAV `calendar-query` parsing now rejects malformed RFC 4791 component
filters that omit the required `name` attribute, use a non-`VCALENDAR`
top-level component, omit the top-level component filter, or send multiple
top-level component filters, so native-client search/filter requests fail at
the protocol grammar boundary instead of being silently widened to
whole-calendar matches. The parser now also rejects `time-range` elements
placed directly under `filter` and duplicate `time-range` elements within the
same component filter, avoiding ambiguous range semantics instead of accepting
the last matching XML element.
CalDAV object and collection preconditions now evaluate repeated `If-Match`
and `If-None-Match` headers as a single ETag list, so cache validation and
write guards match HTTP field-combination semantics instead of depending on the
first wire header. Date-based CalDAV conditionals now reject repeated
`If-Modified-Since` or `If-Unmodified-Since` headers before object reads,
object writes, object deletes, or collection precondition checks, avoiding
ambiguous timestamp guards that used to depend on the first header value.
CalDAV object `PUT` and `DELETE` also carry observed strong ETags into the
repository request when `If-Match` succeeds, including the existing-resource
`If-Match: *` form, and revalidate them inside mutation transactions before
write or soft deletion.
CalDAV mutating repository paths now enqueue a transactional `dav.event`
outbox row whenever they append a durable calendar sync-change row. The v1
`calendar.changed` payload carries the DAV kind, action, owner user, actor
user, delegation flag, collection, object name, ETag, sync token, and changed
timestamp, giving future Notification & Sync, search indexing, reminders, and
mobile delta fan-out a clean event-stream boundary without making CalDAV call
push/vendor adapters directly.
The generic event worker now has `calendar.changed` and `contacts.changed`
audit handlers, so a worker instance pointed at `GOGOMAIL_EVENT_STREAM=dav.event`
can validate those payloads and write durable DAV audit rows with actor/owner
context without coupling reminder, push, or indexing decisions into the
protocol gateway.
CalDAV `sync-collection` parsing now also requires an explicit `DAV:sync-token`
element while preserving empty-token initial sync semantics, avoiding ambiguous
requests that omit the sync state anchor entirely.
Stale-token sync delta handling now probes one row beyond the requested
`limit/nresults`, so exact-limit change sets can complete while genuinely
truncating delta responses still fail closed until continuation semantics are
implemented. Initial sync snapshots now use the same bounded one-extra-object
probe through the PostgreSQL discovery repository, preventing large collections
from being silently truncated while still returning a current collection sync
token.

Calendar product features must not grow as isolated CRUD. Before delegated
calendars, shared ownership, attendees, resource booking, reminders, or
organization calendars become public features, the project should establish
clear Directory/Identity, Contacts/CardDAV, Notification & Sync, Search, and
Policy/Audit boundaries. Directory is the platform/org layer for users, teams,
groups, aliases, resources, memberships, delegation, and principal resolution;
Contacts/CardDAV is the user-owned address-book layer for personal/external
people and user-specific metadata.
CalDAV principal discovery now maps the Directory primary email plus active
user-targeted Directory aliases into the RFC 4791 `calendar-user-address-set`
property as normalized deduplicated `mailto:` hrefs when available. This is
intentionally a principal identity/scheduling foundation, not public scheduling
support: organizer/attendee workflows, resource booking, and shared calendar
access remain gated on Directory policy, Contacts/CardDAV linkage, and audit
semantics.
CalDAV now has its first explicit delegated-access integration point: handler
authorization can distinguish authenticated actor and resource owner user IDs,
resolve read requests against the owner calendar store, and call a pluggable
access authorizer instead of hard-coding every cross-user path as forbidden.
Runtime `caldav` mode wires that authorizer through Directory active principal
resolution, `accesspolicy.DelegatedAccessAuthorizer`, and the shared audit
repository for read/write/manage role checks. Delegated `PROPFIND` responses
now keep `DAV:current-user-principal` anchored to the authenticated delegate
while owner hrefs and storage lookups remain owner-scoped. They also derive
`DAV:current-user-privilege-set` from the same access-policy boundary, so
read-only delegates are not incorrectly shown bind/unbind or write privileges.
Delegated REPORT and sync responses now use the same privilege shaping for
calendar-object properties, and missing Directory principals fail closed as
access denial instead of surfacing as a distinct server-error path.
The delegated policy boundary also verifies that resolved owner and actor
principals are still `user` principals before audit or delegation checks run,
so future organization, group, and resource principals cannot accidentally flow
through the user-calendar storage path.
This is still a foundation, not full public shared-calendar support: delegated
write/manage UX, resource booking, scheduling policy, and broad native-client
sharing tests remain release gates.

Contacts/CardDAV work has started as a standards-first backend boundary, not a
generic contacts CRUD API. The initial `internal/carddavgw` package defines
RFC/WebDAV/CardDAV tokens, canonical principal, address-book home,
address-book collection, and `.vcf` contact-object path/href handling, plus
metadata validation for address books, contact object names, UIDs, strong
ETags, size limits, sync tokens, and bounded vCard 3.0/4.0 semantic checks.
The vCard content-line parser now treats the value separator as the first
unquoted colon, so quoted parameters such as address labels can contain colons
without being rejected as malformed cards.
PostgreSQL storage groundwork now exists for address books, contact objects,
and address-book change logs. A first repository boundary can create/list/get
address-book collections through active user/domain/company scope and records
address-book creation changes. Contact-object repository methods can now
upsert/list/get/delete `.vcf` resources through active address-book scope,
using bounded vCard validation, strong ETags, optional observed-ETag guards,
same-address-book UID uniqueness preflight before upsert, sync-token updates,
and durable change rows. Public CardDAV compatibility now
has bounded WebDAV `PROPFIND` parsing, an internal `OPTIONS`/`PROPFIND`
discovery handler, bounded REPORT request parsing, and internal REPORT
execution for `addressbook-query`, `addressbook-multiget`, and
`sync-collection`. `addressbook-multiget` now requires an explicit `Depth`
header before resolving requested hrefs, matching the REPORT's depth-scoped
request model while retaining 0/1 client compatibility. `addressbook-query`
now preserves the first
`prop-filter` property name and applies `text-match` to parsed unfolded vCard
property values instead of only scanning the whole object body. It also honors
the RFC 6352 default `i;unicode-casemap` collation plus `equals`, `contains`,
`starts-with`, `ends-with`, and `negate-condition` text-match attributes, and
evaluates nested `param-filter` entries against parsed vCard parameter values.
It now honors RFC 6352 `test=anyof|allof` composition on the top-level
`filter` and individual `prop-filter` elements. `addressbook-query` now rejects
unsupported vCard property or parameter filters with the RFC 6352
`CARDDAV:supported-filter` precondition instead of returning misleading empty
success responses, even for `Depth: 0` requests that otherwise return no child
objects. Unsupported CardDAV filter child elements use the same
`CARDDAV:supported-filter` precondition instead of a generic bad request.
CardDAV filter parsing now rejects duplicate `text-match` elements inside a
single `prop-filter` or `param-filter`, matching RFC 6352's singular
text-match grammar instead of silently widening or ignoring filter predicates.
REPORT `address-data` can now
project returned vCards to requested property names while preserving structural
BEGIN/VERSION/END lines and rejects unsupported requested `content-type` or
`version` values with the RFC 6352 `CARDDAV:supported-address-data`
precondition instead of returning data under an unimplemented format contract.
Unsupported text-match collations now fail with the RFC 6352
`CARDDAV:supported-collation` precondition instead of a generic parse error.
Address-book collections advertise RFC 6352
`CARDDAV:supported-collation-set` with `i;ascii-casemap` and
`i;unicode-casemap`, and query evaluation implements both advertised
collations. Capability properties that should not appear in a bare `allprop`
response remain available through explicit `prop`, `include`, and `propname`
discovery.
Address-book collections advertise `text/vcard` 4.0 and 3.0 support, and
returned `address-data` elements carry explicit `content-type` plus the stored
vCard `version` attribute. Contact-object `PUT` also validates explicit
`text/vcard` `version` media-type parameters against the stored vCard
`VERSION`, rejecting repeated `Content-Type` headers plus unsupported or
mismatched version contracts before write mutation.
`addressbook-query` execution now honors bounded `limit/nresults` responses so
large address books can be queried with explicit result caps, and repository
backends can stream contact objects through a walker interface so matching can
stop once the response cap is reached instead of materializing the whole
collection. Address-data projection failures are returned as explicit errors
instead of silently falling back to full contact bodies. RFC 6352
`addressbook-query` now requires an explicit `Depth` header; `Depth: 1` scans
address-object children, `Depth: 0` stays collection-scoped without returning
child objects, and `Depth: infinity` is rejected before XML body parsing so
native-client traversal probes cannot trigger broad address-book scans. It
remains gated on broader vCard compatibility and native-client tests. The
handler is deliberately experimental and does not yet make CardDAV
public/client-ready.
CardDAV `REPORT` and `PROPFIND` now reject repeated HTTP `Depth` headers
before request-body parsing, preventing ambiguous address-book traversal scope
from reaching REPORT, sync, or discovery execution.
CardDAV object and address-book collection preconditions now evaluate repeated
`If-Match` and `If-None-Match` headers as a single ETag list, preserving HTTP
conditional request semantics for cache validation, object writes, deletes, and
collection metadata mutations. Date-based CardDAV conditionals now reject
repeated `If-Modified-Since` or `If-Unmodified-Since` headers before contact
object reads, contact object writes, contact object deletes, or collection
precondition checks, avoiding ambiguous timestamp guards and matching the
CalDAV HTTP conditional boundary.
`gogomail --mode=carddav` now starts a dedicated CardDAV HTTP listener with
Basic-auth backed by the existing Submission authenticator. WebDAV multistatus
response building is available for CardDAV principal, address-book collection,
contact-object, REPORT, and sync responses.
CardDAV now resolves the advertised `/carddav/principals/` principal collection
for `PROPFIND`, returning collection metadata at `Depth: 0` and the
authenticated principal as a `Depth: 1` child without exposing other users. The
Directory-to-CardDAV conversion boundary now explicitly accepts only Directory
user principals, keeping organization, group, and resource principals out of
the user-owned address-book path until their CardDAV semantics are designed.
CardDAV PROPFIND responses now also expose a conservative RFC 3744-shaped
`current-user-privilege-set`: readable resources advertise `DAV:read`, and
address-book collections now also advertise `DAV:bind`/`DAV:unbind` for child
contact-object `PUT`/`DELETE` semantics plus `DAV:write-properties` because
collection `PROPPATCH` semantics exist. Contact objects advertise
`DAV:write-content` because object `PUT`/`DELETE` semantics exist. ACL and
unimplemented write privileges remain unadvertised until their exact WebDAV
semantics are implemented. Address-book homes advertise `DAV:bind` because
extended `MKCOL` can create child address-book collections there and
`DAV:unbind` because collection `DELETE` can remove them.
Address-book collection discovery also exposes the CalendarServer-compatible
`getctag` extension from the same durable sync token used for WebDAV
`sync-token`, improving legacy native-client change detection without adding a
second collection-version source of truth.
It also returns RFC 6352 `addressbook-description` from the stored address-book
metadata so client-visible collection discovery stays aligned with repository
state.
Address-book collection `PROPFIND Depth: 1` child-object discovery is now
bounded with the shared one-extra-row truncation probe, rejecting partial
contact metadata listings explicitly instead of returning incomplete
multistatus bodies.
CardDAV now handles WebDAV `PROPPATCH` for authenticated address-book
collection metadata, parsing bounded namespace-aware `propertyupdate` bodies
for `DAV:displayname` and RFC 6352 `addressbook-description`. Updates flow
through a small repository boundary, refresh the durable sync token, append an
`addressbook-updated` change row, reject unsafe targets, and keep contact-object
I/O separate from collection metadata mutation.
Address-book collections now derive a strong collection ETag from the durable
sync token, expose it through PROPFIND `getetag`, and enforce `If-Match` and
`If-Unmodified-Since` on collection `PROPPATCH` before reading request bodies.
CardDAV now handles RFC 6352-style extended `MKCOL` for authenticated
address-book collection creation at UUID request-URI paths. The handler parses
bounded WebDAV creation XML for `DAV:resourcetype`, `DAV:displayname`, and
`CARDDAV:addressbook-description`, rejects existing collections, cross-user
paths, missing homes, and unsafe non-UUID path ids before body reads, creates
the collection through the repository, refreshes sync state, and returns
`201 Created` with `Location`.
CardDAV now handles `DELETE` on authenticated address-book collection paths,
soft-deleting the collection and active child contact objects in one repository
transaction, honoring collection `If-Match`/`If-Unmodified-Since` preconditions,
recording an `addressbook-deleted` change row, and rejecting home/object/cross-
user targets. Address-book homes now also advertise `DAV:unbind` because child
collection delete semantics exist.
CardDAV `sync-collection` can now answer stale-token requests after an address
book collection has been deleted by reading the durable change log and returning
the latest deletion sync token without requiring the collection to remain active.
It now enforces RFC 6578 Depth behavior for `sync-collection`, accepting the
default/explicit `Depth: 0` request scope and rejecting `Depth: 1` before sync
work. `sync-collection` parsing also distinguishes an empty initial
`DAV:sync-token` element from a missing element and rejects requests that omit
the required token element. Stale-token change delta handling now probes one
row beyond bounded `limit/nresults`, matching CalDAV so exact-limit address
book changes are not falsely rejected while genuinely truncating deltas still
fail closed. CardDAV sync-change retention now also has a bounded
`PruneAddressBookChanges` repository boundary plus a prune-order index that can
dry-run or delete old address-book change rows while preserving the newest
marker per address book, and `dav-sync-retention-worker` runs that prune path
on the same dry-run-by-default schedule as CalDAV. Unknown or expired CardDAV
sync tokens fail with DAV `valid-sync-token`; deployment retention-age policy
and native-client expiry testing remain future work.
CardDAV mutating repository paths now mirror the same transactional outbox
boundary: each durable address-book sync-change row also queues a `dav.event`
row with a v1 `contacts.changed` payload. Those payloads now preserve owner
user, actor user, and delegated-vs-direct context for contact-object and
address-book mutations, matching the CalDAV event boundary so future Contacts,
Notification & Sync, search, mobile delta, and audit consumers can distinguish
who changed whose address book without coupling product logic into CardDAV.
Contacts/CardDAV remains the user-owned address-book source of truth, while
Notification & Sync can consume domain events asynchronously rather than
reaching into CardDAV mutation code.
Initial address-book sync snapshots now use a sync-specific
bounded object list path as well, so a large address book cannot be clipped by
the generic repository list default and then reported as fully synchronized.
CardDAV contact-object `PUT` now rejects duplicate active vCard UIDs within the
same address book before the SQL upsert path, while the PostgreSQL partial
unique index remains the final concurrency guard. Repository error mapping also
turns final unique-index races into predictable duplicate UID/name failures
instead of leaking raw driver details.
CardDAV contact-object `PUT` and `DELETE` now carry observed strong ETags into
the repository transaction when `If-Match` succeeds, including
`If-Match: *`, so contact writes and deletes are rechecked under the
address-book lock before the active object row is changed.
CardDAV now has its first explicit delegated contacts access integration point:
handler authorization distinguishes authenticated actor and address-book owner,
uses the owner store when a delegated read/write/manage decision allows access,
and runtime `carddav` mode wires the authorizer through Directory active
principal resolution, `accesspolicy.DelegatedAccessAuthorizer`, and the shared
audit repository using the `contacts` delegation scope. Delegated PROPFIND
responses keep `DAV:current-user-principal` anchored to the authenticated
delegate while owner hrefs and storage lookups remain owner-scoped. They also
derive `DAV:current-user-privilege-set` from the same policy boundary, so
read-only delegates see read-only CardDAV/WebDAV privileges rather than
owner-level bind/unbind/write capabilities; REPORT and sync responses use the
same delegated privilege shaping for contact-object properties. Missing
principals fail closed as access denial instead of exposing a different
server-error path. The CardDAV delegated policy boundary also requires resolved
owner and actor principals to remain `user` principals before audit or
delegation checks run, keeping organization, group, and resource principals out
of the personal address-book storage path until explicit product semantics
exist. This remains an
experimental/backend-only capability: public contacts sharing UX, native-client
shared address-book compatibility, and group/resource/person product workflows
remain release gates.

The first Directory/Identity slice now exists as `internal/directory`: it owns
bounded platform-principal identifiers, principal kinds, active user principal
resolution over user/domain/company state, and organization principal
resolution over organization/domain/company state. Directory schema groundwork
also covers groups, resources, aliases, and group memberships, with resolver
support for group and resource principals plus normalized alias-to-principal
lookup and direct group-membership checks. Active aliases are globally unique
by normalized address. CalDAV discovery uses this shared resolver instead of
embedding its own active-user join, but delegated access, shared calendar
ownership, attendee resolution, and resource booking semantics remain future
release gates.
Directory/Identity now also has an initial delegation table and repository
check boundary keyed by company, owner principal, delegate principal, product
scope (`calendar`, `contacts`, `drive`, or `mailbox`), and hierarchical role
(`read`, `write`, `manage`). This is intentionally not wired into public
shared-calendar or contacts-sharing UX yet; CalDAV runtime authorization can
now consume it for cross-user calendar access checks, and CardDAV runtime
authorization can now consume it for cross-user address-book/contact access
checks while future resource calendars, shared inboxes, and Drive shares keep
one auditable principal relationship model instead of each module inventing a
separate one. Effective
delegation can now expand group delegates through bounded nested membership, so
a group-granted delegation can satisfy user, organization, group, or resource
members while preserving active-only owner/delegate principal checks, group
filtering, role hierarchy, depth caps, and cycle guards. Direct delegation
checks now share the same active-only owner/delegate principal gate as effective
delegation checks, so policy callers do not honor otherwise-active delegation
rows after either endpoint principal is suspended or deleted. The first
`internal/accesspolicy` adapter wraps effective delegation as an explicit
allow/deny decision boundary so CalDAV, CardDAV, Drive, mailbox sharing, and
admin APIs do not need to branch directly on Directory rows. It also provides
an RFC 4918-shaped WebDAV privilege mapper for delegated read/write/manage
decisions before those privileges are exposed by protocol modules. The same
boundary now emits bounded delegated-access audit detail JSON with normalized
company, owner, actor, scope, role, decision, reason, and WebDAV privilege
fields; reasons are fixed enum values so operational logs stay predictable
instead of accumulating caller-supplied strings. It also builds the standard
`audit.Log` envelope for delegated access checks (`access` /
`delegation.access_checked` with fixed `allowed`/`denied` results), giving
future CalDAV/CardDAV/Drive/mailbox adapters one auditable shape to insert.
Directory now exposes a bounded `SearchPrincipals` repository boundary over
users, organizations, groups, and resources. The search request validates
company/domain/organization scope, deduplicates allowed principal kinds, caps
query size and result count, and escapes SQL `LIKE` wildcard input before
querying. This is the intended foundation for CalDAV attendee/resource lookup,
contacts auto-complete, shared inbox targeting, and admin consoles without
letting product modules invent their own principal search semantics.
Directory also exposes a bounded `ListDelegations` read boundary for
owner/delegate/scope/role-filtered delegation inspection. This keeps admin
console diagnostics, shared-calendar management, Drive sharing, shared inboxes,
and future Contacts/CardDAV delegation on the same normalized company-scoped
relationship model instead of forcing product modules to query
`directory_delegations` directly. The CalDAV release tier should therefore stay
experimental while the platform-level Directory/Identity, Contacts/CardDAV,
Notification & Sync, Search, and Policy/Audit foundations continue to close.
Directory delegation creation now has the same transaction-audited mutation
shape: `CreateDelegationWithAudit` normalizes owner/delegate principal kinds,
scope, and role, rejects self-delegation, verifies both principals are active in
the same company, maps active duplicate grants to a stable error, and records
`directory_delegation.create` in the same transaction as the grant insert.
Directory delegation role changes now use the same audited mutation boundary
and admin API shape: `UpdateDelegationRoleWithAudit` normalizes grant IDs and
roles, updates only active grants under active companies, records
`directory_delegation.role_update` with previous/new role detail in the same
transaction, and is exposed as
`PATCH /admin/v1/directory/delegations/{id}/role`. This lets CalDAV, Drive,
Contacts/CardDAV, and shared inbox access management evolve through one
predictable delegation lifecycle instead of product-local role semantics.
The Directory delegation schema now enforces one active grant per
company/owner/delegate/scope, independent of role, so role changes are true
mutations of a single relationship rather than parallel active grants with
conflicting privilege semantics.
Directory delegation reassignment now has the same lifecycle shape:
`ReassignDelegationWithAudit` verifies the new owner/delegate principals are
active in the same company, preserves the existing role, maps duplicate active
grants to the stable duplicate-delegation error, records
`directory_delegation.reassign`, and is exposed as
`PATCH /admin/v1/directory/delegations/{id}/assignment`.
Directory group membership creation now follows that same platform boundary:
`CreateGroupMembershipWithAudit` normalizes membership roles
(`member|manager|owner`), verifies the active group and member principal belong
to the same company, rejects self-membership and nested group cycles, maps
active duplicate memberships to a stable error, and records
`directory_group_membership.create` with the membership insert.
Directory alias listing is now a bounded repository boundary as well:
`ListAliases` validates company/domain scope, target principal filters, query
size, active-only state, and result limits before querying, then resolves each
bounded alias row back to its target principal. This prepares shared inbox
management, mail-routing diagnostics, and admin alias screens without exposing
raw `directory_aliases` SQL to product modules.
Directory alias creation now has the same repository-owned policy boundary:
`CreateAliasWithAudit` normalizes the address, requires an active
company/domain, checks that the alias address belongs to that Directory domain,
resolves an active target principal in the same company, maps the
active-address unique-index race to a stable duplicate-alias error, and writes
the `directory_alias.create` admin audit row in the same transaction as the
alias insert.
The admin backend API now exposes that read boundary as
`GET /admin/v1/directory/delegations`, returning
`{"directory_delegations":[...]}` with bounded company, owner, delegate, scope,
role, active-only, and limit filters. This is intentionally an operator/admin
diagnostic surface, not a public CalDAV sharing UX.
Audited delegation creation is also exposed through
`POST /admin/v1/directory/delegations`, returning
`{"directory_delegation":{...}}`; this is a platform admin relationship
operation and still does not make delegated CalDAV/CardDAV/Drive/shared-inbox
UX public.
Audited delegation deletion is exposed through
`DELETE /admin/v1/directory/delegations/{id}`, soft-deleting an active grant
and recording `directory_delegation.delete` in the same transaction.
Directory group membership listing is exposed through
`GET /admin/v1/directory/group-memberships`, returning
`{"directory_group_memberships":[...]}` with bounded company, group, member,
role, active-only, and limit filters for operator diagnostics.
Audited group membership creation is exposed through
`POST /admin/v1/directory/group-memberships`, returning
`{"directory_group_membership":{...}}` for group-backed delegation management.
Audited group membership deletion is exposed through
`DELETE /admin/v1/directory/group-memberships/{id}`, soft-deleting an active
membership and recording `directory_group_membership.delete` in the same
transaction. This gives group-backed delegation and future shared inbox/resource
access one auditable revoke path instead of product-local membership mutation.
Audited group membership role updates are exposed through
`PATCH /admin/v1/directory/group-memberships/{id}/role`, recording
`directory_group_membership.role_update` with the role change in the same
transaction.
Audited group membership reassignment is exposed through
`PATCH /admin/v1/directory/group-memberships/{id}/assignment`, preserving the
role while moving the active membership to a different group/member assignment
and recording `directory_group_membership.reassign` in the same transaction.
Admin APIs also expose bounded Directory principal search through
`GET /admin/v1/directory/principals`, returning
`{"directory_principals":[...]}` for company-scoped user, organization, group,
and resource lookup. This gives the future admin console, CalDAV attendee and
resource lookup, Contacts/CardDAV autocomplete, Drive sharing, and shared inbox
targeting one contract-first principal discovery path instead of product-local
search endpoints.
Admin alias diagnostics now use the same Directory boundary:
`GET /admin/v1/directory/aliases/resolve` normalizes an email address and
returns the matching alias plus its target principal. This keeps mail routing,
attendee resolution, shared inbox targeting, and operator debugging on the same
alias-to-principal contract instead of duplicating address parsing.
Admin alias listing now exposes `ListAliases` through
`GET /admin/v1/directory/aliases`, returning `{"directory_aliases":[...]}` with
bounded company, domain, target principal, query, active-only, and limit
filters. Future admin alias screens and shared inbox management can now use the
same alias inspection contract as the repository boundary.
Admin alias creation is now exposed through
`POST /admin/v1/directory/aliases`, returning `{"directory_alias":{...}}` after
the audited Directory mutation boundary succeeds. This is still a platform
admin operation, not a public shared-inbox UX.
Admin alias deletion is exposed through
`DELETE /admin/v1/directory/aliases/{id}`, soft-deleting an active alias through
the same transaction-audited Directory boundary and recording
`directory_alias.delete`.
An `accesspolicy` recorder can now insert those delegated-access audit logs
through the shared audit repository interface, keeping future protocol modules
on one testable policy/audit boundary instead of open-coding audit writes.
`accesspolicy` now also provides a composed delegated-access authorizer that
normalizes the request once, checks effective delegation, records the resulting
allow/deny audit envelope, and fails closed on checker or audit insertion
errors so future CalDAV/CardDAV/Drive/mailbox adapters do not accidentally
permit unaudited delegated access.
The admin audit-log list API now accepts bounded `actor_id` and `target_id`
filters, backed by partial actor/time and target/time read indexes, so
operators can trace delegated-access checks by acting principal or
owner/resource target without scanning unrelated audit history.

## Completed or materially advanced

- SMTP receive engine with real TCP integration coverage.
- Authenticated Submission MTA with STARTTLS and SMTPS support.
- Outbound SMTP delivery with direct MX, smart-host, TLS policy, retry, and
  partial recipient failure handling. Admin-created delivery routes reject
  impossible TLS/auth combinations before relay routes are stored.
  Static smart-host configuration now rejects password-only auth plus
  CR/LF-bearing or oversized auth username, password, and identity values during
  startup config validation.
- DSN/bounce handling with RFC 3461/3464-oriented metadata, null reverse-path,
  `NOTIFY=NEVER`, deterministic outbox dedupe, retry-exhaustion failure
  notifications, and loop-risk reduction.
- Shared high-performance-minded EML parsing boundary under `internal/message`.
- PostgreSQL metadata model for companies, domains, users, folders, messages,
  attachments, outbox, audit logs, DKIM keys, trusted relays, delivery routes,
  domain DNS checks, Drive nodes, and policy-bearing domain settings.
- Admin APIs for domains, users, quotas, DKIM keys, trusted relays, delivery
  routes, delivery route resolution, queue stats, delivery attempts,
  outbox event metadata, suppression list, quota usage, domain DNS
  checks/history, backpressure inspection/update, domain policy, per-domain
  stats, DKIM DNS verification, delivery route runtime counters, and exhausted
  delivery attempts with recipient-domain and recent-window filters.
- Admin API exposes `GET /admin/v1/console/capabilities` so a production
  operator console can discover backend contract version, available
  modules, list/cleanup/retention limits, tenancy controls, operational triage
  surfaces, auth/no-store behavior, and a redacted storage backend profile
  before rendering console navigation. The storage profile reports the
  normalized configured backend, active labels, supported object primitives,
  local-vs-S3-compatible class, path-style S3 addressing status, sanitized
  endpoint origin/bucket/prefix/region when applicable, and confirms that
  secrets/local roots are not exposed.
- Delivery-attempt list, stats, and exhausted-attempt reads can filter by
  message id, farm, sender, recipient domain, and recent time window for
  targeted retry/bounce incident triage.
- Domain listing can filter by company, lifecycle status, and latest DNS-check
  status for onboarding and tenant triage.
- Domain DNS check history can filter by summary status and RFC3339 `since`
  windows so operators can inspect recent onboarding or deliverability failures
  without re-querying DNS or scanning every persisted check.
- Company listing can filter by lifecycle status for tenant-level suspension
  and disabled-account triage.
- Delivery-route listing can filter by status, farm, and domain pattern for
  targeted route audits.
- Admin delivery-route creation now rejects oversized farm, SMTP hello, pool,
  description, and relay auth identity/username/password metadata before route
  storage or audit work.
- Suppression-list reads can filter by domain, email, and reason for targeted
  bounce triage without direct database access.
- Queue stats include ready, delayed, stale-processing, oldest-ready, and
  next-available metadata so operators can distinguish backlog from scheduled
  retry delay.
- Outbox event metadata can be filtered by topic, partition key, status, and
  recent time window without exposing payload bodies.
- Outbox event list responses bound `last_error` previews at UTF-8 boundaries
  so operational dashboards do not pull oversized diagnostics by default.
- Outbox event detail responses expose full stored `last_error` by id while
  still omitting raw payload bodies.
- Mail APIs for folders, messages, flags, bulk operations, drafts, send, and
  attachments, cursor-paginated thread lists/thread messages and draft search,
  plus user-scoped sent-message delivery/bounce status.
- Mail API exposes `GET /api/v1/webmail/capabilities` so a production webmail
  frontend can discover backend contract version, active/planned modules, list
  limits, supported message flags, bulk-action bounds, compose/search/thread,
  attachment, and push-device capabilities without hard-coding server limits.
- Mail API exposes `GET /api/v1/mailbox/overview` so production webmail chrome
  can render aggregate total/unread/starred/size counts and system-folder ID
  shortcuts without duplicating folder-list aggregation in every client.
- Mail API message lists now support optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters alongside
  folder and cursor controls, enabling production webmail quick views such as
  unread, read, starred, unstarred, and attachment-bearing messages without
  switching to full-text search. Omitted or empty `limit` query parameters now
  resolve to the documented default of 50 before dispatch, and regression
  coverage now spans message lists, thread lists, thread-message lists, active
  search, and draft search so runtime behavior stays aligned with the OpenAPI
  pagination contract.
- Mail API thread lists now support optional `read=true|false`,
  `starred=true|false`, and `has_attachment=true|false` filters, where
  `read=false` means conversations with at least one unread message and
  `read=true` means fully-read conversations.
- Mail API thread lists now also support `folder_id`, enabling folder-scoped
  conversation views for inbox, sent, archive, and custom folders without
  falling back to flat message lists.
- Mail API message and thread lists now support `sort=newest|oldest` with
  bounded query validation, giving production webmail clients explicit
  newest-first and oldest-first mailbox/conversation list controls.
- Mail API message and thread summaries now expose a required bounded
  `preview` string sourced from the asynchronous search-document read model,
  letting production webmail lists render body context without opening and
  parsing stored `.eml` objects on the list hot path.
- Mail API now supports bounded thread-level bulk flag updates for
  conversation-list read/starred/answered/forwarded actions, while publishing
  best-effort IMAP flag events for the updated messages.
- Mail API now supports bounded thread-level folder moves, validating
  destination folders, invalidating affected IMAP UID rows transactionally, and
  publishing best-effort IMAP expunge events from the pre-move UID snapshot.
- Mail API now supports bounded thread-level soft deletes, deleting every
  active message in selected conversations while invalidating IMAP UID rows,
  decrementing quota transactionally, and publishing best-effort IMAP expunge
  events from the pre-delete UID snapshot.
- Mail API now supports single-message and bounded bulk message restore for
  soft-deleted messages, clearing `deleted_at` and re-checking/re-incrementing
  the hierarchical quota ledger before messages become active again.
- Mail API now supports bounded thread-level restore for soft-deleted
  conversations, reactivating selected conversation messages only after the
  same hierarchical quota guard used by message restore succeeds.
- Mail API restore flows now best-effort assign IMAP UIDs to restored active
  messages and publish IMAP `EXISTS` events, reducing stale selected-mailbox
  views for clients that are connected while webmail recovery actions run.
- Inbound parsing now extracts RFC `In-Reply-To`/`References`; inbound and
  reply/forward outbound persistence inherit local thread IDs when matching
  source messages exist.
- Reply composition writes RFC `In-Reply-To`/`References` headers into outgoing
  `.eml` messages.
- Outbound text composition rejects CR/LF-bearing subject, display-name, email,
  and explicit Message-ID inputs before writing RFC 5322 headers.
- Outbound RFC 5322 text composition folds long headers, rejects malformed
  explicit Message-ID values, and drops malformed thread IDs before writing
  `In-Reply-To`/`References`.
- Mail API exposes a first Postgres-backed search endpoint for active message
  metadata, with an FTS index for small deployments.
- Received-message body search now has an asynchronous indexing boundary:
  `search-index-worker` consumes `mail.stored`, reads stored `.eml` objects,
  extracts bounded text through the shared parser, and upserts Postgres search
  documents used by the existing search endpoint.
- Search indexing rejects ambiguous `mail.stored` storage paths that would be
  changed by path cleaning, preventing parent-traversal or duplicate-separator
  event payloads from opening a different object key.
- Search indexing caps `mail.stored` event `References` metadata before
  document construction, matching the parser's bounded metadata stance.
- The OpenSearch indexing adapter bounds UTF-8 metadata fields and reference
  arrays before JSON document submission, keeping direct adapter calls aligned
  with worker/parser metadata limits.
- OpenSearch relevance queries bound UTF-8 search/filter text and escape
  wildcard metacharacters in sender/subject filters so client-supplied `*` or
  `?` remain literal substring filters.
- OpenSearch relevance hits now clean bounded message IDs from `_source`/`_id`
  before Postgres hydration, dropping CR/LF-bearing IDs from external search
  responses.
- OpenSearch indexing now rejects blank, CR/LF-bearing, or oversized document
  message IDs before constructing `_doc/{id}` URLs, keeping URL IDs aligned
  with bounded JSON metadata.
- OpenSearch writer/searcher construction now trims usernames while preserving
  password bytes, and rejects CR/LF-bearing or oversized endpoint credentials
  before BasicAuth request headers can be generated.
- OpenSearch username/password configuration is also CR/LF-rejected and
  size-bounded during startup config validation when the OpenSearch backend is
  selected, surfacing credential formatting mistakes before worker/search setup.
- OpenSearch writer construction now rejects CR/LF-bearing direct endpoint
  values before URL parsing, keeping adapter calls aligned with startup config
  endpoint validation.
- OpenSearch relevance response decoding is capped before JSON parsing so
  oversized search backend responses cannot allocate unbounded highlight or hit
  payloads in the Mail API path, and trailing JSON tokens are rejected before
  search hits are accepted.
- OpenSearch index/bootstrap/search status-error diagnostics now collapse
  backend response bodies into bounded one-line UTF-8 previews, preventing
  CR/LF-bearing backend errors from leaking into logs or API diagnostics.
- Shared EML text extraction, retained header metadata, and attachment
  metadata are bounded with UTF-8 boundary preservation; attachment filenames
  are basename-normalized, control-character cleaned, and capped before
  reaching storage/API/search consumers. Subject, address display-name/address,
  message-id, address-list, and `References` metadata are capped before
  downstream storage, search, and threading use them. Oversized structured
  subject, address, and message-id-list headers are pre-bounded before decoding
  or list parsing, with truncation flags for retained metadata/list caps.
- Search responses can now opt into relevance sorting, rank scores, and bounded
  Postgres headline snippets while preserving date-sorted results by default.
- Postgres and OpenSearch relevance search now share a metadata-first tuning
  intent: subject and sender matches rank above indexed body text matches.
- Draft rows remain excluded from `GET /api/v1/search`; drafts now have a
  separate compose-focused `GET /api/v1/drafts/search` contract over active
  draft subject, sender, recipient JSON, body text, attachment state, and
  newest-updated ordering.
- `gogomail --mode=all-in-one` serves Mail API and Admin API routes from the
  same HTTP process, keeping single-node/local release smoke tests aligned with
  the documented backend mode.
- `/health/ready` can now include runtime database and Redis dependency probes
  for HTTP modes that use those services, returning `degraded` with HTTP 503
  when a required probe fails.
- Database readiness now also compares the applied `goose_db_version` against
  the latest local SQL migration, so stale schemas degrade `/health/ready`
  instead of passing on connectivity alone.
- Mail/Admin HTTP readiness now probes configured storage with a write/read/delete
  cycle, and unsupported HTTP storage backends fail fast instead of silently
  using local storage wiring.
- Local/NFS storage configuration now requires a non-empty bounded
  `GOGOMAIL_MAILSTORE_ROOT` without line breaks when
  `GOGOMAIL_STORAGE_BACKEND=local`, so broken filesystem roots fail during
  config validation instead of surfacing later as storage probe errors.
- Local and S3-compatible storage writes now reject nil `Put` bodies before
  filesystem or HTTP request work, keeping empty object creation explicit and
  adapter behavior consistent.
- S3-compatible `PutObject` now requires an exact `200 OK` response, rejecting
  accepted/deferred or otherwise non-OK 2xx acknowledgements before mail,
  Drive, or lifecycle callers can treat an ambiguous provider write as durable.
- Local/NFS and S3-compatible storage now expose a shared object `Stat`
  contract, allowing future Drive, lifecycle, and verification paths to inspect
  canonical keys, byte size, and backend metadata without streaming object
  bodies. The S3-compatible adapter implements this with signed `HEAD`
  requests.
- Local/NFS and S3-compatible storage now expose a shared object `Copy`
  contract. Local/NFS copies stream through the same atomic temporary-file
  commit path as normal writes, while S3-compatible copies use signed
  server-side copy requests with escaped `x-amz-copy-source` values.
- S3-compatible `Copy` now requires exact `200 OK` responses with bounded
  `CopyObjectResult` bodies, rejecting empty bodies, unexpected XML, and
  embedded `<Error>` XML inside `200 OK` responses so AWS S3/compatible copy
  failures do not masquerade as successful Drive or lifecycle duplication.
- Local/NFS and S3-compatible storage now expose a shared bounded object
  `List` contract for validated prefixes, giving future Drive, lifecycle, and
  reconciliation workflows a cursor-paginated way to browse object metadata
  without binding callers to filesystem walks or S3 `ListObjectsV2` directly.
- S3-compatible `ListObjectsV2` success bodies now must decode as
  `ListBucketResult` XML, preventing unexpected provider success XML from
  being treated as an empty canonical object page.
- S3-compatible `ListObjectsV2` object-size validation now runs only after a
  provider key maps back to the requested canonical gogomail prefix, so
  out-of-scope bucket noise cannot fail an otherwise valid canonical list page.
- S3-compatible `ListObjectsV2` request queries now use SigV4 canonical URI
  encoding instead of form-style query escaping, preserving literal spaces,
  `+`, `/`, `=`, and `@` characters in signed list prefixes and opaque
  continuation tokens across AWS S3, MinIO, and stricter compatible providers.
- Shared storage list cursors now reject leading/trailing whitespace and
  control characters instead of silently trimming opaque provider tokens,
  keeping local/NFS and S3-compatible pagination identity exact across cleanup
  and reconciliation workers.
- Local/NFS and S3-compatible storage now expose a shared object `Move`
  contract for Drive-ready rename/relocation workflows. Local/NFS uses
  filesystem rename semantics, while S3-compatible storage performs signed
  server-side copy followed by source delete. S3-compatible post-copy source
  delete failures now return a structured cleanup error carrying source and
  destination paths, so Drive/lifecycle callers can distinguish duplicate-object
  cleanup from pre-copy move failure.
- Shared storage now provides a bounded `DeletePrefix` helper that composes
  validated prefix `List` pages with idempotent object deletes, giving future
  Drive folder deletion, attachment lifecycle, and reconciliation jobs a
  cursor-driven cleanup path without backend-specific recursive delete logic.
  Unsafe object paths returned by a listing source now surface as a structured
  partial-progress error instead of blending with ordinary delete failures.
  Truncated list pages must also carry a continuation cursor before
  `DeletePrefix` deletes any listed object, keeping local/NFS and
  S3-compatible cleanup orchestration fail-closed if a backend violates the
  shared pagination contract.
- Drive backend groundwork now has ADR 0009, a `drive_nodes` PostgreSQL
  metadata table, and an internal node-name/type/status validation package.
  Drive object bytes remain behind the shared storage interface while metadata,
  folder trees, lifecycle state, and future quota enforcement stay in the
  database/service boundary.
- Drive now has a first internal repository mutation for active user folder
  creation, deriving company/domain scope from the user row, validating parent
  folders before insertion, using only bound request parameters in SQL, and
  applying Drive node-name/type/status validation before future HTTP routes
  expose the module.
- Drive now has an internal file-finalize repository boundary that validates
  storage backend/object metadata, verifies the object through `storage.Stat`,
  and increments the company/domain/user quota ledger in the same transaction
  as the `drive_nodes` file insert.
- Drive now has an internal node-list repository read model for active,
  trashed, or deleted nodes under a parent folder, with bounded limits and
  folder-first stable ordering for future webmail/Drive clients.
- Drive now has an internal trash repository mutation that marks an active
  node and active descendants as trashed in one transaction, preserving object
  bytes and quota usage for future restore or delayed permanent deletion.
- Drive now has an internal restore repository mutation that marks a trashed
  node and trashed descendants active again in one transaction, clears
  `trashed_at`, and relies on the active sibling uniqueness constraint to keep
  restored folder contents conflict-safe.
- Drive now has an internal permanent-delete repository mutation that marks a
  trashed node and trashed descendants deleted, decrements company/domain/user
  quota for deleted files in the same transaction, and returns storage object
  references for backend-specific byte cleanup.
- Drive now has a backend-object cleanup helper that consumes permanent-delete
  object references, validates storage backend/path input, de-duplicates
  repeated references, honors cancellation, and deletes through the configured
  storage stores with progress-preserving errors. On the first cleanup
  failure, it now carries the failed object plus every not-yet-attempted
  object so post-commit metadata deletion cannot leave trailing storage
  objects untracked.
- Drive now has a small internal service layer that composes repository
  permanent-delete with backend object cleanup, preserving cleanup progress on
  post-transaction storage failures for future retry/reconciliation handling.
- Drive now has canonical object path builders for staged uploads, committed
  node objects, and user prefixes under `drive/users/{user_id}/...`, with
  path-segment-safe ID validation so future cleanup and prefix operations stay
  tenant/user scoped.
- Drive permanent-delete cleanup failures now have a PostgreSQL retry record
  boundary. Structured cleanup errors can be recorded with user/node/object
  context for every object not proven deleted after a committed permanent
  delete, pending failures are de-duplicated per backend/path, attempts are
  incremented on repeat failures, object paths must stay under the owning
  user's `drive/users/{user_id}/...` prefix, and error text is one-line/UTF-8
  bounded for future operator and worker surfaces.
- Drive cleanup-failure records now have bounded repository list and resolve
  methods with status/user filters, oldest-first pending ordering, limit caps,
  and pending-only resolution, preparing retry workers and admin visibility
  without exposing HTTP contracts yet.
- Drive now has an internal cleanup retry service method that lists pending
  cleanup-failure records, deletes each referenced object through configured
  storage stores, resolves successful records, and re-records failed attempts
  so retry diagnostics remain fresh and bounded.
- Drive cleanup retry can now run as a first-class backend worker mode,
  `drive-cleanup-worker`, with validated interval/batch/run-once config and
  local/MinIO/S3-compatible storage wiring through the shared storage adapter.
- Mail API now exposes the first Drive HTTP routes for production webmail
  integration: bounded node listing, single-node metadata reads, folder
  creation, trash, restore, and permanent delete. The routes use the existing
  user auth/fallback path, shared Drive repository/service boundaries, and
  OpenAPI-documented response envelopes without starting frontend
  implementation.
- Mail API also exposes `POST /api/v1/drive/files/finalize`, letting a
  previously staged object become quota-accounted Drive file metadata through
  the shared storage `Stat` contract and Drive file-finalize repository
  boundary.
- Mail API now exposes `PUT /api/v1/drive/files/staged/{upload_id}/body`,
  streaming a bounded object body to the configured local/NFS, MinIO, or
  S3-compatible backend, deriving the canonical Drive staging key, computing
  size and SHA-256, and returning a frontend-ready staged-object envelope for
  file finalization.
- Drive nodes can now be renamed through `PATCH /api/v1/drive/nodes/{id}/name`,
  keeping active file/folder metadata aligned with normalized-name validation
  and sibling uniqueness before future production Drive UI work begins.
- Drive nodes can now be moved through
  `PATCH /api/v1/drive/nodes/{id}/parent`, validating destination folders,
  root moves, and active-subtree cycle prevention at the repository boundary.
- Drive upload sessions now have a PostgreSQL metadata boundary and
  `internal/drive` validation contract for upload IDs, parent folders,
  declared size, MIME type, storage backend, lifecycle status, and bounded
  expiration before HTTP upload-session routes are exposed.
- `internal/drive.Repository.CreateUploadSession` can create pending Drive
  upload sessions for active users under optional active parent folders,
  preserving the same backend-neutral storage metadata and bounded expiration
  rules that future HTTP clients will use.
- Mail API now exposes `POST /api/v1/drive/upload-sessions`, returning stable
  `drive_upload_session` envelopes for frontend clients that need to declare
  Drive upload metadata before body transfer/finalization.
- Mail API now exposes `GET /api/v1/drive/upload-sessions/{id}`, giving
  frontend clients a stable upload-session status refresh path before body
  retry/finalize routes are added.
- Mail API now exposes `DELETE /api/v1/drive/upload-sessions/{id}`, allowing
  clients to explicitly cancel pending/uploading/failed Drive upload sessions
  instead of waiting for expiry cleanup.
- Drive upload-session body storage now has service/repository boundaries that
  stream each retry to a distinct canonical object path, verify declared size
  and optional SHA-256, update session storage metadata, and best-effort clean
  superseded or failed staged bodies across local/NFS, MinIO, and S3-compatible
  stores.
- Mail API now exposes `PUT /api/v1/drive/upload-sessions/{id}/body`, wiring
  the retry-safe body storage service to frontend clients with an optional
  `X-Content-SHA256` integrity header and explicit `Content-Range` rejection
  until chunked/resumable semantics are specified.
- Drive upload-session finalization now has a repository/service boundary that
  locks a writable session, verifies the stored object size through the shared
  storage `Stat` contract, increments quota, inserts the Drive file node, and
  marks the session finalized in one transaction.
- Mail API now exposes `POST /api/v1/drive/upload-sessions/{id}/finalize`,
  letting frontend clients commit uploaded session bodies into Drive file
  metadata through the same quota and storage verification boundary.
- Webmail capabilities now advertise Drive node operations, upload-session
  create/read/cancel/body/finalize support, checksum preconditions, and Drive
  upload size/TTL limits so production clients can enable Drive flows without
  copying backend constants.
- Drive upload sessions can now be expired in bounded repository batches, and
  the Drive service deletes stored session bodies from the configured backend
  after rows are marked expired.
- `drive-cleanup-worker` now expires stale Drive upload sessions on each run
  before retrying permanent-delete object cleanup failures, keeping abandoned
  upload-session objects out of request paths.
- Mail API now exposes `GET /api/v1/drive/upload-sessions` with status and
  limit filters, and webmail capabilities advertise the list surface for
  production upload manager recovery.
- Admin API now exposes `GET /admin/v1/drive-upload-sessions` with required
  user scope plus status/limit filters, and admin capabilities mark Drive
  upload-session inspection available for operator consoles.
- Drive upload session body storage now accepts `Content-Range` headers for
  complete body uploads. The `PUT /api/v1/drive/upload-sessions/{id}/body`
  endpoint parses and validates RFC 7233 `Content-Range` headers, accepting
  both `bytes */<size>` asterisk form and `bytes 0-<size-1>/<size>` explicit
  range form when the range matches the session's declared size. Malformed
  Content-Range headers or mismatched sizes return HTTP 400 with descriptive
  errors. This enables clients to verify upload completeness through standard
  HTTP Content-Range semantics before finalization.
- Drive node listing now supports a bounded `q` name filter on both Mail and
  Admin API list surfaces, with case-insensitive normalization and literal SQL
  wildcard handling inside the selected parent/status scope.
- Admin API now exposes `GET /admin/v1/drive-nodes` with required user scope
  plus parent/status/name/limit filters so operator consoles can inspect a
  user's Drive inventory through bounded backend contracts.
- Admin API now exposes `GET /admin/v1/drive-nodes/{id}` with required user
  scope and lifecycle status filtering so operator consoles can inspect one
  Drive file or folder without entering user-facing auth paths.
- Admin API now exposes `GET /admin/v1/drive-usage` with required user scope
  so operator consoles can render quota, node-count, byte-count, and pending
  upload-session dashboard summaries.
- Mail API now exposes `GET /api/v1/drive/usage`, and webmail capabilities
  advertise the usage summary surface for future Drive storage cards.
- Mail API now exposes `GET /api/v1/drive/nodes/{id}/download`, streaming
  active Drive file bytes from the configured local/NFS, MinIO, or S3-compatible
  backend with bounded identity validation, safe attachment headers,
  `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`; webmail
  capabilities advertise `node_download`.
- Mail API also exposes `HEAD /api/v1/drive/nodes/{id}/download` so production
  clients can verify active file metadata and object existence without opening
  or transferring the object body.
- Drive downloads now support a single satisfiable HTTP byte range through the
  shared local/NFS and S3-compatible `GetRange` storage contract, giving
  production webmail clients resumable download and media-preview building
  blocks without backend-specific object access.
- Drive download, range-download, and download-header responses now expose a
  sanitized `X-Gogomail-Drive-SHA256` header when file metadata carries a
  stored whole-object digest, giving clients an integrity check without
  trusting backend-specific ETags.
- IMAP `ENABLE` now rejects malformed capability atoms before authentication
  or session mutation, keeping RFC 5161 syntax failures distinct from valid
  unauthenticated enable attempts.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/candidates` so
  operators can preview stale Drive upload-session cleanup counts and bounded
  candidate rows before worker cleanup handles them.
- Admin API now exposes `POST /admin/v1/drive-upload-cleanup/runs` for
  explicit, audited, one-shot stale Drive upload-session expiry outside the
  worker loop.
- Admin API now exposes `GET /admin/v1/drive-cleanup-failures` with user,
  status, and limit filters so operator consoles can inspect Drive backend
  object cleanup drift.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/{id}/resolve`,
  allowing audited operator closure after external Drive object cleanup
  verification.
- Admin API now exposes `POST /admin/v1/drive-cleanup-failures/retry-runs`,
  letting operators trigger audited bounded retries for pending Drive object
  cleanup drift and inspect scanned/deleted/resolved/failed run counts.
- S3-compatible storage requests now reject canceled contexts before object-key
  validation, SigV4 signing, or HTTP dispatch, keeping cancellation behavior
  aligned with local/NFS storage and reducing wasted request work.
- S3-compatible `PUT`, failed `GET`, and `DELETE` responses now drain a small
  bounded response-body window before close, improving HTTP connection reuse
  for normal S3/MinIO responses without allowing oversized bodies to stall
  cleanup.
- S3-compatible deletes now accept completed `200 OK`/`204 No Content`
  responses plus idempotent `404 Not Found`, while rejecting accepted/deferred
  or other ambiguous non-OK 2xx statuses before cleanup workers can mark object
  deletion as complete.
- S3-compatible `PutObject`, full-object `GET`, `HEAD`/`Stat`, and
  `ListObjectsV2` now require exact `200 OK` responses so accepted/deferred
  writes, unexpected partial-content, or other non-OK 2xx statuses cannot
  masquerade as durable or complete backend-neutral object results.
- S3-compatible standard `<Error>` diagnostics now suppress previews when safe
  fields such as `Code`, `Message`, `RequestId`, or `HostId` appear more than
  once or contain nested XML, avoiding ambiguous S3 provider status
  diagnostics.
- S3-compatible request construction now has explicit regression coverage for
  automatic path-style addressing on HTTPS dotted buckets and local/IP
  endpoints, preserving AWS S3 certificate compatibility and MinIO-style local
  behavior even when the generic `s3` backend is used.
- Local/NFS and S3-compatible readiness probes now read the verification object
  through a tight expected-size bound, so malformed or proxy-inflated probe
  responses cannot allocate unbounded memory during `/health/ready` checks.
- Local/NFS and S3-compatible readiness probes now also `Stat` the verification
  object and compare its canonical key and byte size before cleanup, catching
  broken filesystem metadata or S3 `HEAD` paths during readiness instead of
  later Drive/mail object workflows.
- Local/NFS and S3-compatible readiness probes now issue a short `GetRange`
  verification against the same object, catching broken filesystem seek/range
  handling or S3 `Range` response compatibility before partial-read workflows
  report ready.
- SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment
  cleanup, CalDAV scaffold, and HTTP runtimes now share storage backend validation and factory
  wiring for local filesystem/NFS-style storage plus S3-compatible object
  storage. `GOGOMAIL_STORAGE_BACKEND=s3` can target AWS S3, while
  `GOGOMAIL_STORAGE_BACKEND=minio` uses the same S3-compatible adapter with
  path-style requests for local MinIO-style deployments. Both paths use endpoint,
  region, bucket, prefix, credential, and session-token settings.
- Drive runtime wiring now treats persisted `s3` and `minio` storage-backend
  labels as aliases for the configured S3-compatible store, so deployments can
  flip between local MinIO and AWS S3-style configuration without stranding
  existing Drive/upload rows solely because their stored backend label differs.
- Drive runtime wiring can now opt into explicit storage-backend compatibility
  labels with `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS`. This lets operators
  perform staged local/NFS-to-S3-compatible Drive migrations after object bytes
  have been copied while keeping the default behavior fail-closed for legacy
  labels that were not intentionally mapped to the active store.
- S3-compatible runtime option construction is now isolated and covered by app
  tests, pinning MinIO to path-style requests while preserving virtual-hosted
  S3 defaults unless `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` is set.
- S3-compatible bucket validation now rejects IP-address-shaped names plus
  AWS-reserved bucket prefixes and suffixes during config validation, and
  requires bucket names to start and end with a letter or digit, so S3
  deployment mistakes fail before adapter construction or readiness probes.
- S3-compatible endpoint validation now rejects userinfo, query strings,
  fragments, non-HTTP schemes, CR/LF-bearing targets, and non-canonical base
  paths before adapter construction. Endpoint base paths also reject encoded
  path separators such as `%2F` and `%5C`, keeping SigV4 signing and object
  addressing unambiguous across AWS S3, MinIO, and compatible providers.
- Shared storage object path and prefix validation now also rejects encoded
  path separators such as `%2F` and `%5C`, avoiding local/NFS-to-S3 portability
  drift before object keys reach filesystem paths, configured S3 prefixes,
  signed object requests, copy/move endpoints, or returned list-key exposure.
- S3-compatible request construction automatically uses path-style addressing
  for dotted bucket names on HTTPS endpoints, avoiding AWS S3 virtual-hosted
  TLS wildcard certificate mismatches while preserving virtual-hosted requests
  for ordinary bucket names by default.
- S3-compatible request construction also automatically uses path-style
  addressing for localhost and IP-address endpoints, avoiding
  `bucket.localhost`/`bucket.127.0.0.1` style drift for local MinIO and other
  local compatible object stores even when the generic `s3` backend is used.
- S3-compatible object key escaping now preserves literal `+` characters as
  `%2B` in segment-escaped paths, keeping object identity and SigV4 canonical
  request paths aligned across AWS S3, MinIO, and strict compatible providers.
- S3-compatible endpoint base paths are now segment-escaped with the same
  literal `+` preservation as object keys, keeping proxy/base-path deployments
  aligned with SigV4 canonical request paths.
- S3-compatible `ListObjectsV2` pages now reject provider responses that return
  more matching objects than the requested bounded page size, keeping S3,
  MinIO, and local/NFS pagination under the same storage contract.
- S3-compatible `ListObjectsV2` standard metadata now also shares the
  namespace boundary used by core list controls: normal AWS fields such as
  `Prefix`, `Name`, `KeyCount`, `MaxKeys`, `StorageClass`, and `Owner` remain
  accepted, while foreign-namespace variants fail closed before unmarshalling.
- S3-compatible `ListObjectsV2` simple standard metadata such as `Prefix` and
  `StorageClass` now also rejects nested XML before unmarshalling, while
  structured AWS fields such as `Owner` remain compatible when nested children
  are namespace-free or AWS S3-namespaced, use known AWS child names, and do
  not repeat the same child name.
- S3-compatible `ListObjectsV2` mapped object keys with leading/trailing
  whitespace or encoded separators now fail closed instead of being silently
  skipped after they match the configured storage prefix.
- S3-compatible uploads now set a deterministic `Content-Length` for seekable
  PUT bodies without buffering the object in memory, improving compatibility
  for file-backed mail and attachment writes while preserving streaming-first
  storage paths.
- S3-compatible deletes now treat `404 Not Found` as already-cleaned success,
  aligning compatible-provider cleanup behavior with local/NFS idempotent
  deletes.
- S3-compatible secret access keys and session tokens now reject spaces, tabs,
  and line breaks during config validation and adapter construction, surfacing
  copied env/config credential mistakes before readiness probes or runtime
  PUT/GET requests fail with opaque authentication errors.
- S3-compatible access key IDs now reject spaces, tabs, and line breaks during
  config validation and adapter construction, preventing copied credential
  mistakes from being silently trimmed before SigV4 signing.
- S3-compatible access key IDs, secret access keys, and session tokens now also
  reject oversized direct adapter inputs using the same bounds as startup
  config validation, preventing oversized SigV4 header material from reaching
  runtime request construction.
- Local/NFS-style storage writes now stage through unique temporary files in
  the target directory before `rename`, avoiding fixed `.tmp` collisions while
  preserving atomic object replacement semantics.
- Local/NFS-style storage writes now honor context cancellation during body
  copy, removing staged temp objects instead of committing partial data after a
  canceled request.
- Local/NFS-style storage deletes now treat already-missing objects as success,
  aligning cleanup semantics with S3-compatible delete behavior across storage
  backends.
- IMAP `LIST`/`LSUB` CHILDREN attributes now infer immediate parents from
  nested `FullPath` values when backend rows do not carry `ParentID`, so deeper
  hierarchies such as `Projects/2026/Jan` still mark `Projects/2026` with
  `\HasChildren` for clients that depend on hierarchy metadata.
- IMAP `APPEND`, `STORE`, and `UID STORE` flag-list parsing now rejects
  unparenthesized or unbalanced flag lists instead of silently trimming stray
  parentheses, keeping flag mutation syntax closer to RFC-shaped client
  expectations.
- IMAP `APPEND` internaldate parsing now enforces RFC 3501 fixed-width
  `date-day-fixed` syntax, accepting zero-padded or space-padded days while
  rejecting bare one-digit dates such as `"5-May-2026 ..."`. Date-month atoms
  are canonicalized ASCII-case-insensitively before parsing, preserving strict
  date shape while accepting common uppercase or lowercase client month
  literals.
- IMAP selected-mailbox `STORE` and `UID STORE` now honor advertised
  `[PERMANENTFLAGS]`, rejecting otherwise valid system flags when the selected
  mailbox did not permit them instead of dispatching unsupported mutations to
  storage. Empty `+FLAGS ()` and `-FLAGS ()` remain successful no-ops, while
  `FLAGS ()` replacement is rejected when no permanent flags are permitted.
- IMAP message sequence sets now explicitly reject sequence numbers above the
  selected mailbox size with tagged `BAD` responses, preserving RFC 3501 bounds
  behavior for `FETCH`, `STORE`, `COPY`, and `MOVE` sequence arguments.
- IMAP quoted-string parsing now rejects adjacent tokens after a closing quote
  and unsupported backslash escapes before authentication or backend work,
  keeping command tokenization aligned with RFC 3501 quoted-special handling.
- IMAP mailbox wire-name formatting now preserves ordinary internal spacing
  while still collapsing control-character runs, preventing `LIST`, `LSUB`, and
  `STATUS` responses from changing distinct user-visible mailbox names.
- IMAP UID `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` commands now resolve
  `*` UID sequence ranges against selected-mailbox UIDs, so common client
  requests such as `UID FETCH 1:*` include the last visible UID without
  expanding through non-existent UID gaps.
- IMAP `SEARCH UID <sequence-set>` and `UID SEARCH UID <sequence-set>` now
  resolve `*` UID ranges against the selected mailbox's visible UIDs, aligning
  search-key filtering with UID command range handling.
- IMAP command tag validation now rejects `+` in tags before command routing,
  matching RFC 3501 tag grammar and avoiding ambiguity with continuation
  protocol markers.
- IMAP `SEARCH`/`UID SEARCH` date criteria now reject malformed date atoms that
  still contain quote characters after command parsing, so broken inputs such
  as `SINCE 05-May-2026"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` date criteria now accept one-digit date-day atoms
  such as `SINCE 5-May-2026` while preserving the malformed quote rejection,
  improving compatibility with clients that do not zero-pad day values. Search
  date-month atoms are also canonicalized ASCII-case-insensitively before parsing.
- IMAP `SEARCH`/`UID SEARCH` date criteria now reject whitespace-padded date
  strings such as `SINCE " 05-May-2026 "` instead of trimming them into valid
  date atoms.
- IMAP `SEARCH` and `UID SEARCH` now reject `CHARSET` prefixes that omit the
  required following search-key before authentication or selected-mailbox
  checks, preserving RFC 3501 search grammar on the hot command boundary.
- IMAP `FETCH` and `UID FETCH` now validate malformed fetch data-item syntax
  such as nested `((FLAGS))` before authentication or selected-mailbox checks,
  keeping RFC 3501 fetch grammar errors from being masked as state failures.
- IMAP `FETCH` and `UID FETCH` now reject unsupported data items before
  authentication or selected-mailbox checks instead of silently returning the
  default fetch attributes. Supported `BODY`, `BODY.PEEK`, `RFC822.*`,
  `HEADER.FIELDS`, partial fetch, MIME section, macro, and `CHANGEDSINCE`
  shapes remain accepted through the same syntax boundary.
- IMAP `STORE` and `UID STORE` now validate malformed `UNCHANGEDSINCE`, store
  mode, and flag-list syntax before authentication or selected-mailbox checks,
  keeping RFC 3501/CONDSTORE mutation grammar distinct from state failures.
- IMAP `STORE`/`UID STORE` mode atoms and `UNCHANGEDSINCE` markers now reject
  whitespace-padded quoted or literal values instead of trimming them into
  valid mutation controls.
- IMAP `STORE`/`UID STORE` flag-list values now reject whitespace-padded
  quoted or literal lists such as ` (\\Seen) ` while preserving exact `()`
  and parenthesized flag-list semantics.
- IMAP APPEND/STORE flag-list parsing now rejects malformed inner list
  whitespace such as `( \\Seen)`, `(\\Seen )`, `(\\Seen  \\Flagged)`, or
  tab-separated flag names instead of collapsing them into valid flags.
- IMAP APPEND/STORE flag-list parsing now also rejects duplicate canonical
  system flags such as `(\\Seen \\Seen)`, keeping flag-lists set-shaped before
  backend mutation or APPEND body handling.
- IMAP selected-state commands now validate malformed message sequence-set and
  UID set syntax, including signed values such as `+1`/`+7`, before
  authentication or selected-mailbox checks while leaving selected-mailbox
  bounds validation at execution time. UID subcommands now also drain queued
  selected-mailbox events before execution, so `UID FETCH *` and other
  UID-addressed workflows see pending `EXISTS`/`EXPUNGE`/`FLAGS` updates
  before resolving live mailbox state.
- IMAP `SEARCH` and `UID SEARCH` now also validate malformed search
  sequence-set and `UID` search-key set syntax before authentication or
  selected-mailbox checks, so signed values such as `SEARCH +1` and
  `UID SEARCH UID +7` fail as grammar errors rather than state errors.
- IMAP `SORT`, `UID SORT`, `THREAD`, and `UID THREAD` now reuse the same
  syntax-only search-key validation before authentication or selected-mailbox
  checks, so malformed embedded search criteria are reported consistently
  across the RFC 5256/ESORT-style command family.
- IMAP command tokenization now rejects embedded quote characters inside
  unquoted atoms while preserving escaped quotes inside proper quoted strings,
  keeping RFC 3501 atom and quoted-string handling separate.
- IMAP parenthesized `SEARCH`/`UID SEARCH` groups now reject empty `()` groups
  instead of treating them as match-all, while preserving valid `(ALL)` groups.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` numeric thresholds now reject malformed
  values that still contain quote characters after command parsing, so broken
  inputs such as `MODSEQ 20"` are not silently normalized.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types now reject malformed atoms
  that still contain quote characters after command parsing, preventing broken
  `MODSEQ "/flags/\\Seen" all" 17` style inputs from being silently normalized.
- IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types now also reject
  whitespace-padded `ALL`, `PRIV`, or `SHARED` atoms instead of trimming them
  into valid RFC 7162 entry-type controls.
- IMAP RFC 2971 `ID` parameter-list parsing now rejects unsupported quoted
  escapes and adjacent quoted tokens without whitespace, while preserving valid
  escaped quoted-special characters inside ID strings.
- IMAP RFC 2971 `ID` parameter-list parsing now also rejects quote and
  backslash atom-special characters inside unquoted ID tokens, keeping raw ID
  argument parsing aligned with the broader RFC 3501 atom/quoted-string split.
- IMAP RFC 2971 `ID` unquoted field/value tokens now reuse the same atom
  validator as command tags and atoms, rejecting literal markers, response
  specials, wildcard specials, quoted specials, and controls consistently.
- IMAP RFC 2971 `ID` parameter-list parsing now accepts bounded synchronizing
  and non-synchronizing string literals inside the parenthesized field/value
  list, while still rejecting missing or unused literal payloads.
- IMAP RFC 2971 `ID` now accepts the bare no-argument command form as an empty
  client parameter set, returning server identity while preserving strict
  `NIL` and parenthesized field/value-list validation.
- IMAP `SEARCH`/`UID SEARCH` `LARGER` and `SMALLER` size criteria now require
  digit-only RFC 3501 number atoms, rejecting signed values such as `+20`
  and values above the unsigned 32-bit IMAP `number` range instead of silently
  treating them as valid sizes.
- IMAP `SEARCH`/`UID SEARCH` size and MODSEQ numeric criteria now reject
  whitespace-padded numeric strings such as `LARGER " 20 "` or
  `MODSEQ " 20 "` instead of trimming them into valid number atoms.
- IMAP mod-sequence numeric inputs now require digit-only atoms across
  `SEARCH MODSEQ`, `FETCH CHANGEDSINCE`, and conditional `STORE`
  `UNCHANGEDSINCE`, rejecting signed values such as `+17`. Positive
  `mod-sequence-value` contexts now reject zero, while `UNCHANGEDSINCE 0`
  remains zero-allowed and is preserved as a conditional STORE guard rather
  than being treated as no modifier.
- IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` and `STORE`/`UID STORE`
  `UNCHANGEDSINCE` modifier values now reject whitespace-padded numeric atoms
  instead of trimming them into valid CONDSTORE thresholds.
- IMAP UID and message sequence-set numbers now require digit-only atoms,
  rejecting signed values such as `UID FETCH +7` and `FETCH +1` before command
  execution.
- IMAP UID and message sequence-set expansion now accepts common client-scale
  ranges such as `1:1000` and `1:*` while still enforcing an explicit expansion
  cap, reducing false `BAD` responses during mailbox synchronization.
- IMAP UID set resolution now intersects authenticated selected-mailbox UID
  ranges and comma-separated UID sets with visible message UIDs, so sparse
  requests such as `UID FETCH 1:999` and `UID FETCH 1,7,999` skip missing UIDs
  instead of failing the whole command.
- IMAP MIME body-part paths and partial body fetch windows now require
  digit-only number atoms, rejecting signed forms such as `BODY[+1]` and
  `BODY[]<+12.34>`, and partial fetch counts must be non-zero as required by
  RFC 3501 `nz-number` grammar. MIME part, offset, and count values are capped
  to IMAP's unsigned 32-bit `number` range, and partial fetch tokens also
  reject trailing characters after the closing `>`. Padded MIME path atoms are
  rejected before section lookup.
- IMAP `SEARCH`, `SORT`, and `THREAD` charset arguments now reject malformed
  atoms that still contain quote characters or outer whitespace after command
  parsing, preventing broken values such as `UTF-8"` or `" UTF-8 "` from being
  silently normalized.
- IMAP `THREAD` algorithm arguments now reject malformed atoms that still
  contain quote characters or outer whitespace after command parsing,
  preventing broken values such as `ORDEREDSUBJECT"` or `" ORDEREDSUBJECT "`
  from being silently normalized.
- IMAP `SEARCH`/`UID SEARCH` text, body, and header string arguments now reject
  malformed atoms that still contain quote characters after command parsing,
  preventing broken values such as `SUBJECT IMAP"` from being normalized.
- IMAP `SEARCH` text arguments now preserve valid RFC quoted-special escaped
  quotes from proper quoted strings, so standards-shaped searches such as
  `SUBJECT "Project \"Q2\""` remain compatible while malformed atom quotes are
  rejected by command parsing.
- IMAP `SEARCH`/`UID SEARCH` `KEYWORD` and `UNKEYWORD` criteria now reject
  malformed keyword atoms that still contain quote characters after command
  parsing, preventing broken values such as `KEYWORD custom"` from being
  silently normalized. They now share the common IMAP atom validator, so
  system flags such as `\Seen` and response-special atoms such as `bad]flag`
  cannot be treated as RFC `flag-keyword` search criteria.
- IMAP command tokenization now rejects dangling quote characters at the end of
  unquoted atoms, preventing broken commands such as `SUBJECT IMAP"` and
  `LIST "" INBOX"` from reaching command-specific normalization while
  preserving valid escaped quotes inside proper quoted strings.
- IMAP `FETCH`/`UID FETCH` `HEADER.FIELDS` and `HEADER.FIELDS.NOT` lists now
  validate RFC-shaped header field names instead of trimming stray brackets,
  rejecting malformed requests such as `HEADER.FIELDS ([Subject])`.
- IMAP `FETCH`/`UID FETCH` accepts RFC-valid empty `HEADER.FIELDS ()` and
  `HEADER.FIELDS.NOT ()` lists, returning only the header terminator for empty
  include requests and the full header block when the exclude list is empty.
- IMAP `FETCH`/`UID FETCH` now rejects whitespace-only, padded, or collapsed
  `HEADER.FIELDS` and `HEADER.FIELDS.NOT` field-list forms such as
  `HEADER.FIELDS ( )`, while preserving exact empty-list `()` compatibility.
- IMAP `FETCH`/`UID FETCH` data items now reject whitespace-padded quoted or
  literal values such as `" (FLAGS) "` or `" FLAGS "` instead of trimming them
  into valid fetch attributes.
- IMAP `FETCH`/`UID FETCH` has regression coverage for partial-window empty
  top-level header-field-list requests, including `HEADER.FIELDS ()<0.1>` and
  `HEADER.FIELDS.NOT ()<0.10>` preview forms.
- IMAP `FETCH`/`UID FETCH` applies the same empty `HEADER.FIELDS ()` and
  `HEADER.FIELDS.NOT ()` semantics to `message/rfc822` MIME-part sections such
  as `BODY[1.HEADER.FIELDS ()]` and `BODY[2.HEADER.FIELDS.NOT ()]`.
- IMAP `SEARCH HEADER` validates RFC-shaped header field names before state
  checks, rejecting empty, space-bearing, colon-suffixed, or IMAP
  atom-special-bearing field-name arguments as malformed criteria.
- IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` now requires the RFC-shaped
  parenthesized modifier form and rejects bare or over-closed variants such as
  `FETCH 7 FLAGS CHANGEDSINCE 17`.
- IMAP `FETCH`/`UID FETCH` macros now remain valid only as standalone macro
  arguments, rejecting malformed list usage such as `FETCH 1 (FAST)` or
  `UID FETCH 7 (FLAGS FAST)`.
- IMAP `STORE`/`UID STORE` `UNCHANGEDSINCE` now requires the RFC-shaped
  parenthesized modifier form and rejects malformed over-closed values such as
  `(UNCHANGEDSINCE 27))`.
- IMAP `FETCH`/`UID FETCH` data items now reject over-parenthesized tokens
  before item normalization, preventing malformed requests such as
  `FETCH 1 ((FLAGS))` and `UID FETCH 7 BODY.PEEK[]))` from being repaired.
- `docs/storage-backends.md` documents local/NFS, MinIO, and AWS S3-style
  configuration, including the `GOGOMAIL_STORAGE_ROOT` compatibility alias for
  `GOGOMAIL_MAILSTORE_ROOT`, and the development compose stack includes
  `minio-init` to create the default local `gogomail` bucket.
- HTTP server runtime guardrails are configurable and validated: read, write,
  idle, read-header timeout, and maximum header bytes are wired into the shared
  Mail/Admin/API-metered HTTP server.
- Admin backpressure overrides now persist bounded hash-chain audit rows after
  Redis state changes, recording previous/current SMTP pressure levels without
  silently accepting unaudited operational receive throttles.
- Admin suppression-list deletions now persist hash-chain audit rows in the
  same transaction as the delete, preserving suppression entry, domain, email,
  reason, and source-message evidence for deliverability forensics.
- Admin outbox retry now persists a hash-chain audit row in the same transaction
  as the retry reset, preserving previous topic, partition key, status,
  attempts, and bounded error evidence for replay forensics.
- Admin push-notification outcome updates now persist hash-chain audit rows in
  the same transaction as provider-status updates and invalid-token device
  deletion, without including raw push tokens or token suffixes in audit detail.
- Admin attachment cleanup runs now persist bounded hash-chain audit rows after
  stale upload and upload-session expiry sweeps, recording cutoff, normalized
  limit, expired counts, and bounded ID samples without storage paths.
- Admin IMAP UID backfill now persists a hash-chain audit row in the same
  transaction as UID assignment, recording mailbox/user scope, normalized
  limit, assigned count, and a bounded message/UID sample.
- Admin API-usage export batch creation now persists a hash-chain audit row in
  the same transaction as the batch, recording tenant/principal scope, export
  window, event/request counts, bytes, latency totals, and export format.
- Admin API-usage export artifact creation/upsert now persists a hash-chain
  audit row in the same transaction as the artifact row, recording object key,
  storage backend, content type, byte/event counts, and SHA-256 digest without
  copying artifact metadata into the audit detail.
- Admin API-usage export manifest digest and signature creation now persist
  hash-chain audit rows in the same transaction as the evidence rows, recording
  bounded digest/signature evidence without copying raw manifests, metadata, or
  full signature material into audit detail.
- Admin API-usage ledger retention runs now persist hash-chain audit rows in the
  same transaction as run records and destructive deletes, recording dry-run,
  blocked, no-op, and completed outcomes with bounded readiness evidence.
- Admin user creation and password-hash rotation can persist a validated
  `password_hash`, giving operators a path to create and maintain SMTP
  Submission-capable local users without storing raw production passwords
  through the API. User read models expose `password_configured` without
  returning stored password hashes, and Admin user listing can filter by status
  and that readiness flag.
- Mail API send/draft-send applies domain outbound policy in enforce mode for
  recipient-count and composed-message-size guardrails.
- Authenticated SMTP Submission now carries the authenticated user's available
  sender addresses from `user_addresses`; `MAIL FROM` accepts either the
  primary address or an authorized additional address while still rejecting
  unrelated senders.
- IMAP authentication now rejects users marked `must_change_password`, using
  the DB authentication result to prevent long-lived protocol sessions before
  the user completes the required password rotation.
- IMAP server connections now carry configurable read, write, and IDLE
  timeouts. The protocol loop refreshes deadlines around command reads,
  response flushes, STARTTLS handshakes, and IDLE waits so slow or abandoned
  clients cannot hold goroutines and connection slots indefinitely.
- IMAP `AUTHENTICATE PLAIN` now enforces `PRIVACYREQUIRED` before decoding an
  initial SASL response on plaintext connections that require TLS, while still
  rejecting malformed command/mechanism tokens with `BAD`.
- IMAP failed reselect attempts now preserve the current selected mailbox and
  subscription. `SELECT`/`EXAMINE` only deselects the previous mailbox after
  the replacement mailbox and event subscription are ready.
- IMAP `COPY` now validates the destination mailbox even when the source UID
  set is empty, so empty `$`/SEARCHRES copies still return `[TRYCREATE]` for
  missing destinations while keeping `OK COPY completed` for existing targets.
- POP3 authentication now rejects users marked `must_change_password`, matching
  IMAP session policy so password-rotation-required accounts cannot open
  protocol sessions before completing the web reset/change flow.
- Authenticated SMTP Submission now mirrors receive-path storage cleanup:
  if the stored hook, submitted recorder, or mailbox-quota path fails before
  database commit, the just-written submitted `.eml` object is deleted.
- Mail API attachment reservation/direct upload applies enforced domain
  `max_attachment_bytes` policy before quota reservation or object storage
  writes.
- Per-domain inbound policy enforced at SMTP receive and Submission MTA (max
  recipients, max message size, inbound mode).
- SMTP receive evaluates inbound policy for every recipient domain in a
  multi-recipient transaction, aggregates the strictest enforced recipient and
  message-size limits, and returns a temporary SMTP policy error instead of
  failing open when policy lookup fails.
- SMTP receive now deletes a just-written `.eml` object when the stored hook,
  recorder, or mailbox-quota path fails before the message is committed to the
  database, preventing orphaned raw-message objects after failed DATA.
- Hierarchical quota ledger enforced at mail storage write/delete boundaries:
  company, domain, and user usage counters are updated atomically in the same
  PostgreSQL transaction. User quota source is tracked as `default|custom`, and
  domain default user quota updates apply to default-following users while
  preserving custom overrides.
- Attachment upload metadata creation reserves bytes from the same
  company/domain/user quota ledger, stale upload cleanup releases them, and the
  Mail API returns HTTP 507 `insufficient_storage` for quota exhaustion.
- Admin quota views now expose runtime remaining capacity, child-allocation
  usage, allocatable capacity, and over-allocation indicators for
  company/domain/user operations.
- Admin quota usage pressure reads can filter by scope, domain, over-limit
  status, and over-allocation status for targeted capacity triage.
- Admin API exposes a read-only quota reconciliation report comparing ledger
  counters with message and attachment source rows.
- Admin API can run operator-controlled quota reconciliation corrections with
  transaction/advisory locking and bounded audit-log detail for dry-run and
  applied correction attempts.
- Product quota direction is company pool → domain allocation → user unified
  storage allowance. User quota should cover mailbox, attachments, future Drive,
  and other user-owned storage features.
- API metering direction is agreed for future SaaS operations: collect usage
  dimensions early through an async middleware/event boundary, but keep
  billing/rate-limit enforcement policy-driven and disabled by default.
- API metering has a first disabled-by-default middleware boundary with a
  `slog` sink for low-risk operational visibility and an outbox sink for durable
  `api.usage` event emission.
- API metering now has an aggregation worker boundary: `api-metering-worker`
  consumes `api.usage` events from `api.event`, upserts Postgres daily
  and monthly aggregates, and exposes `GET /admin/v1/api-usage/daily` plus
  `GET /admin/v1/api-usage/monthly` for operations.
- API usage daily/monthly aggregate reads can filter by tenant, company, domain,
  user, API key, principal, auth source, method, route, status, and time window
  for scoped billing and operational triage.
- API metering events now use `2026-05-04.api-usage.v2` payloads with
  tenant/company/domain/user/API-key/principal/auth-source dimensions. The
  worker stores those dimensions in the idempotency ledger and keys daily/monthly
  aggregates by identity so usage from different tenants or principals does not
  merge.
- API metering auth-source dimensions are normalized to the known set
  `anonymous|bearer|admin_token|query_user_id|unknown`; unexpected values fold
  to `unknown` before ledger/aggregate storage.
- API metering request identity extraction trims tenant/company/domain/user/API
  key/principal dimensions, drops CR/LF-bearing or oversized default request
  dimensions, and no longer classifies blank or unsafe `Authorization: Bearer`
  headers as bearer traffic.
- API metering durable event metrics clamp negative byte/latency values to zero
  and default nonpositive request counts to one before ledger/aggregate storage.
- API metering outbox payloads clamp negative byte/latency values to zero before
  deterministic event IDs are generated.
- API metering durable events require nonblank method/route keys and HTTP-like
  status codes before ledger/aggregate storage.
- API metering middleware route-key extraction drops CR/LF-bearing or oversized
  ServeMux patterns and fallback paths before sink dispatch.
- API metering durable event decoding rejects CR/LF-bearing method, route,
  event-id, tenant, company, domain, user, API-key, and principal dimensions
  before ledger/aggregate storage.
- Admin API usage ledger, NDJSON export, stats, export-batch creation, and
  retention-readiness tenant/principal filters now reject CR/LF-bearing or
  oversized values before service dispatch.
- Admin user listing, IMAP UID backfill, DKIM key listing, and delivery-route
  resolution query filters now share the same CR/LF and size boundary checks;
  DKIM key listing can also filter by `active|inactive` status.
- API usage export batch, artifact, manifest-digest, and signature path
  identifiers now reject blank, CR/LF-bearing, or oversized values before
  service dispatch.
- Admin company, domain, and user detail/mutation path identifiers now use the
  same blank, CR/LF, and size validation before service dispatch.
- Admin IMAP UID backfill mailbox IDs, outbox event/retry IDs, DKIM key IDs,
  suppression IDs, trusted-relay IDs, and delivery-route IDs now use the same
  path boundary validation before service dispatch.
- Mail API development `user_id` query fallback values now reject CR/LF-bearing
  or oversized identifiers before service dispatch.
- OpenAPI now wires the Mail API development `user_id` fallback parameter into
  every user-scoped Mail operation, keeping local/all-in-one generated clients
  aligned with JWT-disabled runtime behavior.
- Mail API folder, thread, message, draft, attachment, and push-device path
  identifiers now reject blank, CR/LF-bearing, or oversized values before
  service dispatch.
- Push-device create/update validation now rejects invalid-UTF-8,
  CR/LF-bearing, or oversized user and token metadata before repository upsert,
  keeping raw provider tokens bounded at the storage boundary.
- Mail API message-list `folder_id` and search text/filter query parameters now
  reject CR/LF-bearing or oversized values before service dispatch.
- Mail API bearer JWT `user_id` and `sub` identities now reject CR/LF-bearing
  or oversized claims during signing and verification before request scoping.
- Mail API bearer JWT verification now rejects oversized token, header,
  payload, and signature segments before base64 decoding claim data.
- Mail and Admin API authentication headers now reject oversized `Authorization`
  and `X-Admin-Token` values before bearer/JWT parsing or token comparison.
- Password hash verification now rejects oversized stored hashes, excessive
  PBKDF2 iteration counts, and oversized PBKDF2 salt/key metadata before
  expensive derivation or decoded allocation.
- Mail API search control query values and direct multipart attachment
  `draft_id` fields now reject CR/LF-bearing or oversized values at the HTTP
  boundary before service dispatch.
- VERP return-path parsing now rejects oversized addresses, local parts, tokens,
  and encoded recipients before base64 decoding DSN recipient metadata.
- API usage export Ed25519 signer/verifier key configuration now rejects
  oversized base64 public/private keys before decoding.
- API usage export manifest signer configuration now rejects CR/LF-bearing or
  oversized key IDs and remote signer tokens, and local HMAC signing rejects
  oversized secrets before MAC generation.
- API usage export HMAC and Ed25519 signature verification now rejects
  incorrectly sized signature hex before decoding.
- Remote Ed25519 manifest signer responses now reject oversized bodies and
  trailing JSON tokens before signature evidence is accepted.
- Remote Ed25519 manifest signer status-error diagnostics now collapse signer
  response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing
  external signer errors from leaking into export/billing diagnostics. Remote
  signer HTTP responses now use the shared bounded drain-and-close helper so
  keep-alive connections can be reused without unbounded cleanup reads.
- Attachment scan and push-notification webhooks now reject CR/LF-bearing
  configured tokens or endpoints and collapse non-2xx HTTP response bodies into
  bounded one-line UTF-8 previews before surfacing delivery failures. Shared
  webhook HTTP response cleanup now drains a small bounded body window before
  close so keep-alive connections can be reused without unbounded cleanup reads.
- API metering middleware falls back to `METHOD /path` when no `http.ServeMux`
  route pattern is available, keeping durable event route keys nonblank.
- API metering now records immutable `api_usage_ledger` rows before aggregate
  upserts. Admin API exposes bounded ledger list, NDJSON export, and stats
  endpoints for billing/export preparation while keeping HTTP request handling
  fail-open.
- Admin API exposes API usage ledger retention readiness so operators can check
  whether non-future cutoff-bound ledger rows are covered by a completed export
  batch with artifact, manifest digest, and signature evidence before retention
  is allowed.
- Admin API exposes bounded API usage ledger retention runs. Destructive runs
  require `confirm_ready=true`, reuse the readiness gate, and delete only a
  normalized batch of ready ledger rows, while dry-runs return the same envelope
  without mutation.
- Optional PostgreSQL integration coverage verifies retention runs do not delete
  blocked candidates, dry-runs do not mutate ready candidates, and destructive
  ready runs persist retention-run audit rows, delete only the requested bounded
  batch, and preserve newer ledger rows.
- Admin API exposes list/detail reads for persisted API usage ledger retention
  runs so operators can inspect blocked, dry-run, and destructive retention
  attempts after the fact.
- `api-usage-retention-worker` can run bounded API usage ledger retention on an
  interval or once-and-exit, dry-run by default, reusing the same readiness gate
  and persisted retention-run audit rows as the Admin API.
- Destructive API usage retention worker runs require both explicit
  `confirm_ready` configuration and a production-oriented `remote-ed25519`
  export manifest signer backend.
- API usage export capabilities now advertise retention-run support, retention
  worker support, and the remote-key requirement for destructive worker runs.
- API usage ledger retention now rejects future cutoffs at the repository
  boundary as well as the HTTP boundary, keeping worker/direct-call behavior
  aligned with the Admin API guardrail.
- Admin API exposes bounded audit-log list/detail reads with category, action,
  result, target-type, company/domain/user, and recent-window filters so stored
  operational audit records can be inspected through the release API surface.
- Domain DNS check and quota reconciliation correction audit rows now reuse the
  shared audit writer hash-chain logic instead of inserting empty hash fields.
- Trusted relay create/delete mutations now write hash-chain audit rows in the
  same database transaction as the policy change, keeping inbound relay-policy
  administration inspectable through the Admin audit API.
- Delivery route create/status/delete mutations now write hash-chain audit rows
  in the same database transaction as the gateway policy change, excluding
  relay auth secrets from audit detail.
- DKIM key create/upsert, deactivate, and DNS-verification mutations now write
  hash-chain audit rows in the same database transaction as the persisted key
  lifecycle change, without including private key material in audit detail.
- Domain and user lifecycle status updates now write hash-chain audit rows in
  the same database transaction as the status change, scoped by company/domain
  identifiers for tenant forensics.
- Company, domain, and user quota mutations now write hash-chain audit rows in
  the same database transaction as the quota change, including domain default
  user quota propagation counts for quota forensics.
- Domain policy mutations now write hash-chain audit rows in the same database
  transaction as the policy change, preserving inbound/outbound mode and size
  guardrail evidence for SMTP/Mail API enforcement forensics.
- Domain/user provisioning and user password-hash rotation now write hash-chain
  audit rows in the same database transaction as the persisted change, without
  including password hash material in audit detail.
- Shared audit-log normalization now bounds scalar metadata and JSON detail size
  before hash computation or database insertion.
- Admin API exposes a bounded audit-log integrity check that recomputes recent
  row hashes and reports hash or in-window prev-hash breaks without mutating the
  audit trail.
- API usage exports now have persisted batch manifests/checkpoints. Admin API
  can create/list/get manifest rows and replay a saved manifest window as NDJSON
  by batch ID. Batch creation now requires explicit RFC3339 `from`/`to`
  bounds, preventing accidental all-ledger checkpoints.
- API usage export batch listing can filter by tenant, principal, status, and
  export window so operators can find covering manifests without scanning every
  saved batch.
- API usage ledger/export/retention tenant and principal query filters are
  trimmed at the Admin API boundary before billing/export service dispatch.
- API usage export batches can now carry external artifact metadata rows with
  object key, content type, byte count, SHA-256, event count, and JSON metadata.
  Artifacts are deduplicated per batch by object key and SHA-256.
- API usage export artifact writes reject ambiguous object keys that would be
  changed by path cleaning or contain backslash/path-traversal segments before
  writing billing handoff objects.
- Admin API can now write API usage export batch artifacts to the configured
  object store, register the resulting byte count/SHA-256 metadata, and download
  or verify stored NDJSON artifacts for handoff verification.
- API usage export batches now have canonical manifest digest rows and
  verification endpoints. Operators can generate SHA-256 digests over the saved
  batch plus registered artifacts, list/fetch digest records, and re-check the
  stored manifest against its canonical digest before billing handoff.
- API usage export manifest digesting rejects unsupported explicit manifest
  schema versions before canonical digest evidence is generated.
- API usage export manifest digests can now be signed through disabled-by-
  default local HMAC, local Ed25519, or remote Ed25519 signers. The remote
  signer is intended for an external KMS-backed signing service and is verified
  locally with a configured public key before persistence. Admin API exposes
  signature create/list/detail and verification endpoints while keeping the
  signer backend pluggable.
- API usage export manifest signing validates key IDs for local and remote
  signers, rejecting blank, CR/LF-bearing, or oversized key metadata before
  signature evidence is returned.
- Admin API exposes API usage export handoff readiness by batch. The report
  summarizes artifact coverage, latest digest/signature state, operational
  readiness, and a separate billing readiness grade so local signers are not
  mistaken for invoice-grade exports.
- Handoff readiness can now opt into `deep=true`, which streams registered
  artifacts from object storage for byte/SHA verification and verifies the
  latest manifest digest/signature in one operator report while keeping
  metadata-only readiness fields stable.
- Manifest signature verification now sits behind an
  `apimeter.ExportManifestSignatureVerifier` boundary parallel to signing. The
  current wired verifiers are local-HMAC and Ed25519, supporting both local and
  remote Ed25519 signer backends.
- Admin API exposes API usage export capabilities so operators can see the
  configured signer backend, signer key ID, verifier availability, and whether
  production/verified billing readiness is supported before creating handoff
  batches.
- Push notification enqueue now has an async worker boundary:
  `push-notification-worker` consumes `mail.stored` events, resolves active
  user devices from PostgreSQL, and can emit disabled-by-default `slog`
  notification candidates or POST raw-token targets to a configured HTTP
  webhook push gateway with an optional bounded bearer token and Postgres
  candidate-attempt audit rows without touching SMTP hot paths or committing to
  FCM/APNs SDKs. `docs/webhook-integrations.md` documents the push gateway
  payload, authentication, HTTPS requirement, and queued/failed attempt
  semantics. Malformed resolved targets with blank or CR/LF-bearing device IDs
  or tokens, oversized device IDs or tokens, or unsupported platforms, are
  dropped before candidate recording and sink handoff; optional target labels
  and token suffixes are UTF-8 safely bounded. The webhook sink also bounds and
  normalizes direct-call payload metadata before JSON serialization.
- Admin API exposes `GET /admin/v1/push-notification-attempts` for inspecting
  push notification candidate fan-out by message, status, user, platform,
  device, provider status, provider message id, or recent time window.
- Admin API exposes `GET /admin/v1/push-notification-attempts/{id}` for
  single-attempt troubleshooting.
- Admin API exposes
  `PATCH /admin/v1/push-notification-attempts/{id}/outcome` for authenticated
  operator/provider handoff updates to queued, delivered, failed, or
  invalid-token outcomes with bounded provider diagnostics.
- Admin API exposes `GET /admin/v1/push-notification-stats` for a compact
  active-device and attempt-status summary, with optional `message_id`,
  `user_id`, `platform`, `device_id`, and `since` scoping for message-level,
  user-level, provider-platform, device-level, and recent-window
  troubleshooting.
- Push notification sinks receive the persisted candidate attempt id with each
  target, preparing clean vendor outcome updates later.
- Push notification candidate and provider-outcome diagnostics are capped at
  UTF-8 boundaries before Postgres storage, preserving internationalized
  subjects and vendor messages in Admin API views.
- Push notification candidate recording rejects invalid-UTF-8, CR/LF-bearing,
  or oversized message/user/device/company/domain IDs before SQL insert
  dispatch, and rejects unsupported platforms at the recorder boundary.
- The push worker marks attempts `queued` after a successful sink handoff while
  marking failed sink handoffs as `failed` with the sink error before returning
  the handler error for Redis stream retry.
- Existing push attempts can be updated to queued, delivered, failed, or
  invalid-token outcomes through the internal recorder or Admin API.
- The push worker's internal outcome recorder now delegates to the same
  `maildb` outcome update path used by the Admin API, keeping provider status
  validation, diagnostic bounds, timestamp updates, and invalid-token device
  deletion in one storage boundary.
- Push notification outcome recording rejects invalid-UTF-8, CR/LF-bearing, or
  oversized attempt IDs before SQL update dispatch.
- Invalid-token outcomes automatically soft-delete the affected push device in
  the same Postgres transaction.
- `mail.stored` events now carry an explicit
  `2026-05-04.mail-stored.v1` schema version for downstream audit, search, and
  push workers.
- Audit, search indexing, and push notification consumers reject unsupported
  explicit `mail.stored` schema versions while accepting versionless legacy
  events.
- The audit `mail.stored` consumer trims event, tenant, recipient, subject,
  storage, and timestamp fields and rejects CR/LF-bearing message identifiers
  before audit-log persistence.
- Delivery-status audit consumers trim event, tenant, sender, recipient, farm,
  status, error, and timestamp fields and reject CR/LF-bearing message
  identifiers before audit-log persistence.
- Delivery outcome and exhausted outbox event payloads trim message, tenant,
  farm, sender, recipient, error, and DSN metadata before event persistence.
- Mail API now has user-scoped push device registration/list/delete contracts
  for `apns`, `fcm`, and `webpush`; raw device tokens are accepted only on
  write and are not returned in API JSON responses.
- Push-device list and delete service methods trim user and device identifiers
  before repository work, and delete rejects blank, CR/LF-bearing, or oversized
  device identifiers before repository dispatch.
- DKIM key DNS verification workflow with `dns_verified_at` persistence.
- Delivery route runtime counters (`RouteCounters`) with Admin API exposure.
- Retry exhaustion hook: `mail.delivery_exhausted` outbox event emitted and
  `delivery_attempts` row with status `exhausted` written when all retries fail.
- The delivery worker wires retry exhaustion recording at runtime, so terminal
  retry exhaustion diagnostics and `mail.delivery_exhausted` events are emitted
  by the actual worker path.
- Retry-exhausted delivery events now carry recipient-level DSN metadata and
  safe original storage paths into the event worker, generating sender-facing
  RFC 3464 failure DSNs with deterministic dedupe keys while preserving
  `NOTIFY=NEVER` and null reverse-path suppression.
- Admin delivery attempt lists can be scoped by status, recipient domain, and
  recent time window for bounded retry/bounce triage.
- Admin delivery attempt stats summarize total attempts, unique messages,
  unique recipients, and delivered/failed/bounced/exhausted buckets with the
  same status, recipient-domain, and recent-window filters.
- Admin delivery-route status/delete handlers trim route IDs at the HTTP
  boundary before operator mutations are passed to the service layer.
- User-scoped sent-message delivery status treats failed attempts with RFC 3463
  `4.x.x` enhanced status codes as retrying rather than terminal failed.
- User-scoped sent-message delivery status treats terminal `exhausted`
  attempts as failed so retry budgets do not remain visible as pending forever.
- DMARC reject policy enforcement at SMTP receive (`DMARCEnforce` flag).
- Authentication-Results trace header formatting now strips control characters
  and bounds verifier metadata before formatting SPF/DKIM/DMARC results,
  preventing DNS/library diagnostics from injecting or bloating stored headers.
- SMTPUTF8 declared correctly on outbound MAIL FROM for all internationalized
  addresses, and outbound delivery now fails closed with a permanent SMTPUTF8
  error when the remote MTA does not advertise SMTPUTF8.
- DSN composition supports optional `text/rfc822-headers` and `message/rfc822`
  returned-content parts for RFC 3464 reports, keeping header-only returns
  sanitized while allowing bounded full-message returns.
- Bounce DSN generation now honors `RET=HDRS` when the delivery event carries a
  safe original message storage path, reading bounded original EML headers and
  attaching them as sanitized `text/rfc822-headers` content.
- Bounce DSN generation now also honors `RET=FULL` by attaching the bounded
  original `.eml` as `message/rfc822` after validating the stored object key and
  message parseability.
- Migration guardrails now require every SQL migration to declare explicit
  goose Up/Down sections, and legacy API-usage, push, IMAP, and audit-index
  migrations have been normalized to that structure without changing their
  applied SQL.
- OpenAPI draft with route, request body, response envelope, operationId, and
  component reference drift tests. Path parameters, Mail search/Admin query filters,
  request schemas, response envelopes, and status enums are contract-tested for
  generated-client readiness. The draft is parsed as YAML and checked for stale
  documented routes that are not registered by the Go HTTP mux. Thread list
  parameters are guarded against accidental Admin/API-usage filter leakage.
  Non-JSON download/export responses are guarded so NDJSON streams and binary
  attachments are not modeled as JSON envelopes. All schemas are kept in sync
  with Go types.
- Admin token authorization and API metering admin-token classification compare
  fixed-length SHA-256 digests of trimmed token values for both bearer tokens
  and `X-Admin-Token`.
- Mail API JWT verification rejects unsupported JWT `alg` values and non-JWT
  `typ` headers before accepting signed bearer claims. JWT `user_id`/`sub`
  identities are whitespace-normalized and blank identities are rejected during
  both signing and verification. Tokens with `iat` values more than one minute
  in the future are rejected before Mail API claims are trusted.
- Redis event consumers acknowledge malformed stream entries after logging
  decode failures and move repeatedly handler-failing messages into a durable
  Redis dead-letter stream before acknowledging the original event, preventing
  poison messages from pinning worker progress indefinitely. Event,
  search-index, API-metering, push-notification, and delivery workers expose
  per-worker max-delivery and dead-letter-stream settings for production tuning.
- Redis event/search/API-metering/push/delivery workers reclaim idle pending
  Redis Stream messages via configurable claim-idle settings, improving crash
  recovery for at-least-once event processing. Startup validation now also
  rejects nonpositive event and delivery consumer count/block settings before
  workers run with unusable Redis Stream options.
- Redis worker stream, group, and consumer-name settings for event,
  search-index, API-metering, push-notification, and delivery workers are now
  required, CR/LF-rejected, and size-bounded during startup config validation,
  surfacing worker identity mistakes before consumer construction.
- `eventstream.NewRedisConsumer` now applies the same trim, required,
  CR/LF-rejection, and size-bound guardrails to stream, group, and consumer
  identifiers, keeping direct adapter callers aligned with runtime config
  validation.
- Event routing trims registered and payload event names and rejects
  CR/LF-bearing or oversized event names before worker dispatch.
- Redis stream event decoding rejects CR/LF-bearing or oversized outbox IDs,
  partition keys, and payloads before worker fan-out.
- Redis stream event decoding trims outbox id, partition key, and payload
  fields and rejects blank metadata before handler dispatch.
- Redis outbox publishing trims event id, topic, partition key, and payload
  metadata and rejects invalid topics or non-JSON payloads before stream writes.
- Admin API exposes a bounded IMAP mailbox UID backfill endpoint for future
  IMAP bootstrap/operator runs without enabling an IMAP protocol listener.
- Push notification workers no longer redeliver a Redis event solely because
  queued-outcome recording failed after the sink accepted the notification,
  reducing duplicate push risk while keeping the candidate attempt visible.
- Backend release verification script and SMTP release runbook.
- API usage export runbook covering capability checks, artifact/digest/signature
  handoff evidence, deep readiness, and retention-readiness gates.
- Public GitHub repository:
  <https://github.com/parkjangwon/gogomail>

## Explicitly not started

- Next.js shell/webmail/admin frontend implementation.
- Built-in spam scoring or pattern filtering.
- POP3 protocol server work. The future POP3 server should follow the same
  strict RFC, performance, and client-compatibility standard as IMAP.
- OpenSearch as the default/mandatory search backend.
- Kafka migration.
- etcd/Vault production control plane.
- Vendor push notification delivery adapters.

## Important guardrails

- Implemented SMTP features must strictly follow the relevant email RFCs.
- Do not advertise SMTP extensions until end-to-end semantics are implemented
  and tested.
- Do not turn SMTP core into a spam engine. Spam relay/filtering belongs behind
  explicit hooks/adapters.
- Keep hot paths streaming and allocation-aware.
- Preserve domain-as-tenant isolation.
- Commit by feature and push after completed work.

## Latest direction

The platform hardening sprint completed the following:

- Mailbox quota enforcement (receive, send, delete)
- Per-domain SMTP inbound policy (max recipients, max message size)
- DKIM DNS verification workflow
- Delivery route runtime counters
- Retry exhaustion events and Admin API exposure
- SMTPUTF8 outbound RFC 6531 fix
- DMARC reject policy enforcement hook
- Domain aggregate stats endpoint
- OpenAPI schema expansion (DKIMKey, DeliveryAttempt, DKIMKeyDNSVerification)
- Hierarchical quota ledger first implementation: company/domain/user Admin
  quota APIs, user quota source, domain default user quota propagation, and
  aggregate quota enforcement for mail writes/deletes.
- Attachment upload quota integration: upload metadata reserves quota, stale
  upload cleanup releases quota, and API quota exhaustion maps to 507.
- `attachment-cleanup-worker` can now run the stale upload cleanup loop
  periodically with configurable interval, stale age, and batch size, turning
  the repository/service cleanup path into an operational mode. It can also run
  once and exit for CronJob or timer-style deployments.
- Search indexing boundary: bounded received body extraction runs in
  `search-index-worker` and stores Postgres search documents outside SMTP hot
  paths.
- OpenSearch indexing has a first `internal/searchindex` writer adapter behind
  the same indexing interface, and `search-index-worker` can select it with
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`.
- The OpenSearch writer can bootstrap a strict message index mapping for
  message IDs, tenant/user filters, subject/body text, timestamps, and bounded
  body metadata.
- `search-index-worker` can optionally bootstrap the OpenSearch index on startup
  with `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`.
- OpenSearch query-side groundwork can search user-scoped documents and return
  ranked gogomail message IDs for later metadata hydration.
- `maildb` can hydrate ordered message ID search hits back into active
  `MessageSummary` rows without changing the Mail API response envelope.
- `mailservice` can compose OpenSearch relevance ID hits with Postgres summary
  hydration when the current API search contract can be preserved; unsupported
  filter/highlight combinations fall back to Postgres search.
- Mail API app wiring can inject the OpenSearch search source when
  `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, enabling safe relevance-search
  read-side rollout while preserving fallback behavior.
- OpenSearch indexed documents now include parsed sender and attachment
  presence fields needed for Mail API search-filter parity.
- OpenSearch relevance search can apply folder, from, subject, and attachment
  filters before Postgres metadata hydration; sender and subject filtering use
  lower-cased keyword fields to preserve case-insensitive filter behavior.
- OpenSearch relevance search can return subject/from/body highlights and map
  them into the existing Mail API `search_highlights` response field with
  bounded UTF-8-safe fragments.
- Mail API OpenSearch hydration deduplicates repeated external hit IDs before
  loading Postgres summaries while preserving the first rank/highlight result.
- Optional OpenSearch integration coverage can create a disposable index and
  verify indexing plus folder-aware relevance search when
  `GOGOMAIL_TEST_OPENSEARCH_URL` is available.
- Search index worker startup logs include non-secret backend diagnostics,
  including OpenSearch index name and bootstrap state when that backend is
  selected.
- OpenSearch writer/searcher HTTP calls use a configurable timeout through
  `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT`, defaulting to 10 seconds.
- OpenSearch writer/searcher HTTP responses now use the shared bounded
  drain-and-close helper, improving connection reuse for indexing, bootstrap,
  and relevance queries without allowing oversized responses to stall cleanup.
- OpenSearch endpoint configuration is now validated as an HTTP(S) URL with a
  host during startup config validation, so malformed search backend endpoints
  fail before worker/search adapter construction.
- OpenSearch index names are now validated during startup config validation
  using the same unsafe-character and reserved-prefix guardrails as the
  adapter, so invalid index configuration fails before worker/search setup.
- Search contract expansion: clients can request `sort=relevance`,
  `include_rank=true`, and `include_highlights=true` without changing the
  default message list shape.
- Quota operations read models: capacity fields and reconciliation reporting
  show ledger pressure and drift without mutating counters.
- Quota correction actions: operators can explicitly apply reconciliation
  results to company/domain/user ledgers after reviewing drift, with dry-run and
  applied correction attempts recorded in audit logs.
- IMAP gateway planning: native backend interfaces, RFC-shaped flag/mailbox
  helpers, and durable UID/MODSEQ storage exist without starting a TCP protocol
  server.
- The first IMAP adapter path can list/get mailboxes, list mailbox messages,
  resolve messages by UID, stream raw stored message bodies, and mutate
  persisted IMAP-visible flags with MODSEQ advancement as `internal/imapgw`
  DTOs while ensuring UID state.
- Existing active mailbox contents can be backfilled with stable mailbox-local
  IMAP UIDs in bounded batches before any live IMAP listener is enabled.
- The shared `event-worker` now consumes committed `mail.stored` events through
  an IMAP UID handler that ensures newly received active messages get
  mailbox-local UIDs asynchronously after SMTP storage commits. Stale
  `mail.stored` events for messages that were moved or deleted before UID
  assignment are treated as no-ops instead of retrying forever.
- IMAP UID assignment event decoding rejects CR/LF-bearing or oversized
  message, user, and folder IDs before UID work or mailbox event fan-out.
- Push notification `mail.stored` event decoding rejects CR/LF-bearing or
  oversized message/user IDs before target resolution or candidate fan-out.
- Search indexing `mail.stored` event decoding rejects oversized message/user
  IDs and storage paths before stored EML objects are opened.
- Mail receive audit event decoding rejects CR/LF-bearing or oversized
  message IDs before immutable audit log construction.
- Delivery status audit event decoding rejects CR/LF-bearing or oversized
  message IDs before immutable audit log construction.
- Delivery `mail.queued` decoding rejects oversized message identities and
  storage paths, and rejects ambiguous, absolute, parent-traversal,
  backslash-bearing, or non-`.eml` storage object keys before SMTP transport or
  message storage access.
- Delivery `mail.queued` DSN option decoding rejects oversized
  `original_recipient` values before retry/delivery attempt recording.
- Delivery `mail.queued` decoding rejects oversized recipient and DSN-recipient
  arrays before normalization, routing, or retry bookkeeping.
- Authenticated Submission now applies enforcing per-domain recipient caps during
  `RCPT TO`, not only after `DATA`, so oversized envelopes receive earlier
  SMTP feedback before message streaming/spooling.
- Attachment scanner hook rejection/tempfail reasons are CR/LF-stripped and
  UTF-8 safely bounded before they are surfaced as SMTP hook errors.
- `GOGOMAIL_ENV` now accepts only `development`, `test`, or `production`, so
  environment typos cannot silently bypass production-only safety gates.
- Redis-backed deduplication, recipient rate limiting, and SMTP backpressure
  backend selectors now accept only `none` or `redis`, preventing typos from
  silently disabling operational controls.
- Redis-backed RCPT rate-limit keys now normalize remote addresses to the
  remote host/IP bucket instead of the full `ip:port`, preventing source-port
  churn from bypassing recipient abuse controls.
- RCPT rate-limit and outbox relay batch, poll, and max-attempt settings are
  now validated as positive values during startup config validation, surfacing
  relay/limit misconfiguration before workers start.
- HTTP, SMTP, inbound SMTP, Submission, and optional SMTPS listener addresses
  are now validated as TCP `host:port` values at startup, surfacing bind
  configuration mistakes before runtime listener setup.
- Delivery retry delay schedules and maximum delay caps are now validated as
  positive durations, preventing retry jobs from being exhausted accidentally
  or scheduled in the past by malformed runtime configuration.
- `GOGOMAIL_DELIVERY_SMTP_HELLO` is now validated as a non-empty
  whitespace-free hostname during startup config validation, surfacing outbound
  SMTP EHLO configuration mistakes before delivery worker startup.
- Attachment scanning can be enabled with a configured HTTP webhook backend;
  the hook remains disabled by default, supports an optional bounded bearer
  token, requires HTTPS in production, and is wired only at SMTP
  receive/submission app boundaries. `docs/webhook-integrations.md` documents
  the scanner request, bounded response, and accept/reject/tempfail verdict
  contract. Scanner webhook requests bound and normalize message, address,
  subject, recipient, and attachment metadata before JSON serialization.
- Redis duplicate-message detection uses fixed-length hashed dedup keys so raw
  message IDs or recipient addresses cannot create oversized Redis keys.
- Mail API move/delete operations invalidate stale IMAP UID rows in the same
  transaction, and IMAP UID idempotency checks require the same active
  user/mailbox before reusing an existing UID, keeping mailbox-local UID state
  from leaking across folders.
- Optional PostgreSQL integration coverage now exercises IMAP UID backfill and
  move invalidation when `GOGOMAIL_TEST_DATABASE_URL` is available.
- `internal/imapgw` has a small in-memory mailbox event broker for live IDLE
  and NOOP fan-out through the protocol listener; broker delivery is scoped by
  both user and mailbox to preserve tenant isolation.
- `mailservice.StoreIMAPFlags` can publish IMAP mailbox `flags` events through
  an optional event publisher after repository flag mutations succeed.
- Mail API single and bulk flag mutations can look up existing IMAP UID rows and
  publish mailbox `flags` events for UID-visible messages after the database
  update succeeds.
- Mail API detail reads that auto-mark unread messages as read now also publish
  mailbox `flags` events for UID-visible messages after the read-flag write
  succeeds.
- Mail API single and bulk move mutations can publish mailbox `expunge` events
  for previously UID-visible source messages after the database move succeeds.
- Mail API single and bulk delete mutations can publish mailbox `expunge`
  events for previously UID-visible messages after soft-delete succeeds.
- `mailservice` exposes IMAP mailbox/message listing and mailbox-event
  subscription methods, keeping the protocol listener pointed at the service
  boundary instead of `maildb` internals.
- `mailservice` exposes bounded IMAP UID backfill through the same service
  boundary for future operator/bootstrap modes.
- IMAP mailbox event publication from service mutations is best-effort, so a
  fan-out failure does not turn an already-committed mail mutation into a client
  error.
- Mail API move/delete expunge notifications carry mailbox sequence numbers
  from IMAP UID lookup, allowing selected `NOOP`/`IDLE` clients to receive
  renderable untagged `EXPUNGE` updates.
- `mailservice` has an `IMAPStoreAdapter` that satisfies `imapgw.Store`, so a
  protocol listener can depend on the gateway interface while still routing
  through service methods.
- `IMAPStoreAdapter` now also satisfies `imapgw.MailboxSessionStore` for
  mailbox selection, service-backed COPY/MOVE/EXPUNGE, and event subscription.
- IMAP `UID FETCH` and `UID STORE` untagged `FETCH` responses use message
  sequence numbers per RFC 3501 while keeping the requested UID in response
  attributes, and `RFC822.SIZE` metadata requests do not trigger body streaming.
- IMAP `UID FETCH` accepts bounded numeric UID sets/ranges and recognizes
  `BODY.PEEK[]` for clients that batch reads without read-flag side effects.
- IMAP non-UID `FETCH` accepts bounded sequence sets, including `*`, and maps
  them through the selected mailbox list before returning fetch responses.
- IMAP `EXAMINE` supports read-only mailbox selection and blocks `UID STORE`
  mutations in that state.
- IMAP `EXAMINE` passes read-only selection intent through the backend
  `SelectMailboxRequest`, so service adapters can distinguish read-only
  sessions from writable `SELECT`.
- IMAP `SELECT`/`EXAMINE` now establish mailbox event subscriptions before
  emitting selected-mailbox response data, avoiding ambiguous partial selection
  state when subscription setup fails.
- IMAP `CHECK` and `CLOSE` support selected-mailbox lifecycle handling; `CLOSE`
  silently expunges `\Deleted` messages for writable selections before clearing
  selected state, while read-only selections only clear state.
- IMAP `STATUS` validates requested status data items and returns only the
  requested mailbox metadata fields.
- IMAP mailbox lookup resolves wire names such as `INBOX` and `Archive/2026`
  to the stored mailbox ID before selected-mailbox state is used by follow-up
  commands.
- IMAP `LIST` filters mailbox responses with exact, `*`, and `%` patterns over
  decoded mailbox names, then emits non-ASCII names and ampersands as RFC 3501
  modified UTF-7 instead of raw UTF-8 while `UTF8=ACCEPT` is not advertised.
- IMAP `CAPABILITY` advertises `SPECIAL-USE` and RFC 3348 `CHILDREN`; `LIST`
  includes RFC 3348 `\HasChildren` / `\HasNoChildren` hierarchy attributes
  plus RFC 6154 special-use attributes for system folders such as Drafts, Sent,
  Trash, Junk, Archive, All, and Flagged when those folder roles are present in
  storage metadata, and extended
  `LIST (SPECIAL-USE)`, `RETURN (SPECIAL-USE)`, and no-op
  `RETURN (CHILDREN)` forms are accepted.
- IMAP `CAPABILITY` advertises RFC 5258 `LIST-EXTENDED` and RFC 5819
  `LIST-STATUS`; extended
  `LIST ... RETURN (STATUS (...))` emits requested `STATUS` metadata after each
  matching selectable mailbox, can be combined with `RETURN (CHILDREN)`, and
  rejects malformed `RETURN (STATUS MESSAGES)` style status item lists before
  mailbox listing work.
- IMAP `CAPABILITY` advertises RFC 8438 `STATUS=SIZE`; `STATUS` and
  `LIST-STATUS` can return per-mailbox total active message octets without
  fetching every message's `RFC822.SIZE`.
- IMAP `CAPABILITY` advertises RFC 5256 `SORT`; `SORT` and `UID SORT` evaluate
  the existing selected-mailbox search criteria, require `US-ASCII` or `UTF-8`
  charset arguments, and return sequence-number or UID `SORT` responses over
  RFC 5256 sort keys including base-subject, sent-date, arrival-date, address,
  and size ordering. Sort criterion atoms and `REVERSE` are interpreted
  case-insensitively while the strict parenthesized atom-list grammar remains
  enforced.
- IMAP `CAPABILITY` advertises RFC 5256 `THREAD=ORDEREDSUBJECT`; `THREAD
  ORDEREDSUBJECT` and `UID THREAD ORDEREDSUBJECT` reuse the selected-mailbox
  search evaluator, enforce mandatory `US-ASCII`/`UTF-8` charset handling, and
  return RFC-shaped ordered-subject thread trees while leaving the more complex
  `REFERENCES` algorithm unadvertised until its Message-ID normalization and
  ancestry rules are implemented.
- IMAP RFC 5256 base-subject handling decodes RFC 2047 encoded-word subjects
  before removing reply/forward artifacts, keeping internationalized
  `SORT SUBJECT` and `THREAD ORDEREDSUBJECT` behavior aligned with compatible
  clients.
- IMAP `LIST "" ""` and `LSUB "" ""` return the hierarchy root with
  `\Noselect` and `/` delimiter metadata for clients that probe namespace
  delimiters through LIST-compatible commands.
- IMAP `SELECT`/`EXAMINE` emit `[PERMANENTFLAGS]` response codes for writable
  versus read-only selected-mailbox state.
- IMAP `SELECT`/`EXAMINE` emit RFC-shaped untagged `RECENT` counts alongside
  `EXISTS`, optional `[UNSEEN n]` first-unseen sequence hints, `UIDVALIDITY`,
  `UIDNEXT`, and optional `[HIGHESTMODSEQ ...]` metadata from durable mailbox
  UID state.
- IMAP `SELECT`/`EXAMINE` now emit RFC 4551-shaped `[NOMODSEQ]` when a
  CONDSTORE-aware selection has no persistent mailbox mod-sequence baseline,
  so clients that use `SELECT ... (CONDSTORE)` or prior `ENABLE CONDSTORE`
  receive an explicit cache-coherency signal instead of silence.
- IMAP `SELECT`/`EXAMINE` now emit `[UIDNOTSTICKY]` when the backend marks a
  mailbox's UIDs as non-sticky, keeping UIDPLUS-adjacent client state aligned
  with the selected mailbox's persistence guarantees.
- IMAP `UID STORE` supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses when requested.
- IMAP `FETCH`/`UID FETCH` can include `INTERNALDATE` and RFC-shaped `ENVELOPE`
  attributes from message summaries for mailbox list rendering.
- Service-backed IMAP message summaries now hydrate stored `To`, `Cc`, and
  `Bcc` address JSON into RFC-shaped ENVELOPE address lists, keeping real
  repository-backed `FETCH ENVELOPE`, address search, and address sort behavior
  aligned with the advertised protocol surface.
- IMAP shared fetch failure paths now tag failures with the command actually
  issued by the client, so regular `FETCH` failures no longer surface as
  `UID FETCH failed` responses while UID fetches retain UID-specific wording.
- IMAP `FETCH`/`UID FETCH` now apply RFC 3501 `\Seen` side effects for
  successful `BODY[...]`, `RFC822`, and `RFC822.TEXT` literal reads while
  preserving `BODY.PEEK[...]` and `RFC822.HEADER` as non-mutating preview
  requests.
- IMAP `FETCH`/`UID FETCH` now preserves RFC 3501 `RFC822`,
  `RFC822.HEADER`, and `RFC822.TEXT` response data item names instead of
  returning their `BODY[...]` equivalents on the wire.
- IMAP `CAPABILITY` advertises `CONDSTORE` and `ENABLE`; RFC 5161-shaped
  `ENABLE CONDSTORE` marks sessions CONDSTORE-aware before mailbox selection.
- IMAP `FETCH`/`UID FETCH` can include RFC 4551-shaped `MODSEQ (n)` attributes
  when requested, using durable per-message mod-sequences.
- IMAP `SEARCH`/`UID SEARCH` can match RFC 4551-shaped `MODSEQ` criteria,
  including optional metadata entry/type arguments, and append the highest
  matched mod-sequence to non-empty SEARCH responses.
- IMAP `CAPABILITY` advertises RFC 4731 `ESEARCH`; `SEARCH RETURN (...)` and
  `UID SEARCH RETURN (...)` can return `MIN`, `MAX`, compact `ALL`, `COUNT`,
  UID indicators, and CONDSTORE `MODSEQ` data in single untagged `ESEARCH`
  responses.
- IMAP `CAPABILITY` advertises RFC 5182 `SEARCHRES`; `SEARCH RETURN (SAVE)`
  stores the last search result in the selected session so `$` can be reused in
  subsequent `FETCH`, `UID FETCH`, `SEARCH`, `UID SEARCH`, `STORE`, `COPY`,
  `MOVE`, and `UID EXPUNGE` set positions.
- IMAP `SORT`/`UID SORT` and `THREAD`/`UID THREAD` now accept leading
  `RETURN (SAVE)` and save their matched result set for `$` reuse, extending
  RFC 5182 SEARCHRES coverage to SEARCH-based SORT/THREAD workflows without
  changing their normal untagged `SORT`/`THREAD` responses.
- Direct `ESEARCH` and `UID ESEARCH` commands now fail with an explicit `BAD`
  response explaining that RFC 7377 `MULTISEARCH` is required; the server
  continues advertising RFC 4731 `ESEARCH` only for `SEARCH RETURN (...)` and
  `UID SEARCH RETURN (...)`.
- IMAP `SEARCH RETURN (SAVE)` now clears the selected-session `$` result when a
  save-requested search fails with tagged `NO`, matching RFC 5182 failure
  semantics while leaving tagged `BAD` searches non-mutating.
- IMAP `FETCH`/`UID FETCH` supports RFC 4551-shaped `CHANGEDSINCE` modifiers,
  returning only messages with greater per-message mod-sequences and
  implicitly including `MODSEQ` response attributes.
- IMAP sessions become CONDSTORE-aware after `FETCH MODSEQ`,
  `FETCH CHANGEDSINCE`, `SEARCH MODSEQ`, or `STATUS HIGHESTMODSEQ`, and
  subsequent flag `FETCH` event/STORE echo responses include `MODSEQ`.
  `STATUS HIGHESTMODSEQ` now has end-to-end regression coverage through a
  following `SELECT` and `UID STORE`, so the awareness state is verified
  across mailbox selection rather than only on the immediate STATUS response.
  `ENABLE CONDSTORE` issued after mailbox selection now returns the selected
  mailbox's `HIGHESTMODSEQ` or `NOMODSEQ` before completion, matching RFC 7162
  first-enabling-command semantics; selected-session mod-sequence state is also
  refreshed by known APPEND/COPY/MOVE/STORE/event mutations. Repeated
  `ENABLE CONDSTORE` after the session is already CONDSTORE-aware now avoids
  re-emitting the selected mailbox baseline, keeping the "first enabling
  command" behavior precise while still returning `ENABLED CONDSTORE`.
  Mailboxes selected with `NOMODSEQ` now reject `FETCH`/`UID FETCH`
  `MODSEQ`/`CHANGEDSINCE`, `SEARCH`/`SORT`/`THREAD` `MODSEQ`, and
  `STORE`/`UID STORE` `UNCHANGEDSINCE` before backend mutation or scan work,
  matching RFC 7162's non-persistent mod-sequence semantics.
- IMAP `STORE`/`UID STORE` supports RFC 4551-shaped `(UNCHANGEDSINCE n)`
  modifiers with transactional per-message mod-sequence checks, applying
  passing updates and returning `[MODIFIED uid-set]` / `[MODIFIED sequence-set]`
  for stale messages. Conditional store response/event paths filter modified
  stale UIDs out of successful `FETCH` echoes and mailbox flag notifications.
- IMAP `SELECT` and `EXAMINE` accept the RFC 4551-shaped `(CONDSTORE)`
  parameter and mark the session CONDSTORE-aware.
- IMAP `FETCH`/`UID FETCH` keep a conservative single-part `BODYSTRUCTURE`
  fallback when only message headers are available, while metadata-only
  structure fetches now reopen the bounded raw message stream for richer MIME
  tree serialization.
- IMAP single-part `BODY`/`BODYSTRUCTURE` responses now derive content type,
  parameters, content-transfer-encoding, ID, and description from bounded raw
  message headers instead of always reporting text/plain defaults.
- IMAP `BODYSTRUCTURE` now uses the streaming MIME-structure parser for
  metadata-only fetches, returning multipart child order, subtype, parameters,
  transfer encodings, dispositions, body octets, and text line counts without
  retaining attachment payloads.
- IMAP `BODYSTRUCTURE` now emits RFC 3501-shaped `message/rfc822` bodies with
  encapsulated message header-derived envelope metadata, parsed nested body
  structure, and line counts instead of treating attached messages as generic
  basic parts.
- The shared MIME-structure parser now descends into `message/rfc822` parts
  while counting the encapsulated message bytes/lines and capturing bounded
  envelope metadata, so forwarded-message attachments expose nested body
  metadata without retaining payloads.
- IMAP `FETCH`/`UID FETCH` can now return RFC 3501-shaped
  `BODY[n.HEADER]` and `BODY[n.TEXT]` literals for `message/rfc822` parts,
  including forwarded-message attachments inside multipart messages.
- IMAP `FETCH`/`UID FETCH` can now return `BODY[n.HEADER.FIELDS (...)]` and
  `BODY[n.HEADER.FIELDS.NOT (...)]` subsets for `message/rfc822` parts, so
  clients can preview forwarded-message headers without fetching whole nested
  headers.
- IMAP `FETCH`/`UID FETCH` can now follow multipart body-part numbering inside
  top-level `message/rfc822` parts, including nested part MIME headers such as
  `BODY[1.2]` and `BODY[1.2.MIME]`.
- IMAP literal-fetch regression coverage now includes multipart messages that
  attach a `message/rfc822` whose encapsulated body is itself multipart,
  guarding forwarded-message paths such as `BODY[2.2]` and `BODY[2.2.MIME]`.
- IMAP `BODYSTRUCTURE` regression coverage now includes the same forwarded
  multipart shape, guarding nested `MESSAGE/RFC822` serialization when the
  encapsulated message body is multipart.
- Malformed encapsulated `message/rfc822` literals now degrade gracefully for
  nested section fetches, returning an empty header section and raw text bytes
  instead of failing the whole IMAP `FETCH`.
- IMAP combined `BODYSTRUCTURE` plus literal body/header fetches can reopen the
  raw message for MIME metadata while preserving the original reader for
  literal streaming, so common preview/header fetch batches keep rich structure
  responses.
- IMAP `FETCH`/`UID FETCH` supports standard `FAST`, `ALL`, and `FULL` macros,
  including the non-extensible `BODY` attribute for `FULL`.
- IMAP `FETCH`/`UID FETCH` can stream bounded header-only literals for
  `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`.
- IMAP non-UID `FETCH` uses the same bounded header literal path as `UID FETCH`
  for `BODY[HEADER]` and `RFC822.HEADER`.
- IMAP `FETCH`/`UID FETCH` can stream bounded text-only literals for
  `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and `RFC822.TEXT`, with regression coverage
  rejecting oversized section bodies before unbounded allocation.
- IMAP `FETCH`/`UID FETCH` can stream conservative single-part text literals
  for `BODY[1]` and `BODY.PEEK[1]`.
- IMAP `FETCH`/`UID FETCH` can stream bounded top-level multipart body-section
  literals such as `BODY[1]` and `BODY[2]`, allowing clients to read individual
  MIME parts without fetching the full message.
- IMAP `FETCH`/`UID FETCH` can stream bounded nested multipart body-section
  literals such as `BODY[1.2]` with a capped MIME part path depth.
- IMAP `FETCH`/`UID FETCH` can stream bounded partial windows over multipart
  body-section literals such as `BODY.PEEK[2]<4.4>`.
- IMAP `FETCH`/`UID FETCH` can answer conservative single-part MIME header
  requests for `BODY[1.MIME]` and `BODY.PEEK[1.MIME]`.
- IMAP `FETCH`/`UID FETCH` can stream actual multipart child MIME headers for
  `BODY[n.MIME]`/`BODY.PEEK[n.MIME]` requests when the selected part exists.
- IMAP `UID STORE` accepts bounded UID sets/ranges for batched flag mutation.
- IMAP non-UID `STORE` accepts bounded sequence sets/ranges and maps them to
  the same service-backed flag mutation boundary as `UID STORE`.
- IMAP non-UID `STORE` supports `.SILENT` flag mutation modes and suppresses
  untagged flag echo responses for those requests.
- IMAP `NOOP` drains queued selected-mailbox events into untagged `EXISTS`,
  `EXPUNGE`, and flag `FETCH` updates, suppressing stale or duplicate
  exact-count `EXISTS` events relative to the selected mailbox state.
- IMAP selected-state commands that resolve sequence sets now drain queued
  selected-mailbox events before dispatch, so `FETCH *`, sequence ranges,
  SEARCH/SORT/THREAD criteria, and mutation commands see the latest
  `EXISTS`/`EXPUNGE` state instead of operating on a stale selected count.
- IMAP advertises and accepts `IDLE`, entering continuation mode and streaming
  selected-mailbox `EXISTS`, `EXPUNGE`, and flag `FETCH` updates while waiting
  for `DONE`.
- IMAP `SEARCH ALL`, `SEARCH UID <set>`, and `UID SEARCH ALL` work over the
  selected mailbox message list.
- IMAP `SEARCH`/`UID SEARCH` accepts sequence-set criteria such as `2:*`,
  letting clients intersect standard search predicates with selected mailbox
  sequence ranges.
- IMAP `SEARCH`/`UID SEARCH` can combine supported criteria with RFC default
  AND semantics, including `ALL` plus flag, date, size, address, and UID
  filters.
- IMAP `SEARCH`/`UID SEARCH` supports RFC `NOT` and binary `OR` criteria
  composition over the supported search predicate set.
- IMAP `SEARCH`/`UID SEARCH` accepts parenthesized search-key groups, combining
  grouped predicates with RFC default AND semantics and allowing grouped
  operands inside `OR`.
- IMAP `FETCH`/`UID FETCH` can stream bounded partial full-body literals for
  `BODY[]<offset.count>` and `BODY.PEEK[]<offset.count>`.
- IMAP `FETCH`/`UID FETCH` can stream bounded partial section literals for
  common `BODY[HEADER]`, `BODY[TEXT]`, `BODY[1]`, and `BODY[1.MIME]` requests.
- IMAP `SEARCH`/`UID SEARCH` supports common flag criteria for unread, starred,
  answered, and draft client views.
- IMAP `STORE`/`UID STORE` can persist the IMAP-specific `\Deleted` flag
  separately from gogomail's soft-delete status, and `FETCH`/`SEARCH` expose
  that flag through `FLAGS`, `DELETED`, and `UNDELETED`.
- IMAP `SEARCH`/`UID SEARCH` supports `RECENT`, `OLD`, and `NEW` against the
  `MessageSummary.Recent` boundary. `NEW` now means recent and unseen, while
  `OLD` means not recent, preserving RFC-shaped semantics for backends that can
  expose per-session recentness and keeping legacy zero-value summaries old.
- IMAP custom keyword flags are now modeled in the protocol core through
  `MessageFlags.Keywords`. `SELECT`/`PERMANENTFLAGS` can advertise
  backend-provided keyword atoms, `FETCH FLAGS` renders canonical duplicate-free
  keywords, `SEARCH KEYWORD`/`UNKEYWORD` evaluates them, and `STORE` accepts
  permitted custom keywords. The PostgreSQL `maildb` adapter now persists
  user keywords in the IMAP-specific `imap_keywords` JSONB flag array across
  `APPEND`, `STORE`, `COPY`, `MOVE`, `FETCH`, and `SEARCH` read paths without
  mixing product labels into protocol state.
- IMAP `FETCH`/`UID FETCH` supports bounded `BODY[HEADER.FIELDS (...)]` and
  `BODY.PEEK[HEADER.FIELDS (...)]` literals.
- IMAP `FETCH`/`UID FETCH` supports bounded partial windows over
  `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
  `BODY[HEADER.FIELDS.NOT (...)]`, and `BODY.PEEK[HEADER.FIELDS.NOT (...)]`
  literals.
- IMAP `FETCH`/`UID FETCH` now preserves requested `HEADER.FIELDS` and
  `HEADER.FIELDS.NOT` section names in literal response items, including
  partial-window suffixes, instead of collapsing subset reads to
  `BODY[HEADER]`.
- IMAP `FETCH`/`UID FETCH` supports bounded `BODY[HEADER.FIELDS.NOT (...)]` and
  `BODY.PEEK[HEADER.FIELDS.NOT (...)]` literals.
- IMAP `SEARCH`/`UID SEARCH` supports `SINCE`, `BEFORE`, and `ON` over message
  `INTERNALDATE`, plus `SENTSINCE`, `SENTBEFORE`, and `SENTON` over envelope
  dates.
- IMAP `SEARCH`/`UID SEARCH` supports basic `FROM`, `TO`, `CC`, `BCC`, and
  `SUBJECT` substring criteria over selected-mailbox summaries.
- IMAP `SEARCH`/`UID SEARCH` supports bounded `BODY` and `TEXT` raw-message
  criteria scans, with `BODY` excluding the RFC 5322 header block.
- IMAP `SEARCH`/`UID SEARCH` supports bounded RFC `HEADER <field> <value>`
  criteria scans over the raw message header block.
- IMAP `SEARCH`/`UID SEARCH` now preserves RFC 3501 zero-length search string
  semantics for quoted empty strings across envelope, body/text, and header
  substring criteria, so client-generated `SEARCH SUBJECT ""` or
  `SEARCH BODY ""` requests do not become false empty-mailbox results.
  Escaped quote characters inside search strings are also preserved as literal
  query text instead of being stripped from the query boundary.
- IMAP `SEARCH`/`UID SEARCH` supports RFC 3501 `LARGER` and `SMALLER`
  criteria over message `RFC822.SIZE` metadata.
- IMAP `SEARCH`/`UID SEARCH` accepts `CHARSET US-ASCII` and `CHARSET UTF-8`
  prefixes and returns an RFC-shaped `[BADCHARSET]` response for unsupported
  search charsets.
- IMAP supports authenticated `NAMESPACE` for personal namespace and hierarchy
  delimiter discovery.
- IMAP `CAPABILITY` now advertises `NAMESPACE` alongside the implemented
  namespace command so client discovery matches the supported command surface.
- IMAP persists authenticated `SUBSCRIBE`/`UNSUBSCRIBE` mailbox subscriptions
  through the service/repository boundary, and `LSUB` now returns the saved
  subscription set instead of every visible mailbox.
- IMAP subscription canonicalization preserves hierarchy delimiters, quoting,
  internal spacing, and real leading/trailing spaces while keeping
  case-insensitive matching, preventing distinct subscribed mailbox names from
  silently collapsing into one `LSUB` row. The service/repository
  `SUBSCRIBE`/`UNSUBSCRIBE` boundary now preserves decoded mailbox-name
  spacing instead of trimming it before subscription persistence.
- IMAP live mailbox-event subscription now preserves the decoded mailbox ID
  after validation, so selected-mailbox IDLE/NOOP fan-out keys do not collapse
  mailbox names that intentionally contain leading or trailing spaces.
- IMAP service-backed mailbox lookup for `SELECT`/`EXAMINE` now preserves the
  decoded mailbox ID after validation before repository delegation, keeping the
  protocol adapter from trimming legitimate leading/trailing mailbox-name
  characters at the service boundary.
- PostgreSQL IMAP mailbox and APPEND-target lookup now separates exact decoded
  mailbox-name matching from compatibility aliases. Compatibility forms such
  as `INBOX` and slash-trimmed paths remain available for unpadded inputs, but
  names with real leading/trailing spaces no longer fall through to trimmed
  aliases before storage lookup.
- IMAP service-backed `APPEND` now preserves the decoded destination mailbox
  ID after validation before resolving the append target, so literal storage
  and UID assignment begin from the same mailbox identity semantics as
  `SELECT`/`EXAMINE`.
- IMAP service-backed `FETCH` and message listing now preserve decoded mailbox
  IDs after validation before repository delegation, keeping read-side storage
  access aligned with exact selected-mailbox identity rather than a trimmed
  service-local variant.
- IMAP service-backed `STORE`, `COPY`, `MOVE`, and `EXPUNGE` now preserve
  decoded mailbox IDs after validation before repository delegation. Mutation
  event fan-out continues to use repository-returned summary mailbox IDs, so
  exact selected-mailbox identity is not collapsed by the service layer.
- PostgreSQL IMAP UID/message operations now validate mailbox IDs for
  emptiness without trimming them before UUID-bound queries. Padded mailbox
  UUIDs therefore fail closed instead of being silently promoted to canonical
  folder IDs across list, fetch, store, copy, move, expunge, append-store,
  backfill, mailbox state, and message UID assignment boundaries.
- Service-level IMAP UID backfill now preserves mailbox IDs after validation
  before repository delegation, keeping operator/bootstrap work aligned with
  the same exact mailbox identity and audit semantics as client-visible IMAP
  paths.
- IMAP `SUBSCRIBE` can retain a mailbox name even when that mailbox does not
  currently exist, allowing `LSUB` to expose it with `\Noselect` for
  standards-friendly client migration and deleted-mailbox recovery flows.
- IMAP `LSUB` preserves subscribed mailbox names even when the mailbox no
  longer exists, returning missing names with `\Noselect`, and handles the RFC
  3501 `%` hierarchy case by returning subscribed parent levels.
- IMAP mailbox-taking commands now decode RFC 3501 modified UTF-7 at the
  protocol boundary for `SELECT`, `EXAMINE`, `STATUS`, `APPEND`, `COPY`,
  `MOVE`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, `UNSUBSCRIBE`, `LIST`,
  and `LSUB`, reject raw 8-bit or malformed modified UTF-7 forms, and keep
  internal service/storage mailbox names as UTF-8.
- IMAP quoted-string response formatting now escapes quotes/backslashes and
  cleans controls without collapsing ordinary spacing, preserving mailbox names,
  MIME parameters, and other wire values whose internal spaces are significant.
- IMAP advertises and supports RFC 2971 `ID`, validating bare no-argument
  probes, `NIL`, or bounded field/value parameter lists before returning
  gogomail server identity.
- IMAP advertises and supports `UNSELECT`, clearing selected-mailbox state
  without invoking `CLOSE`/EXPUNGE semantics.
- IMAP `EXPUNGE` and `UID EXPUNGE` delete only messages marked with the
  IMAP-specific `\Deleted` flag, emit RFC-shaped untagged sequence-number
  `EXPUNGE` responses, remove stale mailbox UID rows, and publish best-effort
  expunge events through the service boundary.
- IMAP `COPY` and `UID COPY` resolve sequence/UID sets through the selected
  mailbox, validate the destination mailbox, duplicate active message metadata
  and attachment rows transactionally, assign fresh destination mailbox UIDs,
  return UIDPLUS `[COPYUID ...]` response codes when destination UIDs are
  available, return `[TRYCREATE]` when the destination mailbox is missing, and
  publish best-effort destination `EXISTS` events through the service boundary.
- IMAP `MOVE` and `UID MOVE` resolve source sequence/UID sets through the
  selected mailbox, validate the destination mailbox, move active messages
  transactionally, assign fresh destination UIDs, and allow moves back into the
  selected mailbox by creating a fresh same-mailbox message before expunging
  the source UID. MOVE responses return UIDPLUS `[COPYUID ...]` mappings in
  the final tagged OK when destination UIDs are available, advance and return
  source mailbox `[HIGHESTMODSEQ ...]` metadata for CONDSTORE-aware clients,
  emit RFC-shaped source `EXPUNGE` responses, and return `[TRYCREATE]` when
  the destination mailbox is missing.
- IMAP `APPEND` now has a protocol-to-backend request boundary for mailbox,
  optional flag-list, optional internal date-time, literal body, and size after
  bounded literal framing. The boundary now returns UIDPLUS-ready append
  metadata so successful storage can emit `[APPENDUID uidvalidity uid]`; the
  service layer now spools and size-checks the literal body, parses the RFC
  message, writes the raw `.eml` through the configured storage backend, asks
  `maildb` to insert metadata, quota, outbox, and mailbox UID state in one
  transaction, publishes best-effort destination `EXISTS` events, and returns
  `[TRYCREATE]` when the destination mailbox is missing or `[OVERQUOTA]` when
  the quota ledger rejects the append. Commands without an RFC-shaped literal
  are rejected as syntax `BAD` responses instead of being reported as
  unsupported. Successful append results include the appended message sequence
  number, which is used as the precise `EXISTS` event count when available.
  APPEND internaldate parsing accepts RFC 3501 space-padded one-digit date-days
  such as `" 5-May-2026 ..."` while rejecting bare one-digit date-days such as
  `"5-May-2026 ..."`. The service boundary now rejects CR/LF-bearing or
  oversized APPEND user and mailbox identifiers before repository lookup,
  spooling, parsing, storage, or quota work.
- IMAP empty flag-lists are accepted where RFC-shaped clients can send them:
  `APPEND ()` stores without initial flags, `STORE FLAGS ()` clears supported
  flags, and empty `+FLAGS ()`/`-FLAGS ()` are treated as successful no-ops.
- IMAP service-backed `STORE`, `COPY`, `MOVE`, and `EXPUNGE` mutations reject
  CR/LF-bearing or oversized user and mailbox identifiers before repository
  mutation dispatch or mailbox event publication.
- IMAP service-backed read/list/subscription/backfill operations reject
  CR/LF-bearing or oversized user and mailbox identifiers before repository
  reads, storage opens, event subscriptions, or UID backfill work.
- IMAP service-backed `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` calls
  reject zero UIDs before repository or storage work, keeping direct service
  callers aligned with RFC 3501's positive UID model.
- IMAP service-backed `STORE`, `COPY`, and `MOVE` calls reject empty UID sets
  before repository work, while `EXPUNGE` preserves nil UID sets for `CLOSE`
  style "all deleted messages" semantics.
- IMAP selected-mailbox `APPEND` now prefers the backend-returned appended
  message sequence number for the untagged `EXISTS` count, falling back to a
  local increment only when precise sequence metadata is unavailable.
- IMAP selected-mailbox `COPY` and same-mailbox `MOVE` now also prefer
  backend-returned destination message sequence numbers for untagged `EXISTS`
  counts, falling back to local increments only when precise metadata is
  unavailable.
- IMAP selected-mailbox `EXPUNGE` events delivered through `NOOP` or `IDLE`
  now adjust saved SEARCHRES `$` sequence numbers the same way explicit
  `EXPUNGE` commands do, keeping subsequent `$` sequence-set reuse aligned with
  the client-visible mailbox state.
- IMAP `CREATE`, `DELETE`, and `RENAME` delegate to the service folder
  boundary for authenticated flat user-mailbox management, resolving wire names
  before destructive or rename operations and preserving the existing folder
  validation/storage constraints.
- IMAP `CREATE INBOX` and `DELETE INBOX` return explicit RFC 3501-shaped `NO`
  failures, and `RENAME INBOX` is rejected instead of incorrectly routing it
  through generic mailbox rename before its required special "move messages and
  leave INBOX empty" semantics are implemented.
- IMAP `EXAMINE` setup failures now return `NO EXAMINE failed` instead of
  `NO SELECT failed`, keeping tagged failure responses aligned with the command
  clients actually issued.
- IMAP malformed `UID` subcommands now route to their specific validators when
  the subcommand is recognized, so incomplete or structurally invalid
  `UID SEARCH`, `UID SORT`, `UID THREAD`, `UID FETCH`, `UID STORE`,
  `UID EXPUNGE`, and `UID COPY` requests receive precise tagged `BAD`
  responses before authentication or selected-mailbox state checks instead of a
  generic `UID command not implemented` failure.
- IMAP bare `UID` commands now return `BAD UID requires subcommand`, keeping
  missing-subcommand diagnostics separate from well-formed but unsupported UID
  subcommands.
- IMAP missing-mailbox failures for `SELECT`, `EXAMINE`, `STATUS`, `DELETE`,
  and `RENAME` now return tagged `[NONEXISTENT]` response codes instead of
  generic command failures, making absent folder state machine-readable for
  standards-aware clients.
- IMAP selected-state no-argument commands `CHECK`, `CLOSE`, `UNSELECT`, and
  `EXPUNGE` now reject extra arguments with tagged `BAD` responses instead of
  ignoring malformed input; this prevents accidental destructive expunge work
  from malformed `EXPUNGE` commands.
- IMAP any-state no-argument commands `CAPABILITY`, `NOOP`, and `LOGOUT` now
  reject extra arguments with tagged `BAD` responses instead of silently
  ignoring them or ending the session for malformed logout attempts.
- IMAP `STATUS` now requires a parenthesized status item list, rejecting
  malformed `STATUS mailbox MESSAGES`-style requests before mailbox metadata
  lookup.
- IMAP command dispatch now rejects malformed tags containing atom-special
  characters with untagged `BAD` responses before command handling, avoiding
  ambiguous tagged replies for invalid client command tags.
- IMAP command parsing now rejects control characters inside unquoted atoms,
  aligning atom parsing with the existing quoted-string control-character
  guardrail before command dispatch. Parser failures now return tagged `BAD`
  when a syntactically valid command tag can still be recovered, while malformed
  tags continue to receive untagged `BAD`.
- IMAP supports `STARTTLS` on plaintext listeners with configured TLS and stops
  advertising it after upgrade.
- IMAP `STARTTLS` completion includes an updated `[CAPABILITY ...]` response
  code for the post-TLS command surface.
- IMAP advertises `LOGINDISABLED` and rejects plaintext `LOGIN`/`AUTHENTICATE`
  with `[PRIVACYREQUIRED]` when insecure auth is disabled before STARTTLS.
- IMAP `CAPABILITY` drops `AUTH=PLAIN` after authentication, and unsupported
  command literal tokens are rejected instead of being treated as ordinary
  atoms. Bounded synchronizing command literals are consumed with a
  continuation response.
- IMAP now advertises `LITERAL+` and accepts bounded non-synchronizing command
  literals such as `APPEND ... {n+}` without an extra continuation round trip,
  while preserving the existing synchronizing literal path for conservative
  clients.
- IMAP command reading now supports bounded literals in non-final command
  positions and multiple literals in one command, so RFC-shaped literalized
  credentials or string arguments parse consistently with the advertised
  `LITERAL+` capability.
- IMAP command reading now enforces the same command-literal memory cap across
  the cumulative literal payloads in a single command, so several
  individually valid literals cannot exceed the per-command memory ceiling.
- IMAP server coverage now exercises `LOGIN` with multiple synchronizing
  literals in one command and verifies the reconstructed credentials reach the
  authentication boundary unchanged.
- IMAP command and IDLE line reads now enforce the command-line byte cap while
  reading from the socket instead of after an unbounded line allocation,
  keeping malformed clients from accumulating oversized lines in memory before
  syntax rejection.
- IMAP command framing now reports oversized command literals with a tagged
  `BAD` response when a valid tag is available, emits `BYE`, and closes the
  connection cleanly instead of surfacing an internal server error. This keeps
  unrecoverable literal-size violations visible to clients while preserving the
  bounded hot path.
- IMAP `AUTHENTICATE PLAIN` supports the standard continuation response,
  RFC-shaped tagged `BAD` cancellation, and SASL PLAIN credential decoding over
  the existing protocol auth adapter. Non-empty SASL PLAIN authorization
  identities are accepted only when they match the authentication identity,
  preventing delegated auth requests from being silently ignored until the
  backend contract explicitly supports them. Failed `LOGIN` and
  `AUTHENTICATE` attempts include RFC 5530 `[AUTHENTICATIONFAILED]` response
  codes for client-readable auth diagnostics.
- IMAP `LOGIN` and SASL PLAIN decoded credentials now reject blank,
  CR/LF-bearing, or oversized authentication identities plus empty, oversized,
  or CR/LF-bearing passwords at the protocol boundary before backend auth work,
  while preserving intentional leading/trailing spaces in quoted or decoded
  credential values.
- IMAP advertises `SASL-IR` before authentication and accepts
  `AUTHENTICATE PLAIN` initial responses to reduce client auth round trips.
- `gogomail --mode=imap` initializes the service-backed IMAP store adapter,
  a process-local mailbox event broker for future IDLE/session fan-out, and the
  configured TCP protocol listener.
- `gogomail --mode=imap` now runs a dedicated Redis consumer group for
  committed `mail.stored` events and publishes UID-bearing `EXISTS` updates
  into the process-local mailbox event broker for live IDLE sessions.
- Runtime config now includes validated `GOGOMAIL_IMAP_ADDR` listener metadata
  for the IMAP protocol listener.
- EML parser guardrails include a truncation-probe test and benchmark for the
  bounded text-body reader on large bodies, plus a `MaxParts` cap that reports
  `PartsTruncated` for pathological MIME part counts, plus address/reference
  metadata caps for oversized headers.
- `internal/message` exposes a bounded streaming MIME-structure parser that
  walks multipart trees, preserves raw transfer-encoding metadata, counts body
  octets/lines, and avoids retaining attachment payloads for future IMAP
  `BODYSTRUCTURE` serialization.
- EML attachment detection records inline parts with filenames and non-text
  inline parts from headers only, improving `has_attachment` accuracy without
  reading attachment bodies.
- Push notification worker boundary: `mail.stored` can be consumed by a
  dedicated notification worker with a replaceable sink and a bounded Postgres
  device-target resolver plus candidate-attempt persistence.
- Push notification attempts are inspectable through the Admin API without
  introducing vendor push delivery as a required runtime dependency.
- Push notification device storage: authenticated users can register, list, and
  delete active device tokens through the Mail API while responses expose only a
  short token suffix. Registration normalizes user, platform, token, and label
  fields before validation/storage.
- API metering boundary: HTTP middleware can emit fail-open usage events to
  logs or the durable outbox, while the disabled-by-default aggregation worker
  can build daily/monthly Postgres read models for operations.
- API metering events now carry an explicit schema version and deterministic
  event ID groundwork for future idempotent billing-grade aggregation.
- API metering aggregation now has an `api_usage_events` idempotency ledger so
  duplicate `event_id` deliveries do not increment daily/monthly counters again.
- API metering Admin API responses now expose tenant/company/domain/user/API-key,
  principal, and auth-source dimensions for daily/monthly aggregates.
- API metering Admin API now exposes immutable ledger list/export/stats endpoints
  so future billing and warehouse jobs can consume event-level usage instead of
  operational aggregates.
- API usage export batch manifests now capture fixed event/request/byte/latency
  totals for an explicitly bounded ledger window, preparing idempotent
  downstream export workflows.
- API usage export artifact metadata is now persisted and inspectable through
  Admin API endpoints, preparing object-store handoff without adding a vendor
  dependency to the core service.
- API usage export manifests now have canonical SHA-256 digest generation,
  local-HMAC/local-Ed25519/remote-Ed25519 signing, and verification Admin API
  endpoints, tightening the audit trail before invoice-grade signer deployment.
- API usage export artifact writing now has a local object-store adapter path
  through Admin API, including full-batch streaming, retry-friendly artifact
  registration, stored artifact download, and object body byte/SHA verification.
- API usage export handoff readiness now has a compact Admin API report that
  shows whether a batch has artifact coverage, a latest manifest digest, and a
  signature for that digest while keeping local signatures billing-blocked until
  production signing is wired.
- API usage export handoff readiness can now run an explicit deep verification
  mode for release/warehouse checks, returning artifact, digest, and signature
  verification evidence plus `verified_billing_ready` without turning the
  default readiness read into an object-store streaming operation.
- Attachment policy hardening: domain outbound policy can cap individual
  attachment upload sizes.
- Domain policy service lookups trim domain and user identifiers before
  repository policy reads for outbound and attachment enforcement.
- Direct multipart attachment uploads now distinguish over-limit HTTP request
  envelopes from malformed multipart bodies, returning 413 for the former and
  preserving 400 for bad multipart syntax.
- Attachment upload reservation and direct-upload service requests normalize
  user, draft, filename, MIME type, and storage-path metadata before quota,
  storage, and repository work, and reject CR/LF-bearing or oversized user,
  draft, and upload-session identifiers before quota reservation, object
  writes, or repository work.
- Stale attachment-upload cleanup validates its time window and limit at the
  service boundary before repository cleanup/object deletion work, and app
  configuration validates the worker interval, stale age, and batch size before
  runtime. Stored-object delete failures are now surfaced to the caller while
  missing objects are treated as already-cleaned idempotent deletes. Cleanup
  batch sizes use an attachment-specific 1000-row cap instead of the smaller
  message-list pagination cap.
- Admin API exposes `POST /admin/v1/attachment-cleanup/runs` for authenticated
  on-demand stale upload cleanup with an explicit non-future RFC3339 cutoff,
  and supports `dry_run` preview responses with total and batch-limited
  candidate counts before destructive cleanup. Cleanup run responses also
  report upload-session candidate and expired counts so operator dry-runs match
  the background worker's full cleanup scope. Operators can also list bounded
  legacy attachment-upload candidates plus stale upload-session candidates through
  `POST /admin/v1/attachment-cleanup/candidates`.
- Mail API exposes `DELETE /api/v1/attachments/{id}` so users can cancel
  unbound pending uploads immediately, releasing quota and removing any stored
  upload object without waiting for stale cleanup. Draft binding and send
  handoff ignore canceled/deleted uploads, and canceling a draft-bound upload
  clears the draft binding while refreshing the draft attachment-state cache.
- Mail API exposes `GET /api/v1/attachments/capabilities` so clients can
  discover upload limits, supported modes, and resumable-upload readiness
  without hard-coded constants.
- ADR 0007 records the future resumable/chunked upload boundary around explicit
  upload sessions, quota reservation, storage adapters, final attachment rows,
  and bounded cleanup.
- A migration now creates `attachment_upload_sessions` with lifecycle status,
  declared/received byte counts, expiry, checksum, storage adapter metadata, and
  indexes for user lookup and stale-session cleanup.
- `maildb` can create upload session records and reserve declared session bytes
  in the shared quota ledger transactionally.
- `maildb` can cancel resumable upload sessions in `pending`, `uploading`, or
  `failed` state, releasing the declared byte reservation once.
- `maildb` can expire stale resumable upload sessions in bounded batches,
  marking them `expired` and releasing declared quota reservations.
- `maildb` can count stale resumable upload sessions with the same normalized
  cleanup batch cap used by expiry, supporting non-destructive Admin previews.
- `maildb` can list bounded stale resumable upload-session candidates for
  operator cleanup previews without mutating quota reservations.
- Admin API can list bounded attachment upload sessions by user, draft, and
  lifecycle status, giving operators a direct inspection surface before cleanup
  or user-support actions.
- `mailservice` now owns resumable upload session create/cancel/expire methods,
  preserving attachment validation, max-size checks, and domain attachment
  policy enforcement above the repository boundary.
- `attachment-cleanup-worker` now expires stale resumable upload sessions during
  its normal bounded sweep, releasing reserved quota alongside stale direct
  upload cleanup.
- Mail API now exposes resumable upload session create/read/cancel endpoints under
  `/api/v1/attachments/upload-sessions`, reserving declared quota at session
  creation. Chunked upload is now enabled: `PUT /api/v1/attachments/upload-sessions/{id}/body`
  accepts `Content-Range: bytes first-last/total` headers for incremental chunk commits.
  Overlapping or gapped ranges return HTTP 416. Session creation rejects
  already-expired `expires_at` values before quota reservation.
- Attachment upload capabilities now distinguish upload session availability
  from full resumable chunk support so generated clients can adopt the staged
  lifecycle without assuming chunk receive/finalize routes exist, and expose
  the maximum upload session TTL.
- Mail API can store a complete body for an upload session, persisting it under
  session-scoped storage and recording received bytes plus SHA-256 digest before
  finalize creates the normal attachment row.
- Upload session body replacement writes retries to distinct staged object paths
  before repository metadata updates, preserving the previously recorded body if
  the DB update fails and best-effort deleting the previous staged body after a
  successful replacement.
- Upload-session staged object paths are validated as relative
  `upload-sessions/` keys before repository persistence and again before
  service-side storage reads/deletes, reducing risk from corrupted or manually
  edited rows.
- Upload session body storage can verify an optional client-provided
  `X-Content-SHA256` digest before recording the staged body.
- Upload session body storage accepts `Content-Range: bytes first-last/total` headers
  for chunked uploads, storing each chunk at `upload-sessions/{user}/{session}/chunks/{first}-{last}`
  paths. Overlapping or gapped chunks return HTTP 416. Repeated `X-Content-SHA256`
  control headers are still rejected before reading or storing the body.
- Attachment upload capabilities now advertise upload session checksum
  precondition support separately from body storage and finalization support.
- Upload session finalization now converts a ready stored session body into the
  normal pending attachment row without double-reserving quota, and marks the
  session finalized.
- Upload session finalization now verifies the staged object exists and still
  matches the recorded size and SHA-256 before creating the attachment row.
- Upload session cancellation now deletes a staged session body when the
  canceled session has already written one, preventing storage leaks alongside
  quota release.
- Upload session expiry now also deletes staged session bodies after the
  repository marks sessions expired and releases quota.
- Attachment list/download and draft-delete service methods trim user, message,
  attachment, and draft identifiers before repository/storage work; attachment
  reads reject blank, CR/LF-bearing, or oversized message/attachment
  identifiers before repository/storage dispatch.
- Mail API path identifiers and direct-upload `draft_id` fields are trimmed at
  the HTTP boundary before service dispatch, and direct multipart uploads reject
  repeated `draft_id` or `file` parts before storage work begins.
- Mail API search query, folder, sender, and subject filters are trimmed at the
  HTTP and service boundaries before search backend dispatch, and service
  search validation rejects CR/LF-bearing or oversized query/filter fields
  before Postgres or OpenSearch work.
- Mail compose draft/save/send requests normalize user/source/from/address and
  attachment identifier fields at the service boundary before repository,
  storage, suppression, and outbound composition work; draft saves share the
  send-time attachment-count cap so oversized compose payloads cannot drift
  into draft storage, and from/subject plus recipient display names/emails
  reject CR/LF before draft persistence or outbound header composition.
- Draft save/delete/send and reply/forward compose validation reject blank,
  CR/LF-bearing, or oversized draft/source-message identifiers before
  repository dispatch.
- Single-message flag, move, and delete service methods trim user/message/flag
  and folder identifiers before repository mutation and IMAP event fan-out, and
  reject blank, CR/LF-bearing, or oversized message/folder identifiers before
  repository or IMAP UID lookup work.
- Bulk flag, move, and delete service methods also trim user/message/flag and
  folder identifiers before repository mutation, IMAP UID lookup, and mailbox
  event fan-out; bulk message and folder identifiers reject CR/LF and oversized
  values before database query construction.
- Folder, message-list, thread-list, and message-detail service reads trim
  user, folder, thread, message, and folder-name inputs before repository work;
  user folder create/rename rejects blank, path-bearing, CR/LF-bearing, or
  oversized names, and rename/delete reject unsafe folder identifiers before
  repository dispatch. Folder list/create/rename/delete now also reject
  CR/LF-bearing or oversized user identifiers before repository work.
  Folder-scoped message lists and thread-message reads also reject unsafe
  folder/thread identifiers before repository work.
- Message, thread, and push-device list service methods normalize list limits
  to the documented message-list bounds before repository work.
- Message-list cursor decoding rejects oversized opaque cursor strings before
  base64 decode and JSON parsing.
- IMAP service methods trim user/mailbox identifiers and normalize list/backfill
  limits before repository, storage, broker, or mailbox-event work.
- Mail search service queries normalize user, text, folder, sender, subject,
  and sort inputs before Postgres or OpenSearch dispatch.
- Message delivery-status and reply source-thread service lookups trim user,
  message, and source-message identifiers before repository work.
- Admin API domain query identifiers for user listing, DKIM key listing, and
  delivery-route resolution are trimmed at the HTTP boundary before service
  dispatch.
- Admin API DKIM key deactivate and DNS-verify path identifiers are trimmed at
  the HTTP boundary before service dispatch and response envelopes.
- Admin API suppression-list and trusted-relay delete path identifiers are
  trimmed at the HTTP boundary before service dispatch and response envelopes.
- Admin API trusted relay listing now supports bounded CIDR and description
  filters so operators can inspect inbound relay policy without client-side
  full-list scans.
- Admin API company, domain, and user quota/status/policy mutation path
  identifiers are trimmed at the HTTP boundary before service dispatch and
  response envelopes.
- Admin API outbox event topic, partition key, and status filters are trimmed
  at the HTTP boundary before operational queue inspection, and CR/LF-bearing
  or oversized filter values are rejected before service dispatch.
- Admin API delivery-attempt status and recipient-domain filters are trimmed at
  the HTTP boundary before retry/bounce inspection, and CR/LF-bearing or
  oversized filter values are rejected before service dispatch.
- Admin API push-notification attempt and stats filters are trimmed at the HTTP
  boundary before device/provider troubleshooting queries, and CR/LF-bearing
  or oversized filter values are rejected before service dispatch.
- Admin push-notification attempt/stats repository filters also reject
  invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL
  dispatch.
- OpenAPI drift tests now pin the push-device list `limit` query parameter so
  generated clients keep pagination controls for device management.
- OpenAPI drift tests now pin attachment reservation/direct-upload HTTP 413
  error responses for size-cap failures.
- Mail and Admin API JSON request handlers now reject trailing JSON tokens and
  unknown object fields before service dispatch, and common JSON request
  decoding is capped at 1 MiB before parsing.
- Mail and Admin API JSON mutation bodies now require `Content-Type:
  application/json`, accepting normal media-type parameters such as
  `charset=utf-8` but rejecting missing, repeated, or non-JSON content types
  before dispatch.
- Mail API read and bodyless mutation routes now reject request bodies and
  `Content-Type` headers before dispatch, preventing ignored JSON or multipart
  metadata on resource reads, deletes, draft-send, upload-session finalization,
  capability discovery, downloads, and push-device list/delete operations.
- Admin GET/DELETE routes and bodyless Admin POST commands now reject request
  bodies and `Content-Type` headers before dispatch, preventing ignored payloads
  on operator reads, deletes, route verification, retry, IMAP UID backfill,
  API-usage export-batch creation, and manifest digest/signature creation.
- Health and service-info GET routes now reject request bodies and
  `Content-Type` headers before writing probe or contract metadata responses,
  keeping bodyless read semantics consistent across HTTP surfaces.
- Health and service-info GET routes now also reject unknown query parameter
  names, making release probe and metadata endpoint typos visible as HTTP 400
  instead of silently ignored inputs.
- Admin bodyless command/delete routes for IMAP UID backfill, DKIM DNS verify,
  outbox retry, DKIM deactivation, suppression deletion, trusted-relay
  deletion, and delivery-route deletion now reject unknown query parameter names
  before dispatch, preventing ignored `dry_run`/`force`-style operator flags.
- Admin JSON mutation routes for tenant quotas, domain/user lifecycle and
  policy, backpressure, attachment cleanup, quota correction, push outcomes,
  trusted relays, delivery routes, and DKIM keys now reject unknown query
  parameter names before dispatch.
- Mail JWT and Admin token authentication now reject repeated credential
  headers, and Admin routes reject mixed `X-Admin-Token` plus bearer credentials
  before dispatch.
- Mail and Admin API scalar query parameters now reject duplicate values before
  dispatch, preventing ambiguous user IDs, list limits, booleans, timestamps,
  and operational filters from being interpreted by first-value wins behavior.
- Mail API read/search/list, draft-search, attachment capability/session/download,
  and push-device list routes now reject unknown query parameter names before
  dispatch, making generated-client typos visible as HTTP 400 responses.
- Mail API mutation routes now reject unknown query parameter names before
  dispatch, and JSON-backed compose/draft/attachment/send mutations honor the
  documented development-only `user_id` query fallback when JWT auth is
  disabled.
- Admin company/domain/DNS-check/user list routes now reject unknown query
  parameter names before dispatch, keeping core operator filters aligned with
  the documented contract.
- Admin API usage aggregate, ledger, retention, export-batch, artifact,
  manifest-digest, and manifest-signature routes now reject unknown query
  parameter names before dispatch, including unexpected query strings on
  detail, download, verification, and mutation routes with no query controls.
- Admin queue, outbox, audit, backpressure, quota, attachment-session,
  delivery-attempt, push-notification, suppression-list, trusted-relay,
  delivery-route, and DKIM read routes now reject unknown query parameter names
  before dispatch.
- API error responses now use `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`, with the reusable OpenAPI error response
  documenting both headers for generated clients.
- Successful Mail/Admin JSON, health, and service-info responses now return
  `X-Content-Type-Options: nosniff`, aligning browser-visible envelopes with
  error, NDJSON, and download response hardening.
- Successful Mail/Admin JSON responses now return `Cache-Control: no-store`
  through the shared writer so sensitive message, audit, usage, and control
  envelopes are not cached.
- Attachment download responses now emit both ASCII fallback and UTF-8
  `filename*` `Content-Disposition` parameters for internationalized filenames,
  with stored filenames bounded before response headers are written.
- Attachment downloads now fall back to `application/octet-stream` for blank,
  unsafe, or media-type-invalid stored MIME types before setting response
  headers.
- OpenAPI now documents attachment download `Content-Disposition` and
  `Cache-Control: no-store` headers with drift coverage.
- API usage artifact downloads now sanitize stored content type and SHA-256
  response headers before streaming export objects, including media-type
  validation before the response `Content-Type` is written.
- API usage outbox production now rejects CR/LF-bearing method, route,
  event-id, tenant, company, domain, user, API-key, and principal dimensions
  before inserting durable usage events.
- API usage aggregate storage now applies the same route-key, identity, event-id,
  schema-version, and HTTP-like status validation when called directly by
  internal adapters.
- API usage NDJSON exports and stored export artifact downloads now return
  `Cache-Control: no-store`, with OpenAPI drift coverage.
- Attachment downloads, usage NDJSON exports, and stored export artifact
  downloads now return `X-Content-Type-Options: nosniff`, with OpenAPI drift
  coverage.
- Successful JSON responses now return `X-Content-Type-Options: nosniff` across
  Mail, Admin, health, and service-info routes.
- Successful Mail/Admin JSON envelopes now use `Cache-Control: no-store` through
  the shared writer.
- Mailservice now validates DB-returned message and attachment storage object
  paths before body reads or deletes, preventing corrupted rows from reaching
  the storage adapter with absolute, traversal, newline, backslash-bearing, or
  oversized keys.
- Local storage now shares the strict object-path validator used by mailservice,
  rejecting non-canonical, oversized, duplicate-separator, or dot-segment keys
  at the adapter boundary before reads, writes, or deletes.
- IMAP read-only selected-state mutation commands now let malformed
  `STORE`/`MOVE`/`UID STORE`/`UID MOVE`/`UID EXPUNGE` requests return
  command-specific tagged `BAD` responses before valid mutations are rejected
  with `NO mailbox is read-only`, including invalid UID/sequence sets, STORE
  modes/flags, and modified UTF-7 destination mailbox names.
- IMAP mailbox rename handling now rejects attempts to rename any mailbox to
  `INBOX`, keeping the special INBOX namespace out of generic folder mutation
  paths.
- IMAP command dispatch now validates command and UID subcommand atoms before
  routing, so atom-special-bearing command names are rejected as malformed
  syntax instead of falling through as unknown commands.
- IMAP `UID` dispatch now validates missing, malformed, unknown, or
  state-independent malformed subcommands before authentication or
  selected-mailbox state, while valid unauthenticated UID commands still return
  `NO authentication required`.
- Authenticated selected-state IMAP commands now validate obvious malformed
  `FETCH`, `STORE`, `COPY`, `MOVE`, `SEARCH`, `SORT`, and `THREAD` syntax
  before returning `NO mailbox must be selected` for valid commands issued
  outside selected state.
- Selected-state action commands now also validate malformed `FETCH`, `STORE`,
  `COPY`, and `MOVE` arity or modified UTF-7 destination mailbox names before
  authentication failures, while valid unauthenticated commands still return
  `NO authentication required`.
- Search-oriented selected-state commands now validate malformed `SEARCH`,
  `SORT`, and `THREAD` argument shape, return options, and sort/thread
  argument lists before authentication failures, while valid unauthenticated
  commands still return `NO authentication required`.
- Selected-state no-argument commands now reject extra arguments on `CHECK`,
  `IDLE`, `CLOSE`, `UNSELECT`, and `EXPUNGE` before returning authentication
  or selected-mailbox state errors for well-formed commands.
- IMAP `STARTTLS` now rejects extra arguments before TLS availability or
  authentication-state checks, preserving no-argument command syntax diagnostics
  during capability probing.
- IMAP `UID` dispatch validates subcommand arity and destination mailbox-name
  syntax for `FETCH`, `STORE`, `EXPUNGE`, `COPY`, and `MOVE` before
  authentication or selected-mailbox state, while leaving selected-state-
  dependent UID set resolution to the selected command handlers.
- IMAP `LOGIN` and `AUTHENTICATE` now validate malformed argument shape before
  returning `[PRIVACYREQUIRED]` on plaintext TLS-required listeners, while
  syntactically valid but unsupported SASL mechanisms return tagged `NO` so
  clients can fall back cleanly.
- IMAP SASL PLAIN decoding now rejects oversized encoded and decoded responses
  before credential splitting or backend authentication, keeping literal
  initial-response paths bounded by the same username/password policy.
- IMAP successful `LOGIN` and `AUTHENTICATE PLAIN` responses now include an
  authenticated `[CAPABILITY ...]` response code, so clients can learn the
  post-auth extension set without an immediate follow-up `CAPABILITY` command
  and without carrying pre-auth `SASL-IR`/`AUTH=PLAIN` state forward.
- IMAP connection greetings now include a state-aware `[CAPABILITY ...]`
  response code. Plaintext TLS-required sessions advertise `STARTTLS` and
  `LOGINDISABLED`, while implicit TLS sessions advertise immediate
  `SASL-IR`/`AUTH=PLAIN` login capability without a redundant capability probe.
- IMAP mailbox management and subscription commands now validate malformed
  `LIST`, `LSUB`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, and
  `UNSUBSCRIBE` argument shape or modified UTF-7 mailbox names before
  authentication failures, while valid unauthenticated commands still return
  `NO authentication required`.
- IMAP selected-mailbox discovery commands now validate malformed `NAMESPACE`,
  `SELECT`, `EXAMINE`, and `STATUS` argument shape, CONDSTORE options, status
  item lists, or modified UTF-7 mailbox names before authentication failures,
  while valid unauthenticated commands still return `NO authentication
  required`.
- IMAP `APPEND` now validates missing literals, malformed append options, and
  modified UTF-7 mailbox names before authentication failures, while valid
  unauthenticated appends still consume the RFC literal and return
  `NO authentication required` before backend storage.
- IMAP `ENABLE` now validates missing capability arguments before
  authentication failures, while valid unauthenticated enable attempts still
  return `NO authentication required` without mutating session feature state.
- IMAP `ENABLE` now has regression coverage for RFC 5161-compatible unknown
  capability handling: well-formed unsupported capability names are ignored and
  return an empty `ENABLED` response instead of being treated as syntax errors.
- Storage portability now has a reusable backend-neutral contract test covering
  object keys with literal `+`, `@`, spaces, and `=` plus `Put`, `Get`,
  `GetRange`, `Stat`, `List`, `Copy`, `Move`, idempotent `Delete`, and bounded
  `DeletePrefix`; the local backend always runs it and the S3 integration test
  reuses it when `GOGOMAIL_TEST_S3_*` variables are present.
- Local/NFS and S3-compatible `Get`/`GetRange` readers now honor context
  cancellation after the object stream has opened, and local/NFS `GetRange`
  reports `io.ErrUnexpectedEOF` for short object windows so partial-read
  behavior stays aligned when operators switch storage backends.
- Local/NFS storage now rejects filesystem symbolic links for object reads,
  range reads, metadata probes, deletes, copies, moves, writes, and prefix
  listings, including symlinked intermediate directories. Final-object
  symlinks remain hidden from list pages, and direct directory deletes are
  rejected. Local and NFS-backed deployments therefore keep object-key
  semantics instead of accidentally following host-specific links or treating
  filesystem folders as objects.
- Backend release verification now fails when standard tests leave pending
  repository changes behind, while local OpenChrome session artifacts are
  ignored as developer-machine state.
- Mail API attachment downloads now support `HEAD` metadata probes, validating
  the same message/attachment/storage-object boundary as `GET` and returning
  safe `Content-Disposition`, object-backed `Content-Length`, `no-store`, and
  `nosniff` headers without streaming bytes.
- Drive node copy is now available through `POST /api/v1/drive/nodes/{id}/copy`.
  It copies active files and bounded active folder trees through the configured
  storage adapter, creates quota-accounted metadata with caller-provided
  destination folder/name, exposes `copy_nodes` plus `max_copy_nodes` in
  webmail capabilities, and removes copied objects if DB metadata creation or
  bounded folder-tree copy fails.
- Drive file copy cleanup now records a pending cleanup-failure row if metadata
  creation fails after object copy and the copied object cannot be deleted,
  keeping object-storage drift visible to operator retry/resolve tooling.
- Drive file copies now preallocate the destination node UUID and use that same
  identifier in the copied object's committed storage path and `drive_nodes.id`,
  keeping copy metadata and object keys aligned.
- Drive upload-session finalization, staged-object finalization, and file copy
  now map quota exhaustion to HTTP 507 `insufficient_storage`, giving webmail
  clients a precise storage-pressure response.
- Drive node listing now supports explicit `sort=name|updated|created|size`
  controls on both webmail and admin APIs while preserving folder-first
  ordering for production Drive browser ergonomics.
- Drive node listing now supports `node_type=folder|file` filters on webmail
  and admin APIs, with webmail capabilities advertising supported node types.
- Webmail Drive node listing now accepts `all_parents=true` for whole-user
  Drive search/list views while rejecting ambiguous `parent_id` combinations,
  and webmail capabilities advertise the whole-tree search mode for production
  file pickers and compose-side Drive insertion flows.
- Drive now has a first authenticated share-link metadata boundary:
  `drive_share_links` stores user/file-scoped token hashes, bounded suffixes,
  permissions, status, and expiry; Mail API routes can create, list, and revoke
  links while returning raw share tokens only in the create response.
- Drive share links now have public metadata resolution plus `GET`/`HEAD`
  download routes that resolve tokens by SHA-256 hash, enforce expiry,
  revocation, active owner/domain/node checks, keep storage internals out of
  metadata responses, and reuse the Drive no-store/range-download header
  contract for `download`-permission links.
- OpenAPI now documents public share-link download `HEAD`, full-body `200`,
  and byte-range `206` as non-JSON binary/header responses, with drift tests
  keeping generated Drive clients aligned with runtime streaming semantics.
- Public share-link metadata and download OpenAPI operations now explicitly
  opt out of global bearer auth, matching the unauthenticated runtime boundary
  used by external recipients and generated public-share clients.
- Drive public share endpoints now have an optional Redis-backed fixed-window
  abuse-control boundary (`GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND=redis` and
  `GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE`) that buckets anonymous traffic
  by normalized remote address plus a share-token SHA-256 digest, returns HTTP
  429 with `Retry-After` on quota exhaustion, and fails open on transient
  limiter errors after startup. Raw public share tokens are not passed across
  the limiter interface.
- Drive public share metadata/download successes, denied token/permission
  checks, and rate-limited requests now emit best-effort
  hash-chain audit rows (`category=drive`, `share_link.resolve`,
  `share_link.download`, `share_link.download_head`) with link/node identity,
  when available plus normalized remote address, user agent, token suffix,
  result/status, and byte range when present, without recording raw tokens or
  storage backend/path values.
- Admin audit-log listing now accepts a bounded `action_prefix` filter, so
  operators can query action families such as `share_link.` across successful,
  denied, and rate-limited public Drive share activity without waiting for a
  dedicated aggregate activity dashboard.
- CalDAV work has started with ADR 0010, a `caldav` runtime scaffold, and an
  `internal/caldavgw` boundary for RFC/WebDAV standards, DAV tokens, principal
  paths, calendar-home paths, calendar collections, and `.ics` object paths.
- CalDAV storage groundwork now has PostgreSQL `caldav_calendars` and
  `caldav_calendar_objects` tables with user-scoped active uniqueness, ETag,
  sync-token, component, and bounded `.ics` body constraints. `internal/caldavgw`
  also validates calendar metadata, component types, object UIDs, strong ETags,
  and sync-token derivation before WebDAV handlers are exposed.
- CalDAV WebDAV XML parsing groundwork now accepts bounded namespace-aware
  PROPFIND bodies (`allprop`, `propname`, `prop`, and `allprop` `include`),
  parses safe `Depth` header values, and classifies core CalDAV/WebDAV REPORT
  roots (`calendar-query`, `calendar-multiget`, `free-busy-query`, and
  `sync-collection`) with body/property/href/depth limits before handlers are
  advertised.
- CalDAV now has a PostgreSQL repository boundary for calendar create/list/get
  and calendar-object upsert/list/get/soft-delete. Object writes validate `.ics`
  resource names, UID/component metadata, strong ETags, optional observed ETags,
  object-size limits, and bump calendar sync tokens in the same transaction.
  Object writes also preflight duplicate active iCalendar UIDs within the same
  calendar before SQL upsert, while PostgreSQL unique-index races for active
  object names or UIDs are mapped back into predictable repository errors
  instead of exposing raw driver details.
- CalDAV `DELETE` now shares the same default authenticated user resolver path
  as `GET`, `PUT`, `PROPFIND`, and `REPORT`, keeping manually assembled
  handlers fail-closed/predictable instead of risking a nil resolver panic.
- CalDAV object validation now uses `github.com/emersion/go-ical` for RFC 5545
  iCalendar decoding, requiring one `VCALENDAR` with exactly one `VERSION:2.0`,
  exactly one non-empty `PRODID`, exactly one supported top-level calendar
  component, exactly one bounded UID, and explicit component/property count
  caps before `.ics` bodies reach storage. It rejects stored root `METHOD`
  properties per RFC 4791 calendar object resource rules while still allowing
  server-generated free-busy responses to carry `METHOD:REPLY`. It also
  rejects RFC-invalid duration/end combinations for stored `VEVENT` and `VTODO`
  objects, including `VEVENT` `DTEND`+`DURATION`, `VTODO` `DUE`+`DURATION`, and
  `VTODO` `DURATION` without `DTSTART`; singleton time/status properties such
  as `DTSTART`, `DTEND`, `DUE`, `DURATION`, `STATUS`, `TRANSP`,
  `RECURRENCE-ID`, and `DTSTAMP` are rejected when duplicated on supported
  calendar components.
- CalDAV REPORT `calendar-data` parsing now rejects unsupported
  `content-type` and non-`2.0` `version` attributes instead of silently
  projecting data for media variants the server does not advertise.
- CalDAV now has a WebDAV `multistatus` response builder for future PROPFIND
  and REPORT handlers. It renders per-property `propstat` statuses, principal
  discovery properties, calendar-home hints, calendar collection metadata
  (`supported-calendar-component-set`, `supported-calendar-data`,
  `max-resource-size`, sync token), and calendar-object ETag/content metadata.
- CalDAV now has an internal `OPTIONS`/`PROPFIND` discovery handler boundary
  over a pluggable discovery store. It advertises DAV capabilities, rejects
  unsafe infinite-depth discovery, enforces authenticated user/path scope, and
  can render the service root, advertised principal collection, authenticated
  principal, calendar-home, calendar-collection, and calendar-object
  multistatus responses before the public listener is enabled. The service root
  is intentionally modeled as a read-only collection discovery anchor rather
  than as the user principal, so principal-only properties such as
  `calendar-home-set` stay on the principal resource.
- CalDAV `OPTIONS` and 405 responses now share an explicit implemented-method
  list so `Allow` stays tied to real gateway handlers. Future-only methods such
  as `COPY` and `MOVE` remain unadvertised until their WebDAV semantics exist.
  `OPTIONS` discovery also returns `Cache-Control: no-store` and
  `X-Content-Type-Options: nosniff`; 405 method-probe responses now carry the
  same safety headers, matching the CardDAV discovery surface so native clients
  and intermediaries do not retain stale capability headers.
- CalDAV calendar-home discovery now keeps WebDAV `current-user-principal` and
  `owner` anchored to the canonical principal URL instead of the calendar-home
  collection, keeping principal discovery semantics aligned with delegated and
  shared calendar work still gated behind Directory/Identity.
- CalDAV discovery now returns RFC 3744-style `current-user-privilege-set`
  values without advertising ACL semantics: principals expose `DAV:read`,
  calendar homes expose implemented child calendar bind/unbind, calendar
  collections expose child object bind/unbind plus metadata write properties,
  and calendar objects expose read plus content-write capability.
- The PostgreSQL CalDAV repository now satisfies the discovery store boundary,
  including active principal lookup through active user/domain/company scope and
  calendar/object list/get adapters for the internal `PROPFIND` handler.
- CalDAV now has a Basic-auth user resolver that reuses the existing
  authenticated Submission password verifier boundary, requires TLS or an
  HTTPS forwarding signal unless explicitly allowed for development, and
  returns the authenticated user ID for future native CalDAV clients.
- Configuration now includes `GOGOMAIL_CALDAV_ADDR` and
  `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH`, with production validation rejecting
  insecure CalDAV Basic-auth credentials before runtime wiring is enabled.
- `gogomail --mode=caldav` now starts a dedicated HTTP listener using
  `GOGOMAIL_CALDAV_ADDR`, the CalDAV PostgreSQL discovery repository, and the
  Basic-auth resolver. Full CalDAV client-ready compatibility still depends on
  scheduling, recurrence semantics, sync tombstone/change-log support, and
  broader native-client compatibility coverage.
- CalDAV REPORT parsing now validates more protocol shape before handlers run:
  `calendar-query` requires a filter and extracts nested CalDAV time ranges,
  `calendar-multiget` requires bounded hrefs, `free-busy-query` requires a UTC
  single time range, and `sync-collection` requires an explicit `sync-token`
  element, supported `sync-level=1`, a requested property set, and a bounded
  optional `limit` while keeping empty sync-token elements valid for initial
  sync.
- CalDAV REPORT parsing now also preserves RFC 4791 `calendar-data` projection
  requests for nested `VCALENDAR`/component property selection instead of
  flattening the property to "full body" only.
- CalDAV now implements a first `REPORT calendar-multiget` handler for
  authenticated calendar collections, returning multistatus object metadata and
  requested `calendar-data` bodies while representing missing hrefs through
  per-resource 404 propstats.
- `calendar-multiget`, `calendar-query`, and `sync-collection` responses can
  now project returned iCalendar bodies to requested `calendar-data` calendar
  and component properties while preserving required RFC 5545 structure fields
  needed for valid encoded objects.
- CalDAV now handles calendar object `GET`, `HEAD`, `PUT`, and `DELETE` over
  authenticated `.ics` object paths. Reads return strong ETags and
  `text/calendar` bodies, writes enforce bounded iCalendar validation and
  `If-Match`/`If-None-Match` preconditions, and deletes honor optional ETag
  preconditions before soft-deleting repository objects.
- CalDAV now handles `REPORT calendar-query` for authenticated calendar
  collections, listing matching `.ics` objects through WebDAV multistatus
  responses, keeping object reads behind bounded `limit/nresults` handling
  with one-extra-row truncation detection, applying RFC 5545-backed VEVENT
  overlap checks, and matching VTODO time-ranges with RFC 4791
  `DTSTART`/`DUE`/`DURATION`/`COMPLETED`/`CREATED` semantics. VJOURNAL and
  VFREEBUSY components return `false` for time-range queries per RFC 4791
  Section 9.6.1 (time-range only applies to VEVENT/VTODO) and Section 7.8.1
  (VFREEBUSY MUST NOT occur in calendar-query time-range). Unsupported
  CalDAV filter elements such as `prop-filter` now fail closed with a
  `CALDAV:supported-filter` precondition instead of being silently ignored and
  widening query results.
- CalDAV now handles a conservative RFC 6578 `REPORT sync-collection` path for
  authenticated calendar collections: empty sync tokens return all active
  objects plus the collection sync token, current tokens return only the
  top-level sync token, stale-but-known tokens return deltas/tombstones,
  unknown or expired tokens return a DAV `valid-sync-token` precondition error,
  and truncating limits are rejected until continuation semantics exist.
- CalDAV now handles RFC 4791-shaped `REPORT free-busy-query` for authenticated
  calendar collections. `Depth: 1` collects child VEVENT busy periods into a
  `200 OK` `text/calendar` `VFREEBUSY` response while keeping child object
  reads behind bounded `limit/nresults` handling with one-extra-row truncation
  detection. It clips periods to the requested UTC range, skips `TRANSPARENT`
  and `CANCELLED` events, maps tentative events to `BUSY-TENTATIVE`, ingests
  stored VFREEBUSY `FREEBUSY` period lists, supports UTC start/end and
  start/duration periods, coalesces same-type overlaps, and rejects duplicate
  free-busy time ranges. Scheduling, recurrence expansion, and broader
  native-client compatibility coverage remain incomplete.
- CalDAV now handles `MKCALENDAR` for authenticated calendar collection paths
  whose Request-URI calendar segment is a UUID. The handler parses bounded
  CalDAV/WebDAV creation XML for display name, description, and CalendarServer
  or Apple calendar color, creates the collection at the requested URI, returns
  `201 Created` with `Location`, rejects cross-user paths, existing calendars,
  missing homes, and unsafe non-UUID path ids, and advertises `MKCALENDAR` again
  only because handler semantics now exist. ADR 0014 slug/alias implementation
  is complete: calendars support slug-based paths, MKCALENDAR accepts slug
  paths and `calendar-slug` property, and PROPPATCH can update slug.
- CalDAV now handles `DELETE` on authenticated calendar collection paths,
  soft-deleting the collection and active child objects in one repository
  transaction while rejecting calendar-home or cross-user deletes. Those
  deletes now append durable sync-change rows so stale-token clients can see
  object tombstones and final collection-deleted sync tokens; long-history
  retention and continuation semantics remain future compatibility work.
- CalDAV now has a durable calendar sync-change table for RFC 6578-style
  `sync-collection` deltas. Calendar create/upsert/delete paths record sync
  markers in the same transaction as object mutations, migrated calendars get a
  baseline marker on first object change, stale-but-known sync tokens can return
  changed object properties or response-level `404 Not Found` tombstones, and
  collection-deleted tokens can return a final top-level sync token even after
  the calendar row is gone. CalDAV now also has a bounded
  `PruneCalendarSyncChanges` repository boundary plus a prune-order index that
  can dry-run or delete old sync-change rows while preserving the newest marker
  per calendar, so retention workers can expire history without destroying the
  token needed for current clients. The `dav-sync-retention-worker` now runs
  that CalDAV prune path on an interval or once-and-exit, dry-run by default
  and guarded by explicit confirmation before destructive runs. Each worker
  execution now records a `dav_sync_retention_runs` audit/read-model row with
  cutoff, limit, dry-run/confirmation flags, status, bounded error text, and
  CalDAV/CardDAV candidate/deleted counts, including partial failures after one
  side has pruned successfully. The DAV retention repository can now list
  bounded run history by status/created-at window and fetch one run by bounded
  ID. Admin API now exposes that read model through
  `GET /admin/v1/dav-sync/retention-runs` and
  `GET /admin/v1/dav-sync/retention-runs/{id}` with explicit response
  envelopes and documented query allowlists. Admin API also exposes
  `GET /admin/v1/dav-sync/retention-readiness` as a read-only dry-run preview:
  operators must provide a non-future cutoff, optional per-backend probe limit,
  and receive aggregate plus CalDAV/CardDAV candidate counts with `truncated`,
  `ready`, and `destructive_guarded` status. Admin API can now also run
  DAV sync retention through `POST /admin/v1/dav-sync/retention-runs`: dry-run
  requests persist candidate counts, destructive requests require
  `confirm_ready=true`, reuse the readiness preview, and fail closed if either
  DAV backend hits the probe limit. Optional PostgreSQL integration coverage
  applies release migrations and round-trips completed/failed DAV retention run
  rows, including sanitized failure text and status-window list filters.
  Unknown or expired tokens still fail with DAV `valid-sync-token`; deployment
  retention-age policy and native-client expiry testing remain future work.
- CalDAV now handles RFC 6764-style `/.well-known/caldav` discovery by
  redirecting to `/caldav/`, and `PROPFIND /caldav/` can return
  `current-user-principal`, `principal-collection-set`, and
  `calendar-home-set` for authenticated clients.
- CalDAV now handles WebDAV `PROPPATCH` for authenticated calendar collection
  metadata, using bounded namespace-aware `propertyupdate` parsing and a small
  repository update boundary for `DAV:displayname`,
  `CALDAV:calendar-description`, and CalendarServer/Apple calendar color.
  Updates are transactional, refresh the collection sync token, append a
  `collection-updated` sync marker, and keep calendar objects, scheduling, and
  product-specific policy out of the gateway path.
- CalDAV calendar collection `PROPFIND` now exposes WebDAV
  `supported-report-set` for the REPORT handlers that actually exist today:
  CalDAV `calendar-query`, `calendar-multiget`, `free-busy-query`, and WebDAV
  `sync-collection`. Collection `Depth: 1` object metadata discovery is bounded
  with the shared one-extra-row truncation probe and now rejects partial
  listings explicitly. This keeps native-client capability discovery aligned
  with implemented semantics instead of advertising future scheduling or
  recurrence features prematurely.
- CalDAV `REPORT calendar-query` now honors simple top-level component filters
  such as `VEVENT` and `VTODO` by using the repository's stored
  `component_type` metadata before expensive time-range/body work. This keeps
  common client filters from returning unrelated object types while preserving
  the bounded iCalendar parser as the write-time source of truth.
- CalDAV `REPORT calendar-multiget` now scopes href resolution to the request
  resource: collection requests only return objects from that collection, while
  calendar-home requests can fetch the authenticated user's calendar objects
  across collections. Out-of-scope hrefs render WebDAV 404 propstats instead
  of leaking object metadata or `calendar-data`.
- CalDAV `PROPFIND` now returns RFC 4918-shaped `owner`, `creationdate`, and
  `getlastmodified` metadata where the current model can answer them exactly.
  Owners point at the authenticated user's principal URL, creation dates use
  UTC RFC3339 timestamps, and last-modified values use HTTP-date formatting.
- CalDAV calendar object `GET` and `HEAD` now honor `If-None-Match` against
  stored strong ETags, returning `304 Not Modified` with safe cache headers and
  no body when clients already have the current `.ics` representation.
- CalDAV calendar object `PUT` now validates explicit `Content-Type` headers
  before body parsing, accepting `text/calendar` with ordinary parameters while
  rejecting repeated headers, incompatible media types, or non-`2.0` `version`
  parameters with HTTP 415.
- CalDAV calendar object `PUT` now treats `If-Match: *` as an existing-resource
  precondition, returning HTTP 412 when the target `.ics` object does not yet
  exist instead of accidentally creating it.
- CalDAV calendar object `PUT` now evaluates specific `If-Match` and
  `If-None-Match` ETag preconditions before body reads or storage mutation,
  returning HTTP 412 for stale overwrite or no-overwrite requests.
- CalDAV calendar object `GET` and `HEAD` now reject stale `If-Match`
  preconditions before `If-None-Match` revalidation, and `DELETE` accepts
  comma-listed strong ETags through the same comparison helper used by writes.
- CalDAV calendar object `DELETE` now treats `If-Match: *` as an
  existing-resource precondition, returning HTTP 412 for missing `.ics`
  resources instead of surfacing a plain not-found result.
- CalDAV calendar object `GET` and `HEAD` now emit `Last-Modified` from stored
  object update time and honor `If-Modified-Since` revalidation with
  second-precision comparisons, avoiding unnecessary `.ics` body streaming for
  timestamp-valid client caches.
- CalDAV calendar object `PUT` and `DELETE` now honor `If-Unmodified-Since`
  against stored object update timestamps before body reads or repository
  mutation, returning HTTP 412 for stale timestamp-based overwrite/delete
  attempts.
- S3-compatible `GetRange` now caps the returned reader at the validated
  requested byte length even if a provider returns an oversized `206 Partial
  Content` body, matching local/NFS range semantics and keeping partial Drive,
  attachment, and IMAP reads bounded at the storage adapter boundary.
- CalDAV calendar object `GET` and `HEAD` now also honor
  `If-Unmodified-Since` before `If-None-Match` / `If-Modified-Since`
  revalidation, returning HTTP 412 when timestamp preconditions are stale.
- S3-compatible `GetRange` now requires the provider's `Content-Range` header
  to match the requested byte window before exposing the response body, closing
  mismatched partial responses early instead of letting Drive, attachment, or
  IMAP partial-read callers consume the wrong range.
- S3-compatible `GetRange` now reports `io.ErrUnexpectedEOF` when a provider
  returns a matching `Content-Range` header but truncates the response body
  before the requested byte count, making partial-read corruption visible to
  Drive, attachment, and IMAP callers.
- S3-compatible `GetRange` now drains a small bounded remainder on successful
  range-reader close, improving HTTP connection reuse when providers send extra
  partial-response bytes without exposing those bytes to callers.
- S3-compatible `GetRange` now applies the same bounded close-drain behavior
  when callers close a range reader before consuming the full requested window,
  improving connection reuse for preview/cancel paths without unbounded drain
  work.
- S3-compatible full-object `GET` readers now also drain a small bounded
  remainder on close, keeping preview/cancel download paths friendly to HTTP
  connection reuse without unbounded cleanup reads.
- IMAP `STATUS` and LIST-STATUS item parsing now rejects duplicate status data
  items and duplicated LIST-STATUS `STATUS` return options before mailbox
  metadata lookup, avoiding ambiguous duplicate client-visible status pairs.
- CalDAV `MKCALENDAR` now rejects non-UUID creation path IDs before reading or
  parsing the XML request body when no active collection already exists at that
  path, keeping the UUID-only creation contract cheap and predictable while
  preserving existing-collection 405 behavior.
- CalDAV calendar collection `DELETE` now honors `If-Unmodified-Since` against
  collection update time and evaluates strong collection ETags derived from
  collection sync state, including comma-listed `If-Match` values and
  `If-Match: *`, before repository mutation.
- CalDAV collection `PROPPATCH` now uses the same collection precondition gate,
  rejecting stale `If-Unmodified-Since` and mismatched collection `If-Match`
  requests before reading XML bodies or updating calendar metadata.
- CalDAV `REPORT` now validates malformed and `Depth: infinity` headers before
  reading XML bodies, applying one shared Depth gate across calendar-query,
  calendar-multiget, sync-collection, and free-busy-query handling.
- CalDAV `calendar-multiget` now accepts HTTP(S) absolute URI `<D:href>` values
  by evaluating only their canonical path component through the same user and
  collection scope checks, while rejecting userinfo-bearing authorities, query,
  fragment, opaque, non-HTTP(S), or unsafe href forms.
- Directory/Identity now has a first protocol-neutral principal resolver under
  `internal/directory`, and CalDAV active principal discovery delegates to it
  instead of owning the user/domain/company active-scope query directly.
- Directory/Identity principal resolution now also supports organization
  principals from the existing organization/domain/company model, preparing
  organization calendars and policy scopes without exposing them publicly yet.
- Directory/Identity storage now has first-class group, resource, alias, and
  group-membership tables plus group/resource principal resolution hooks,
  preparing shared inboxes, resource calendars, delegated access, and admin
  directory workflows without hard-coding those semantics into CalDAV.
- Directory/Identity can resolve normalized alias email addresses to target
  user, organization, group, or resource principals, with active alias
  uniqueness enforced at the normalized address boundary.
- CalDAV discovery now has an explicit Directory-to-CalDAV principal conversion
  guard: only Directory user principals can become CalDAV user principals.
  Organization, group, and resource principals remain Directory-owned release
  gates for delegated/shared calendars and resource booking rather than being
  silently treated as personal calendar homes.
- CalDAV principal discovery now carries the Directory primary email into
  `CALDAV:calendar-user-address-set` as a normalized `mailto:` href when one
  exists. This gives future organizer/attendee and scheduling code a
  standards-shaped identity address boundary without advertising scheduling or
  delegated access semantics prematurely.
- Directory/Identity can check direct active group membership across user,
  organization, group, and resource principals, establishing the first
  auditable membership read boundary before recursive/effective delegation is
  implemented.
- Directory/Identity also has a bounded effective-membership check that expands
  nested groups with an explicit recursion cap and cycle guard, preparing
  delegated access and resource policy evaluation without unbounded graph
  traversal.
- Directory/Identity now has a company-scoped delegation foundation for owner
  and delegate principals across calendar, contacts, drive, and mailbox scopes,
  with `read`/`write`/`manage` role hierarchy checks. CalDAV shared/delegated
  calendars remain experimental gates until this boundary is connected to
  protocol semantics and policy/audit decisions.
- Directory/Identity effective delegation now expands group delegates through
  bounded nested group membership, preserving direct delegation semantics,
  cycle/depth guardrails, active-only owner/delegate principal checks, group
  filtering, and `manage >= write >= read` role satisfaction. Direct delegation
  checks also require active owner/delegate principals when `ActiveOnly` is set.
  This prepares shared calendars, resource calendars, Drive shares, shared
  inboxes, and Contacts delegation without adding product-local access models.
- Directory/Identity delegation inspection now has a bounded repository
  boundary. `ListDelegations` validates company scope, optional owner/delegate
  principal filters, delegation scope, role, active-only state, and result
  limits before querying, giving admin consoles and shared resource management
  a reusable read path without product-specific SQL.
- Directory/Identity delegation role updates now have an audited repository and
  admin API boundary. `PATCH /admin/v1/directory/delegations/{id}/role`
  returns the `directory_delegation` envelope after
  `UpdateDelegationRoleWithAudit` changes active grants in-place and commits
  `directory_delegation.role_update` with previous/new role detail, preparing
  shared calendar, Drive, Contacts/CardDAV, and shared inbox access management
  without product-local role mutation semantics.
- Directory/Identity delegation reassignment now has an audited repository and
  admin API boundary. `PATCH /admin/v1/directory/delegations/{id}/assignment`
  moves an active grant to a new owner/delegate/scope while preserving its role
  and commits `directory_delegation.reassign`, giving shared calendar, Drive,
  Contacts/CardDAV, and shared inbox modules a complete create/update/reassign/
  revoke delegation lifecycle before product-facing sharing UX starts.
- Directory/Identity alias inspection now has a bounded repository boundary.
  `ListAliases` validates company/domain scope, optional target principal
  filters, text query, active-only state, and result limits before querying,
  then resolves target principals through the shared resolver.
- Admin APIs now expose Directory delegation inspection through
  `GET /admin/v1/directory/delegations`, with OpenAPI and backend contract
  coverage for the response envelope and bounded filter set. This gives future
  admin console work a contract-first path for shared-calendar/resource access
  diagnostics while CalDAV itself remains experimental.
- Admin APIs now expose Directory principal search through
  `GET /admin/v1/directory/principals`, with OpenAPI and backend contract
  coverage for company/domain/organization/kind/query/active-only filters. This
  moves admin console and future product autocomplete flows onto the shared
  Directory/Identity boundary.
- Admin APIs now expose Directory alias resolution through
  `GET /admin/v1/directory/aliases/resolve`, with OpenAPI and backend contract
  coverage for address normalization and active-only lookup. This gives
  operators and future product flows one address-to-principal diagnostic path.
- Admin APIs now expose Directory alias listing through
  `GET /admin/v1/directory/aliases`, with OpenAPI and backend contract coverage
  for company/domain/target/query/active-only filters and target-principal
  response hydration.
- `internal/accesspolicy` now provides a small effective-delegation evaluator
  that normalizes principal/scope/role inputs, forces active principal checks,
  and returns explicit allow/deny decisions. It is deliberately product-neutral
  so future protocol modules can integrate Directory access through policy and
  audit adapters instead of embedding row-level sharing logic. It also maps
  allowed delegation decisions to WebDAV privilege names (`read`,
  `write-content`, `write-properties`, `bind`, and `unbind`) so CalDAV/CardDAV
  privilege exposure can reuse one audited role mapping.
- CardDAV groundwork has started with ADR 0012 and `internal/carddavgw`, which
  owns RFC/WebDAV/CardDAV standards names, DAV capability tokens, canonical
  principal/address-book/contact-object paths, `.vcf` resource validation, and
  safe relative or HTTP(S) absolute href parsing.
- CardDAV storage groundwork now has PostgreSQL `carddav_addressbooks`,
  `carddav_contact_objects`, and `carddav_addressbook_changes` tables with
  user-scoped active uniqueness, strong ETag, sync-token, status, size, and
  `.vcf` body constraints. `internal/carddavgw` also validates address-book
  metadata, contact object names/UIDs, strong ETags, object-size limits, and
  sync-token derivation before repository methods are exposed.
- CardDAV address-book repository methods now create/list/get collections
  behind active user/domain/company scope, normalize names, bound list limits,
  and insert durable `addressbook-created` change rows in the create
  transaction.
- CardDAV vCard validation now performs bounded RFC-oriented checks for vCard
  4.0 and common vCard 3.0 contact objects, including BEGIN/END structure, exactly one
  VERSION, required UID/FN, folded content-line handling, line/body caps, and
  nested VCARD rejection. Content-line parsing now honors quoted parameter
  values containing colons before the value separator.
- CardDAV contact-object repository methods now upsert/list/get/delete active
  `.vcf` objects under active address-book scope, enforce vCard UID alignment,
  compute strong ETags, honor optional observed ETags before overwrite, refresh
  address-book sync tokens, and record `contact-upserted`/`contact-deleted`
  changes transactionally.
- CardDAV REPORT parsing now recognizes bounded `addressbook-query`,
  `addressbook-multiget`, and WebDAV `sync-collection` request bodies,
  collecting requested properties, hrefs, sync token/level, limits, CardDAV
  `filter`/`prop-filter` `test` attributes, text-match predicates, and
  nested param-filters while rejecting malformed, oversized, deeply nested,
  invalid filter/test attributes, or unsupported sync-level shapes before
  handlers are exposed.
- CardDAV now has a WebDAV `multistatus` response builder for future PROPFIND,
  REPORT, and sync handlers. It renders principal discovery, address-book
  collection metadata, contact-object metadata, requested `address-data`,
  supported reports, supported vCard data types, sync tokens, and per-property
  404 propstats.
- CardDAV now has an internal RFC 6764/WebDAV-style discovery handler for
  `/.well-known/carddav`, `OPTIONS`, and `PROPFIND` over root, the advertised
  principal collection, principal, address-book home, address-book collection,
  and contact-object resources.
  It rejects cross-user paths, `Depth: infinity`, malformed WebDAV XML, and
  contact-object `PROPFIND` above `Depth: 0`; CardDAV `REPORT` also rejects
  `Depth: infinity` before XML body parsing. The PostgreSQL repository
  satisfies the discovery store by delegating active user principal lookup to
  the shared Directory resolver. This remains backend-only until auth/listener
  wiring, REPORT execution, object mutation, and native-client compatibility
  are implemented and tested.
- CardDAV now executes the three parsed REPORT shapes internally:
  `addressbook-multiget` returns requested contact metadata and optional
  `address-data` with per-href 404 propstats, `addressbook-query` scans active
  address-book objects with the current bounded CardDAV filter tree, and
  `sync-collection` returns full snapshots or bounded change rows with root
  sync-token emission and deleted contact 404 responses. Query filtering now
  evaluates multiple `prop-filter` entries and per-property text/parameter
  conditions using RFC 6352 `test=anyof|allof` composition. The text-match
  evaluator honors the RFC 6352 default `i;unicode-casemap` collation, rejects
  unsupported collations instead of silently changing semantics, and supports
  `equals`, `contains`, `starts-with`, `ends-with`, and `negate-condition`.
  Param-filter evaluation parses vCard content-line parameters and matches
  parameter existence, absence, or text-match values. REPORT `address-data`
  responses can project returned vCards to requested property names, keeping
  structural BEGIN/VERSION/END lines present while omitting unrequested contact
  properties. REPORT parsing also rejects unsupported requested address-data
  `content-type` or `version` values so the handler stays aligned with the
  advertised supported vCard data types. Returned `address-data` elements carry
  explicit `content-type="text/vcard"` and the stored vCard `version`
  attribute. The
  query path also honors bounded `limit/nresults` values before returning
  matched responses and can use a repository walker to stream objects until the
  response cap is satisfied. Address-data projection failures surface as
  explicit handler errors rather than returning broader data than requested. The
  repository can
  list address-book changes since a stored sync token and rejects missing or
  unsafe sync tokens before SQL work. This still does not advertise public
  native-client compatibility because broader vCard compatibility and client
  compatibility tests are still pending.
- CardDAV `OPTIONS` and 405 responses now share an explicit implemented-method
  list, keeping `Allow` aligned with actual address book handlers instead of
  future WebDAV method ambitions. Future `COPY`/`MOVE` method constants are
  regression-covered as intentionally unadvertised until handler semantics
  exist. Unsupported-method discovery responses also return
  `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`, so native
  client method probes follow the same safety contract as `OPTIONS`.
- CardDAV now handles contact-object `GET`, `HEAD`, `PUT`, and `DELETE` inside
  the internal handler. Reads emit `text/vcard; charset=utf-8`, strong ETags,
  content length, no-store headers, and `Last-Modified`, while honoring
  `If-Match`, `If-None-Match`, `If-Modified-Since`, and
  `If-Unmodified-Since`. Writes accept `text/vcard`, reject unsupported
  explicit vCard media-type versions, require any explicit content-type
  `version` parameter to match the body `VERSION`, enforce bounded body reads,
  reuse vCard validation and observed-ETag repository guards, and map
  create/update/delete to standard 201/204/precondition outcomes. This remains
  backend-only until auth/listener wiring and native-client compatibility tests
  are in place.
- CardDAV now has Basic-auth and runtime wiring. `gogomail --mode=carddav`
  opens a dedicated HTTP server on `GOGOMAIL_CARDDAV_ADDR` (default `:8082`),
  reuses the Submission password verifier through `internal/carddavgw`, and
  rejects insecure Basic auth in production through
  `GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH=false`. This enables deployment smoke
  testing of the CardDAV gateway while keeping public/client-ready status gated
  on broader vCard compatibility and native-client verification.
- Admin Drive node listing now accepts `all_parents=true` for whole-user Drive
  inventory search while rejecting ambiguous `parent_id` combinations.
- Drive file finalize, upload-session cleanup/retry-body replacement,
  permanent-delete cleanup, cleanup-failure retry, download, and copy paths now
  enforce that stored object keys remain under the owning user's
  `drive/users/{user_id}/...` prefix before storage adapter access, tightening
  tenant isolation at the storage boundary.
- S3-compatible `HEAD`/`Stat` metadata now rejects non-empty malformed
  `Last-Modified` headers instead of silently returning zero timestamps,
  while preserving HTTP optional-whitespace compatibility around otherwise
  valid timestamp values.
- S3-compatible `ListObjectsV2` and `CopyObjectResult` XML validation now
  applies the namespace boundary to core child elements as well as roots, so
  foreign-namespace pagination controls, object metadata, copy metadata, or
  embedded copy errors cannot be collapsed into canonical provider metadata.
- S3-compatible `CopyObjectResult` `LastModified` metadata now rejects
  present-but-blank, malformed, or whitespace-padded timestamp values instead
  of accepting ambiguous successful copy metadata.
- S3-compatible `CopyObjectResult` `ETag` metadata now uses the same bounded
  safe single-line validation as `Stat` and `List`, rejecting malformed copy
  success metadata before copy/move callers treat the provider response as
  durable.
- IMAP `IDLE` now requires an exact case-insensitive `DONE` continuation
  token, rejecting leading/trailing whitespace variants as malformed
  termination instead of silently ending the idle state.
- IMAP `AUTHENTICATE PLAIN` SASL-IR initial responses now validate malformed
  PLAIN payloads before plaintext privacy policy checks, preserving
  syntax-before-policy diagnostics without authenticating before TLS.
- IMAP `AUTHENTICATE PLAIN` continuation cancellation now requires an exact
  `*` token, rejecting whitespace-padded cancellation attempts as malformed
  SASL responses while keeping the session usable.
- IMAP UID and message sequence-set syntax now rejects whitespace-padded
  quoted or literal set strings such as `SEARCH " 1 "` or
  `UID SEARCH UID " 7 "` instead of trimming them into valid set atoms.
- IMAP `COPY`/`UID COPY` and `UID MOVE` now have regression coverage for
  quoted and literal destination mailbox names containing spaces, plus escaped
  quoted-special text, preserving RFC 3501 string tokenization through mailbox
  lookup and service-backed backend mutation requests instead of splitting or
  flattening folder names.
- IMAP selected-state sequence-set and UID-set command arguments now must be
  raw atom tokens for `FETCH`, `STORE`, `COPY`, `MOVE`, `UID FETCH`,
  `UID STORE`, `UID COPY`, `UID MOVE`, and `UID EXPUNGE`; quoted or
  command-literal values such as `"1"` or `{1}\r\n1` are rejected before
  authentication/selected-mailbox state can normalize them into valid sets.
- IMAP command names and `UID` subcommand names now must also arrive as raw
  atom tokens, so quoted or command-literal probes such as `"NOOP"`,
  `{4}\r\nNOOP`, `UID "COPY"`, or `UID {4}\r\nCOPY` receive tagged
  `BAD malformed command` responses instead of being dequoted into executable
  command words.
- IMAP command tags now must arrive as raw atom tokens as well; quoted or
  command-literal tag probes such as `"a1" NOOP` or `{2}\r\na1 NOOP` receive
  untagged `BAD malformed command` responses because the server cannot trust a
  string value as a recoverable IMAP tag.
- IMAP `SEARCH` top-level sequence-set criteria and `UID SEARCH UID`
  sequence-set values now follow the same atom-only boundary: exact quoted or
  command-literal set values such as `SEARCH "1"` or `UID SEARCH UID {1+}`
  are rejected as unsupported criteria instead of being dequoted into valid
  sets.
- IMAP `SORT`/`UID SORT` and `THREAD`/`UID THREAD` embedded search
  sequence-set criteria now use that same raw atom-only set boundary, rejecting
  quoted or command-literal set operands before authentication or selected
  mailbox state.
- IMAP search-family numeric operands now also require raw atom tokens:
  `LARGER`, `SMALLER`, simple `MODSEQ` thresholds, and `MODSEQ` entry-type/
  threshold operands reject quoted or command-literal strings while preserving
  the RFC string-shaped `MODSEQ` entry-name operand such as `"/flags/\\Seen"`.
- IMAP search-family charset and date operands now also require raw atom
  tokens: quoted `CHARSET`, `SORT`/`THREAD` charset, `SINCE`, `BEFORE`, `ON`,
  `SENTSINCE`, `SENTBEFORE`, and `SENTON` values are rejected instead of being
  dequoted into command controls.
- IMAP `KEYWORD` and `UNKEYWORD` search operands now require raw atom tokens,
  so quoted flag-keyword strings such as `KEYWORD "custom"` or
  `UNKEYWORD "custom"` are rejected instead of being dequoted into valid flag
  keywords.
- IMAP `STORE` and `UID STORE` mutation controls now require raw atom tokens
  for flag update modes and preserve the structured `UNCHANGEDSINCE` modifier
  boundary, rejecting quoted or command-literal `+FLAGS`/`FLAGS.SILENT` style
  controls before authentication or selected-mailbox state.
- IMAP `FETCH` and `UID FETCH` data-item controls now reject exact quoted or
  command-literal data item atoms such as `"FLAGS"` or literal `FLAGS` instead
  of dequoting them into valid fetch attributes before authentication or
  mailbox state checks.
- IMAP `ENABLE` capability operands now require raw atom tokens, rejecting
  quoted or command-literal `CONDSTORE` probes before authentication while
  preserving valid atom capability probes and unsupported atom ignore behavior.
- IMAP `AUTHENTICATE` mechanism names and SASL-IR initial responses now
  require raw atom tokens, so quoted `PLAIN` mechanisms and quoted base64
  initial responses are rejected before unsupported-mechanism, privacy, or
  backend authentication policy checks.
- IMAP parenthesized control lists now preserve their raw list boundary:
  quoted or command-literal `STORE`/`UID STORE` flag-lists, `APPEND`
  flag-lists, and `STATUS` item-lists are rejected instead of being dequoted
  into valid mutation or status controls.
- IMAP `SELECT` and `EXAMINE` optional `CONDSTORE` select parameters now use
  that same raw parenthesized-list boundary, rejecting quoted or command-literal
  `(CONDSTORE)` values before authentication or mailbox selection state.
- IMAP `LIST` selection option-lists, `RETURN` introducers, and `RETURN`
  option-lists now also keep their raw atom/list boundary, rejecting quoted or
  command-literal `SPECIAL-USE`/`STATUS` controls while preserving RFC mailbox
  pattern lists.
- IMAP `SEARCH`/`UID SEARCH` date criteria now reject whitespace-padded date
  strings such as `SINCE " 05-May-2026 "` instead of trimming them into valid
  date atoms.

Next focus areas:

1. Keep draft search separate from `GET /api/v1/search` until an explicit draft
   search contract and indexing path are added.
2. Extend the quota ledger to future Drive writes and large share-link objects.
3. Wire mailbox event publication from append/flag/move/delete paths behind the
   IMAP gateway boundary.
4. Add FCM/APNs/Web Push sink adapters and invalid-token cleanup behind the push
   notification worker.
5. Frontend planning and API contract review before webmail implementation.
6. Extend Directory/Identity from stored users, organizations, groups,
   resources, aliases, group memberships, and bounded membership expansion into
   explicit delegated principal relationships before public shared-calendar or
   resource-booking CalDAV features.
8. Extend CardDAV from internal discovery and delegated contacts access into
   authenticated shared-client workflows: add broader vCard compatibility,
   native-client shared address-book tests, product/admin sharing UX semantics,
   and Contacts/CardDAV autocomplete integration before webmail contacts,
   attendee auto-complete, or public native CardDAV compatibility are exposed.

## milter security fixes (TASK-025/026)
- Context deadline enforced in all milter callbacks
- Header CRLF injection blocked
- Packet size limits enforced (max 65535 bytes)

## ldapgw security fixes (TASK-007)
- BER length cap: PDUs with declared body > 16 MB are rejected; connection closed
- Context deadline enforced: ctx.Done() checked before every PDU operation
- Filter validation: validateFilter() rejects empty, non-context-specific, unsupported, or truncated LDAP filters before processing

## drive chunk upload fixes (TASK-016)
- TOCTOU race eliminated: StoreUploadSessionBody now uses SELECT FOR UPDATE inside a transaction; concurrent writers for the same session are serialised and the authoritative prior storage_path is returned from the locked row
- Chunk ordering enforced: ValidateChunkSequence rejects non-asterisk content-range chunks whose Start != session.ReceivedSize; out-of-order and duplicate chunks return an error before any I/O
- Orphan cleanup: newly written storage objects are deleted immediately on DB failure or validation error so no chunk objects are left stranded

## webmail spam folder + system folder migration (2026-05-12)
- Added Spam folder to createSystemFolders list for new users
- ListFolders now calls createSystemFolders (idempotent, ON CONFLICT DO NOTHING) so existing users get missing system folders (e.g. Spam) on next folder fetch

## Webmail dev seed bootstrap (TASK-163, 2026-05-12, complete)
- Development smoke testing exposed that fresh PostgreSQL migrations only create the system domain, while `scripts/seed_dev_data.sql` assumed the beta company/domain/user/folder already existed.
- The development seed now bootstraps `고구마컴퍼니`, `parkjw.org`, the beta login user `pjw@parkjw.org / pass1234`, the user's primary address, and the fixed Inbox/Sent/Drafts/Trash folders before loading rich webmail/admin data.
- The seed now creates system folders for all 16 auxiliary beta users, not only the first six, so directory/admin testing has more complete mailbox fixtures.
- Verification: `./scripts/seed_dev_beta.sh` now completes against the Docker PostgreSQL service with 17 users, 16 contacts, and 7 inbox messages. `go test ./...` and `pnpm type-check` in `apps/webmail` pass. Next target: backend and webmail frontend smoke tests.

## Webmail beta smoke hardening (TASK-164, 2026-05-12, complete)
- Smoke testing found three beta-readiness issues: stale folder IDs could keep the UI pointed at deleted seed folders, message flag PATCH failed due to a PostgreSQL placeholder/type issue, and org/addressbook fixtures needed to cover the beta flows more completely.
- The development seed now uses the backend-created canonical system folders first, migrates old fixed-folder message references, and removes obsolete fixed seed folders when safe.
- Organization fixtures now include all 21 internal beta users in both the hierarchical `organizations`/`users.org_id` path and the legacy `organization_members` path used by older admin views.
- Address book fixtures now include the full internal user directory (20 internal contacts) plus an additional `외부 협력사` address book with 4 partner contacts containing phone, organization, title, and notes.
- Webmail now recovers from stale active folder IDs by selecting a valid Inbox once folders load. The `directory/users` backend handler now supports the same dev `user_id` fallback as org-tree while still using JWT claims in authenticated mode.
- Mail flag updates now use correctly numbered PostgreSQL parameters and `COALESCE(flags, '{}'::jsonb)` for single-message, bulk-message, and bulk-thread flag mutations.
- Verification so far: `go test ./...` passes after the flag-update SQL fix. Seed message fixtures now upsert back to the canonical Inbox so repeated smoke tests restore the expected 7-message inbox.

## Webmail organization/address-book recipient expansion (TASK-165, 2026-05-12, complete)
- Organization and address-book selection now represents true group-send intent instead of eagerly flattening only visible users in the picker.
- The org picker middle pane now shows the selected organization, direct child organizations, and direct members; organization rows can be added to To/Cc/Bcc.
- Organization recipient tokens preserve whether child organizations should be included.
- Address-book rows can be added as recipient tokens, so an entire address book can be sent to the same way as an organization.
- Compose validation accepts internal recipient tokens while retaining normal email validation for ordinary recipients.
- `mailservice.SendText` expands organization and address-book recipient tokens immediately before send validation, so both direct send and draft-send paths resolve to concrete recipient emails at the backend boundary.
- Organization expansion is scoped to the sender's company/domain and can include descendants recursively; address-book expansion is scoped to the sender's own address book.
- Expanded recipients are deduplicated across To, Cc, and Bcc in precedence order. Verification: `go test ./...` and `pnpm --dir apps/webmail type-check` pass; API smoke sends returned 202 for an organization token and an address-book token, storing 9 and 4 concrete To recipients respectively. Smoke testing also hardened recursive organization expansion and local mailstore root creation for Docker dev sends.

## Webmail recipient token display cleanup (TASK-166, 2026-05-12, complete)
- Organization and address-book recipient tokens now display natural names in the picker and compose fields.
- Removed visible `[조직]`, `[하위 조직]`, `+ 하위 조직`, and `[주소록]` wording from token display names while preserving the internal token values used for backend expansion.
- The child-organization inclusion checkbox remains visible because it controls actual send expansion behavior.

## Webmail drive share link hotfix (2026-05-12)
- Fixed `/api/mail/` proxy behavior for public drive share link routes (`/api/mail/drive/share-links/...`) so `user_id` is stripped and not injected for GET/HEAD shared-download/resolve calls.
- This prevents 400 `"user_id is not supported"` responses when opening shared links directly in the browser.

## Webmail drive multi-select drag feedback (in progress, 2026-05-12)
- Added drag feedback for multi-selection in Drive move UX: starting drag with multiple selected items now shows a custom stack-style drag ghost preview and toolbar badge (`N개 항목 이동`).
- Data transfer still uses the existing node-list payload, while users get stronger visual confirmation that selected files are moving together as one batch.
- Added subtle motion polish to multi-select drag feedback:
  - drag badge and drag ghost now pulse softly while dragging,
  - active drop targets pulse their border to make destination intent more obvious.

## Webmail drive breadcrumb move target support (Drive UX polish, 2026-05-12, in progress)
- Drive breadcrumb(상단 경로) 항목을 드래그 드롭 대상에서 제외하지 않고, 내부 노드 드래그 시 해당 경로(현재 폴더의 상위 경로 포함)로 직접 이동할 수 있게 동작을 보강했다.
- 브레드크럼의 각 경로 항목이 폴더 이동 목표가 되면 이동 대상 하이라이트가 표시되도록 했고, 폴더 카드 기준 이동 동작과 동일하게 다중 선택 노드 이동도 처리한다.
- 브레드크럼 대상으로 외부 파일을 드롭하면 해당 경로에 파일 업로드가 동작하도록 유지했다.

## Admin/webmail contract alignment audit (2026-05-13)
- Webmail message/folder/attachment client types now include the required OpenAPI fields (`order_index`, message `size`, RFC `message_id`, `flags`, `storage_path`, and attachment storage metadata) while keeping UI-only `reply_all` normalized to backend `reply` before send/draft requests.
- Console domain, role, and user hooks now use the current Admin API envelopes and endpoints (`/domains`, `/roles`, `/users`) instead of stale `/companies/{id}/domains|roles` or generic user update/delete routes that are not in the backend contract.
- Company user management now scopes list loading by the company's domains and uses the proxied Admin API path for role and bulk status changes, avoiding cross-tenant user mixing in the console.
- Domain DNS re-check actions now use the documented `GET /admin/v1/domains/{id}/dns-check` contract.
- The Admin user-create contract now documents the temporary plain `password` field that the backend hashes before persistence, alongside the existing `password_hash` advanced path.
- Console shared hooks for SSO config, auth policy, reports, organization hierarchy, and audit logs now call the backend routes and response envelope keys that actually exist (`/companies/{id}/sso/config`, `/security/auth-policy`, `/reports`, `/organization/hierarchy`, and `/audit-logs`).
- Console API key, alert, identity-provider, and statistics hooks no longer assume stale `res.data` wrappers or removed `/statistics|identity-providers|ldap-sync|rdbms-sync` routes. API key and alert hooks now use backend envelopes directly; deferred identity-provider writes/sync operations now fail explicitly instead of calling nonexistent endpoints.
- User runtime-config and MFA management pages now load users through the current company's domains before querying `/users`, preventing cross-company user rows from appearing in company-scoped admin tools.
- Console organization webhooks no longer POST caller-supplied `secret` fields that the backend rejects, and notification templates now edit the backend's `subject`/`body`/`enabled` model instead of stale locale/body_text/body_html fields.
- Webmail settings no longer expose the stale mailbox-import flow that called nonexistent `/messages/restore`; the UI now keeps only supported local export and backend trash restore paths.
- Backend company user exports, SCIM status counts, and security posture now enumerate users through the company's domains instead of unscoped `ListUsers` calls, keeping tenant-scoped admin data aligned with the console behavior.
- The console `/admin/v1` proxy now mirrors `/api/admin` 204 and download-header handling, so direct admin-v1 DELETE/export screens preserve backend response semantics.
- Admin OpenAPI metadata now matches organization webhook and notification-template behavior: webhook create requests omit caller-provided secrets, webhook tests return `status_code`, and notification templates expose `subject`/`body`/`enabled`.

## Company/domain settings inheritance polish (2026-05-14)
- Console company settings now presents the same policy sections as domain settings: user registration, security, password/reset policy, and per-user storage quota.
- Company policy defaults are stored under the reserved company config key `domain_settings_defaults`; newly created domains inherit that policy into `domain_settings` at the backend API boundary.
- Domain settings still override those defaults after creation, and both company/domain per-user quota fields are shown in MB while the backend continues storing bytes.
- The quota UX now lets admins enter per-user quota as a number with an explicit MB/GB/TB unit selector; values are converted back to bytes before saving.

## CalDAV/CardDAV WebDAV If header duplicate delimiter hardening (TASK-232, 2026-05-14)
- CalDAV collection `PROPPATCH` now has regression coverage for WebDAV `If` resource tags with duplicate closing delimiters, rejecting them as malformed resource tags with HTTP 400 before reading the XML body.
- CardDAV collection `PROPPATCH` now has the same duplicate-delimiter coverage, preserving existing address book `xml:lang` metadata and avoiding any update call when the `If` header is malformed.
