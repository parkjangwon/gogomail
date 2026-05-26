package jmap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VacationResponse is the RFC 8621 §8 singleton object.
type VacationResponse struct {
	ID        string  `json:"id"`
	IsEnabled bool    `json:"isEnabled"`
	FromDate  *string `json:"fromDate"`
	ToDate    *string `json:"toDate"`
	Subject   *string `json:"subject"`
	TextBody  *string `json:"textBody"`
	HTMLBody  *string `json:"htmlBody"`
}

const vacationResponseID = "singleton"
const vacationPrefKey = "vacationResponse"

// defaultVacationResponse returns the default VacationResponse when none is stored.
func defaultVacationResponse() VacationResponse {
	return VacationResponse{
		ID:        vacationResponseID,
		IsEnabled: false,
		FromDate:  nil,
		ToDate:    nil,
		Subject:   nil,
		TextBody:  nil,
		HTMLBody:  nil,
	}
}

// vacationResponseGetArgs is the argument object for VacationResponse/get.
type vacationResponseGetArgs struct {
	AccountID  string   `json:"accountId"`
	IDs        []string `json:"ids,omitempty"`
	Properties []string `json:"properties,omitempty"`
}

// vacationResponseGetResponse is the response for VacationResponse/get.
type vacationResponseGetResponse struct {
	AccountID string             `json:"accountId"`
	State     string             `json:"state"`
	List      []VacationResponse `json:"list"`
	NotFound  []string           `json:"notFound"`
}

// vacationResponseGetMethod implements VacationResponse/get (RFC 8621 §8).
type vacationResponseGetMethod struct{ deps Deps }

func (m *vacationResponseGetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return nil, fmt.Errorf("serverFail")
	}
	var req vacationResponseGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	vr := defaultVacationResponse()

	prefs, err := m.deps.Repo.GetWebmailPreferences(ctx, userID)
	if err == nil {
		var p struct {
			VacationResponse *VacationResponse `json:"vacationResponse"`
		}
		if json.Unmarshal(prefs, &p) == nil && p.VacationResponse != nil {
			vr = *p.VacationResponse
			vr.ID = vacationResponseID // always singleton
		}
	}

	singleton := vr

	// Apply ids filter if provided.
	if len(req.IDs) > 0 {
		var list []VacationResponse
		var notFound []string
		for _, id := range req.IDs {
			if id == "singleton" {
				list = append(list, singleton)
			} else {
				notFound = append(notFound, id)
			}
		}
		if list == nil {
			list = []VacationResponse{}
		}
		if notFound == nil {
			notFound = []string{}
		}
		resp := vacationResponseGetResponse{
			AccountID: userID,
			State:     "vacation-v1",
			List:      list,
			NotFound:  notFound,
		}
		return json.Marshal(resp)
	}

	// ids=null: return all (just the singleton).
	resp := vacationResponseGetResponse{
		AccountID: userID,
		State:     "vacation-v1",
		List:      []VacationResponse{singleton},
		NotFound:  []string{},
	}
	return json.Marshal(resp)
}

// vacationResponseSetArgs is the argument object for VacationResponse/set.
type vacationResponseSetArgs struct {
	AccountID string                     `json:"accountId"`
	Create    map[string]json.RawMessage `json:"create,omitempty"`
	Update    map[string]json.RawMessage `json:"update,omitempty"`
	Destroy   []string                   `json:"destroy,omitempty"`
}

// vacationResponseSetResponse is the response for VacationResponse/set.
type vacationResponseSetResponse struct {
	AccountID    string                     `json:"accountId"`
	OldState     string                     `json:"oldState"`
	NewState     string                     `json:"newState"`
	Created      map[string]json.RawMessage `json:"created"`
	NotCreated   map[string]SetError        `json:"notCreated"`
	Updated      map[string]json.RawMessage `json:"updated"`
	NotUpdated   map[string]SetError        `json:"notUpdated"`
	Destroyed    []string                   `json:"destroyed"`
	NotDestroyed map[string]SetError        `json:"notDestroyed"`
}

// vacationResponseSetMethod implements VacationResponse/set (RFC 8621 §8).
type vacationResponseSetMethod struct{ deps Deps }

func (m *vacationResponseSetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return nil, fmt.Errorf("serverFail")
	}
	var req vacationResponseSetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	// Create and destroy are forbidden — VacationResponse is a singleton.
	notCreated := make(map[string]SetError)
	for cid := range req.Create {
		notCreated[cid] = SetError{Type: "forbidden"}
	}
	notDestroyed := make(map[string]SetError)
	for _, did := range req.Destroy {
		notDestroyed[did] = SetError{Type: "forbidden"}
	}

	updated := make(map[string]json.RawMessage)
	notUpdated := make(map[string]SetError)

	// Only the singleton can be updated.
	for id, patch := range req.Update {
		if id != vacationResponseID {
			notUpdated[id] = SetError{Type: "notFound"}
			continue
		}

		// Load existing vacation response.
		vr := defaultVacationResponse()
		prefs, err := m.deps.Repo.GetWebmailPreferences(ctx, userID)
		if err != nil {
			notUpdated[id] = SetError{Type: "serverFail", Description: "failed to load preferences"}
			continue
		}
		if prefs != nil {
			var p struct {
				VacationResponse *VacationResponse `json:"vacationResponse"`
			}
			if json.Unmarshal(prefs, &p) == nil && p.VacationResponse != nil {
				vr = *p.VacationResponse
				vr.ID = vacationResponseID
			}
		}

		// Apply JMAP patch (RFC 8620 §3.3): iterate keys in the patch object
		// and merge them into the current VacationResponse.
		var patchMap map[string]json.RawMessage
		if err := json.Unmarshal(patch, &patchMap); err != nil {
			notUpdated[id] = SetError{Type: "invalidPatch"}
			continue
		}
		vrBytes, _ := json.Marshal(vr)
		var vrMap map[string]json.RawMessage
		_ = json.Unmarshal(vrBytes, &vrMap)
		for k, v := range patchMap {
			// Strip leading slash if present (JMAP path pointer).
			key := strings.TrimPrefix(k, "/")
			vrMap[key] = v
		}
		mergedBytes, err := json.Marshal(vrMap)
		if err != nil {
			notUpdated[id] = SetError{Type: "serverFail"}
			continue
		}
		if err := json.Unmarshal(mergedBytes, &vr); err != nil {
			notUpdated[id] = SetError{Type: "invalidPatch"}
			continue
		}
		vr.ID = vacationResponseID // preserve singleton ID

		// Persist back to preferences.
		var existing map[string]json.RawMessage
		if prefs != nil {
			_ = json.Unmarshal(prefs, &existing)
		}
		if existing == nil {
			existing = make(map[string]json.RawMessage)
		}
		vrRaw, err := json.Marshal(vr)
		if err != nil {
			notUpdated[id] = SetError{Type: "serverFail", Description: "failed to encode vacation response"}
			continue
		}
		existing[vacationPrefKey] = vrRaw
		newPrefs, err := json.Marshal(existing)
		if err != nil {
			notUpdated[id] = SetError{Type: "serverFail", Description: "failed to encode preferences"}
			continue
		}
		if err := m.deps.Repo.SetWebmailPreferences(ctx, userID, newPrefs); err != nil {
			notUpdated[id] = SetError{Type: "serverFail", Description: err.Error()}
			continue
		}

		// Return the updated object.
		updated[id] = vrRaw
	}

	oldState := "vacation-v1"
	newState := oldState
	if len(updated) > 0 {
		newState = fmt.Sprintf("vacation-%d", time.Now().UnixMicro())
	}

	resp := vacationResponseSetResponse{
		AccountID:    userID,
		OldState:     oldState,
		NewState:     newState,
		Created:      make(map[string]json.RawMessage),
		NotCreated:   notCreated,
		Updated:      updated,
		NotUpdated:   notUpdated,
		Destroyed:    []string{},
		NotDestroyed: notDestroyed,
	}
	return json.Marshal(resp)
}
