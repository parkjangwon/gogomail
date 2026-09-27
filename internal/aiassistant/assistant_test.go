package aiassistant

import (
	"context"
	"strings"
	"testing"
	"time"
)

func sampleThread() Thread {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	return Thread{
		Subject: "Q1 Planning",
		Messages: []Message{
			{
				ID:         "m1",
				FromName:   "Alice Kim",
				FromAddr:   "alice@example.com",
				ToAddrs:    []string{"bob@example.com"},
				Subject:    "Q1 Planning",
				Body:       "Let's align on the Q1 roadmap. Could you send me the budget draft by Friday? The roadmap must cover marketing and engineering.",
				ReceivedAt: base,
			},
			{
				ID:         "m2",
				FromName:   "Bob Lee",
				FromAddr:   "bob@example.com",
				ToAddrs:    []string{"alice@example.com"},
				Subject:    "Re: Q1 Planning",
				Body:       "Sure. I'll prepare the budget draft. Please confirm the marketing headcount.\n> Let's align on the Q1 roadmap.",
				ReceivedAt: base.Add(time.Hour),
			},
		},
	}
}

func TestLocalSummarizerBasics(t *testing.T) {
	s := NewLocalSummarizer()
	if s.Name() != "local" {
		t.Fatalf("expected provider name 'local', got %q", s.Name())
	}
	if !s.Offline() {
		t.Fatal("local summarizer must report offline")
	}
	sum, err := s.Summarize(context.Background(), sampleThread())
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if sum.Subject != "Q1 Planning" {
		t.Errorf("subject = %q", sum.Subject)
	}
	if sum.MessageCount != 2 {
		t.Errorf("message count = %d", sum.MessageCount)
	}
	if !sum.GeneratedOffline {
		t.Error("expected generated_offline true")
	}
	if len(sum.Participants) < 2 {
		t.Errorf("expected >=2 participants, got %v", sum.Participants)
	}
	// alice and bob should both appear.
	joined := strings.Join(sum.Participants, " ")
	if !strings.Contains(joined, "alice@example.com") || !strings.Contains(joined, "bob@example.com") {
		t.Errorf("participants missing expected addresses: %v", sum.Participants)
	}
	if len(sum.KeyPoints) == 0 {
		t.Error("expected at least one key point")
	}
	if len(sum.ActionItems) == 0 {
		t.Error("expected at least one action item")
	}
	if sum.SuggestedReply == "" {
		t.Error("expected a suggested reply")
	}
}

func TestLocalSummarizerExcludesQuotedLines(t *testing.T) {
	s := NewLocalSummarizer()
	sum, err := s.Summarize(context.Background(), sampleThread())
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	for _, kp := range sum.KeyPoints {
		if strings.HasPrefix(strings.TrimSpace(kp), ">") {
			t.Errorf("key point should not include quoted line: %q", kp)
		}
	}
}

func TestLocalSummarizerDeterministic(t *testing.T) {
	s := NewLocalSummarizer()
	a, err := s.Summarize(context.Background(), sampleThread())
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Summarize(context.Background(), sampleThread())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(a.KeyPoints, "|") != strings.Join(b.KeyPoints, "|") {
		t.Error("key points are not deterministic")
	}
	if a.SuggestedReply != b.SuggestedReply {
		t.Error("suggested reply is not deterministic")
	}
}

func TestLocalSummarizerTruncates(t *testing.T) {
	s := &LocalSummarizer{MaxMessages: 1}
	th := sampleThread()
	sum, err := s.Summarize(context.Background(), th)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.Truncated {
		t.Error("expected truncated=true when messages exceed MaxMessages")
	}
	if sum.MessageCount != 1 {
		t.Errorf("message count = %d, want 1", sum.MessageCount)
	}
}

func TestLocalSummarizerContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewLocalSummarizer().Summarize(ctx, sampleThread())
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestActionItemOwnerAttribution(t *testing.T) {
	s := NewLocalSummarizer()
	sum, err := s.Summarize(context.Background(), sampleThread())
	if err != nil {
		t.Fatal(err)
	}
	foundOwner := false
	for _, ai := range sum.ActionItems {
		if ai.Owner != "" {
			foundOwner = true
		}
	}
	if !foundOwner {
		t.Error("expected at least one action item with an owner")
	}
}

func TestCategorizerRuleMatching(t *testing.T) {
	c := NewCategorizer()
	rules := []CategorizationRule{
		{ID: "r1", Category: "Newsletters", Field: FieldFrom, Match: MatchContains, Keyword: "news@", Weight: 1},
		{ID: "r2", Category: "Finance", Field: FieldSubject, Match: MatchContains, Keyword: "invoice", Weight: 2},
	}
	msg := Message{
		FromAddr: "news@vendor.com",
		Subject:  "Your monthly newsletter",
		Body:     "Latest updates. Unsubscribe here.",
		ToAddrs:  []string{"me@example.com"},
	}
	res := c.Categorize(msg, rules, false, false)
	if res.Category != "Newsletters" {
		t.Errorf("category = %q, want Newsletters", res.Category)
	}
	if len(res.MatchedRuleIDs) != 1 || res.MatchedRuleIDs[0] != "r1" {
		t.Errorf("matched rules = %v", res.MatchedRuleIDs)
	}
	if !res.Features.HasUnsubscribe {
		t.Error("expected HasUnsubscribe feature")
	}
	if res.Features.FromDomain != "vendor.com" {
		t.Errorf("from domain = %q", res.Features.FromDomain)
	}
}

func TestCategorizerPriorityWins(t *testing.T) {
	c := NewCategorizer()
	rules := []CategorizationRule{
		{ID: "low", Category: "General", Field: FieldSubject, Match: MatchContains, Keyword: "report", Weight: 1, Priority: 0},
		{ID: "high", Category: "Finance", Field: FieldSubject, Match: MatchContains, Keyword: "report", Weight: 1, Priority: 10},
	}
	msg := Message{Subject: "Quarterly report", ToAddrs: []string{"me@example.com"}}
	res := c.Categorize(msg, rules, false, false)
	if res.Category != "Finance" {
		t.Errorf("category = %q, want Finance (higher priority)", res.Category)
	}
	if res.Confidence <= 0 || res.Confidence > 1 {
		t.Errorf("confidence out of range: %f", res.Confidence)
	}
}

func TestCategorizerNoMatch(t *testing.T) {
	c := NewCategorizer()
	rules := []CategorizationRule{
		{ID: "r1", Category: "Finance", Field: FieldSubject, Match: MatchContains, Keyword: "invoice", Weight: 1},
	}
	msg := Message{Subject: "lunch plans", ToAddrs: []string{"a@b.com", "c@d.com"}}
	res := c.Categorize(msg, rules, true, false)
	if res.Category != "" {
		t.Errorf("expected no category, got %q", res.Category)
	}
	// Features must still be extracted even with no match.
	if !res.Features.HasAttachment {
		t.Error("expected HasAttachment feature to be preserved")
	}
	if res.Features.RecipientCount != 2 {
		t.Errorf("recipient count = %d", res.Features.RecipientCount)
	}
}

func TestCategorizerMatchTypes(t *testing.T) {
	c := NewCategorizer()
	msg := Message{FromAddr: "alerts@ci.example.com", Subject: "BUILD FAILED", ToAddrs: []string{"me@x.com"}}
	cases := []struct {
		rule CategorizationRule
		want bool
	}{
		{CategorizationRule{Category: "CI", Field: FieldFrom, Match: MatchPrefix, Keyword: "alerts@", Weight: 1}, true},
		{CategorizationRule{Category: "CI", Field: FieldFrom, Match: MatchSuffix, Keyword: "example.com", Weight: 1}, true},
		{CategorizationRule{Category: "CI", Field: FieldSubject, Match: MatchExact, Keyword: "build failed", Weight: 1}, true},
		{CategorizationRule{Category: "CI", Field: FieldSubject, Match: MatchExact, Keyword: "build", Weight: 1}, false},
	}
	for i, tc := range cases {
		res := c.Categorize(msg, []CategorizationRule{tc.rule}, false, false)
		got := res.Category == "CI"
		if got != tc.want {
			t.Errorf("case %d: got match=%v want %v", i, got, tc.want)
		}
	}
}

func TestComposeMissingAttachmentCheck(t *testing.T) {
	a := NewComposeAssistant()
	res := a.Assist(ComposeDraft{
		Subject:         "Report",
		Body:            "Please find the report attached.",
		Recipients:      []string{"boss@example.com"},
		AttachmentCount: 0,
	})
	if !hasCheck(res.Checks, "missing_attachment") {
		t.Error("expected missing_attachment check")
	}

	res2 := a.Assist(ComposeDraft{
		Subject:         "Report",
		Body:            "Please find the report attached.",
		Recipients:      []string{"boss@example.com"},
		AttachmentCount: 1,
	})
	if hasCheck(res2.Checks, "missing_attachment") {
		t.Error("did not expect missing_attachment check when file is attached")
	}
}

func TestComposeSubjectSuggestions(t *testing.T) {
	a := NewComposeAssistant()
	// With an existing subject.
	res := a.Assist(ComposeDraft{Subject: "Budget", Body: "hello", Recipients: []string{"x@y.com"}})
	if len(res.SubjectSuggestions) == 0 {
		t.Fatal("expected subject suggestions")
	}
	// Without a subject: should derive from body and flag empty_subject.
	res2 := a.Assist(ComposeDraft{Body: "Please review the marketing plan for next quarter.", Recipients: []string{"x@y.com"}})
	if len(res2.SubjectSuggestions) == 0 {
		t.Error("expected derived subject suggestions")
	}
	if !hasCheck(res2.Checks, "empty_subject") {
		t.Error("expected empty_subject check")
	}
}

func TestComposeVariants(t *testing.T) {
	a := NewComposeAssistant()
	res := a.Assist(ComposeDraft{Subject: "Hi", Body: "First point. Second point.", Recipients: []string{"x@y.com"}})
	labels := map[string]bool{}
	for _, v := range res.Variants {
		labels[v.Label] = true
	}
	for _, want := range []string{"concise", "formal", "friendly"} {
		if !labels[want] {
			t.Errorf("missing variant %q", want)
		}
	}
}

func TestComposeEmptyChecks(t *testing.T) {
	a := NewComposeAssistant()
	res := a.Assist(ComposeDraft{})
	if !hasCheck(res.Checks, "empty_body") || !hasCheck(res.Checks, "empty_subject") || !hasCheck(res.Checks, "no_recipients") {
		t.Errorf("expected empty checks, got %+v", res.Checks)
	}
}

func hasCheck(checks []ComposeCheck, code string) bool {
	for _, c := range checks {
		if c.Code == code {
			return true
		}
	}
	return false
}
