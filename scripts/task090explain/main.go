package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func must(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func runExplain(ctx context.Context, conn *pgx.Conn, query string, out io.Writer) error {
	rows, err := conn.Query(ctx, query)
	must(err, "querying explain")
	defer rows.Close()

	for rows.Next() {
		var line string
		must(rows.Scan(&line), "scanning explain row")
		fmt.Fprintln(out, line)
	}
	must(rows.Err(), "reading explain rows")
	return nil
}

func main() {
	dsn := flag.String("dsn", "", "PostgreSQL DSN URL")
	outPath := flag.String("out", "", "output file")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("dsn is required: set --dsn")
	}

	var out io.Writer = os.Stdout
	if *outPath == "" {
		*outPath = fmt.Sprintf("/tmp/task090-explain-%s.log", time.Now().Format("20060102-150405"))
	}

	f, err := os.Create(*outPath)
	must(err, "opening output file")
	defer f.Close()

	multi := io.MultiWriter(os.Stdout, f)
	out = multi

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, *dsn)
	must(err, "connecting to database")
	defer conn.Close(ctx)

	var hasTarget bool
	err = conn.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM messages
  WHERE status = 'active' AND user_id IS NOT NULL
  LIMIT 1
);
`).Scan(&hasTarget)
	must(err, "checking messages")
	if !hasTarget {
		fmt.Fprintln(out, "No active messages found; skip EXPLAIN ANALYZE baseline for TASK-090.")
		return
	}

	fmt.Fprintf(out, "[TASK-090] EXPLAIN ANALYZE snapshots -> %s\n", *outPath)

	queries := []struct {
		label string
		sql   string
	}{
		{
			label: "-- TASK-090: LIST BY IDS hydration",
			sql: `
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
`,
		},
		{
			label: "\n-- TASK-090: LIST MESSAGES IN FOLDER",
			sql: `
EXPLAIN (ANALYZE, BUFFERS, WAL)
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
`,
		},
		{
			label: "\n-- TASK-090: MESSAGE SEARCH PATH (sample query)",
			sql: `
EXPLAIN (ANALYZE, BUFFERS, WAL)
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
`,
		},
	}

	for _, q := range queries {
		fmt.Fprintln(out, q.label)
		must(runExplain(ctx, conn, q.sql, out), "running query")
	}

	fmt.Fprintf(out, "Saved explain snapshots to %s\n", *outPath)
}
