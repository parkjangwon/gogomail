package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/imapimport"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/storage"
)

// maildbSink implements imapimport.MessageSink by reusing gogomail's own IMAP
// APPEND path: it stores the raw message to object storage and inserts the
// metadata row via maildb.AppendStoredIMAPMessage. Folder creation reuses the
// mail folder model and is memoized. Cross-run idempotency is enforced by a
// direct Message-ID existence check against the target user's messages.
type maildbSink struct {
	repo   *maildb.Repository
	store  storage.Store
	userID string

	mu           sync.Mutex
	folderCache  map[string]maildb.IMAPAppendTarget // destination name → resolved target
	seededDedupe bool
}

func newMaildbSink(repo *maildb.Repository, store storage.Store, userID string) *maildbSink {
	return &maildbSink{
		repo:        repo,
		store:       store,
		userID:      userID,
		folderCache: map[string]maildb.IMAPAppendTarget{},
	}
}

// EnsureFolder creates the destination folder if it does not already exist and
// caches the resolved append target.
func (s *maildbSink) EnsureFolder(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("destination folder name is required")
	}
	s.mu.Lock()
	_, cached := s.folderCache[name]
	s.mu.Unlock()
	if cached {
		return nil
	}

	// INBOX and other existing folders resolve directly; create only when the
	// resolve fails because the folder is absent.
	target, err := s.repo.ResolveIMAPAppendTarget(ctx, s.userID, name)
	if err != nil {
		if _, createErr := s.repo.CreateFolder(ctx, maildb.CreateFolderRequest{UserID: s.userID, Name: name}); createErr != nil {
			// A concurrent worker may have created it; retry resolve.
			if target, err = s.repo.ResolveIMAPAppendTarget(ctx, s.userID, name); err != nil {
				return fmt.Errorf("create/resolve folder %q: %w (create: %v)", name, err, createErr)
			}
		} else if target, err = s.repo.ResolveIMAPAppendTarget(ctx, s.userID, name); err != nil {
			return fmt.Errorf("resolve created folder %q: %w", name, err)
		}
	}

	s.mu.Lock()
	s.folderCache[name] = target
	s.mu.Unlock()
	return nil
}

// Store persists a single imported message.
func (s *maildbSink) Store(ctx context.Context, msg imapimport.StoredMessage) error {
	s.mu.Lock()
	target, ok := s.folderCache[msg.Destination]
	s.mu.Unlock()
	if !ok {
		if err := s.EnsureFolder(ctx, msg.Destination); err != nil {
			return err
		}
		s.mu.Lock()
		target = s.folderCache[msg.Destination]
		s.mu.Unlock()
	}

	// Cross-run idempotency: skip when a message with this RFC Message-ID
	// already exists for the user (covers resuming after a crash where the
	// progress file lagged behind committed inserts).
	if key := msg.IdempotencyKey; key != "" && !strings.HasPrefix(key, "sha256:") {
		exists, err := s.messageIDExists(ctx, key)
		if err == nil && exists {
			return nil
		}
	}

	parsed, err := message.ParseEML(bytes.NewReader(msg.Raw))
	if err != nil {
		return fmt.Errorf("parse imported message: %w", err)
	}

	internalDate := msg.InternalDate
	if internalDate.IsZero() {
		internalDate = time.Now().UTC()
	}
	path := buildImportStoragePath(target, randomObjectID(), internalDate)
	if err := s.store.Put(ctx, path, bytes.NewReader(msg.Raw)); err != nil {
		return fmt.Errorf("store imported message body: %w", err)
	}

	if _, err := s.repo.AppendStoredIMAPMessage(ctx, maildb.AppendStoredIMAPMessageRequest{
		Target:       target,
		StoragePath:  path,
		Parsed:       parsed,
		Flags:        msg.Flags,
		InternalDate: internalDate,
		Size:         int64(len(msg.Raw)),
	}); err != nil {
		// Best-effort cleanup of the orphaned object.
		_ = s.store.Delete(context.Background(), path)
		return fmt.Errorf("append imported message metadata: %w", err)
	}
	return nil
}

// messageIDExists reports whether the target user already has a message with the
// given RFC Message-ID (with or without surrounding angle brackets).
func (s *maildbSink) messageIDExists(ctx context.Context, messageID string) (bool, error) {
	db := s.repo.DB()
	if db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	bare := strings.TrimSpace(messageID)
	angled := "<" + bare + ">"
	const query = `
SELECT EXISTS (
  SELECT 1 FROM messages
  WHERE user_id = $1::uuid
    AND status = 'active'
    AND (rfc_message_id = $2 OR rfc_message_id = $3)
)`
	var exists bool
	if err := db.QueryRowContext(ctx, query, s.userID, bare, angled).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func buildImportStoragePath(target maildb.IMAPAppendTarget, objectID string, internalDate time.Time) string {
	return strings.Join([]string{
		"mailstore",
		sanitizeImportPathSegment(target.CompanyID),
		sanitizeImportPathSegment(target.DomainID),
		sanitizeImportPathSegment(target.UserID),
		"imap-import",
		internalDate.UTC().Format("2006"),
		internalDate.UTC().Format("01"),
		sanitizeImportPathSegment(objectID) + ".eml",
	}, "/")
}

func sanitizeImportPathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func randomObjectID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
