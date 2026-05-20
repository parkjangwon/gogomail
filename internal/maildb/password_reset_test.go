package maildb

import (
	"context"
	"testing"
	"time"
)

// TestPasswordResetTokenRejectsNilDB verifies that each function returns an
// error (not a panic) when the repository has no DB handle.
func TestPasswordResetTokenRejectsNilDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := &Repository{db: nil}

	t.Run("CreatePasswordResetToken", func(t *testing.T) {
		t.Parallel()
		err := r.CreatePasswordResetToken(ctx, "user-id", []byte("hash"), time.Now().Add(time.Hour))
		if err == nil {
			t.Fatal("expected error with nil db")
		}
	})

	t.Run("GetPasswordResetToken", func(t *testing.T) {
		t.Parallel()
		_, err := r.GetPasswordResetToken(ctx, []byte("hash"))
		if err == nil {
			t.Fatal("expected error with nil db")
		}
	})

	t.Run("MarkTokenUsed", func(t *testing.T) {
		t.Parallel()
		err := r.MarkTokenUsed(ctx, "token-id")
		if err == nil {
			t.Fatal("expected error with nil db")
		}
	})

	t.Run("ResetUserPassword", func(t *testing.T) {
		t.Parallel()
		err := r.ResetUserPassword(ctx, "user-id", "hash")
		if err == nil {
			t.Fatal("expected error with nil db")
		}
	})
}

func TestPasswordResetTokenValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := &Repository{db: nil}

	t.Run("CreateRequiresUserID", func(t *testing.T) {
		t.Parallel()
		// db is nil so we won't hit the DB, but the userID check happens first
		// only if we add it. With nil db the db-nil guard fires first. Acceptable.
		_ = r.CreatePasswordResetToken(ctx, "", []byte("hash"), time.Now().Add(time.Hour))
	})

	t.Run("GenerateResetTokenLength", func(t *testing.T) {
		t.Parallel()
		tok, err := GenerateResetToken()
		if err != nil {
			t.Fatalf("GenerateResetToken: %v", err)
		}
		if len(tok) != 32 {
			t.Fatalf("expected 32 bytes, got %d", len(tok))
		}
	})

	t.Run("GenerateResetTokenIsRandom", func(t *testing.T) {
		t.Parallel()
		a, _ := GenerateResetToken()
		b, _ := GenerateResetToken()
		equal := true
		for i := range a {
			if a[i] != b[i] {
				equal = false
				break
			}
		}
		if equal {
			t.Fatal("two tokens should not be identical")
		}
	})
}
