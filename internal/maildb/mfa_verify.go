package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gogomail/gogomail/internal/authmfa"
	"github.com/lib/pq"
)

// ErrMFANotEnrolled is returned when a user has no MFA secret row.
var ErrMFANotEnrolled = errors.New("mfa not enrolled")

// ErrMFAInvalidCode is returned when a TOTP or recovery code is invalid.
var ErrMFAInvalidCode = errors.New("invalid mfa code")

// ErrMFACodeAlreadyUsed is returned when a TOTP code was already used (replay attack).
var ErrMFACodeAlreadyUsed = errors.New("mfa code already used")

// GetMFASecret returns the TOTP secret and current recovery codes for a user.
// Returns ErrMFANotEnrolled if the user has no row in user_mfa_secrets.
func (r *Repository) GetMFASecret(ctx context.Context, userID string) (secret string, recoveryCodes []string, err error) {
	const q = `
		SELECT secret, recovery_codes
		FROM user_mfa_secrets
		WHERE user_id = $1::uuid AND enabled = true`
	err = r.db.QueryRowContext(ctx, q, userID).Scan(&secret, pq.Array(&recoveryCodes))
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrMFANotEnrolled
	}
	return secret, recoveryCodes, err
}

// GetPendingMFASecret returns the TOTP secret for a user whose MFA setup has been
// started but not yet confirmed (enabled=false). Returns ErrMFANotEnrolled if no row.
func (r *Repository) GetPendingMFASecret(ctx context.Context, userID string) (secret string, err error) {
	const q = `SELECT secret FROM user_mfa_secrets WHERE user_id = $1::uuid AND enabled = false`
	err = r.db.QueryRowContext(ctx, q, userID).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMFANotEnrolled
	}
	return secret, err
}

// VerifyAndRecordTOTP verifies a TOTP code against the user's secret and records
// it in totp_used_codes to prevent replay. Returns ErrMFAInvalidCode for a bad
// code, ErrMFACodeAlreadyUsed if the code was already used, ErrMFANotEnrolled
// if the user has no active MFA secret.
func (r *Repository) VerifyAndRecordTOTP(ctx context.Context, userID, secret, code string, now time.Time) error {
	if !authmfa.VerifyTOTP(secret, code, now) {
		return ErrMFAInvalidCode
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO totp_used_codes (user_id, code) VALUES ($1::uuid, $2)`,
		userID, code,
	)
	if err != nil {
		// Unique constraint violation means the code was already used.
		if isUniqueViolation(err) {
			return ErrMFACodeAlreadyUsed
		}
		return fmt.Errorf("record totp code: %w", err)
	}
	return nil
}

// VerifyAndConsumeRecoveryCode verifies a recovery code and atomically removes
// it from the user's stored codes. Returns ErrMFAInvalidCode if not found,
// ErrMFANotEnrolled if the user has no MFA row.
func (r *Repository) VerifyAndConsumeRecoveryCode(ctx context.Context, userID, code string) error {
	const q = `
		UPDATE user_mfa_secrets
		SET recovery_codes = array_remove(recovery_codes, $2),
		    updated_at     = now()
		WHERE user_id = $1::uuid
		  AND enabled  = true
		  AND $2 = ANY(recovery_codes)
		RETURNING user_id`

	var id string
	err := r.db.QueryRowContext(ctx, q, userID, code).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// Either the code wasn't found or the user isn't enrolled.
		status, serr := r.GetUserMFAStatus(ctx, userID)
		if serr != nil || !status.Enrolled {
			return ErrMFANotEnrolled
		}
		return ErrMFAInvalidCode
	}
	if err != nil {
		return fmt.Errorf("consume recovery code: %w", err)
	}
	return nil
}

// SetupMFASecret stores a new TOTP secret and recovery codes for a user.
// enabled stays false until ActivateMFA is called after the user confirms a code.
// If the user already has a row, it is replaced (idempotent setup restart).
func (r *Repository) SetupMFASecret(ctx context.Context, userID, secret string, recoveryCodes []string) error {
	const q = `
		INSERT INTO user_mfa_secrets (user_id, secret, recovery_codes, enabled)
		VALUES ($1::uuid, $2, $3, false)
		ON CONFLICT (user_id) DO UPDATE
		SET secret         = EXCLUDED.secret,
		    recovery_codes = EXCLUDED.recovery_codes,
		    enabled        = false,
		    updated_at     = now()`
	_, err := r.db.ExecContext(ctx, q, userID, secret, pq.Array(recoveryCodes))
	return err
}

// ActivateMFA marks the user's MFA as enabled. The secret row must already exist.
func (r *Repository) ActivateMFA(ctx context.Context, userID string) error {
	const q = `
		UPDATE user_mfa_secrets
		SET enabled    = true,
		    updated_at = now()
		WHERE user_id = $1::uuid`
	res, err := r.db.ExecContext(ctx, q, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrMFANotEnrolled
	}
	return nil
}

// DisableMFA sets enabled=false but keeps the secret so the user can re-activate
// without re-scanning the QR code.
func (r *Repository) DisableMFA(ctx context.Context, userID string) error {
	const q = `
		UPDATE user_mfa_secrets
		SET enabled    = false,
		    updated_at = now()
		WHERE user_id = $1::uuid`
	_, err := r.db.ExecContext(ctx, q, userID)
	return err
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
