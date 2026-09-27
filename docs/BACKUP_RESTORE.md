# Backup & Restore

First-class backup and restore tooling for gogomail. The `gogomail backup` and
`gogomail restore` subcommands produce and consume a self-describing **backup
bundle** — a directory containing the PostgreSQL dump, an object-storage
manifest, a redacted config snapshot, and a checksummed `manifest.json` that
ties them together and records the schema migration version.

These subcommands supersede the legacy `scripts/backup.sh` wrapper (see
[Deprecation](#deprecation-of-scriptsbackupsh)).

---

## What a bundle contains

A bundle is the directory `gogomail-backup-<UTC-timestamp>/`:

| File | Description |
|---|---|
| `manifest.json` | Bundle descriptor: schema version, creation time, tool version, migration version, storage backend, per-file SHA-256 checksums, and scope. |
| `database.sql.gz` | `pg_dump --format=plain --no-owner --no-privileges`, gzip-compressed. |
| `storage-manifest.json` | Listing of every object key in the storage backend with size, ETag, and (optionally) a SHA-256 content hash. |
| `config.json` | Redacted [config snapshot](#redaction-and-safety): storage topology, migration dir, and the *names* of secret keys that were present — never their values. |

Every non-manifest file's size and SHA-256 are recorded in `manifest.json`, and
`restore` verifies them before touching any target.

**Scope is always instance-wide.** A gogomail database is a shared,
multi-tenant schema (company → domain → user). A logical `pg_dump` and an
object-store listing both span every tenant, so a bundle covers the whole
instance. See [Per-tenant vs whole-instance](#per-tenant-vs-whole-instance).

---

## Requirements

- `pg_dump` on `PATH` for `backup`; `psql` on `PATH` for `restore` (PostgreSQL
  client tools, version ≥ the server's major version).
- The same config the service uses (via `--config` or `GOGOMAIL_*` env vars) so
  the tool observes the same database DSN and storage backend.

---

## Taking a backup

```bash
gogomail backup --config /etc/gogomail/config.yaml --out /var/backups/gogomail
```

Flags:

| Flag | Default | Purpose |
|---|---|---|
| `--config` | (env) | YAML config file; falls back to `GOGOMAIL_*` env vars. |
| `--out` | `./backups` | Parent directory for the timestamped bundle. |
| `--hash-storage` | `false` | Stream every object to compute a SHA-256. Exact but O(total bytes) — leave off for large S3 buckets where bucket versioning + ETags suffice. |
| `--storage-prefix` | `""` | Restrict the storage manifest to keys under a prefix. |
| `--max-storage-objects` | `0` | Fail if the manifest would exceed N objects (guards against unbounded manifests). `0` = unlimited. |
| `--skip-storage` | `false` | Skip the storage manifest entirely (DB-only bundle). |

The command captures the source database's current migration version and warns
if it is behind the migrations bundled with the binary.

> **Object bytes are not copied into the bundle.** The storage manifest is a
> *listing*, not a byte-for-byte archive. Protect object content with backend
> features (see [Object storage](#object-storage-backup)). For `local`/`nfs`
> backends, archive the `MailstoreRoot` directory alongside the bundle.

---

## Restoring

```bash
gogomail restore --config /etc/gogomail/config.yaml \
  --bundle /var/backups/gogomail/gogomail-backup-2026-09-27T120000Z
```

Restore, in order:

1. **Validates the bundle** — manifest schema version understood, every file
   present with a matching size and SHA-256, required `database.sql.gz` present.
2. **Inspects the target** — is it empty? what migration version is applied?
3. **Evaluates restore guards** (below) and prints a **pre-restore checklist**.
4. Unless `--dry-run`, streams `database.sql.gz` through `psql --set
   ON_ERROR_STOP=1` into the target.

Flags:

| Flag | Default | Purpose |
|---|---|---|
| `--config` | (env) | YAML config for the **target** database. |
| `--bundle` | (required) | Path to the bundle directory. |
| `--force` | `false` | Allow restoring into a **non-empty** target (destructive). |
| `--dry-run` | `false` | Validate + checklist only; make no changes. |

### Restore guards

| Guard | Bypassable with `--force`? | Rationale |
|---|---|---|
| Unknown manifest schema version | **No** | The bundle format is not understood by this binary. |
| Bundle migration version **newer** than the binary supports | **No** | The dump references schema this binary cannot serve; upgrade gogomail first. |
| Target database is **not empty** | **Yes** | Refuse to clobber live data by default; `--force` opts in explicitly. |

`--dry-run` is the recommended first step for any restore into an unfamiliar
target.

---

## Redaction and safety

`config.json` in the bundle is a **redacted** [`ConfigSnapshot`](../internal/backup/config_snapshot.go).
It captures only non-secret deployment coordinates:

- environment, storage backend, migration dir, mailstore root
- S3 endpoint/region/bucket/prefix and replica bucket/region
- boolean flags for whether the primary/replica DSNs were configured

Every field that could carry a credential — `GOGOMAIL_DATABASE_URL`,
`GOGOMAIL_DATABASE_REPLICA_URL`, `GOGOMAIL_REDIS_PASSWORD`, the three
`GOGOMAIL_STORAGE_S3_*` credential fields, `GOGOMAIL_SCIM_TOKEN`,
`GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN` — is **never** written to the bundle.
Instead, the *key name* is recorded in `config.json`'s `redacted_secrets` list
so an operator knows which secrets must be re-supplied on the restore host. A
unit test asserts the serialized snapshot contains none of the raw secret
values.

The `database.sql.gz` dump necessarily contains application data (including
password *hashes* stored in the schema — never plaintext). **Treat bundles as
sensitive** and store them encrypted at rest (SSE-S3/SSE-KMS on S3, LUKS/dm-crypt
on local disk).

---

## RPO / RTO guidance

| Metric | This tooling | To improve |
|---|---|---|
| **RPO** (max data loss) | = your backup interval. Hourly cron → up to 1h loss. | Add WAL archiving / PITR (see [Future work](#future-work-point-in-time-recovery)). |
| **RTO** (time to restore) | dominated by `psql` load time + object-store availability. Minutes for small instances, longer for large dumps. | Restore to a warm standby; use physical replication for large datasets. |

Logical dumps (`pg_dump`) are portable and version-tolerant but are a
**point-in-time snapshot**: anything written after the dump is lost on restore.
For sub-minute RPO you need continuous WAL archiving, which is out of scope for
this tooling today.

---

## Scheduling

### cron

```cron
# /etc/cron.d/gogomail-backup — hourly, UTC
0 * * * * gogomail /usr/local/bin/gogomail backup \
  --config /etc/gogomail/config.yaml --out /var/backups/gogomail \
  >> /var/log/gogomail-backup.log 2>&1
```

### systemd timer

`/etc/systemd/system/gogomail-backup.service`:

```ini
[Unit]
Description=gogomail backup
After=network-online.target

[Service]
Type=oneshot
User=gogomail
Environment=PATH=/usr/local/bin:/usr/bin:/bin
ExecStart=/usr/local/bin/gogomail backup --config /etc/gogomail/config.yaml --out /var/backups/gogomail
```

`/etc/systemd/system/gogomail-backup.timer`:

```ini
[Unit]
Description=Run gogomail backup hourly

[Timer]
OnCalendar=hourly
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
```

```bash
systemctl enable --now gogomail-backup.timer
```

---

## Retention pruning

The subcommand writes bundles but does not prune. Prune by age on the backup
host:

```bash
# Keep 7 days of local bundles
find /var/backups/gogomail -maxdepth 1 -type d -name 'gogomail-backup-*' \
  -mtime +7 -exec rm -rf {} +
```

### S3 lifecycle tie-in

When bundles are shipped to S3 (e.g. `aws s3 sync /var/backups/gogomail
s3://my-bucket/gogomail/`), let a bucket **lifecycle policy** handle retention
and cost tiering instead of `find`:

```json
{
  "Rules": [
    {
      "ID": "gogomail-backups",
      "Filter": { "Prefix": "gogomail/" },
      "Status": "Enabled",
      "Transitions": [
        { "Days": 30, "StorageClass": "STANDARD_IA" },
        { "Days": 90, "StorageClass": "GLACIER" }
      ],
      "Expiration": { "Days": 365 }
    }
  ]
}
```

Enable **bucket versioning** on the mail object bucket so that the storage
manifest's listed objects can be recovered even after overwrite/delete — this is
how object *content* is protected, complementing the bundle's manifest.

---

## Restore drill (rehearsal)

Rehearse restores regularly against a scratch database — an untested backup is
not a backup.

1. Provision an **empty** scratch PostgreSQL database.
2. Point a throwaway config at it (`GOGOMAIL_DATABASE_URL=...scratch`).
3. Dry-run first:
   ```bash
   gogomail restore --config scratch.yaml --bundle <bundle> --dry-run
   ```
   Confirm the checklist: manifest understood, migration version compatible,
   target empty.
4. Real restore into the scratch DB:
   ```bash
   gogomail restore --config scratch.yaml --bundle <bundle>
   ```
5. Verify migration version and row counts, then drop the scratch DB.

The existing `scripts/backup-restore-rehearsal.sh` automates a dump→scratch
round-trip against a live source and remains useful for a quick end-to-end
sanity check.

---

## Per-tenant vs whole-instance

**Restores are instance-wide, by design.** gogomail stores all tenants in one
shared PostgreSQL schema with foreign keys spanning company/domain/user tables,
and objects for all tenants live under one storage backend. A logical dump and
an object listing therefore cover every tenant atomically.

Restoring a single tenant from a whole-instance bundle is **not** supported by
this tooling, because:

- Selectively loading one tenant's rows would violate cross-tenant foreign keys
  and shared sequences, risking referential corruption.
- Object keys are namespaced per tenant but share one bucket/root; partial
  object restore has no transactional boundary with the DB.

For tenant-level "undo" use application-level features (soft-delete, audit
logs) rather than storage restore. If you need tenant-isolated backups, run
tenants in **separate databases/instances** — the same binary and config
contract supports that topology.

---

## Future work: point-in-time recovery

Continuous PITR is **out of scope** for this tooling. The recommended approach
when sub-interval RPO is required:

1. Enable PostgreSQL **WAL archiving** (`archive_mode=on`, `archive_command` to
   S3 or a WAL archive such as `pgBackRest` / `wal-g`).
2. Take periodic base backups (`pg_basebackup` or `pgBackRest`).
3. Recover to an arbitrary timestamp by replaying WAL onto a base backup.
4. Pair with S3 **bucket versioning** so object content can be rolled to a
   matching point.

This gives seconds-level RPO at the cost of operating a WAL archive. The
`gogomail backup`/`restore` bundle remains the portable, cross-version disaster-
recovery artifact and complements — rather than replaces — a PITR pipeline.

---

## Deprecation of `scripts/backup.sh`

`scripts/backup.sh` (a 60-line `pg_dump | gzip` wrapper with optional S3 upload)
is **superseded** by `gogomail backup`. The subcommand additionally captures a
storage manifest, a redacted config snapshot, and a checksummed migration
version — none of which the script produced — and its output is validated on
restore.

- **Prefer** `gogomail backup` for all new automation.
- The script is retained temporarily for environments that cannot yet deploy the
  updated binary; it produces only a DB dump and offers no restore validation.
- `scripts/backup-restore-rehearsal.sh` remains supported as a quick
  dump→restore round-trip rehearsal utility.

---

## Object storage backup

The storage manifest records *what* objects exist, not their bytes. Choose one:

- **S3 / MinIO**: enable **bucket versioning** and (optionally) **cross-region
  replication**; use lifecycle rules for retention. The manifest lets you audit
  that expected keys exist and (with `--hash-storage`) that content matches.
- **local / nfs**: archive the `MailstoreRoot` directory (`tar`, `rsync`,
  filesystem snapshot) on the same schedule as the DB backup, and store it
  alongside the bundle. The manifest documents the expected key set.
