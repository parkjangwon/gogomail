package jmap

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Identity represents a JMAP Identity object (RFC 8621 §4.1).
type Identity struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Email         string         `json:"email"`
	ReplyTo       []EmailAddress `json:"replyTo"`
	Bcc           []EmailAddress `json:"bcc"`
	TextSignature string         `json:"textSignature"`
	HTMLSignature string         `json:"htmlSignature"`
	MayDelete     bool           `json:"mayDelete"`
}

type identityGetArgs struct {
	AccountID  string   `json:"accountId"`
	IDs        []string `json:"ids"`
	Properties []string `json:"properties"`
}

type identityGetResponse struct {
	AccountID string     `json:"accountId"`
	State     string     `json:"state"`
	List      []Identity `json:"list"`
	NotFound  []string   `json:"notFound"`
}

type identitySetArgs struct {
	AccountID string                     `json:"accountId"`
	Create    map[string]json.RawMessage `json:"create"`
	Update    map[string]json.RawMessage `json:"update"`
	Destroy   []string                   `json:"destroy"`
}

type identityGetMethod struct{ deps Deps }
type identitySetMethod struct{ deps Deps }

func (m *identityGetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return nil, fmt.Errorf("serverFail")
	}
	var req identityGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	// Fetch primary identity from users table.
	var email, displayName string
	err := m.deps.Repo.DB().QueryRowContext(ctx,
		`SELECT email, COALESCE(display_name, '') FROM users WHERE id = $1::uuid`,
		userID,
	).Scan(&email, &displayName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("serverFail: %w", err)
	}
	if displayName == "" {
		displayName = email
	}

	primary := Identity{
		ID:        userID, // use userID as the primary identity ID
		Name:      displayName,
		Email:     email,
		MayDelete: false, // primary identity cannot be deleted
	}

	// Fetch custom identities from preferences.
	identities := []Identity{primary}
	prefs, err := m.deps.Repo.GetWebmailPreferences(ctx, userID)
	if err == nil {
		var p struct {
			Identities []Identity `json:"identities"`
		}
		if json.Unmarshal(prefs, &p) == nil && len(p.Identities) > 0 {
			for i := range p.Identities {
				p.Identities[i].MayDelete = true
			}
			identities = append(identities, p.Identities...)
		}
	}

	// Filter by requested IDs if provided.
	if len(req.IDs) > 0 {
		var filtered []Identity
		notFound := []string{}
		for _, id := range req.IDs {
			found := false
			for _, ident := range identities {
				if ident.ID == id {
					filtered = append(filtered, ident)
					found = true
					break
				}
			}
			if !found {
				notFound = append(notFound, id)
			}
		}
		if filtered == nil {
			filtered = []Identity{}
		}
		resp := identityGetResponse{
			AccountID: userID,
			State:     "identity-v1",
			List:      filtered,
			NotFound:  notFound,
		}
		return json.Marshal(resp)
	}

	resp := identityGetResponse{
		AccountID: userID,
		State:     "identity-v1",
		List:      identities,
		NotFound:  []string{},
	}
	return json.Marshal(resp)
}

func (m *identitySetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return nil, fmt.Errorf("serverFail")
	}
	var req identitySetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	notCreated := make(map[string]SetError)
	created := make(map[string]Identity)
	notUpdated := make(map[string]SetError)
	updated := make(map[string]json.RawMessage)
	notDestroyed := make(map[string]SetError)

	// Handle creates: store custom identity in preferences.
	if len(req.Create) > 0 {
		prefs, _ := m.deps.Repo.GetWebmailPreferences(ctx, userID)
		var p struct {
			Identities []Identity `json:"identities"`
		}
		if prefs != nil {
			_ = json.Unmarshal(prefs, &p)
		}

		for cid, raw := range req.Create {
			var ident Identity
			if err := json.Unmarshal(raw, &ident); err != nil {
				notCreated[cid] = SetError{Type: "invalidProperties"}
				continue
			}
			// Generate an ID
			ident.ID = "custom-" + cid
			ident.MayDelete = true
			p.Identities = append(p.Identities, ident)
			created[cid] = ident
		}
		// Save back by merging into preferences JSON
		var merged map[string]json.RawMessage
		if prefs != nil {
			_ = json.Unmarshal(prefs, &merged)
		}
		if merged == nil {
			merged = make(map[string]json.RawMessage)
		}
		identJSON, _ := json.Marshal(p.Identities)
		merged["identities"] = identJSON
		mergedJSON, _ := json.Marshal(merged)
		if err := m.deps.Repo.SetWebmailPreferences(ctx, userID, mergedJSON); err != nil {
			for cid := range req.Create {
				delete(created, cid)
				notCreated[cid] = SetError{Type: "serverFail"}
			}
		}
	}

	// Handle updates.
	for uid := range req.Update {
		if uid == userID {
			notUpdated[uid] = SetError{Type: "forbidden"}
			continue
		}
		notUpdated[uid] = SetError{Type: "notImplemented", Description: "Identity update is not yet supported"}
	}

	// Handle destroys — load preferences once, modify in memory, save once.
	var destroyed []string
	if len(req.Destroy) > 0 {
		destroyPrefs, _ := m.deps.Repo.GetWebmailPreferences(ctx, userID)
		var dp struct {
			Identities []Identity `json:"identities"`
		}
		json.Unmarshal(destroyPrefs, &dp)
		originalLen := len(dp.Identities)

		for _, did := range req.Destroy {
			if did == userID {
				notDestroyed[did] = SetError{Type: "forbidden"}
				continue
			}
			found := false
			newList := make([]Identity, 0, len(dp.Identities))
			for _, ident := range dp.Identities {
				if ident.ID == did {
					found = true
				} else {
					newList = append(newList, ident)
				}
			}
			if !found {
				notDestroyed[did] = SetError{Type: "notFound"}
				continue
			}
			dp.Identities = newList
			destroyed = append(destroyed, did)
		}

		// Only save if something was actually destroyed.
		if len(destroyed) > 0 || len(dp.Identities) != originalLen {
			var dMerged map[string]json.RawMessage
			json.Unmarshal(destroyPrefs, &dMerged)
			if dMerged == nil {
				dMerged = make(map[string]json.RawMessage)
			}
			identJSON, _ := json.Marshal(dp.Identities)
			dMerged["identities"] = identJSON
			mergedJSON, _ := json.Marshal(dMerged)
			if err := m.deps.Repo.SetWebmailPreferences(ctx, userID, mergedJSON); err != nil {
				for _, did := range destroyed {
					notDestroyed[did] = SetError{Type: "serverFail"}
				}
				destroyed = []string{}
			}
		}
	}
	if destroyed == nil {
		destroyed = []string{}
	}

	resp := map[string]interface{}{
		"accountId":    userID,
		"oldState":     "identity-v1",
		"newState":     "identity-v1",
		"created":      created,
		"updated":      updated,
		"destroyed":    destroyed,
		"notCreated":   notCreated,
		"notUpdated":   notUpdated,
		"notDestroyed": notDestroyed,
	}
	return json.Marshal(resp)
}
