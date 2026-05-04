# API usage export runbook

This runbook covers the backend-only API usage export path used before billing,
warehouse handoff, or future ledger retention work. It assumes the Admin API is
running with `GOGOMAIL_ADMIN_TOKEN` configured.

## 1. Check export capability

For invoice-grade handoff, configure a production signer such as:

```bash
GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=remote-ed25519
GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID=api-usage-signing-key-1
GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_URL=https://signer.example.com/v1/api-usage/manifest-signatures
GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_TOKEN=<optional-bearer-token>
GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PUBLIC_KEY=<base64-raw-ed25519-public-key>
```

```bash
curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-capabilities"
```

Expected release-safe interpretation:

- `signer_configured` and `verifier_configured` should be true before creating
  signed handoff evidence.
- `production_signature_ready` remains false for `local-hmac` and
  `local-ed25519`; those signers are operational evidence only.
- `remote-ed25519` can clear production signature readiness when it is backed by
  an approved HTTPS signing service and the configured public key verifies the
  returned signatures.
- `blocking_reasons` containing `production_manifest_signer_required` means the
  batch must not be treated as invoice-grade.

## 2. Create the saved export batch

Use the same `tenant_id`, `principal_id`, and time window that the downstream
consumer expects. `from` and `to` are required RFC3339 timestamps, and `to` is
exclusive.

```bash
curl -sS -X POST -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches?tenant_id=$TENANT_ID&from=$WINDOW_START&to=$WINDOW_END"
```

Record the returned `id` as `BATCH_ID`.

## 3. Write and verify the NDJSON artifact

```bash
curl -sS -X POST -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/artifacts/write"

curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/artifacts/$ARTIFACT_ID/verification"
```

The artifact verification result must have `valid: true`.

## 4. Create and verify the manifest digest

```bash
curl -sS -X POST -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/manifest-digests"

curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/manifest-digests/$DIGEST_ID/verification"
```

The digest verification result must have `valid: true`.

## 5. Sign and verify the manifest digest

```bash
curl -sS -X POST -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/manifest-digests/$DIGEST_ID/signatures"

curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/manifest-digests/$DIGEST_ID/signatures/$SIGNATURE_ID/verification"
```

The signature verification result must have `valid: true`. Local signers sign
the lowercase 64-character manifest digest hex string.

## 6. Run handoff readiness

```bash
curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/export-batches/$BATCH_ID/handoff-readiness?deep=true"
```

Release-safe gates:

- `ready` must be true for operational handoff.
- `deep_ready` must be true before relying on stored object evidence.
- `billing_ready` and `verified_billing_ready` remain false for local signers.
- Do not use the batch for invoices or hard Open API limits until a production
  signer backend clears `production_manifest_signer_required`.

## 7. Check retention readiness

Before any future archive/delete worker is enabled, check the exact cutoff and
filters intended for retention:

```bash
curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/ledger/retention-readiness?tenant_id=$TENANT_ID&cutoff=$WINDOW_END"
```

Retention readiness is read-only. It is safe only when `ready` is true. Blocking
reasons mean:

- `covering_export_batch_required`: no completed matching export batch covers
  the candidate event window.
- `covering_export_batch_stale`: a pre-cutoff ledger row was recorded after the
  covering batch completed.
- `covering_export_artifact_required`: the covering batch has no complete
  artifact evidence.
- `covering_manifest_digest_required`: the covering batch has no manifest digest
  evidence.
- `covering_manifest_signature_required`: the covering batch has no manifest
  signature evidence.

Do not archive or purge immutable ledger rows when any blocking reason is
present.

## 8. Run bounded retention

After readiness is true for the exact cutoff and filters, run a dry-run first:

```bash
curl -sS -X POST -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cutoff":"'"$WINDOW_END"'","tenant_id":"'"$TENANT_ID"'","dry_run":true,"limit":1000}' \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/ledger/retention-runs"
```

For a destructive bounded batch, set `dry_run` to `false` and include
`confirm_ready:true`. The run rechecks readiness before deleting rows and
returns the embedded readiness evidence plus candidate, limited, and deleted
counts. Every blocked, dry-run, or destructive retention attempt is persisted in
`api_usage_ledger_retention_runs` with its returned `id`, `created_at`, filters,
counts, and readiness snapshot so operators can audit ledger purges after the
fact.

Use the Admin API to review retention attempts later:

```bash
curl -sS -H "Authorization: Bearer $GOGOMAIL_ADMIN_TOKEN" \
  "$GOGOMAIL_ADMIN_URL/admin/v1/api-usage/ledger/retention-runs?tenant_id=$TENANT_ID&limit=20"
```

Fetch the returned run `id` for the exact readiness snapshot used by that
blocked, dry-run, or destructive attempt.

## 9. Optional scheduled retention worker

The `api-usage-retention-worker` mode runs the same bounded retention operation
on an interval, using `now - GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE` as the
cutoff. It is dry-run by default.

```bash
GOGOMAIL_API_USAGE_RETENTION_INTERVAL=24h
GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE=2160h
GOGOMAIL_API_USAGE_RETENTION_BATCH_SIZE=1000
GOGOMAIL_API_USAGE_RETENTION_RUN_ONCE=false
GOGOMAIL_API_USAGE_RETENTION_DRY_RUN=true
GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY=false
gogomail --mode=api-usage-retention-worker
```

For a one-shot CronJob-style deployment, set
`GOGOMAIL_API_USAGE_RETENTION_RUN_ONCE=true`. For destructive retention, set
`GOGOMAIL_API_USAGE_RETENTION_DRY_RUN=false` and
`GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY=true`; config validation also
requires `GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=remote-ed25519` so
scheduled destructive purges cannot run under local-only signing evidence. The
worker still rechecks readiness and persists every blocked, dry-run, or
destructive attempt in `api_usage_ledger_retention_runs`.
