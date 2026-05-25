package jmap

import (
	"encoding/json"
	"fmt"
)

// Session represents the JMAP Session Resource (RFC 8620 §2).
type Session struct {
	Capabilities    map[string]interface{} `json:"capabilities"`
	Accounts        map[string]Account     `json:"accounts"`
	PrimaryAccounts map[string]string      `json:"primaryAccounts"`
	Username        string                 `json:"username"`
	APIUrl          string                 `json:"apiUrl"`
	DownloadUrl     string                 `json:"downloadUrl"`
	UploadUrl       string                 `json:"uploadUrl"`
	EventSourceUrl  string                 `json:"eventSourceUrl"`
	State           string                 `json:"state"`
}

// Account represents a JMAP account (RFC 8620 §2).
type Account struct {
	Name                string                 `json:"name"`
	IsPersonal          bool                   `json:"isPersonal"`
	IsReadOnly          bool                   `json:"isReadOnly"`
	AccountCapabilities map[string]interface{} `json:"accountCapabilities"`
}

// Request is a JMAP API request (RFC 8620 §3.3).
type Request struct {
	Using       []string          `json:"using"`
	MethodCalls []MethodCall      `json:"methodCalls"`
	CreatedIds  map[string]string `json:"createdIds,omitempty"`
}

// MethodCall is a single method invocation serialised as the JSON array
// [name, arguments, client-id] per RFC 8620 §3.3.
type MethodCall struct {
	Name   string
	Args   json.RawMessage
	CallID string
}

// MarshalJSON encodes a MethodCall as a three-element JSON array.
func (m MethodCall) MarshalJSON() ([]byte, error) {
	args := m.Args
	if args == nil {
		args = json.RawMessage("{}")
	}
	return json.Marshal([3]json.RawMessage{
		mustRawString(m.Name),
		args,
		mustRawString(m.CallID),
	})
}

// UnmarshalJSON decodes a three-element JSON array into a MethodCall.
func (m *MethodCall) UnmarshalJSON(data []byte) error {
	var raw [3]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("jmap: MethodCall must be a 3-element array: %w", err)
	}
	if err := json.Unmarshal(raw[0], &m.Name); err != nil {
		return fmt.Errorf("jmap: MethodCall name: %w", err)
	}
	m.Args = raw[1]
	if err := json.Unmarshal(raw[2], &m.CallID); err != nil {
		return fmt.Errorf("jmap: MethodCall call-id: %w", err)
	}
	return nil
}

// Response is a JMAP API response (RFC 8620 §3.4).
type Response struct {
	MethodResponses []MethodResponse  `json:"methodResponses"`
	CreatedIds      map[string]string `json:"createdIds,omitempty"`
	SessionState    string            `json:"sessionState"`
}

// MethodResponse is a single method result serialised as the JSON array
// [name, result, client-id] per RFC 8620 §3.4.
type MethodResponse struct {
	Name   string
	Result json.RawMessage
	CallID string
}

// MarshalJSON encodes a MethodResponse as a three-element JSON array.
func (m MethodResponse) MarshalJSON() ([]byte, error) {
	result := m.Result
	if result == nil {
		result = json.RawMessage("{}")
	}
	return json.Marshal([3]json.RawMessage{
		mustRawString(m.Name),
		result,
		mustRawString(m.CallID),
	})
}

// UnmarshalJSON decodes a three-element JSON array into a MethodResponse.
func (m *MethodResponse) UnmarshalJSON(data []byte) error {
	var raw [3]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("jmap: MethodResponse must be a 3-element array: %w", err)
	}
	if err := json.Unmarshal(raw[0], &m.Name); err != nil {
		return fmt.Errorf("jmap: MethodResponse name: %w", err)
	}
	m.Result = raw[1]
	if err := json.Unmarshal(raw[2], &m.CallID); err != nil {
		return fmt.Errorf("jmap: MethodResponse call-id: %w", err)
	}
	return nil
}

// Error constants per RFC 8620 §3.6.1.
const (
	ErrUnknownCapability = "unknownCapability"
	ErrNotJSON           = "notJSON"
	ErrNotRequest        = "notRequest"
	ErrUnknownMethod     = "unknownMethod"
	ErrServerFail        = "serverFail"
	ErrInvalidArguments  = "invalidArguments"
)

// methodError is the JSON object returned in a method-level error response.
type methodError struct {
	Type string `json:"type"`
}

// mustRawString encodes s as a JSON string, panicking on error (impossible for
// a plain Go string).
func mustRawString(s string) json.RawMessage {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return b
}
