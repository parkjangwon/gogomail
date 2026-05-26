package jmap

import (
	"context"
	"encoding/json"
)

// EmailAddress represents a JMAP EmailAddress (RFC 8621 §4.1.2).
type EmailAddress struct {
	Name  *string `json:"name"`
	Email string  `json:"email"`
}

// EmailBodyValue is a decoded body part value (RFC 8621 §4.1.4).
type EmailBodyValue struct {
	Value             string `json:"value"`
	IsEncodingProblem bool   `json:"isEncodingProblem"`
	IsTruncated       bool   `json:"isTruncated"`
}

// EmailBodyPart is a body structure descriptor (RFC 8621 §4.1.4).
type EmailBodyPart struct {
	PartID string `json:"partId,omitempty"`
	Type   string `json:"type,omitempty"`
	BlobID string `json:"blobId,omitempty"`
	Size   int    `json:"size,omitempty"`
	Name   string `json:"name,omitempty"`
}

// EmailObject is the full JMAP Email object (RFC 8621 §4.1).
type EmailObject struct {
	ID         string                    `json:"id"`
	BlobID     string                    `json:"blobId,omitempty"`
	ThreadID   string                    `json:"threadId,omitempty"`
	MailboxIDs map[string]bool           `json:"mailboxIds,omitempty"`
	Keywords   map[string]bool           `json:"keywords,omitempty"`
	Size       int                       `json:"size,omitempty"`
	ReceivedAt string                    `json:"receivedAt,omitempty"`
	Subject    string                    `json:"subject,omitempty"`
	From       []EmailAddress            `json:"from,omitempty"`
	To         []EmailAddress            `json:"to,omitempty"`
	Cc         []EmailAddress            `json:"cc,omitempty"`
	Bcc        []EmailAddress            `json:"bcc,omitempty"`
	Preview    string                    `json:"preview,omitempty"`
	BodyValues map[string]EmailBodyValue `json:"bodyValues,omitempty"`
	TextBody   []EmailBodyPart           `json:"textBody,omitempty"`
	HTMLBody   []EmailBodyPart           `json:"htmlBody,omitempty"`
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
	InMailbox  string `json:"inMailbox,omitempty"`
	Text       string `json:"text,omitempty"`
	HasKeyword string `json:"hasKeyword,omitempty"`
	NotKeyword string `json:"notKeyword,omitempty"`
	Before     string `json:"before,omitempty"`
	After      string `json:"after,omitempty"`
	MinSize    int    `json:"minSize,omitempty"`
	MaxSize    int    `json:"maxSize,omitempty"`
	Subject    string `json:"subject,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
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
