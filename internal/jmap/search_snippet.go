package jmap

import (
	"context"
	"encoding/json"
	"fmt"
)

// SearchSnippet is a JMAP SearchSnippet object (RFC 8621 §7).
type SearchSnippet struct {
	EmailID string `json:"emailId"`
	Subject string `json:"subject"`
	Preview string `json:"preview"`
}

type searchSnippetGetArgs struct {
	AccountID string          `json:"accountId"`
	Filter    json.RawMessage `json:"filter"` // ignored; present for RFC compliance
	EmailIDs  []string        `json:"emailIds"`
}

type searchSnippetGetResponse struct {
	AccountID string          `json:"accountId"`
	List      []SearchSnippet `json:"list"`
	NotFound  []string        `json:"notFound"`
}

type searchSnippetGetMethod struct{ deps Deps }

func (m *searchSnippetGetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return nil, fmt.Errorf("serverFail")
	}
	var req searchSnippetGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	var list []SearchSnippet
	var notFound []string

	for _, emailID := range req.EmailIDs {
		msg, err := m.deps.Repo.GetMessage(ctx, userID, emailID)
		if err != nil {
			notFound = append(notFound, emailID)
			continue
		}
		preview := msg.TextBody
		if len([]rune(preview)) > 255 {
			runes := []rune(preview)
			preview = string(runes[:255])
		}
		list = append(list, SearchSnippet{
			EmailID: emailID,
			Subject: msg.Subject,
			Preview: preview,
		})
	}
	if list == nil {
		list = []SearchSnippet{}
	}
	if notFound == nil {
		notFound = []string{}
	}

	resp := searchSnippetGetResponse{
		AccountID: userID,
		List:      list,
		NotFound:  notFound,
	}
	return json.Marshal(resp)
}
