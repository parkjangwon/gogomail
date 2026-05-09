package spam

import (
	"testing"
)

func TestDecisionEngineAccept(t *testing.T) {
	engine := NewDecisionEngine(DefaultThresholds())

	score := SpamScore{
		Score: 2.0,
		Rules: []string{"BAYES_SPAM"},
	}

	verdict := engine.Decide(score)
	if verdict.Action != ActionAccept {
		t.Fatalf("expected ActionAccept, got %s", verdict.Action)
	}
	if verdict.Score != 2.0 {
		t.Fatalf("score mismatch: %f", verdict.Score)
	}
}

func TestDecisionEngineQuarantine(t *testing.T) {
	engine := NewDecisionEngine(DefaultThresholds())

	score := SpamScore{
		Score: 8.0,
		Rules: []string{"BAYES_SPAM", "PHISHING_NAMEDROP"},
	}

	verdict := engine.Decide(score)
	if verdict.Action != ActionQuarantine {
		t.Fatalf("expected ActionQuarantine, got %s", verdict.Action)
	}
}

func TestDecisionEngineReject(t *testing.T) {
	engine := NewDecisionEngine(DefaultThresholds())

	score := SpamScore{
		Score: 20.0,
		Rules: []string{"BAYES_SPAM", "PHISHING_NAMEDROP", "URI_OBFUSCATION"},
	}

	verdict := engine.Decide(score)
	if verdict.Action != ActionReject {
		t.Fatalf("expected ActionReject, got %s", verdict.Action)
	}
}

func TestDecisionEngineCustomThresholds(t *testing.T) {
	thresholds := Thresholds{
		AcceptScore:     0.0,
		QuarantineScore: 3.0,  // Lower quarantine threshold
		RejectScore:     10.0, // Lower reject threshold
	}
	engine := NewDecisionEngine(thresholds)

	// Score of 4.0 should quarantine with lower thresholds
	score := SpamScore{Score: 4.0}
	verdict := engine.Decide(score)
	if verdict.Action != ActionQuarantine {
		t.Fatalf("expected ActionQuarantine at 4.0 with custom thresholds, got %s", verdict.Action)
	}
}

func TestDecisionEngineNegativeScore(t *testing.T) {
	engine := NewDecisionEngine(DefaultThresholds())

	// Very low score indicates legitimate mail (Rspamd compatible)
	score := SpamScore{
		Score: -5.0,
		Rules: []string{"BAYES_HAM"},
	}

	verdict := engine.Decide(score)
	if verdict.Action != ActionAccept {
		t.Fatalf("expected ActionAccept for negative score, got %s", verdict.Action)
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.AcceptScore != 0.0 {
		t.Fatalf("AcceptScore should be 0.0, got %f", thresholds.AcceptScore)
	}
	if thresholds.QuarantineScore != 5.0 {
		t.Fatalf("QuarantineScore should be 5.0, got %f", thresholds.QuarantineScore)
	}
	if thresholds.RejectScore != 15.0 {
		t.Fatalf("RejectScore should be 15.0, got %f", thresholds.RejectScore)
	}
}

func TestVerdictReason(t *testing.T) {
	engine := NewDecisionEngine(DefaultThresholds())

	score := SpamScore{
		Score: 10.0,
		Rules: []string{"RULE1", "RULE2"},
	}

	verdict := engine.Decide(score)
	if verdict.Reason == "" {
		t.Fatal("Reason should not be empty")
	}
	if verdict.Score != 10.0 {
		t.Fatalf("Verdict score should match input score: %f", verdict.Score)
	}
	if len(verdict.Tags) != 2 {
		t.Fatalf("Tags should have 2 items, got %d", len(verdict.Tags))
	}
}

func TestDecisionEngineBoundaryValues(t *testing.T) {
	engine := NewDecisionEngine(DefaultThresholds())

	tests := []struct {
		score      float64
		expected   Action
		threshold  string
	}{
		{-1.0, ActionAccept, "below accept"},
		{0.0, ActionAccept, "at accept boundary"},
		{4.999, ActionAccept, "below quarantine"},
		{5.0, ActionQuarantine, "at quarantine boundary"},
		{14.999, ActionQuarantine, "below reject"},
		{15.0, ActionReject, "at reject boundary"},
		{100.0, ActionReject, "way above reject"},
	}

	for _, test := range tests {
		score := SpamScore{Score: test.score}
		verdict := engine.Decide(score)
		if verdict.Action != test.expected {
			t.Fatalf("score %.3f (%s): expected %s, got %s", test.score, test.threshold, test.expected, verdict.Action)
		}
	}
}
