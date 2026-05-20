package maildb

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListAttachmentShareLinksQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	query, args := buildListAttachmentShareLinksQuery(ListAttachmentShareLinksRequest{
		UserID:       " user-1 ",
		AttachmentID: " attachment-1 ",
		Status:       " active ",
		Limit:        25,
	})
	for _, want := range []string{
		"FROM attachment_share_links",
		"WHERE user_id = $1::uuid",
		"AND attachment_id = $2::uuid",
		"AND status = $3",
		"ORDER BY updated_at DESC, id DESC",
		"LIMIT $4",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list attachment share links query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$2 = '' OR",
		"$3 = '' OR",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list attachment share links query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args len = %d, want 4", len(args))
	}
	if args[0] != "user-1" || args[1] != "attachment-1" || args[2] != "active" || args[3] != 25 {
		t.Fatalf("args = %#v", args)
	}

	query, args = buildListAttachmentShareLinksQuery(ListAttachmentShareLinksRequest{
		UserID: "user-1",
		Limit:  50,
	})
	for _, unexpected := range []string{
		"attachment_id = $",
		"status = $",
		"$2 = '' OR",
	} {
		if strings.Contains(query, unexpected) {
			t.Fatalf("unfiltered list attachment share links query unexpectedly includes %q:\n%s", unexpected, query)
		}
	}
	if len(args) != 2 {
		t.Fatalf("unfiltered args len = %d, want 2", len(args))
	}
	if args[0] != "user-1" || args[1] != 50 {
		t.Fatalf("unfiltered args = %#v", args)
	}
}

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
