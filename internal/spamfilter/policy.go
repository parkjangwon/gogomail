package spamfilter

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

const PolicyConfigKey = "spam_filter_policy"

type Policy struct {
	Enabled           bool     `json:"enabled"`
	SpamThreshold     int      `json:"spam_threshold"`
	VirusScanEnabled  bool     `json:"virus_scan_enabled"`
	BlockedExtensions []string `json:"blocked_extensions"`
	BlockedSenders    []string `json:"blocked_senders"`
	AllowedSenders    []string `json:"allowed_senders"`
	QuarantineEnabled bool     `json:"quarantine_enabled"`
	MaxAttachmentMB   int      `json:"max_attachment_mb"`
}

func DefaultPolicy() Policy {
	return Policy{
		Enabled:           true,
		SpamThreshold:     5,
		VirusScanEnabled:  true,
		BlockedExtensions: []string{".exe", ".bat", ".cmd", ".com", ".js", ".jse", ".pif", ".scr", ".vbs", ".vbe", ".wsf", ".ps1", ".jar"},
		BlockedSenders:    []string{},
		AllowedSenders:    []string{},
		QuarantineEnabled: true,
		MaxAttachmentMB:   25,
	}
}

func DecodePolicy(raw json.RawMessage) (Policy, error) {
	policy := DefaultPolicy()
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &policy); err != nil {
			return Policy{}, err
		}
	}
	return NormalizePolicy(policy), nil
}

func NormalizePolicy(policy Policy) Policy {
	if policy.SpamThreshold < 1 {
		policy.SpamThreshold = 1
	}
	if policy.SpamThreshold > 10 {
		policy.SpamThreshold = 10
	}
	if policy.MaxAttachmentMB < 0 {
		policy.MaxAttachmentMB = 0
	}
	policy.BlockedExtensions = normalizeExts(policy.BlockedExtensions)
	policy.BlockedSenders = normalizeList(policy.BlockedSenders)
	policy.AllowedSenders = normalizeList(policy.AllowedSenders)
	return policy
}

func normalizeList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || strings.ContainsAny(value, "\r\n") || len(value) > 320 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeExts(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || strings.ContainsAny(value, "\r\n/\\") || len(value) > 32 {
			continue
		}
		if !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		value = filepath.Ext("x" + value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
