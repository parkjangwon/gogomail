package dm

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) AddMembers(ctx context.Context, principal Principal, roomID string, userIDs []string) ([]Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	roomID = strings.TrimSpace(roomID)
	userIDs = cleanIDs(userIDs)
	if roomID == "" || len(userIDs) == 0 {
		return nil, fmt.Errorf("%w: room_id and user_ids are required", ErrInvalid)
	}
	key, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, userIDs)
	if err != nil {
		return nil, err
	}
	if len(users) != len(userIDs) {
		return nil, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessages, err := s.memberSystemMessages(key, roomID, users, s.messages.MemberInvited)
	if err != nil {
		return nil, err
	}
	records, err := s.store.AddMembers(ctx, principal, roomID, userIDs, systemMessages)
	if err != nil {
		return nil, err
	}
	return s.decryptRecords(key, records)
}

func (s *Service) RemoveMember(ctx context.Context, principal Principal, roomID string, userID string) (RoomRemoval, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return RoomRemoval{}, err
	}
	roomID = strings.TrimSpace(roomID)
	userID = strings.TrimSpace(userID)
	if roomID == "" || userID == "" {
		return RoomRemoval{}, fmt.Errorf("%w: room_id and user_id are required", ErrInvalid)
	}
	key, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return RoomRemoval{}, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, []string{userID})
	if err != nil {
		return RoomRemoval{}, err
	}
	if len(users) != 1 {
		return RoomRemoval{}, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.MemberLeft, displayName(users[0])))
	if err != nil {
		return RoomRemoval{}, err
	}
	removal, err := s.store.RemoveMember(ctx, principal, roomID, userID, systemMessage)
	if err != nil || removal.DeletedRoom {
		return removal, err
	}
	messages, err := s.decryptRecords(key, []MessageRecord{removal.systemMessageRecord})
	if err != nil {
		return RoomRemoval{}, err
	}
	removal.SystemMessage = messages[0]
	return removal, nil
}

func (s *Service) TransferOwner(ctx context.Context, principal Principal, roomID string, userID string) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	roomID = strings.TrimSpace(roomID)
	userID = strings.TrimSpace(userID)
	if roomID == "" || userID == "" {
		return Message{}, fmt.Errorf("%w: room_id and user_id are required", ErrInvalid)
	}
	key, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, []string{userID})
	if err != nil {
		return Message{}, err
	}
	if len(users) != 1 {
		return Message{}, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.OwnerTransferred, displayName(users[0])))
	if err != nil {
		return Message{}, err
	}
	record, err := s.store.TransferOwner(ctx, principal, roomID, userID, systemMessage)
	if err != nil {
		return Message{}, err
	}
	msgs, err := s.decryptRecords(key, []MessageRecord{record})
	if err != nil {
		return Message{}, err
	}
	return msgs[0], nil
}

func (s *Service) CreateInvite(ctx context.Context, principal Principal, roomID string) (Invite, error) {
	return s.store.CreateInvite(ctx, normalizePrincipal(principal), strings.TrimSpace(roomID), s.now().UTC().Add(InviteTTL))
}

func (s *Service) JoinInvite(ctx context.Context, principal Principal, token string) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Message{}, fmt.Errorf("%w: invite token is required", ErrInvalid)
	}
	roomID, wrapped, err := s.store.RoomKeyForInvite(ctx, principal, token)
	if err != nil {
		return Message{}, err
	}
	key, err := s.crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(key)
	users, err := s.store.Users(ctx, principal, []string{principal.UserID})
	if err != nil {
		return Message{}, err
	}
	if len(users) != 1 {
		return Message{}, fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.MemberJoined, displayName(users[0])))
	if err != nil {
		return Message{}, err
	}
	record, err := s.store.JoinInvite(ctx, principal, token, systemMessage)
	if err != nil {
		return Message{}, err
	}
	msgs, err := s.decryptRecords(key, []MessageRecord{record})
	if err != nil {
		return Message{}, err
	}
	return msgs[0], nil
}
