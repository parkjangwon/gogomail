package aiassistant

import (
	"sort"
	"strings"
)

// RuleField identifies which part of a message a categorization rule matches
// against.
type RuleField string

const (
	FieldFrom    RuleField = "from"
	FieldSubject RuleField = "subject"
	FieldBody    RuleField = "body"
	FieldTo      RuleField = "to"
	FieldAny     RuleField = "any"
)

// RuleMatchType identifies how a rule keyword is compared.
type RuleMatchType string

const (
	MatchContains RuleMatchType = "contains"
	MatchPrefix   RuleMatchType = "prefix"
	MatchSuffix   RuleMatchType = "suffix"
	MatchExact    RuleMatchType = "exact"
)

// ValidRuleField reports whether f is a supported field.
func ValidRuleField(f RuleField) bool {
	switch f {
	case FieldFrom, FieldSubject, FieldBody, FieldTo, FieldAny:
		return true
	}
	return false
}

// ValidRuleMatchType reports whether m is a supported match type.
func ValidRuleMatchType(m RuleMatchType) bool {
	switch m {
	case MatchContains, MatchPrefix, MatchSuffix, MatchExact:
		return true
	}
	return false
}

// CategorizationRule is a single user-defined rule. Rules are grouped by
// Category; a message is assigned the category of the highest-priority matching
// rule (higher Priority wins; ties break on rule order).
type CategorizationRule struct {
	ID       string        `json:"id"`
	Category string        `json:"category"`
	Field    RuleField     `json:"field"`
	Match    RuleMatchType `json:"match"`
	Keyword  string        `json:"keyword"`
	Priority int           `json:"priority"`
	Weight   float64       `json:"weight"`
}

// CategorizationResult is the outcome of applying rules to a message. It always
// carries the extracted Features so a future trained classifier can be layered
// on top of the same signal set without re-plumbing delivery.
type CategorizationResult struct {
	// Category is the winning category, or "" when nothing matched.
	Category string `json:"category"`
	// Confidence is a normalized 0..1 score for the winning category.
	Confidence float64 `json:"confidence"`
	// MatchedRuleIDs lists the IDs of every rule that fired.
	MatchedRuleIDs []string `json:"matched_rule_ids"`
	// Scores is the accumulated weight per category.
	Scores map[string]float64 `json:"scores"`
	// Features is the extracted, classifier-ready feature set for the message.
	Features MessageFeatures `json:"features"`
}

// MessageFeatures is a compact, extensible feature set extracted from a message
// at categorization time. Persisting features (not just verdicts) keeps the
// design open to a trained classifier later.
type MessageFeatures struct {
	FromDomain      string   `json:"from_domain"`
	SubjectTokens   []string `json:"subject_tokens"`
	BodyTokenCount  int      `json:"body_token_count"`
	HasAttachment   bool     `json:"has_attachment"`
	RecipientCount  int      `json:"recipient_count"`
	IsBulk          bool     `json:"is_bulk"`
	HasUnsubscribe  bool     `json:"has_unsubscribe"`
	HasListHeaders  bool     `json:"has_list_headers"`
	TopBodyKeywords []string `json:"top_body_keywords"`
}

// Categorizer applies rules to messages. It is a pure, deterministic engine
// with no persistence; callers load rules from storage and pass them in.
type Categorizer struct{}

// NewCategorizer returns a ready-to-use rule engine.
func NewCategorizer() *Categorizer { return &Categorizer{} }

// ExtractFeatures computes the classifier-ready feature set for a message.
// hasAttachment and listHeaders are supplied by the caller (delivery pipeline)
// because they are not always present on the plain-text Message view.
func (c *Categorizer) ExtractFeatures(m Message, hasAttachment bool, listHeaders bool) MessageFeatures {
	body := stripHTMLish(m.Body)
	bodyLower := strings.ToLower(body)
	bodyTokens := tokenize(body)
	freq := map[string]int{}
	for _, t := range bodyTokens {
		if isStopWord(t) {
			continue
		}
		freq[t]++
	}
	top := topKeywords(freq, 8)

	return MessageFeatures{
		FromDomain:      emailDomainPart(m.FromAddr),
		SubjectTokens:   tokenize(m.Subject),
		BodyTokenCount:  len(bodyTokens),
		HasAttachment:   hasAttachment,
		RecipientCount:  len(m.ToAddrs),
		IsBulk:          len(m.ToAddrs) > 5,
		HasUnsubscribe:  strings.Contains(bodyLower, "unsubscribe"),
		HasListHeaders:  listHeaders,
		TopBodyKeywords: top,
	}
}

// Categorize applies rules to a message and returns the winning category plus
// the full feature set. Rules must be pre-validated by the caller.
func (c *Categorizer) Categorize(m Message, rules []CategorizationRule, hasAttachment bool, listHeaders bool) CategorizationResult {
	features := c.ExtractFeatures(m, hasAttachment, listHeaders)
	scores := map[string]float64{}
	var matched []string

	for _, rule := range rules {
		keyword := strings.ToLower(strings.TrimSpace(rule.Keyword))
		if keyword == "" || strings.TrimSpace(rule.Category) == "" {
			continue
		}
		if !ruleMatches(m, rule, keyword) {
			continue
		}
		weight := rule.Weight
		if weight <= 0 {
			weight = 1
		}
		// Higher-priority rules contribute more, keeping ordering meaningful.
		weight += float64(rule.Priority) * 0.1
		scores[rule.Category] += weight
		if rule.ID != "" {
			matched = append(matched, rule.ID)
		}
	}

	result := CategorizationResult{
		Scores:         scores,
		MatchedRuleIDs: matched,
		Features:       features,
	}
	if len(scores) == 0 {
		return result
	}

	// Deterministic winner selection: highest score, ties break alphabetically.
	var winner string
	var winnerScore float64
	var total float64
	categories := make([]string, 0, len(scores))
	for cat := range scores {
		categories = append(categories, cat)
	}
	sort.Strings(categories)
	for _, cat := range categories {
		score := scores[cat]
		total += score
		if score > winnerScore {
			winnerScore = score
			winner = cat
		}
	}
	result.Category = winner
	if total > 0 {
		result.Confidence = winnerScore / total
	}
	return result
}

func ruleMatches(m Message, rule CategorizationRule, keyword string) bool {
	match := rule.Match
	if !ValidRuleMatchType(match) {
		match = MatchContains
	}
	field := rule.Field
	if !ValidRuleField(field) {
		field = FieldAny
	}
	var haystacks []string
	switch field {
	case FieldFrom:
		haystacks = []string{strings.ToLower(m.FromAddr), strings.ToLower(m.FromName)}
	case FieldSubject:
		haystacks = []string{strings.ToLower(m.Subject)}
	case FieldBody:
		haystacks = []string{strings.ToLower(stripHTMLish(m.Body))}
	case FieldTo:
		haystacks = make([]string, 0, len(m.ToAddrs))
		for _, to := range m.ToAddrs {
			haystacks = append(haystacks, strings.ToLower(to))
		}
	default: // FieldAny
		haystacks = []string{
			strings.ToLower(m.FromAddr),
			strings.ToLower(m.FromName),
			strings.ToLower(m.Subject),
			strings.ToLower(stripHTMLish(m.Body)),
		}
		for _, to := range m.ToAddrs {
			haystacks = append(haystacks, strings.ToLower(to))
		}
	}
	for _, h := range haystacks {
		if fieldMatches(h, keyword, match) {
			return true
		}
	}
	return false
}

func fieldMatches(haystack, keyword string, match RuleMatchType) bool {
	if haystack == "" {
		return false
	}
	switch match {
	case MatchPrefix:
		return strings.HasPrefix(haystack, keyword)
	case MatchSuffix:
		return strings.HasSuffix(haystack, keyword)
	case MatchExact:
		return haystack == keyword
	default:
		return strings.Contains(haystack, keyword)
	}
}

func emailDomainPart(addr string) string {
	addr = strings.ToLower(strings.TrimSpace(addr))
	_, domain, ok := strings.Cut(addr, "@")
	if !ok {
		return ""
	}
	return strings.TrimSpace(domain)
}

func topKeywords(freq map[string]int, max int) []string {
	type kv struct {
		word  string
		count int
	}
	pairs := make([]kv, 0, len(freq))
	for w, c := range freq {
		if !containsLetter(w) {
			continue
		}
		pairs = append(pairs, kv{word: w, count: c})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].word < pairs[j].word
	})
	if len(pairs) > max {
		pairs = pairs[:max]
	}
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.word)
	}
	return out
}
