# ADR 0009: Drive module metadata boundary

Date: 2026-05-06

## Status

Accepted

## Context

gogomail is moving toward a production webmail platform that can also expose a
Drive-like user storage module. Storage must remain portable across local/NFS,
MinIO, AWS S3, and strict S3-compatible backends, while user capacity remains
part of the existing company -> domain -> user quota pool.

The Drive module needs folder trees, file metadata, trash/delete lifecycle,
future sharing, and future resumable upload reuse without embedding filesystem
or S3 assumptions in product code.

## Decision

Drive metadata lives in PostgreSQL and object bytes live behind the shared
storage interface.

The first persisted boundary is `drive_nodes`:

- every node is scoped by company, domain, and user;
- folders and files share one adjacency-list tree through `parent_id`;
- active sibling names are unique per user and parent through a normalized name;
- files point at a storage backend/object key plus size/checksum metadata;
- folders do not carry storage object keys;
- lifecycle starts with `active`, `trashed`, and `deleted`;
- quota consumption uses the existing unified user storage ledger when file
  create/finalize/copy/delete behavior is implemented.

Drive service code should use the shared storage primitives:

- `Stat` to verify stored file bytes before metadata finalization;
- `Copy` for file copy/version-style workflows;
- `Move` for object relocation when metadata moves require physical object
  relocation;
- `List` and `DeletePrefix` for bounded cleanup and reconciliation work.

## Consequences

- Drive can be added without starting frontend implementation.
- Drive metadata APIs can evolve separately from Mail API message/folder
  contracts while still sharing tenant, user, quota, audit, and storage
  boundaries.
- S3-compatible object moves and cleanup remain non-atomic at the storage layer;
  Drive workflows must handle duplicate cleanup and retryable recovery.
- Share links extend this boundary through `drive_share_links`: links are
  user/node scoped, only active file nodes can mint links, raw bearer tokens are
  returned only at creation time, and PostgreSQL persists only token hashes plus
  bounded suffixes for diagnostics.
- Future public share-link resolution/download, collaborative metadata,
  resumable uploads, and search indexing should extend this boundary instead of
  encoding Drive state only in object paths.
