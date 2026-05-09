package spam

import (
	"context"
	"fmt"
	"strings"
)

// SpamScore represents spam analysis results.
type SpamScore struct {
	Score              float64
	IsSpam             bool
	Rules              []string
	SymbolsAndWeights  map[string]float64
	RequiredScore      float64
	RecommendedAction  string
}

// Filter performs spam filtering/scanning.
type Filter interface {
	Check(ctx context.Context, from string, to []string, body []byte) (SpamScore, error)
}

// Thresholds defines score-based spam decision boundaries.
type Thresholds struct {
	AcceptScore     float64 // >= this → accept
	QuarantineScore float64 // >= this → quarantine
	RejectScore     float64 // >= this → reject
}

// DecisionEngine converts spam scores to SMTP actions.
type DecisionEngine struct {
	thresholds Thresholds
}

// NewDecisionEngine creates a spam verdict engine with score thresholds.
func NewDecisionEngine(thresholds Thresholds) *DecisionEngine {
	return &DecisionEngine{thresholds: thresholds}
}

// Decide returns the appropriate action based on spam score.
// Score interpretation (Rspamd-compatible):
// - < 0: definitely not spam (highly legitimate)
// - 0-1: not spam
// - 1-5: suspicious
// - 5-15: likely spam
// - > 15: definitely spam
func (de *DecisionEngine) Decide(score SpamScore) Verdict {
	action := ActionAccept
	reason := "no spam indicators"

	switch {
	case score.Score >= de.thresholds.RejectScore:
		action = ActionReject
		reason = fmt.Sprintf("spam score %.2f exceeds reject threshold %.2f", score.Score, de.thresholds.RejectScore)

	case score.Score >= de.thresholds.QuarantineScore:
		action = ActionQuarantine
		reason = fmt.Sprintf("spam score %.2f exceeds quarantine threshold %.2f", score.Score, de.thresholds.QuarantineScore)

	case score.Score >= de.thresholds.AcceptScore:
		action = ActionAccept
		reason = fmt.Sprintf("spam score %.2f (rules: %s)", score.Score, strings.Join(score.Rules, ", "))

	default:
		action = ActionAccept
		reason = fmt.Sprintf("score %.2f below all thresholds", score.Score)
	}

	return Verdict{
		Action: action,
		Score:  score.Score,
		Reason: reason,
		Tags:   score.Rules,
	}
}

// DefaultThresholds returns safe defaults compatible with Rspamd.
// Adjust based on operational needs and false positive tolerance.
func DefaultThresholds() Thresholds {
	return Thresholds{
		AcceptScore:     0.0,      // Accept below 0
		QuarantineScore: 5.0,      // Quarantine 5-15
		RejectScore:     15.0,     // Reject > 15
	}
}
