# ADR 0007: Resumable attachment upload boundary

Date: 2026-05-05

## Status

Accepted

## Context

gogomail already supports attachment metadata reservation, direct multipart
uploads, pending upload cancellation, stale upload cleanup, and shared
company/domain/user quota accounting.

Large attachment workflows will need resumable or chunked uploads, but the mail
API must not become a storage-engine-specific multipart protocol. The design
also needs to preserve quota correctness, draft binding semantics, cleanup
safety, and future reuse by large-attachment share links or Drive-like modules.

## Decision

Model resumable uploads as explicit upload sessions owned by the Mail API
service boundary, with storage writes delegated to a replaceable storage/session
adapter.

The future contract should keep these responsibilities separate:

- Mail API: authenticate users, create upload sessions, validate declared total
  size, expose client-safe session state, accept chunk commit calls, and finalize
  or cancel sessions.
- Quota ledger: reserve the declared total size when a session is created and
  release it on cancellation, expiry, or failed finalization cleanup.
- Storage adapter: stage chunks or provider-native multipart parts, verify
  byte ranges and checksums when available, and produce one final object path.
- Attachment repository: create the normal attachment row only after final
  object assembly succeeds, or keep an explicit session table separate from
  message-bound attachment rows.
- Cleanup worker/Admin cleanup: expire stale sessions and staged chunks with the
  same bounded, previewable, idempotent cleanup style used for pending uploads.

The current `GET /api/v1/attachments/capabilities` contract must continue to
advertise `resumable_chunked_uploads=false` until these session contracts and
cleanup paths exist.

## Consequences

- Resumable upload state should not be encoded only in object-storage paths or
  draft attachment rows.
- Direct multipart uploads remain the simple path for small deployments.
- Future session tables should carry tenant/user ownership, declared size,
  received byte count, status, expiry, and storage adapter metadata.
- Chunk APIs should be idempotent and range/checksum aware so mobile and
  unstable-network clients can retry safely.
- Provider-specific multipart behavior belongs behind adapters rather than in
  SMTP, message parsing, or generic attachment repository code.
