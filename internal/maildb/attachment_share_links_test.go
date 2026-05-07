package maildb

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAttachmentShareLinks(t *testing.T) {
	db := openMigratedPostgresTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	seed := seedPostgresMailUser(t, db)
	userID := seed.userID

	// 1. Create an attachment to share
	uploadID := "test-upload-1"
	attachmentID := uuid.New().String()
	_, err := db.Exec(`
INSERT INTO attachments (id, user_id, upload_id, storage_path, filename, size, mime_type, status)
VALUES ($1, $2, $3, 'path/to/file', 'large.zip', 1048576, 'application/zip', 'uploading')`,
		attachmentID, userID, uploadID)
	if err != nil {
		t.Fatalf("failed to create test attachment: %v", err)
	}

	// 2. Create a share link
	req := CreateAttachmentShareLinkRequest{
		UserID:       userID,
		AttachmentID: attachmentID,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	link, err := repo.CreateAttachmentShareLink(ctx, req)
	if err != nil {
		t.Fatalf("CreateAttachmentShareLink failed: %v", err)
	}

	if link.Token == "" {
		t.Errorf("expected non-empty token")
	}
	if link.AttachmentID != attachmentID {
		t.Errorf("got attachment_id %q, want %q", link.AttachmentID, attachmentID)
	}

	// 3. Resolve the share link
	resolved, err := repo.ResolveAttachmentShareLink(ctx, ResolveAttachmentShareLinkRequest{
		Token: link.Token,
	})
	if err != nil {
		t.Fatalf("ResolveAttachmentShareLink failed: %v", err)
	}

	if resolved.Attachment.ID != attachmentID {
		t.Errorf("resolved attachment id %q, want %q", resolved.Attachment.ID, attachmentID)
	}
	if resolved.Attachment.Filename != "large.zip" {
		t.Errorf("resolved filename %q, want %q", resolved.Attachment.Filename, "large.zip")
	}

	// 4. Try resolving with invalid token
	_, err = repo.ResolveAttachmentShareLink(ctx, ResolveAttachmentShareLinkRequest{
		Token: strings.Repeat("a", 32),
	})
	if err == nil {
		t.Errorf("expected error for invalid token")
	}

	// 5. Revoke (manual DB update for now since I haven't implemented the repo method yet)
	_, err = db.Exec("UPDATE attachment_share_links SET status = 'revoked' WHERE id = $1", link.ID)
	if err != nil {
		t.Fatalf("failed to revoke link: %v", err)
	}

	_, err = repo.ResolveAttachmentShareLink(ctx, ResolveAttachmentShareLinkRequest{
		Token: link.Token,
	})
	if err == nil {
		t.Errorf("expected error for revoked link")
	}
}
