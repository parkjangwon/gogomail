package spamfilter

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

const PolicyConfigKey = "spam_filter_policy"
const maxRBLZones = 20

type Policy struct {
	Enabled            bool             `json:"enabled"`
	SpamThreshold      int              `json:"spam_threshold"`
	VirusScanEnabled   bool             `json:"virus_scan_enabled"`
	StrictAuthEnabled  bool             `json:"strict_auth_enabled"`
	RBLCheckEnabled    bool             `json:"rbl_check_enabled"`
	RBLRejectEnabled   bool             `json:"rbl_reject_enabled"`
	RBLZones           []string         `json:"rbl_zones"`
	BlockedExtensions  []string         `json:"blocked_extensions"`
	BlockedSenders     []string         `json:"blocked_senders"`
	AllowedSenders     []string         `json:"allowed_senders"`
	QuarantineEnabled  bool             `json:"quarantine_enabled"`
	MaxAttachmentMB    int              `json:"max_attachment_mb"`
	BulkRecipientLimit int              `json:"bulk_recipient_limit"`
	FilterPacks        FilterPackBundle `json:"filter_packs"`
}

func DefaultPolicy() Policy {
	return Policy{
		Enabled:           true,
		SpamThreshold:     5,
		VirusScanEnabled:  true,
		StrictAuthEnabled: true,
		RBLCheckEnabled:   false,
		RBLRejectEnabled:  true,
		RBLZones:          []string{},
		BlockedExtensions: []string{
			".bat", ".chm", ".cmd", ".com", ".dll", ".docm", ".exe", ".hta",
			".img", ".iso", ".jar", ".js", ".jse", ".lnk", ".msi", ".pif",
			".pptm", ".ps1", ".reg", ".scr", ".vbe", ".vbs", ".wsf", ".xlam",
			".xlsm",
		},
		BlockedSenders:     []string{},
		AllowedSenders:     []string{},
		QuarantineEnabled:  true,
		MaxAttachmentMB:    25,
		BulkRecipientLimit: 50,
		FilterPacks:        DefaultFilterPackBundle(),
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
	if policy.BulkRecipientLimit < 1 {
		policy.BulkRecipientLimit = 50
	}
	if policy.BulkRecipientLimit > 500 {
		policy.BulkRecipientLimit = 500
	}
	policy.BlockedExtensions = normalizeExts(policy.BlockedExtensions)
	policy.BlockedSenders = normalizeList(policy.BlockedSenders)
	policy.AllowedSenders = normalizeList(policy.AllowedSenders)
	policy.RBLZones = normalizeZones(policy.RBLZones)
	policy.FilterPacks = NormalizeFilterPackBundle(policy.FilterPacks)
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

func normalizeZones(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, min(len(values), maxRBLZones))
	for _, value := range values {
		if len(out) >= maxRBLZones {
			break
		}
		value = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(value, ".")))
		if !validDNSZone(value) {
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

func validDNSZone(value string) bool {
	if value == "" || len(value) > 253 || strings.ContainsAny(value, "\r\n\t /\\:@") || !strings.Contains(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}
