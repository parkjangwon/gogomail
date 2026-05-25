package webauthn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	waprotocol "github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
)

// Config holds Relying Party settings for WebAuthn.
type Config struct {
	RPDisplayName string   // e.g. "GoGoMail"
	RPID          string   // e.g. "mail.example.com"
	RPOrigins     []string // e.g. ["https://mail.example.com"]
}

// Service wraps go-webauthn/webauthn and the credential store.
type Service struct {
	wa    *gowebauthn.WebAuthn
	store *Store
}

// NewService constructs a Service, validating config.
func NewService(cfg Config, store *Store) (*Service, error) {
	if cfg.RPID == "" {
		return nil, errors.New("webauthn: RPID is required")
	}
	wa, err := gowebauthn.New(&gowebauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn: init: %w", err)
	}
	return &Service{wa: wa, store: store}, nil
}

// BeginRegistration generates a credential creation challenge for the given user.
// Returns JSON bytes suitable for sending to the browser (PublicKeyCredentialCreationOptions).
func (s *Service) BeginRegistration(ctx context.Context, userID, username, displayName string) ([]byte, error) {
	// Load existing credentials so the browser excludes already-registered authenticators.
	existing, err := s.store.GetCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: load credentials: %w", err)
	}
	u := &waUser{
		id:          []byte(userID),
		name:        username,
		displayName: displayName,
		credentials: toWACredentials(existing),
	}

	creation, session, err := s.wa.BeginRegistration(u)
	if err != nil {
		return nil, fmt.Errorf("webauthn: begin registration: %w", err)
	}

	if err := s.store.SaveChallenge(ctx, userID, "registration", []byte(session.Challenge)); err != nil {
		return nil, fmt.Errorf("webauthn: save challenge: %w", err)
	}

	out, err := json.Marshal(creation)
	if err != nil {
		return nil, fmt.Errorf("webauthn: marshal creation options: %w", err)
	}
	return out, nil
}

// CompleteRegistration verifies the registration response and stores the new credential.
// responseBody is the raw JSON from the browser's navigator.credentials.create() call.
// credName is a user-supplied friendly name for this key.
func (s *Service) CompleteRegistration(ctx context.Context, userID, credName string, responseBody []byte) (*Credential, error) {
	challengeBytes, err := s.store.GetAndDeleteChallenge(ctx, userID, "registration")
	if err != nil {
		return nil, fmt.Errorf("webauthn: challenge: %w", err)
	}

	existing, err := s.store.GetCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: load credentials: %w", err)
	}
	u := &waUser{id: []byte(userID), credentials: toWACredentials(existing)}

	session := gowebauthn.SessionData{
		Challenge:      string(challengeBytes),
		RelyingPartyID: s.wa.Config.RPID,
		UserID:         []byte(userID),
	}

	parsedResponse, err := waprotocol.ParseCredentialCreationResponseBytes(responseBody)
	if err != nil {
		return nil, fmt.Errorf("webauthn: parse registration response: %w", err)
	}

	waCred, err := s.wa.CreateCredential(u, session, parsedResponse)
	if err != nil {
		return nil, fmt.Errorf("webauthn: create credential: %w", err)
	}

	cred := &Credential{
		UserID:       userID,
		CredentialID: waCred.ID,
		PublicKey:    waCred.PublicKey,
		AAGUID:       aaguidString(waCred.Authenticator.AAGUID),
		SignCount:    waCred.Authenticator.SignCount,
		Name:         credName,
	}

	if err := s.store.SaveCredential(ctx, userID, cred); err != nil {
		return nil, fmt.Errorf("webauthn: save credential: %w", err)
	}
	return cred, nil
}

// BeginAuthentication generates an authentication challenge for a known user.
// Returns JSON bytes for PublicKeyCredentialRequestOptions.
func (s *Service) BeginAuthentication(ctx context.Context, userID string) ([]byte, error) {
	existing, err := s.store.GetCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: load credentials: %w", err)
	}
	if len(existing) == 0 {
		return nil, errors.New("webauthn: no credentials registered for user")
	}

	u := &waUser{id: []byte(userID), credentials: toWACredentials(existing)}

	assertion, session, err := s.wa.BeginLogin(u)
	if err != nil {
		return nil, fmt.Errorf("webauthn: begin authentication: %w", err)
	}

	if err := s.store.SaveChallenge(ctx, userID, "authentication", []byte(session.Challenge)); err != nil {
		return nil, fmt.Errorf("webauthn: save challenge: %w", err)
	}

	out, err := json.Marshal(assertion)
	if err != nil {
		return nil, fmt.Errorf("webauthn: marshal assertion options: %w", err)
	}
	return out, nil
}

// CompleteAuthentication verifies the assertion response and updates the sign count.
// Returns the matched Credential on success.
func (s *Service) CompleteAuthentication(ctx context.Context, userID string, responseBody []byte) (*Credential, error) {
	challengeBytes, err := s.store.GetAndDeleteChallenge(ctx, userID, "authentication")
	if err != nil {
		return nil, fmt.Errorf("webauthn: challenge: %w", err)
	}

	existing, err := s.store.GetCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: load credentials: %w", err)
	}
	u := &waUser{id: []byte(userID), credentials: toWACredentials(existing)}

	session := gowebauthn.SessionData{
		Challenge:      string(challengeBytes),
		RelyingPartyID: s.wa.Config.RPID,
		UserID:         []byte(userID),
	}

	parsedResponse, err := waprotocol.ParseCredentialRequestResponseBytes(responseBody)
	if err != nil {
		return nil, fmt.Errorf("webauthn: parse authentication response: %w", err)
	}

	waCred, err := s.wa.ValidateLogin(u, session, parsedResponse)
	if err != nil {
		return nil, fmt.Errorf("webauthn: validate login: %w", err)
	}

	// Update sign count for clone detection.
	if err := s.store.UpdateSignCount(ctx, waCred.ID, waCred.Authenticator.SignCount); err != nil {
		return nil, fmt.Errorf("webauthn: update sign count: %w", err)
	}

	// Return the matched local credential record.
	for i := range existing {
		if bytes.Equal(existing[i].CredentialID, waCred.ID) {
			existing[i].SignCount = waCred.Authenticator.SignCount
			return &existing[i], nil
		}
	}
	return nil, errors.New("webauthn: matched credential not found locally")
}

// ListCredentials returns all credentials for a user (for display).
func (s *Service) ListCredentials(ctx context.Context, userID string) ([]Credential, error) {
	return s.store.GetCredentials(ctx, userID)
}

// DeleteCredential removes a credential by its UUID for a specific user.
func (s *Service) DeleteCredential(ctx context.Context, userID, credentialID string) error {
	return s.store.DeleteCredential(ctx, userID, credentialID)
}

// --- helpers ---

// waUser implements gowebauthn.User for the library.
type waUser struct {
	id          []byte
	name        string
	displayName string
	credentials []gowebauthn.Credential
}

func (u *waUser) WebAuthnID() []byte                           { return u.id }
func (u *waUser) WebAuthnName() string                         { return u.name }
func (u *waUser) WebAuthnDisplayName() string                  { return u.displayName }
func (u *waUser) WebAuthnCredentials() []gowebauthn.Credential { return u.credentials }

// toWACredentials converts our Credential slice to the library's Credential slice.
func toWACredentials(creds []Credential) []gowebauthn.Credential {
	out := make([]gowebauthn.Credential, 0, len(creds))
	for _, c := range creds {
		out = append(out, gowebauthn.Credential{
			ID:        c.CredentialID,
			PublicKey: c.PublicKey,
			Authenticator: gowebauthn.Authenticator{
				SignCount: c.SignCount,
			},
		})
	}
	return out
}

// aaguidString converts a raw AAGUID byte slice (16 bytes) to a UUID string.
func aaguidString(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
