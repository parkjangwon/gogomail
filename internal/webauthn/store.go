package webauthn

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Store manages WebAuthn credentials and challenges in Postgres.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store backed by the given *sql.DB.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Credential holds the stored information for a single WebAuthn credential.
type Credential struct {
	ID           string     // UUID primary key
	UserID       string     // UUID of the owning user
	CredentialID []byte     // WebAuthn credential ID (raw bytes)
	PublicKey    []byte     // CBOR-encoded COSE public key
	AAGUID       string     // Authenticator AAGUID (UUID string or empty)
	SignCount    uint32     // Monotonic counter from the authenticator
	Name         string     // User-assigned friendly name
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

// SaveCredential persists a newly registered credential.
func (s *Store) SaveCredential(ctx context.Context, userID string, cred *Credential) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webauthn_credentials
			(user_id, credential_id, public_key, aaguid, sign_count, name)
		VALUES ($1, $2, $3, $4::uuid, $5, $6)`,
		userID,
		cred.CredentialID,
		cred.PublicKey,
		nullableUUID(cred.AAGUID),
		cred.SignCount,
		cred.Name,
	)
	return err
}

// GetCredentials returns all credentials for a user.
func (s *Store) GetCredentials(ctx context.Context, userID string) ([]Credential, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, credential_id, public_key,
		       COALESCE(aaguid::text, '') AS aaguid,
		       sign_count, name, created_at, last_used_at
		  FROM webauthn_credentials
		 WHERE user_id = $1
		 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Credential
	for rows.Next() {
		var c Credential
		if err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.CredentialID,
			&c.PublicKey,
			&c.AAGUID,
			&c.SignCount,
			&c.Name,
			&c.CreatedAt,
			&c.LastUsedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateSignCount updates the sign count and last_used_at after a successful assertion.
func (s *Store) UpdateSignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE webauthn_credentials
		   SET sign_count = $1, last_used_at = now()
		 WHERE credential_id = $2`,
		signCount,
		credentialID,
	)
	return err
}

// DeleteCredential removes a credential by its UUID for a specific user.
func (s *Store) DeleteCredential(ctx context.Context, userID string, credentialID string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM webauthn_credentials
		 WHERE user_id = $1 AND id = $2`,
		userID,
		credentialID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("credential not found")
	}
	return nil
}

// SaveChallenge stores a challenge for the given user+flow, replacing any existing one.
func (s *Store) SaveChallenge(ctx context.Context, userID, flow string, challenge []byte) error {
	// Delete any stale challenge for this user+flow first (single active challenge per flow).
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM webauthn_challenges WHERE user_id = $1 AND flow = $2`,
		userID, flow,
	)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO webauthn_challenges (user_id, challenge, flow)
		VALUES ($1, $2, $3)`,
		userID, challenge, flow,
	)
	return err
}

// GetAndDeleteChallenge retrieves and atomically deletes a challenge (replay protection).
// Returns an error if not found or expired.
func (s *Store) GetAndDeleteChallenge(ctx context.Context, userID, flow string) ([]byte, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var challenge []byte
	err = tx.QueryRowContext(ctx, `
		DELETE FROM webauthn_challenges
		 WHERE user_id = $1 AND flow = $2 AND expires_at > now()
		RETURNING challenge`,
		userID, flow,
	).Scan(&challenge)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("challenge not found or expired")
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return challenge, nil
}

// PruneExpiredChallenges deletes expired challenges (call periodically).
func (s *Store) PruneExpiredChallenges(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webauthn_challenges WHERE expires_at <= now()`)
	return err
}

// nullableUUID returns nil when s is empty so the driver stores a SQL NULL.
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}
