#!/usr/bin/env bash

set -euo pipefail

DSN="${GOGOMAIL_TEST_DATABASE_URL:-${TASK_090_DATABASE_URL:-${1:-}}}"
if [[ -z "${DSN}" ]]; then
  echo "Usage: TASK_090_DATABASE_URL=<psql-url> $0" >&2
  echo "   or: GOGOMAIL_TEST_DATABASE_URL=<psql-url> $0" >&2
  echo "   or: pass DSN as first arg" >&2
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required: install PostgreSQL client and retry" >&2
  exit 1
fi

TMP_EXPLAIN_FILE="${TASK_090_EXPLAIN_OUT:-/tmp/task090-explain-$(date +%Y%m%d-%H%M%S).log}"

has_target=$(
  psql -Atq "${DSN}" <<'SQL'
SELECT EXISTS (
  SELECT 1
  FROM messages
  WHERE status = 'active' AND user_id IS NOT NULL
  LIMIT 1
);
SQL
)

if [[ "${has_target}" != "t" ]]; then
  echo "No active messages found; skip EXPLAIN ANALYZE baseline for TASK-090." >&2
  exit 0
fi

echo "[TASK-090] EXPLAIN ANALYZE snapshots -> ${TMP_EXPLAIN_FILE}"

{
  echo "-- TASK-090: LIST BY IDS hydration"
  psql "${DSN}" <<'SQL'
EXPLAIN (ANALYZE, BUFFERS, WAL)
WITH sample_user AS (
  SELECT user_id
  FROM messages
  WHERE status = 'active'
  GROUP BY user_id
  ORDER BY MAX(created_at) DESC
  LIMIT 1
),
sample_ids AS (
  SELECT ARRAY(
    SELECT id
    FROM messages
    WHERE user_id = (SELECT user_id FROM sample_user)
      AND status = 'active'
    ORDER BY COALESCE(received_at, sent_at, draft_updated_at, created_at) DESC, id DESC
    LIMIT 200
  ) AS ids
),
requested AS (
  SELECT
    value AS id,
    ordinality
  FROM sample_ids,
       UNNEST(ids) WITH ORDINALITY AS requested(value, ordinality)
)
SELECT
  m.id::text,
  m.folder_id::text,
  m.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS message_at,
  m.size,
  m.has_attachment,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred
FROM requested
JOIN messages m
  ON m.id = requested.id::uuid
LEFT JOIN message_search_documents msd
  ON msd.message_id = m.id
 AND msd.user_id = m.user_id
WHERE m.user_id = (SELECT user_id FROM sample_user)
  AND m.status = 'active'
ORDER BY requested.ordinality;
SQL

  printf '\n-- TASK-090: LIST MESSAGES IN FOLDER\n'
  psql "${DSN}" <<'SQL'
WITH sample_user AS (
  SELECT user_id
  FROM messages
  WHERE status = 'active'
  GROUP BY user_id
  ORDER BY MAX(created_at) DESC
  LIMIT 1
),
sample_folder AS (
  SELECT folder_id
  FROM messages
  WHERE user_id = (SELECT user_id FROM sample_user)
    AND status = 'active'
    AND folder_id IS NOT NULL
  GROUP BY folder_id
  ORDER BY COUNT(*) DESC
  LIMIT 1
)
EXPLAIN (ANALYZE, BUFFERS, WAL)
SELECT
  m.id::text,
  m.folder_id::text,
  m.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS message_at,
  m.size,
  m.has_attachment,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred
FROM messages m
LEFT JOIN message_search_documents msd
  ON msd.message_id = m.id
 AND msd.user_id = m.user_id
WHERE m.user_id = (SELECT user_id FROM sample_user)
  AND m.folder_id = (SELECT folder_id FROM sample_folder)
  AND m.status = 'active'
ORDER BY COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) DESC, m.id DESC
LIMIT 200;
SQL

  printf '\n-- TASK-090: MESSAGE SEARCH PATH (sample query)\n'
  psql "${DSN}" <<'SQL'
WITH sample_user AS (
  SELECT user_id
  FROM messages
  WHERE status = 'active'
  GROUP BY user_id
  ORDER BY MAX(created_at) DESC
  LIMIT 1
),
search_input AS (
  SELECT plainto_tsquery('simple', 'test') AS tsq
),
query_matches AS (
  SELECT messages.id
  FROM messages
  CROSS JOIN search_input
  WHERE messages.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND to_tsvector('simple', coalesce(messages.subject, '') || ' ' || coalesce(messages.from_addr, '') || ' ' || coalesce(messages.from_name, '')) @@ search_input.tsq
  UNION
  SELECT messages.id
  FROM messages
  WHERE messages.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND messages.subject ILIKE '%%' || 'test' || '%%'
  UNION
  SELECT messages.id
  FROM messages
  WHERE messages.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND messages.from_addr ILIKE '%%' || 'test' || '%%'
  UNION
  SELECT messages.id
  FROM messages
  WHERE messages.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND messages.from_name ILIKE '%%' || 'test' || '%%'
  UNION
  SELECT msd.message_id AS id
  FROM message_search_documents msd
  JOIN messages
    ON messages.id = msd.message_id
   AND messages.user_id = msd.user_id
  CROSS JOIN search_input
  WHERE msd.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND to_tsvector('simple', msd.body_text) @@ search_input.tsq
  UNION
  SELECT msd.message_id AS id
  FROM message_search_documents msd
  JOIN messages
    ON messages.id = msd.message_id
   AND messages.user_id = msd.user_id
  WHERE msd.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND msd.body_text ILIKE '%%' || 'test' || '%%'
),
ranked_messages AS (
  SELECT
    messages.id::text AS id,
    messages.folder_id::text AS folder_id,
    messages.subject,
    left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
    messages.from_addr,
    messages.from_name,
    COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at) AS message_at,
    messages.size,
    messages.has_attachment,
    COALESCE((messages.flags->>'read')::boolean, false) AS read,
    COALESCE((messages.flags->>'starred')::boolean, false) AS starred,
    ts_rank_cd(
      setweight(to_tsvector('simple', coalesce(messages.subject, '')), 'A') ||
      setweight(to_tsvector('simple', coalesce(messages.from_addr, '')), 'A') ||
      setweight(to_tsvector('simple', coalesce(messages.from_name, '')), 'B') ||
      setweight(to_tsvector('simple', coalesce(msd.body_text, '')), 'D'),
      search_input.tsq
    ) AS search_rank
  FROM messages
  CROSS JOIN search_input
  LEFT JOIN message_search_documents msd
    ON msd.message_id = messages.id
   AND msd.user_id = messages.user_id
  WHERE messages.user_id = (SELECT user_id FROM sample_user)
    AND messages.status = 'active'
    AND messages.id IN (SELECT id FROM query_matches)
)
EXPLAIN (ANALYZE, BUFFERS, WAL)
SELECT
  id,
  folder_id,
  subject,
  preview,
  from_addr,
  from_name,
  message_at,
  size,
  has_attachment,
  read,
  starred,
  search_rank
FROM ranked_messages
ORDER BY message_at DESC, id DESC
LIMIT 200;
SQL
} | tee "${TMP_EXPLAIN_FILE}"

echo "Saved explain snapshots to ${TMP_EXPLAIN_FILE}"
