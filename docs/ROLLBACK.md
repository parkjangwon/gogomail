# Rollback Runbook

## RTO / RPO Targets

| Tier | Recovery Time Objective (RTO) | Recovery Point Objective (RPO) |
|------|-------------------------------|--------------------------------|
| Database (PostgreSQL) | ≤ 30 minutes | ≤ 5 minutes (WAL streaming) |
| Object storage (MinIO/S3) | ≤ 1 hour | ≤ 24 hours (daily backup) |
| Full service (all-in-one) | ≤ 1 hour | ≤ 5 minutes |

---

## 1. Application Rollback (container / binary)

### Rolling back to the previous image

```bash
# Find the previous image tag
docker images gogomail --format "{{.Tag}}" | head -5

# Update the compose file or Kubernetes manifest to the previous tag,
# then redeploy:
docker compose up -d --force-recreate backend-1 backend-2
```

All schema migrations are forward-only and additive. A previous binary version
is safe to run against a newer schema as long as no columns it depends on were
dropped (check the migration changelog first).

If a migration must be rolled back:

```bash
# Dry-run to see what would be reverted
goose -dir migrations postgres "$GOGOMAIL_DATABASE_URL" status

# Roll back the most recent migration
goose -dir migrations postgres "$GOGOMAIL_DATABASE_URL" down
```

---

## 2. Database Restore from Backup

```bash
# 1. Stop all backend instances to prevent writes
docker compose stop backend-1 backend-2

# 2. Run the rehearsal script pointing at the target database
GOGOMAIL_DATABASE_URL=postgres://gogomail:$PG_PASS@postgres-primary:5432/gogomail \
  bash scripts/backup-restore-rehearsal.sh

# 3. Verify migration metadata in the restored database
psql "$GOGOMAIL_DATABASE_URL" -c "SELECT version FROM goose_db_version ORDER BY id DESC LIMIT 5;"

# 4. Restart backends
docker compose start backend-1 backend-2
```

### Point-in-time recovery (PostgreSQL WAL)

If using `pg_basebackup` + WAL archiving:

```bash
# Set recovery target in recovery.conf (PostgreSQL ≤ 11) or
# postgresql.conf (PostgreSQL ≥ 12):
recovery_target_time = '2026-05-22 03:00:00 UTC'
restore_command = 'aws s3 cp s3://your-wal-bucket/wal/%f %p'

# Then start the replica; it will replay WAL up to the target time.
```

---

## 3. Backup Rehearsal Automation

Run the restore rehearsal regularly (weekly recommended):

```bash
GOGOMAIL_DATABASE_URL=postgres://... \
GOGOMAIL_RESTORE_REHEARSAL_DB_URL=postgres://.../gogomail_rehearsal \
  bash scripts/backup-restore-rehearsal.sh
```

Rehearsal verifies:
- `pg_dump` completes without error
- Restore into scratch database succeeds
- Migration metadata is consistent
- Scratch database is dropped afterward (unless `GOGOMAIL_RESTORE_REHEARSAL_KEEP_DB=1`)

Integrate into CI/CD for automated rehearsal on schedule:

```yaml
# .github/workflows/backup-rehearsal.yml (example)
on:
  schedule:
    - cron: "0 3 * * 0"   # weekly at 03:00 UTC Sunday
```

---

## 4. Object Storage Restore

MinIO multi-node erasure-coded cluster tolerates N/2-1 node failures without data
loss. For full restore from S3-compatible backup:

```bash
# Using mc (MinIO client)
mc mirror s3/gogomail-backup/latest minio/gogomail --overwrite
```

---

## 5. Smoke Test After Restore

```bash
# Health check
curl -f http://localhost:8080/health

# Verify SMTP receives mail
echo "Subject: smoke test" | sendmail -v user@yourdomain.test

# Verify IMAP login
openssl s_client -connect localhost:993 -quiet <<EOF
a1 LOGIN user@yourdomain.test password
a2 LIST "" "*"
a3 LOGOUT
EOF
```
