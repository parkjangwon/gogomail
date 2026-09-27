package dm

import (
	"context"
	"fmt"
	"strings"
)

// MaxPinNoteBytes bounds the optional human-readable note attached to a pin.
const MaxPinNoteBytes = 500

// PinMessage pins a message in a room for quick access. The principal must be a
// member of the room and the message must belong to it. Pinning is idempotent.
// The optional note is a human-readable reason stored with the pin; it never
// contains message plaintext.
func (s *Service) PinMessage(ctx context.Context, principal Principal, roomID string, messageID string, note string) error {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	roomID = strings.TrimSpace(roomID)
	messageID = strings.TrimSpace(messageID)
	if roomID == "" || messageID == "" {
		return fmt.Errorf("%w: room_id and message_id are required", ErrInvalid)
	}
	note = strings.TrimSpace(note)
	if len([]byte(note)) > MaxPinNoteBytes {
		return fmt.Errorf("%w: note is too long", ErrInvalid)
	}
	return s.store.PinMessage(ctx, principal, roomID, messageID, note)
}

// UnpinMessage removes a pin. The principal must be a room member. Unpinning a
// message that is not pinned is a no-op success.
func (s *Service) UnpinMessage(ctx context.Context, principal Principal, roomID string, messageID string) error {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	roomID = strings.TrimSpace(roomID)
	messageID = strings.TrimSpace(messageID)
	if roomID == "" || messageID == "" {
		return fmt.Errorf("%w: room_id and message_id are required", ErrInvalid)
	}
	return s.store.UnpinMessage(ctx, principal, roomID, messageID)
}

// ListPinnedMessages returns pinned messages for a room, newest pins first,
// with their bodies decrypted. The principal must be a room member.
func (s *Service) ListPinnedMessages(ctx context.Context, principal Principal, roomID string, limit int, offset int) ([]PinnedMessage, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, fmt.Errorf("%w: room_id is required", ErrInvalid)
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	roomKey, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(roomKey)

	records, err := s.store.ListPinnedMessages(ctx, principal, roomID, limit, offset)
	if err != nil {
		return nil, err
	}

	out := make([]PinnedMessage, 0, len(records))
	for _, pr := range records {
		decrypted, err := s.decryptRecords(roomKey, []MessageRecord{pr.Record})
		if err != nil {
			return nil, err
		}
		out = append(out, PinnedMessage{
			Message:        decrypted[0],
			PinnedByUserID: pr.PinnedByUserID,
			PinnedAt:       pr.PinnedAt,
			Note:           pr.Note,
		})
	}
	return out, nil
}
