package aiassistant

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	sentenceSplitRe = regexp.MustCompile(`(?m)[.!?。！？\n]+`)
	wordSplitRe     = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	whitespaceRe    = regexp.MustCompile(`\s+`)
)

// splitSentences breaks text into candidate sentences on terminal punctuation
// and newlines. It supports both ASCII and common CJK terminators.
func splitSentences(text string) []string {
	text = stripHTMLish(text)
	parts := sentenceSplitRe.Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// tokenize lower-cases and splits text into word tokens.
func tokenize(text string) []string {
	lowered := strings.ToLower(text)
	parts := wordSplitRe.Split(lowered, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// collapseSpaces normalizes internal whitespace runs to single spaces.
func collapseSpaces(text string) string {
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(text, " "))
}

// stripHTMLish removes a light set of HTML tags so plain-text heuristics work
// even when a caller passes HTML. It is intentionally conservative and not a
// full HTML parser.
func stripHTMLish(text string) string {
	if !strings.Contains(text, "<") {
		return text
	}
	var b strings.Builder
	inTag := false
	for _, r := range text {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isQuotedLine reports whether a line is quoted reply text or a citation
// header, which should be excluded from summaries.
func isQuotedLine(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "on ") && strings.Contains(lower, "wrote:") {
		return true
	}
	if strings.HasPrefix(lower, "-----original message-----") {
		return true
	}
	return false
}

// firstName returns the leading token of a display name, or "" when empty.
func firstName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

var actionPhrases = []string{
	"please", "could you", "can you", "would you", "let me know",
	"need to", "needs to", "should", "must", "make sure", "don't forget",
	"action item", "todo", "to-do", "follow up", "follow-up", "by tomorrow",
	"by monday", "by tuesday", "by wednesday", "by thursday", "by friday",
	"deadline", "due", "asap", "review", "confirm", "send me", "provide",
}

// looksLikeActionItem reports whether a sentence reads like a request or task.
func looksLikeActionItem(text string) bool {
	lower := strings.ToLower(collapseSpaces(text))
	if lower == "" {
		return false
	}
	for _, phrase := range actionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	// Leading imperative verb heuristic: first word is a common imperative.
	first := ""
	for _, w := range tokenize(lower) {
		first = w
		break
	}
	switch first {
	case "send", "review", "confirm", "prepare", "update", "schedule",
		"share", "complete", "submit", "check", "verify", "approve", "sign":
		return true
	}
	return false
}

var stopWords = map[string]struct{}{}

func init() {
	for _, w := range strings.Fields(`a an and are as at be by for from has have i if in into is it its of on or that the this to was were will with you your we our us they them he she his her be been being do does did but not no yes can could would should may might must shall about after again all am any because been before being between both cannot did down during each few more most other over some such than then there these those through under until up very what when where which while who whom why`) {
		stopWords[w] = struct{}{}
	}
}

// isStopWord reports whether w is a common stop word.
func isStopWord(w string) bool {
	if len(w) <= 1 {
		return true
	}
	_, ok := stopWords[w]
	return ok
}

// containsLetter reports whether s has at least one letter rune.
func containsLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
