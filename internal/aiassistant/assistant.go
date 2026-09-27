// Package aiassistant provides the provider-pluggable, privacy-preserving
// foundation for GoGoMail's AI email assistant: thread summarization,
// auto-categorization, and smart compose assistance.
//
// All default implementations in this package are pure, deterministic, local
// heuristics. They perform no network I/O and require no external API key, so
// the assistant works fully offline and no mail content ever leaves the server
// in the default configuration.
//
// The Summarizer interface is the extension point for optional external LLM
// providers. Wiring an external provider is an explicit, opt-in operator
// decision; the local extractive summarizer remains the default.
package aiassistant

import (
	"context"
	"time"
)

// Message is the provider-neutral view of a single message that the assistant
// operates on. It intentionally carries only the fields needed for local
// heuristics so callers never have to hand raw storage rows to a provider.
type Message struct {
	ID         string
	FromName   string
	FromAddr   string
	ToAddrs    []string
	Subject    string
	Body       string
	ReceivedAt time.Time
}

// Thread is an ordered set of messages (oldest first) plus the subject of the
// thread as a whole.
type Thread struct {
	Subject  string
	Messages []Message
}

// ActionItem is a single detected to-do or request extracted from a thread.
type ActionItem struct {
	Text  string `json:"text"`
	Owner string `json:"owner,omitempty"`
}

// Summary is the structured result of summarizing a thread.
type Summary struct {
	Subject         string       `json:"subject"`
	Participants    []string     `json:"participants"`
	KeyPoints       []string     `json:"key_points"`
	ActionItems     []ActionItem `json:"action_items"`
	SuggestedReply  string       `json:"suggested_reply"`
	MessageCount    int          `json:"message_count"`
	Provider        string       `json:"provider"`
	Truncated       bool         `json:"truncated"`
	GeneratedOffline bool        `json:"generated_offline"`
}

// Summarizer produces a structured Summary for a thread. Implementations must
// be safe for concurrent use.
//
// The default implementation (LocalSummarizer) is a deterministic extractive
// summarizer that runs entirely in-process. External LLM-backed
// implementations may be provided later via configuration; they should honor
// the ctx deadline and must document whether they transmit content off-box.
type Summarizer interface {
	// Summarize returns a structured summary of the thread. It must not mutate
	// the input. Name reports a stable provider identifier for auditing.
	Summarize(ctx context.Context, thread Thread) (Summary, error)
	// Name returns a stable identifier for the provider (e.g. "local").
	Name() string
	// Offline reports whether the provider keeps all content on the server.
	Offline() bool
}
