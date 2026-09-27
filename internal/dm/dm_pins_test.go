package dm

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServicePinMessageDelegatesWithNote(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	svc := NewService(store, crypto)
	if err := svc.PinMessage(context.Background(), testPrincipal(), "room-1", "msg-1", "important"); err != nil {
		t.Fatalf("PinMessage: %v", err)
	}
	if len(store.pinCalls) != 1 {
		t.Fatalf("pin calls = %d, want 1", len(store.pinCalls))
	}
	if store.pinCalls[0].roomID != "room-1" || store.pinCalls[0].messageID != "msg-1" || store.pinCalls[0].note != "important" {
		t.Fatalf("pin call = %+v", store.pinCalls[0])
	}
}

func TestServicePinMessageRejectsMissingIDs(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	svc := NewService(store, crypto)
	if err := svc.PinMessage(context.Background(), testPrincipal(), "", "msg-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty room err = %v, want ErrInvalid", err)
	}
	if err := svc.PinMessage(context.Background(), testPrincipal(), "room-1", "", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty message err = %v, want ErrInvalid", err)
	}
	if len(store.pinCalls) != 0 {
		t.Fatalf("store should not be called on validation failure, calls = %d", len(store.pinCalls))
	}
}

func TestServicePinMessageRejectsOversizeNote(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	svc := NewService(store, crypto)
	note := strings.Repeat("x", MaxPinNoteBytes+1)
	if err := svc.PinMessage(context.Background(), testPrincipal(), "room-1", "msg-1", note); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversize note err = %v, want ErrInvalid", err)
	}
	if len(store.pinCalls) != 0 {
		t.Fatalf("store should not be called on oversize note")
	}
}

func TestServicePinMessagePropagatesStoreError(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	// Simulate the store rejecting a non-member or cross-room message.
	store := &fakeStore{wrappedRoomKey: wrappedKey, pinErr: ErrForbidden}
	svc := NewService(store, crypto)
	if err := svc.PinMessage(context.Background(), testPrincipal(), "room-1", "msg-1", ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestServiceUnpinMessageDelegates(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	svc := NewService(store, crypto)
	if err := svc.UnpinMessage(context.Background(), testPrincipal(), "room-1", "msg-1"); err != nil {
		t.Fatalf("UnpinMessage: %v", err)
	}
	if len(store.unpinCalls) != 1 || store.unpinCalls[0].messageID != "msg-1" {
		t.Fatalf("unpin calls = %+v", store.unpinCalls)
	}
}

func TestServiceListPinnedMessagesDecryptsAndPreservesOrder(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	roomKey, err := crypto.UnwrapRoomKey(wrappedKey)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	ct1, _ := crypto.EncryptBody(roomKey, []byte("first pinned"))
	ct2, _ := crypto.EncryptBody(roomKey, []byte("second pinned"))
	now := time.Now().UTC()
	store := &fakeStore{
		wrappedRoomKey: wrappedKey,
		// Store returns newest pins first (as the SQL ORDER BY guarantees).
		pinnedRecords: []PinnedRecord{
			{
				Record:         MessageRecord{Message: Message{ID: "msg-2", RoomID: "room-1", MessageType: MessageTypeText, CreatedAt: now}, BodyCiphertext: ct2},
				PinnedByUserID: "user-9",
				PinnedAt:       now.Add(2 * time.Minute),
				Note:           "later",
			},
			{
				Record:         MessageRecord{Message: Message{ID: "msg-1", RoomID: "room-1", MessageType: MessageTypeText, CreatedAt: now}, BodyCiphertext: ct1},
				PinnedByUserID: "user-1",
				PinnedAt:       now.Add(1 * time.Minute),
			},
		},
	}
	svc := NewService(store, crypto)
	pins, err := svc.ListPinnedMessages(context.Background(), testPrincipal(), "room-1", 25, 5)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pins) != 2 {
		t.Fatalf("pins = %d, want 2", len(pins))
	}
	if pins[0].Message.ID != "msg-2" || pins[0].Message.Body != "second pinned" {
		t.Fatalf("pins[0] = %+v", pins[0].Message)
	}
	if pins[0].PinnedByUserID != "user-9" || pins[0].Note != "later" {
		t.Fatalf("pins[0] metadata = %+v", pins[0])
	}
	if pins[1].Message.ID != "msg-1" || pins[1].Message.Body != "first pinned" {
		t.Fatalf("pins[1] = %+v", pins[1].Message)
	}
	// Verify limit/offset are passed through to the store.
	if store.listPinsLimit != 25 || store.listPinsOffset != 5 {
		t.Fatalf("limit/offset = %d/%d, want 25/5", store.listPinsLimit, store.listPinsOffset)
	}
}

func TestServiceListPinnedMessagesRendersDeletedPlaceholder(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	del := time.Now().UTC()
	store := &fakeStore{
		wrappedRoomKey: wrappedKey,
		pinnedRecords: []PinnedRecord{
			{
				Record:         MessageRecord{Message: Message{ID: "msg-1", RoomID: "room-1", MessageType: MessageTypeText, DeletedAt: &del}},
				PinnedByUserID: "user-1",
				PinnedAt:       del,
			},
		},
	}
	svc := NewService(store, crypto)
	pins, err := svc.ListPinnedMessages(context.Background(), testPrincipal(), "room-1", 50, 0)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pins) != 1 {
		t.Fatalf("pins = %d, want 1", len(pins))
	}
	if pins[0].Message.Body != DefaultSystemMessages().MessageDeleted {
		t.Fatalf("deleted body = %q, want placeholder", pins[0].Message.Body)
	}
}

// TestPinNoteNeverContainsCiphertext is a guard: the pin note is plaintext
// metadata and must never carry encrypted message bodies.
func TestPinNoteNeverContainsCiphertext(t *testing.T) {
	crypto, wrappedKey := testCryptoAndWrappedRoomKey(t)
	roomKey, _ := crypto.UnwrapRoomKey(wrappedKey)
	ct, _ := crypto.EncryptBody(roomKey, []byte("secret body"))
	store := &fakeStore{wrappedRoomKey: wrappedKey}
	svc := NewService(store, crypto)
	if err := svc.PinMessage(context.Background(), testPrincipal(), "room-1", "msg-1", "just a note"); err != nil {
		t.Fatalf("PinMessage: %v", err)
	}
	if bytes.Contains([]byte(store.pinCalls[0].note), ct) {
		t.Fatal("pin note must not contain message ciphertext")
	}
}
