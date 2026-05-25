#!/usr/bin/env sh
# reset_dev_data.sh — wipe all tenant data EXCEPT the GoGoMail admin tenant
#                     (admin@gogomail.io / gogomain.io / company GoGoMail),
#                     then re-seed from seed_dev_data.sql.
#
# Usage:
#   ./scripts/reset_dev_data.sh              # wipe + reseed
#   ./scripts/reset_dev_data.sh --wipe-only  # wipe only, no reseed
#   ./scripts/reset_dev_data.sh --yes        # skip confirmation prompt

set -eu

cd "$(dirname "$0")/.."

CONTAINER="${GOGOMAIL_DEV_POSTGRES_CONTAINER:-gogomail-postgres-dev}"
DB_USER="${GOGOMAIL_DEV_POSTGRES_USER:-gogomail}"
DB_NAME="${GOGOMAIL_DEV_POSTGRES_DB:-gogomail}"
SEED_FILE="${GOGOMAIL_DEV_SEED_FILE:-scripts/seed_dev_data.sql}"

WIPE_ONLY=0
AUTO_YES=0

for arg in "$@"; do
  case "$arg" in
    --wipe-only) WIPE_ONLY=1 ;;
    --yes|-y)    AUTO_YES=1  ;;
  esac
done

# ── Preflight ─────────────────────────────────────────────────────────────────

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2; exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "PostgreSQL container not running: $CONTAINER" >&2
  echo "Start with: docker compose -f docker/docker-compose.dev.yml up -d postgres" >&2
  exit 1
fi

# ── Confirmation ──────────────────────────────────────────────────────────────

echo ""
echo "  ┌─────────────────────────────────────────────────────┐"
echo "  │  GoGoMail DEV DATA RESET                            │"
echo "  │                                                     │"
echo "  │  This will DELETE all tenant data EXCEPT:           │"
echo "  │    • company  : GoGoMail                            │"
echo "  │    • domain   : gogomail.io                         │"
echo "  │    • user     : admin@gogomail.io                   │"
echo "  │                                                     │"
echo "  │  All mail, contacts, calendars, folders, users      │"
echo "  │  in other tenants will be permanently removed.      │"
echo "  └─────────────────────────────────────────────────────┘"
echo ""

if [ "$AUTO_YES" -eq 0 ]; then
  printf "  Type 'yes' to continue: "
  read -r CONFIRM
  if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 0
  fi
fi

# ── Wipe SQL ──────────────────────────────────────────────────────────────────
#
# Preserved IDs (admin tenant):
#   company  10000000-0000-0000-0000-000000000001
#   domain   10000000-0000-0000-0000-000000000002
#   user     10000000-0000-0000-0000-000000000003

echo ""
echo "==> Wiping non-admin tenant data..."

docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" <<'RESET_SQL'
BEGIN;

-- ── Constants ─────────────────────────────────────────────────────────────────
-- Admin tenant preserved:
--   company 10000000-0000-0000-0000-000000000001
--   domain  10000000-0000-0000-0000-000000000002
--   user    10000000-0000-0000-0000-000000000003

-- ── Mail / search ────────────────────────────────────────────────────────────
-- message_tracking_events references pixel_id, not message_id directly
DELETE FROM message_tracking_events
WHERE pixel_id IN (
  SELECT p.pixel_id FROM message_tracking_pixels p
  JOIN messages m ON m.id = p.message_id
  WHERE m.domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM message_tracking_pixels
WHERE message_id IN (
  SELECT id FROM messages
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM message_search_documents
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM delivery_attempts
WHERE message_id IN (
  SELECT id FROM messages
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

-- outbox has no domain_id; purge via payload tenant reference or clear all non-admin rows by topic
-- Since outbox is transient (relay queue), safest is to truncate entire outbox
TRUNCATE outbox;

DELETE FROM mail_flow_logs
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM attachments
WHERE message_id IN (
  SELECT id FROM messages
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM imap_message_uid
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM imap_mailbox_state
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM imap_mailbox_subscriptions
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM messages
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

-- ── Folders ───────────────────────────────────────────────────────────────────
DELETE FROM folders
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

-- ── CalDAV ────────────────────────────────────────────────────────────────────
DELETE FROM caldav_calendar_sync_changes
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM caldav_calendar_objects
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM caldav_calendars
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

-- ── CardDAV ───────────────────────────────────────────────────────────────────
DELETE FROM carddav_acl_rules
WHERE addressbook_id IN (
  SELECT id FROM carddav_addressbooks
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM carddav_addressbook_changes
WHERE addressbook_id IN (
  SELECT id FROM carddav_addressbooks
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM carddav_contact_objects
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM carddav_addressbooks
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

-- ── Drive ─────────────────────────────────────────────────────────────────────
DELETE FROM drive_share_links
WHERE node_id IN (
  SELECT id FROM drive_nodes
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM drive_nodes
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

-- ── Directory / org ──────────────────────────────────────────────────────────
DELETE FROM directory_group_memberships
WHERE group_id IN (
  SELECT id FROM directory_groups
  WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM directory_groups
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM directory_aliases
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM directory_delegations
WHERE company_id != '10000000-0000-0000-0000-000000000001'::uuid;

DELETE FROM directory_resources
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM organization_members
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id IN (
    '10000000-0000-0000-0000-000000000002'::uuid
  )
);

DELETE FROM organization_units
WHERE company_id != '10000000-0000-0000-0000-000000000001'::uuid;

DELETE FROM organizations
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

-- ── Auth / session ───────────────────────────────────────────────────────────
DELETE FROM user_refresh_tokens
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM user_mfa_secrets
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM user_invite_tokens
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM push_devices
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM web_push_subscriptions
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

DELETE FROM api_keys
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM user_mcp_access_keys
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

-- ── Notifications / alerts ───────────────────────────────────────────────────
DELETE FROM notification_preferences
WHERE user_id NOT IN (
  SELECT id FROM users WHERE domain_id = '10000000-0000-0000-0000-000000000002'::uuid
);

-- ── User addresses + users ───────────────────────────────────────────────────
DELETE FROM user_addresses
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM users
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

-- ── Domain + company ─────────────────────────────────────────────────────────
DELETE FROM domain_settings
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM dkim_keys
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM domain_dns_checks
WHERE domain_id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM domains
WHERE id != '10000000-0000-0000-0000-000000000002'::uuid;

DELETE FROM companies
WHERE id != '10000000-0000-0000-0000-000000000001'::uuid;

COMMIT;

-- Summary
SELECT 'companies'  AS tbl, COUNT(*) FROM companies
UNION ALL SELECT 'domains',    COUNT(*) FROM domains
UNION ALL SELECT 'users',      COUNT(*) FROM users
UNION ALL SELECT 'messages',   COUNT(*) FROM messages
UNION ALL SELECT 'folders',    COUNT(*) FROM folders
UNION ALL SELECT 'contacts',   COUNT(*) FROM carddav_contact_objects
UNION ALL SELECT 'cal_events', COUNT(*) FROM caldav_calendar_objects;
RESET_SQL

echo "==> Wipe complete."

# ── Reseed ────────────────────────────────────────────────────────────────────

if [ "$WIPE_ONLY" -eq 1 ]; then
  echo ""
  echo "Skipping reseed (--wipe-only)."
  echo "Run  bash scripts/seed_dev_beta.sh  to reseed."
  exit 0
fi

if [ ! -f "$SEED_FILE" ]; then
  echo "Seed file not found: $SEED_FILE — skipping reseed." >&2
  exit 0
fi

echo ""
echo "==> Reseeding from $SEED_FILE ..."
bash "$(dirname "$0")/seed_dev_beta.sh" --yes 2>/dev/null || \
  docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" < "$SEED_FILE"
