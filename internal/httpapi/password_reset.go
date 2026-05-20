package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
)

// PasswordResetTokenRecord holds the fields returned by GetPasswordResetToken
// in the PasswordResetStore interface.
type PasswordResetTokenRecord struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// PasswordResetStore is satisfied by MaildbPasswordResetAdapter and any test double.
type PasswordResetStore interface {
	GetUserIDByEmail(ctx context.Context, email string) (userID string, primaryEmail string, err error)
	CreatePasswordResetToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, tokenHash []byte) (*PasswordResetTokenRecord, error)
	MarkTokenUsed(ctx context.Context, tokenID string) error
	ResetUserPassword(ctx context.Context, userID, newPasswordHash string) error
}

// MaildbPasswordResetAdapter adapts *maildb.Repository to PasswordResetStore.
type MaildbPasswordResetAdapter struct {
	r *maildb.Repository
}

// NewMaildbPasswordResetAdapter wraps a repository for use as a PasswordResetStore.
func NewMaildbPasswordResetAdapter(r *maildb.Repository) *MaildbPasswordResetAdapter {
	return &MaildbPasswordResetAdapter{r: r}
}

func (a *MaildbPasswordResetAdapter) GetUserIDByEmail(ctx context.Context, email string) (string, string, error) {
	info, err := a.r.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", err
	}
	return info.UserID, info.Email, nil
}

func (a *MaildbPasswordResetAdapter) CreatePasswordResetToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	return a.r.CreatePasswordResetToken(ctx, userID, tokenHash, expiresAt)
}

func (a *MaildbPasswordResetAdapter) GetPasswordResetToken(ctx context.Context, tokenHash []byte) (*PasswordResetTokenRecord, error) {
	t, err := a.r.GetPasswordResetToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	return &PasswordResetTokenRecord{
		ID:        t.ID,
		UserID:    t.UserID,
		ExpiresAt: t.ExpiresAt,
		UsedAt:    t.UsedAt,
	}, nil
}

func (a *MaildbPasswordResetAdapter) MarkTokenUsed(ctx context.Context, tokenID string) error {
	return a.r.MarkTokenUsed(ctx, tokenID)
}

func (a *MaildbPasswordResetAdapter) ResetUserPassword(ctx context.Context, userID, newPasswordHash string) error {
	return a.r.ResetUserPassword(ctx, userID, newPasswordHash)
}

// RegisterPasswordResetRoutes wires up the two password-reset endpoints onto mux.
//
//   - store must implement PasswordResetStore.
//   - emailSender sends the reset link; may be nil (skips email, logs instead).
//   - baseURL is prepended to the reset token path, e.g. "https://mail.example.com".
func RegisterPasswordResetRoutes(
	mux *http.ServeMux,
	store PasswordResetStore,
	emailSender mailservice.SystemEmailSender,
	baseURL string,
) {
	baseURL = strings.TrimRight(baseURL, "/")
	// 5 requests per IP per 15 minutes to prevent token exhaustion attacks.
	limiter := NewAdminIPRateLimiter(5, 15*time.Minute)

	// POST /api/v1/auth/password-reset/request
	//   Body: {"email": "user@domain.com"}
	//   Always returns 200 to prevent user-enumeration.
	mux.HandleFunc("POST /api/v1/auth/password-reset/request", func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req struct {
			Email string `json:"email"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}

		// Issue token in a goroutine so response is always prompt and
		// timing-uniform (no enumeration via response latency). Use a bounded
		// background context so client disconnects do not interrupt token
		// persistence or best-effort email dispatch after the request is
		// accepted.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := issuePasswordResetToken(ctx, store, emailSender, baseURL, req.Email); err != nil {
				slog.Info("password reset token issue failed", "err", err)
			}
		}()

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// POST /api/v1/auth/password-reset/confirm
	//   Body: {"token": "<hex-encoded raw token>", "new_password": "..."}
	mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.Token = strings.TrimSpace(req.Token)
		req.NewPassword = strings.TrimSpace(req.NewPassword)
		if req.Token == "" || req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "token and new_password are required")
			return
		}
		if len(req.NewPassword) < 8 {
			writeError(w, http.StatusBadRequest, "new_password must be at least 8 characters")
			return
		}

		rawToken, err := hex.DecodeString(req.Token)
		if err != nil || len(rawToken) == 0 {
			writeError(w, http.StatusBadRequest, "invalid token format")
			return
		}

		hash := sha256.Sum256(rawToken)
		record, err := store.GetPasswordResetToken(r.Context(), hash[:])
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid or expired token")
			return
		}
		if record.UsedAt != nil {
			writeError(w, http.StatusBadRequest, "token has already been used")
			return
		}
		if time.Now().UTC().After(record.ExpiresAt) {
			writeError(w, http.StatusBadRequest, "token has expired")
			return
		}

		// Hash the new password with PBKDF2.
		var salt [32]byte
		if _, err := rand.Read(salt[:]); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate password salt")
			return
		}
		newHash, err := auth.HashPasswordPBKDF2SHA256(req.NewPassword, salt[:], 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to hash new password")
			return
		}

		// Persist new password (bumps session_version → invalidates all JWTs).
		if err := store.ResetUserPassword(r.Context(), record.UserID, newHash); err != nil {
			slog.Error("reset user password failed", "user_id", record.UserID, "err", err)
			writeError(w, http.StatusInternalServerError, "failed to update password")
			return
		}

		// Mark token consumed (best-effort; password already changed).
		if err := store.MarkTokenUsed(r.Context(), record.ID); err != nil {
			slog.Warn("mark password reset token used failed", "token_id", record.ID, "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
}

// issuePasswordResetToken runs in a background goroutine.
func issuePasswordResetToken(
	ctx context.Context,
	store PasswordResetStore,
	emailSender mailservice.SystemEmailSender,
	baseURL string,
	email string,
) error {
	userID, primaryEmail, err := store.GetUserIDByEmail(ctx, email)
	if err != nil {
		slog.Debug("password reset for unknown email", "err", err)
		return nil
	}

	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	hash := sha256.Sum256(rawToken)
	expiresAt := time.Now().UTC().Add(time.Hour)

	if err := store.CreatePasswordResetToken(ctx, userID, hash[:], expiresAt); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}

	resetURL := baseURL + "/auth/password-reset?token=" + hex.EncodeToString(rawToken)

	if emailSender != nil {
		if err := emailSender.SendPasswordReset(ctx, primaryEmail, resetURL); err != nil {
			slog.Warn("failed to send password reset email", "email", primaryEmail, "err", err)
		}
	}

	return nil
}
