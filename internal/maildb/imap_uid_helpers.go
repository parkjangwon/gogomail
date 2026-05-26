package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

func imapUIDArray(uids []imapgw.UID) []string {
	if len(uids) == 0 {
		return nil
	}
	values := make([]string, 0, len(uids))
	for _, uid := range uids {
		values = append(values, strconv.FormatUint(uint64(uid), 10))
	}
	return values
}

func addIMAPUIDOffset(uid imapgw.UID, offset int64) imapgw.UID {
	if offset <= 0 {
		return uid
	}
	if uint64(uid)+uint64(offset) > uint64(^uint32(0)) {
		return imapgw.UID(^uint32(0))
	}
	return imapgw.UID(uint32(uid) + uint32(offset))
}

func addIMAPModSeqOffset(modseq uint64, offset int64) uint64 {
	if offset <= 0 {
		return modseq
	}
	if ^uint64(0)-modseq < uint64(offset) {
		return ^uint64(0)
	}
	return modseq + uint64(offset)
}

func boolPointer(value bool) *bool {
	return &value
}

func normalizeIMAPUIDBackfillLimit(limit int) int {
	if limit <= 0 {
		return imapUIDBackfillDefaultLimit
	}
	if limit > imapUIDBackfillMaxLimit {
		return imapUIDBackfillMaxLimit
	}
	return limit
}

func imapSequenceNumberForUID(ctx context.Context, querier imapSequenceQuerier, userID string, mailboxID string, uid imapgw.UID) (uint32, error) {
	const query = `
SELECT COUNT(*)
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid <= $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'`

	var count int64
	if err := querier.QueryRowContext(ctx, query, userID, mailboxID, int64(uid)).Scan(&count); err != nil {
		return 0, fmt.Errorf("get imap sequence number: %w", err)
	}
	if count <= 0 || count > math.MaxUint32 {
		return 0, fmt.Errorf("imap sequence number unavailable for uid %d", uid)
	}
	return uint32(count), nil
}

// imapSequenceNumbersForUIDs returns the IMAP sequence numbers for the given
// UIDs in a single query using a window function.  The returned map is keyed by
// UID (as int64).  All requested UIDs must exist in the mailbox; missing UIDs
// are reported via the absence of a map entry — callers must check `ok` when
// looking up values.
func imapSequenceNumbersForUIDs(ctx context.Context, querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID string, mailboxID string, uids []int64) (map[int64]uint32, error) {
	if len(uids) == 0 {
		return map[int64]uint32{}, nil
	}
	const query = `
SELECT uid, seq
FROM (
  SELECT
    i.uid,
    row_number() OVER (ORDER BY i.uid) AS seq
  FROM imap_message_uid i
  JOIN messages m ON m.id = i.message_id
  WHERE i.user_id = $1::uuid
    AND i.mailbox_id = $2::uuid
    AND m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
) ranked
WHERE uid = ANY($3::bigint[])`
	rows, err := querier.QueryContext(ctx, query, userID, mailboxID, pq.Array(uids))
	if err != nil {
		return nil, fmt.Errorf("get imap sequence numbers: %w", err)
	}
	defer rows.Close()
	out := make(map[int64]uint32, len(uids))
	for rows.Next() {
		var uid int64
		var seq int64
		if err := rows.Scan(&uid, &seq); err != nil {
			return nil, fmt.Errorf("scan imap sequence number: %w", err)
		}
		if seq <= 0 || seq > math.MaxUint32 {
			return nil, fmt.Errorf("imap sequence number unavailable for uid %d", uid)
		}
		out[uid] = uint32(seq)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap sequence numbers: %w", err)
	}
	return out, nil
}

func imapSequenceBaseForAfterUID(ctx context.Context, querier imapSequenceQuerier, userID string, mailboxID string, afterUID imapgw.UID) (uint32, error) {
	if afterUID == 0 {
		return 0, nil
	}
	const query = `
SELECT COUNT(*)
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid <= $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'`

	var count int64
	if err := querier.QueryRowContext(ctx, query, userID, mailboxID, int64(afterUID)).Scan(&count); err != nil {
		return 0, fmt.Errorf("get imap sequence base: %w", err)
	}
	if count < 0 || count > math.MaxUint32 {
		return 0, fmt.Errorf("imap sequence base unavailable after uid %d", afterUID)
	}
	return uint32(count), nil
}

func assignIMAPListSequenceNumbers(messages []imapgw.MessageSummary, base uint32) error {
	for i := range messages {
		if base > math.MaxUint32-uint32(i)-1 {
			return fmt.Errorf("imap sequence number overflow")
		}
		messages[i].SequenceNumber = base + uint32(i) + 1
	}
	return nil
}
