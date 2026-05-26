package dm

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

func loadMembersForRooms(ctx context.Context, q roomMemberQuerier, rooms []Room) ([]Room, error) {
	if len(rooms) == 0 {
		return rooms, nil
	}
	roomIDs := make([]string, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}
	const query = `
SELECT p.room_id::text, u.id::text, d.company_id::text, u.domain_id::text, u.display_name, COALESCE(primary_addr.address, ''), COALESCE(u.settings->>'avatar_url', '')
FROM dm_participants p
JOIN users u ON u.id = p.user_id
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT ua.address
  FROM user_addresses ua
  WHERE ua.user_id = u.id
  ORDER BY ua.is_primary DESC, ua.address ASC
  LIMIT 1
) primary_addr ON true
WHERE p.room_id = ANY($1::uuid[])
ORDER BY p.joined_at ASC, u.display_name ASC`
	rows, err := q.QueryContext(ctx, query, pq.Array(roomIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	membersByRoom := make(map[string][]User)
	for rows.Next() {
		var roomID string
		var u User
		if err := rows.Scan(&roomID, &u.ID, &u.CompanyID, &u.DomainID, &u.DisplayName, &u.Email, &u.AvatarURL); err != nil {
			return nil, err
		}
		membersByRoom[roomID] = append(membersByRoom[roomID], u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range rooms {
		rooms[i].Members = membersByRoom[rooms[i].ID]
		if rooms[i].MemberCount == 0 {
			rooms[i].MemberCount = len(rooms[i].Members)
		}
	}
	return rooms, nil
}

func insertParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string, userIDs []string) error {
	for _, id := range cleanIDs(userIDs) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dm_participants (room_id, user_id) VALUES ($1, $2) ON CONFLICT (room_id, user_id) DO NOTHING`, roomID, id); err != nil {
			return err
		}
	}
	return nil
}

func ensureUsersInScope(ctx context.Context, tx *sql.Tx, principal Principal, userIDs []string) error {
	users, err := usersInScope(ctx, tx, principal, userIDs)
	if err != nil {
		return err
	}
	if len(users) != len(cleanIDs(userIDs)) {
		return fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	return nil
}

func usersInScope(ctx context.Context, tx *sql.Tx, principal Principal, userIDs []string) ([]User, error) {
	ids := cleanIDs(userIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	if err := validateUUIDs(ids); err != nil {
		return nil, err
	}
	const query = `
SELECT u.id::text, d.company_id::text, u.domain_id::text, u.display_name, COALESCE(primary_addr.address, ''), COALESCE(u.settings->>'avatar_url', '')
FROM users u
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT ua.address
  FROM user_addresses ua
  WHERE ua.user_id = u.id
  ORDER BY ua.is_primary DESC, ua.address ASC
  LIMIT 1
) primary_addr ON true
WHERE u.id = ANY($1::uuid[])
  AND u.domain_id = $2
  AND d.company_id = $3
  AND u.status = 'active'
  AND d.status = 'active'`
	rows, err := tx.QueryContext(ctx, query, pq.Array(ids), principal.DomainID, principal.CompanyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.CompanyID, &u.DomainID, &u.DisplayName, &u.Email, &u.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func usersInScopeDB(ctx context.Context, db *sql.DB, principal Principal, userIDs []string) ([]User, error) {
	ids := cleanIDs(userIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	if err := validateUUIDs(ids); err != nil {
		return nil, err
	}
	const query = `
SELECT u.id::text, d.company_id::text, u.domain_id::text, u.display_name, COALESCE(primary_addr.address, ''), COALESCE(u.settings->>'avatar_url', '')
FROM users u
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT ua.address
  FROM user_addresses ua
  WHERE ua.user_id = u.id
  ORDER BY ua.is_primary DESC, ua.address ASC
  LIMIT 1
) primary_addr ON true
WHERE u.id = ANY($1::uuid[])
  AND u.domain_id = $2
  AND d.company_id = $3
  AND u.status = 'active'
  AND d.status = 'active'`
	rows, err := db.QueryContext(ctx, query, pq.Array(ids), principal.DomainID, principal.CompanyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.CompanyID, &u.DomainID, &u.DisplayName, &u.Email, &u.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func ensureParticipantTx(ctx context.Context, tx *sql.Tx, principal Principal, roomID string) error {
	const query = `
SELECT 1
FROM dm_rooms r
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4`
	var one int
	if err := tx.QueryRowContext(ctx, query, principal.UserID, roomID, principal.CompanyID, principal.DomainID).Scan(&one); err != nil {
		return mapNoRows(err)
	}
	return nil
}

func ensureParticipantUserTx(ctx context.Context, tx *sql.Tx, roomID string, userID string) error {
	const query = `SELECT 1 FROM dm_participants WHERE room_id = $1 AND user_id = $2`
	var one int
	if err := tx.QueryRowContext(ctx, query, roomID, userID).Scan(&one); err != nil {
		return mapNoRows(err)
	}
	return nil
}

func countParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string, userIDs []string) (int, error) {
	ids := cleanIDs(userIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	const query = `SELECT COUNT(*) FROM dm_participants WHERE room_id = $1 AND user_id = ANY($2::uuid[])`
	var count int
	if err := tx.QueryRowContext(ctx, query, roomID, pq.Array(ids)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func countRoomParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string) (int, error) {
	const query = `SELECT COUNT(*) FROM dm_participants WHERE room_id = $1`
	var count int
	if err := tx.QueryRowContext(ctx, query, roomID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func hardDeleteRoomTx(ctx context.Context, tx *sql.Tx, roomID string) error {
	for _, stmt := range hardDeleteRoomStatements() {
		if _, err := tx.ExecContext(ctx, stmt, roomID); err != nil {
			return err
		}
	}
	return nil
}

func hardDeleteRoomStatements() []string {
	return []string{
		`DELETE FROM dm_room_keys WHERE room_id = $1`,
		`DELETE FROM dm_messages WHERE room_id = $1`,
		`DELETE FROM dm_participants WHERE room_id = $1`,
		`DELETE FROM dm_invites WHERE room_id = $1`,
		`DELETE FROM dm_rooms WHERE id = $1`,
	}
}
