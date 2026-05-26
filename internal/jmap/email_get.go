package jmap

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

// emailGetMethod implements Email/get (RFC 8621 §4.5).
type emailGetMethod struct{ deps Deps }

func (m *emailGetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	var req EmailGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}
	if len(req.IDs) > maxObjectsInGet {
		return errorResult("requestTooLarge"), nil
	}
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}

	state, _ := EmailStateFor(ctx, m.deps.Repo.DB(), userID)
	resp := EmailGetResponse{
		AccountID: req.AccountID, State: state,
		List:     make([]EmailObject, 0),
		NotFound: make([]string, 0),
	}

	for _, id := range req.IDs {
		detail, err := m.deps.Repo.GetMessage(ctx, userID, id)
		if err != nil {
			resp.NotFound = append(resp.NotFound, id)
			continue
		}
		resp.List = append(resp.List, messageDetailToJMAP(detail, req.Properties))
	}
	return json.Marshal(resp)
}

// messageDetailToJMAP converts a maildb.MessageDetail to a JMAP EmailObject.
// If props is non-empty, only those properties (plus "id") are populated.
func messageDetailToJMAP(d maildb.MessageDetail, props []string) EmailObject {
	propSet := make(map[string]bool, len(props))
	filterProps := len(props) > 0
	for _, p := range props {
		propSet[p] = true
	}
	want := func(p string) bool {
		return !filterProps || propSet[p] || p == "id"
	}

	email := EmailObject{ID: d.ID}

	if want("blobId") {
		email.BlobID = d.StoragePath
	}
	if want("threadId") {
		// Use RFC Message-ID as JMAP thread proxy (real threading tracked in threads table)
		email.ThreadID = d.MessageID
	}
	if want("keywords") {
		email.Keywords = flagsToKeywords(d.Flags)
	}
	if want("size") {
		email.Size = int(d.Size)
	}
	if want("receivedAt") {
		email.ReceivedAt = d.ReceivedAt.UTC().Format(time.RFC3339)
	}
	if want("subject") {
		email.Subject = d.Subject
	}
	if want("from") {
		email.From = parseJMAPAddrs(d.FromAddr, d.FromName)
	}
	if want("to") {
		email.To = parseJMAPAddrsJSON(d.ToAddrs)
	}
	if want("cc") {
		email.Cc = parseJMAPAddrsJSON(d.CcAddrs)
	}
	if want("bcc") {
		email.Bcc = parseJMAPAddrsJSON(d.BccAddrs)
	}
	if want("preview") {
		// Use first 200 chars of text body as preview
		if len(d.TextBody) > 200 {
			email.Preview = d.TextBody[:200]
		} else {
			email.Preview = d.TextBody
		}
	}
	if want("textBody") || want("bodyValues") {
		email.BodyValues = map[string]EmailBodyValue{
			"1": {Value: d.TextBody, IsEncodingProblem: false, IsTruncated: false},
		}
		email.TextBody = []EmailBodyPart{{PartID: "1", Type: "text/plain"}}
	}
	if want("htmlBody") && d.HTMLBody != "" {
		if email.BodyValues == nil {
			email.BodyValues = make(map[string]EmailBodyValue)
		}
		email.BodyValues["2"] = EmailBodyValue{Value: d.HTMLBody}
		email.HTMLBody = []EmailBodyPart{{PartID: "2", Type: "text/html"}}
	}
	return email
}

func flagsToKeywords(flagsJSON json.RawMessage) map[string]bool {
	var flags map[string]bool
	if err := json.Unmarshal(flagsJSON, &flags); err != nil {
		return map[string]bool{}
	}
	kw := make(map[string]bool)
	if flags["read"] {
		kw["$seen"] = true
	}
	if flags["starred"] {
		kw["$flagged"] = true
	}
	if flags["draft"] {
		kw["$draft"] = true
	}
	return kw
}

func parseJMAPAddrs(addr, name string) []EmailAddress {
	if addr == "" {
		return nil
	}
	return []EmailAddress{{Email: addr, Name: stringPtr(name)}}
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseJMAPAddrsJSON(raw json.RawMessage) []EmailAddress {
	if raw == nil {
		return nil
	}
	// ToAddrs/CcAddrs stored as JSON array of {Name, Address} objects
	var addrs []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(raw, &addrs); err != nil {
		return nil
	}
	result := make([]EmailAddress, 0, len(addrs))
	for _, a := range addrs {
		result = append(result, EmailAddress{Email: a.Address, Name: stringPtr(a.Name)})
	}
	return result
}
