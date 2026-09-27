package aiassistant

import (
	"context"
	"sort"
	"strings"
)

// LocalSummarizer is the default, offline, deterministic Summarizer. It uses
// extractive heuristics (frequency-weighted sentence ranking, participant
// extraction, imperative/action-phrase detection) and never performs network
// I/O. This guarantees no mail content leaves the server.
type LocalSummarizer struct {
	// MaxKeyPoints caps the number of extracted key points. Zero uses a default.
	MaxKeyPoints int
	// MaxActionItems caps the number of extracted action items. Zero uses a default.
	MaxActionItems int
	// MaxMessages caps how many messages are scanned. Zero uses a default.
	MaxMessages int
}

// NewLocalSummarizer returns a LocalSummarizer with sensible defaults.
func NewLocalSummarizer() *LocalSummarizer {
	return &LocalSummarizer{
		MaxKeyPoints:   5,
		MaxActionItems: 5,
		MaxMessages:    50,
	}
}

// Name implements Summarizer.
func (s *LocalSummarizer) Name() string { return "local" }

// Offline implements Summarizer.
func (s *LocalSummarizer) Offline() bool { return true }

// Summarize implements Summarizer with a fully local extractive strategy.
func (s *LocalSummarizer) Summarize(ctx context.Context, thread Thread) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	maxKeyPoints := s.MaxKeyPoints
	if maxKeyPoints <= 0 {
		maxKeyPoints = 5
	}
	maxActionItems := s.MaxActionItems
	if maxActionItems <= 0 {
		maxActionItems = 5
	}
	maxMessages := s.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 50
	}

	messages := thread.Messages
	truncated := false
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
		truncated = true
	}

	subject := strings.TrimSpace(thread.Subject)
	if subject == "" && len(messages) > 0 {
		subject = strings.TrimSpace(messages[0].Subject)
	}

	participants := extractParticipants(messages)
	sentences := collectSentences(messages)
	keyPoints := rankKeyPoints(sentences, maxKeyPoints)
	actionItems := extractActionItems(messages, participants, maxActionItems)
	reply := buildSuggestedReply(messages, actionItems)

	return Summary{
		Subject:          subject,
		Participants:     participants,
		KeyPoints:        keyPoints,
		ActionItems:      actionItems,
		SuggestedReply:   reply,
		MessageCount:     len(messages),
		Provider:         s.Name(),
		Truncated:        truncated,
		GeneratedOffline: true,
	}, nil
}

// extractParticipants returns a de-duplicated, order-preserving list of
// participant display strings (name <addr> or addr) across the thread.
func extractParticipants(messages []Message) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(name, addr string) {
		addr = strings.ToLower(strings.TrimSpace(addr))
		name = strings.TrimSpace(name)
		if addr == "" && name == "" {
			return
		}
		key := addr
		if key == "" {
			key = strings.ToLower(name)
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		switch {
		case name != "" && addr != "":
			out = append(out, name+" <"+addr+">")
		case addr != "":
			out = append(out, addr)
		default:
			out = append(out, name)
		}
	}
	for _, m := range messages {
		add(m.FromName, m.FromAddr)
		for _, to := range m.ToAddrs {
			add("", to)
		}
	}
	return out
}

// sentence carries a normalized sentence and its source order for stable,
// deterministic ranking.
type sentence struct {
	text  string
	order int
}

// collectSentences splits every message body into sentences, in thread order.
func collectSentences(messages []Message) []sentence {
	var out []sentence
	order := 0
	for _, m := range messages {
		for _, raw := range splitSentences(m.Body) {
			text := strings.TrimSpace(raw)
			if len(text) < 12 {
				continue
			}
			if isQuotedLine(text) {
				continue
			}
			out = append(out, sentence{text: text, order: order})
			order++
		}
	}
	return out
}

// rankKeyPoints selects the top-N sentences by term-frequency score. Ties break
// on original order so output is deterministic.
func rankKeyPoints(sentences []sentence, max int) []string {
	if len(sentences) == 0 {
		return []string{}
	}
	freq := map[string]int{}
	for _, s := range sentences {
		for _, w := range tokenize(s.text) {
			if isStopWord(w) {
				continue
			}
			freq[w]++
		}
	}
	type scored struct {
		s     sentence
		score float64
	}
	ranked := make([]scored, 0, len(sentences))
	for _, s := range sentences {
		words := tokenize(s.text)
		if len(words) == 0 {
			continue
		}
		total := 0
		counted := 0
		for _, w := range words {
			if isStopWord(w) {
				continue
			}
			total += freq[w]
			counted++
		}
		if counted == 0 {
			continue
		}
		// Average term frequency; normalizes for sentence length.
		ranked = append(ranked, scored{s: s, score: float64(total) / float64(counted)})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].s.order < ranked[j].s.order
	})
	if len(ranked) > max {
		ranked = ranked[:max]
	}
	// Restore thread order for readability.
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].s.order < ranked[j].s.order
	})
	out := make([]string, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, collapseSpaces(r.s.text))
	}
	return out
}

// extractActionItems finds imperative / request-style sentences and attributes
// an owner when a clear addressee is present.
func extractActionItems(messages []Message, participants []string, max int) []ActionItem {
	var items []ActionItem
	seen := map[string]struct{}{}
	for _, m := range messages {
		for _, raw := range splitSentences(m.Body) {
			text := strings.TrimSpace(raw)
			if text == "" || isQuotedLine(text) {
				continue
			}
			if !looksLikeActionItem(text) {
				continue
			}
			normalized := collapseSpaces(text)
			key := strings.ToLower(normalized)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			owner := ""
			if len(m.ToAddrs) == 1 {
				owner = strings.ToLower(strings.TrimSpace(m.ToAddrs[0]))
			}
			items = append(items, ActionItem{Text: normalized, Owner: owner})
			if len(items) >= max {
				return items
			}
		}
	}
	return items
}

// buildSuggestedReply drafts a short, neutral reply acknowledging the latest
// message and any detected action items. It is a template, not generated prose,
// so it is deterministic and safe.
func buildSuggestedReply(messages []Message, actions []ActionItem) string {
	if len(messages) == 0 {
		return ""
	}
	last := messages[len(messages)-1]
	greetName := firstName(last.FromName)
	var b strings.Builder
	if greetName != "" {
		b.WriteString("Hi ")
		b.WriteString(greetName)
		b.WriteString(",\n\n")
	} else {
		b.WriteString("Hi,\n\n")
	}
	b.WriteString("Thanks for your message.")
	if len(actions) > 0 {
		b.WriteString(" I'll follow up on the following:\n")
		limit := len(actions)
		if limit > 3 {
			limit = 3
		}
		for _, a := range actions[:limit] {
			b.WriteString("- ")
			b.WriteString(a.Text)
			b.WriteString("\n")
		}
	} else {
		b.WriteString(" I'll review this and get back to you shortly.\n")
	}
	b.WriteString("\nBest regards")
	return b.String()
}
