package jmap

import (
	"context"
	"encoding/json"
)

// EmailAddress represents an RFC 8621 EmailAddress object.
type EmailAddress struct {
	Name  *string `json:"name"`
	Email string  `json:"email"`
}

// EmailObject represents an RFC 8621 Email object (§4.1).
// Only a subset of mandatory and commonly used properties are included;
// the full property set can be expanded when real storage integration lands.
type EmailObject struct {
	ID         string         `json:"id"`
	Subject    string         `json:"subject"`
	From       []EmailAddress `json:"from"`
	To         []EmailAddress `json:"to"`
	ReceivedAt string         `json:"receivedAt"` // UTCDate per RFC 8621 §1.4
	Size       int            `json:"size"`
	Preview    string         `json:"preview"`
}

// EmailGetArgs is the argument object for Email/get (RFC 8621 §4.5).
type EmailGetArgs struct {
	AccountID  string   `json:"accountId"`
	IDs        []string `json:"ids"`
	Properties []string `json:"properties,omitempty"`
}

// EmailGetResponse is the response object for Email/get (RFC 8621 §4.5).
type EmailGetResponse struct {
	AccountID string        `json:"accountId"`
	List      []EmailObject `json:"list"`
	NotFound  []string      `json:"notFound"`
	State     string        `json:"state"`
}

// EmailFilter represents a filter condition for Email/query (RFC 8621 §4.4.1).
type EmailFilter struct {
	InMailbox string `json:"inMailbox,omitempty"`
	Text      string `json:"text,omitempty"`
}

// EmailComparator is a sort comparator for Email/query (RFC 8621 §4.4.2).
type EmailComparator struct {
	Property    string `json:"property"`
	IsAscending bool   `json:"isAscending"`
}

// EmailQueryArgs is the argument object for Email/query (RFC 8621 §4.4).
type EmailQueryArgs struct {
	AccountID string           `json:"accountId"`
	Filter    *EmailFilter     `json:"filter,omitempty"`
	Sort      []EmailComparator `json:"sort,omitempty"`
	Position  int              `json:"position,omitempty"`
	Limit     int              `json:"limit,omitempty"`
}

// EmailQueryResponse is the response object for Email/query (RFC 8621 §4.4).
type EmailQueryResponse struct {
	AccountID           string   `json:"accountId"`
	QueryState          string   `json:"queryState"`
	CanCalculateChanges bool     `json:"canCalculateChanges"`
	Position            int      `json:"position"`
	IDs                 []string `json:"ids"`
	Total               int      `json:"total"`
}

// emailGetMethod implements the Email/get JMAP method (RFC 8621 §4.5).
// Currently returns an empty list; real mail store integration to follow.
type emailGetMethod struct{}

func (emailGetMethod) Call(_ context.Context, accountID string, args json.RawMessage) (json.RawMessage, error) {
	var req EmailGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = accountID
	}

	resp := EmailGetResponse{
		AccountID: req.AccountID,
		List:      []EmailObject{},
		NotFound:  []string{},
		State:     "state-v1",
	}
	// If specific IDs were requested, mark them all as not found (stub).
	resp.NotFound = append(resp.NotFound, req.IDs...)

	return json.Marshal(resp)
}

// emailQueryMethod implements the Email/query JMAP method (RFC 8621 §4.4).
// Currently returns an empty result set; real mail store integration to follow.
type emailQueryMethod struct{}

func (emailQueryMethod) Call(_ context.Context, accountID string, args json.RawMessage) (json.RawMessage, error) {
	var req EmailQueryArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = accountID
	}

	resp := EmailQueryResponse{
		AccountID:           req.AccountID,
		QueryState:          "state-v1",
		CanCalculateChanges: false,
		Position:            req.Position,
		IDs:                 []string{},
		Total:               0,
	}
	return json.Marshal(resp)
}
