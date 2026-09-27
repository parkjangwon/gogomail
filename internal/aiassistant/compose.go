package aiassistant

import (
	"sort"
	"strings"
)

// ComposeDraft is the input to the smart compose assistant.
type ComposeDraft struct {
	Subject         string
	Body            string
	Recipients      []string
	AttachmentCount int
}

// ComposeVariant is a rewritten version of the body in a particular tone/length.
type ComposeVariant struct {
	Label string `json:"label"`
	Body  string `json:"body"`
}

// ComposeCheck is a single detected issue or suggestion about the draft.
type ComposeCheck struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "info" | "warning"
}

// ComposeAssist is the full result of analyzing a draft. All fields are derived
// from local heuristics; no external calls are made.
type ComposeAssist struct {
	SubjectSuggestions []string         `json:"subject_suggestions"`
	Variants           []ComposeVariant `json:"variants"`
	Checks             []ComposeCheck   `json:"checks"`
	WordCount          int              `json:"word_count"`
}

// ComposeAssistant produces subject suggestions, tone/length variants, and
// pre-send checks (such as the "mentions attachment but none attached" check)
// using deterministic local heuristics.
type ComposeAssistant struct{}

// NewComposeAssistant returns a ready-to-use compose assistant.
func NewComposeAssistant() *ComposeAssistant { return &ComposeAssistant{} }

var attachmentMentionPhrases = []string{
	"attached", "attachment", "attaching", "i've attached", "ive attached",
	"please find attached", "see attached", "enclosed", "첨부", "붙임",
}

// Assist analyzes the draft and returns suggestions, variants, and checks.
func (a *ComposeAssistant) Assist(draft ComposeDraft) ComposeAssist {
	body := strings.TrimSpace(draft.Body)
	words := tokenize(body)
	wordCount := len(strings.Fields(body))

	return ComposeAssist{
		SubjectSuggestions: suggestSubjects(draft, words),
		Variants:           buildVariants(body),
		Checks:             runComposeChecks(draft, body),
		WordCount:          wordCount,
	}
}

// suggestSubjects proposes subject lines. If the draft already has a subject,
// it proposes lightly reworded alternatives; otherwise it derives candidates
// from the most salient body content.
func suggestSubjects(draft ComposeDraft, bodyWords []string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	add := func(s string) {
		s = collapseSpaces(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}

	subject := strings.TrimSpace(draft.Subject)
	if subject != "" {
		add(subject)
		add("Re: " + subject)
		add("Follow-up: " + subject)
		return out
	}

	// No subject: derive from first meaningful sentence and top keywords.
	sentences := splitSentences(draft.Body)
	if len(sentences) > 0 {
		first := collapseSpaces(sentences[0])
		add(titleCaseShort(first, 8))
	}
	freq := map[string]int{}
	for _, w := range bodyWords {
		if isStopWord(w) || !containsLetter(w) {
			continue
		}
		freq[w]++
	}
	if kw := topKeywords(freq, 4); len(kw) > 0 {
		add(titleCaseWords(kw))
	}
	if len(out) == 0 {
		add("(no subject)")
	}
	return out
}

// buildVariants produces tone/length variants of the body via light templating.
func buildVariants(body string) []ComposeVariant {
	if body == "" {
		return []ComposeVariant{}
	}
	sentences := splitSentences(body)
	concise := body
	if len(sentences) > 1 {
		concise = collapseSpaces(sentences[0])
		if !strings.HasSuffix(concise, ".") {
			concise += "."
		}
	}
	formal := "Dear recipient,\n\n" + body + "\n\nKind regards,"
	friendly := "Hi there,\n\n" + body + "\n\nThanks!"

	return []ComposeVariant{
		{Label: "concise", Body: concise},
		{Label: "formal", Body: formal},
		{Label: "friendly", Body: friendly},
	}
}

// runComposeChecks runs pre-send heuristics, most notably the missing-attachment
// check.
func runComposeChecks(draft ComposeDraft, body string) []ComposeCheck {
	var checks []ComposeCheck
	lowerBody := strings.ToLower(stripHTMLish(body))
	lowerSubject := strings.ToLower(draft.Subject)

	mentionsAttachment := false
	for _, phrase := range attachmentMentionPhrases {
		if strings.Contains(lowerBody, phrase) || strings.Contains(lowerSubject, phrase) {
			mentionsAttachment = true
			break
		}
	}
	if mentionsAttachment && draft.AttachmentCount == 0 {
		checks = append(checks, ComposeCheck{
			Code:     "missing_attachment",
			Message:  "The message mentions an attachment, but no file is attached.",
			Severity: "warning",
		})
	}

	if strings.TrimSpace(draft.Subject) == "" {
		checks = append(checks, ComposeCheck{
			Code:     "empty_subject",
			Message:  "The subject line is empty.",
			Severity: "warning",
		})
	}
	if strings.TrimSpace(body) == "" {
		checks = append(checks, ComposeCheck{
			Code:     "empty_body",
			Message:  "The message body is empty.",
			Severity: "warning",
		})
	}
	if len(draft.Recipients) == 0 {
		checks = append(checks, ComposeCheck{
			Code:     "no_recipients",
			Message:  "No recipients are specified.",
			Severity: "warning",
		})
	}
	if len(draft.Recipients) > 20 {
		checks = append(checks, ComposeCheck{
			Code:     "many_recipients",
			Message:  "This message has a large number of recipients; consider Bcc.",
			Severity: "info",
		})
	}
	return checks
}

func titleCaseShort(s string, maxWords int) string {
	fields := strings.Fields(s)
	if len(fields) > maxWords {
		fields = fields[:maxWords]
	}
	return strings.Join(fields, " ")
}

func titleCaseWords(words []string) string {
	sorted := append([]string(nil), words...)
	sort.Strings(sorted)
	cased := make([]string, 0, len(sorted))
	for _, w := range sorted {
		if w == "" {
			continue
		}
		cased = append(cased, strings.ToUpper(w[:1])+w[1:])
	}
	return strings.Join(cased, " ")
}
