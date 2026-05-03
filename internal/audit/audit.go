package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Log struct {
	CompanyID  string
	DomainID   string
	UserID     string
	ActorID    string
	Category   string
	Action     string
	TargetType string
	TargetID   string
	IPAddress  string
	UserAgent  string
	Result     string
	Detail     json.RawMessage
	CreatedAt  time.Time
}

func (l Log) normalized() (Log, error) {
	if l.Category == "" {
		return Log{}, fmt.Errorf("audit category is required")
	}
	if l.Action == "" {
		return Log{}, fmt.Errorf("audit action is required")
	}
	if l.TargetType == "" {
		l.TargetType = ""
	}
	if l.Result == "" {
		return Log{}, fmt.Errorf("audit result is required")
	}
	if len(l.Detail) == 0 {
		l.Detail = json.RawMessage(`{}`)
	}
	if !json.Valid(l.Detail) {
		return Log{}, fmt.Errorf("audit detail must be valid json")
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	} else {
		l.CreatedAt = l.CreatedAt.UTC()
	}
	return l, nil
}

func ComputeHash(prevHash string, log Log) (string, error) {
	normalized, err := log.normalized()
	if err != nil {
		return "", err
	}

	envelope := struct {
		PrevHash   string          `json:"prev_hash"`
		CompanyID  string          `json:"company_id"`
		DomainID   string          `json:"domain_id"`
		UserID     string          `json:"user_id"`
		ActorID    string          `json:"actor_id"`
		Category   string          `json:"category"`
		Action     string          `json:"action"`
		TargetType string          `json:"target_type"`
		TargetID   string          `json:"target_id"`
		IPAddress  string          `json:"ip_address"`
		UserAgent  string          `json:"user_agent"`
		Result     string          `json:"result"`
		Detail     json.RawMessage `json:"detail"`
		CreatedAt  string          `json:"created_at"`
	}{
		PrevHash:   prevHash,
		CompanyID:  normalized.CompanyID,
		DomainID:   normalized.DomainID,
		UserID:     normalized.UserID,
		ActorID:    normalized.ActorID,
		Category:   normalized.Category,
		Action:     normalized.Action,
		TargetType: normalized.TargetType,
		TargetID:   normalized.TargetID,
		IPAddress:  normalized.IPAddress,
		UserAgent:  normalized.UserAgent,
		Result:     normalized.Result,
		Detail:     normalized.Detail,
		CreatedAt:  normalized.CreatedAt.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("marshal audit hash envelope: %w", err)
	}

	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
