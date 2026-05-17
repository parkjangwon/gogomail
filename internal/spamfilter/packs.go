package spamfilter

import (
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

const (
	RuleTypePhrase              = "phrase"
	RuleTypeAttachmentExtension = "attachment_extension"
	RuleTypeBulkRecipient       = "bulk_recipient"
	RuleTypeAuthFailure         = "auth_failure"

	RuleTargetSubject     = "subject"
	RuleTargetBody        = "body"
	RuleTargetSubjectBody = "subject_body"
)

const (
	maxEnabledFilterPacks = 50
	maxCustomFilterPacks  = 50
	maxRulesPerPack       = 200
	maxPatternsPerRule    = 100
	maxPatternBytes       = 200
	maxPackIDBytes        = 120
)

var filterPackIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,119}$`)

type FilterPackBundle struct {
	EnabledPackIDs []string     `json:"enabled_pack_ids"`
	CustomPacks    []FilterPack `json:"custom_packs"`
}

type FilterPack struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Source      string                 `json:"source"`
	Enabled     bool                   `json:"enabled"`
	Rules       []FilterRuleDefinition `json:"rules"`
}

type FilterRuleDefinition struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Target   string   `json:"target"`
	Patterns []string `json:"patterns"`
	Score    float64  `json:"score"`
	Enabled  bool     `json:"enabled"`
	Action   Action   `json:"action,omitempty"`
}

func DefaultFilterPackBundle() FilterPackBundle {
	return FilterPackBundle{
		EnabledPackIDs: []string{
			"gogomail-core-auth",
			"gogomail-core-malware",
			"gogomail-core-phishing-ko",
			"gogomail-core-bulk",
		},
		CustomPacks: []FilterPack{},
	}
}

func BuiltinFilterPacks() []FilterPack {
	return []FilterPack{
		{
			ID:          "gogomail-core-auth",
			Version:     "2026.05.17",
			Name:        "Core authentication defense",
			Description: "Scores suspicious SPF, DKIM, and DMARC failure combinations.",
			Category:    "authentication",
			Source:      "system",
			Enabled:     true,
			Rules: []FilterRuleDefinition{
				{ID: "no-auth-pass", Type: RuleTypeAuthFailure, Patterns: []string{"no_auth_pass"}, Score: 1.5, Enabled: true},
				{ID: "dmarc-fail", Type: RuleTypeAuthFailure, Patterns: []string{"dmarc_fail"}, Score: 1.5, Enabled: true},
			},
		},
		{
			ID:          "gogomail-core-malware",
			Version:     "2026.05.17",
			Name:        "Core malware attachment defense",
			Description: "Scores high-risk executable and macro attachment extensions.",
			Category:    "malware",
			Source:      "system",
			Enabled:     true,
			Rules: []FilterRuleDefinition{
				{ID: "dangerous-extension", Type: RuleTypeAttachmentExtension, Patterns: []string{".exe", ".scr", ".js", ".vbs", ".ps1", ".jar", ".docm", ".xlsm"}, Score: 2, Enabled: true},
			},
		},
		{
			ID:          "gogomail-core-phishing-ko",
			Version:     "2026.05.17",
			Name:        "Korean and global phishing phrases",
			Description: "Scores common credential theft, urgency, and payment-lure phrases.",
			Category:    "phishing",
			Source:      "system",
			Enabled:     true,
			Rules: []FilterRuleDefinition{
				{ID: "credential-lures", Type: RuleTypePhrase, Target: RuleTargetSubjectBody, Patterns: []string{"verify your account", "password expired", "login immediately", "계정 확인", "비밀번호 만료", "긴급 로그인"}, Score: 1.5, Enabled: true},
				{ID: "payment-lures", Type: RuleTypePhrase, Target: RuleTargetSubjectBody, Patterns: []string{"wire transfer", "gift card", "crypto giveaway", "송금", "상품권", "당첨"}, Score: 1, Enabled: true},
			},
		},
		{
			ID:          "gogomail-core-bulk",
			Version:     "2026.05.17",
			Name:        "Bulk receive pressure defense",
			Description: "Scores messages above the tenant bulk recipient threshold.",
			Category:    "bulk",
			Source:      "system",
			Enabled:     true,
			Rules: []FilterRuleDefinition{
				{ID: "recipient-fanout", Type: RuleTypeBulkRecipient, Score: 1.5, Enabled: true},
			},
		},
	}
}

func NormalizeFilterPackBundle(bundle FilterPackBundle) FilterPackBundle {
	if bundle.EnabledPackIDs == nil && bundle.CustomPacks == nil {
		return DefaultFilterPackBundle()
	}
	enabled := normalizeFilterPackIDs(bundle.EnabledPackIDs, maxEnabledFilterPacks)
	custom := normalizeCustomFilterPacks(bundle.CustomPacks)
	return FilterPackBundle{EnabledPackIDs: enabled, CustomPacks: custom}
}

func FilterPackCatalog(bundle FilterPackBundle) []FilterPack {
	bundle = NormalizeFilterPackBundle(bundle)
	enabled := make(map[string]struct{}, len(bundle.EnabledPackIDs))
	for _, id := range bundle.EnabledPackIDs {
		enabled[id] = struct{}{}
	}
	catalog := BuiltinFilterPacks()
	for i := range catalog {
		_, catalog[i].Enabled = enabled[catalog[i].ID]
	}
	for _, pack := range bundle.CustomPacks {
		pack.Enabled = slices.Contains(bundle.EnabledPackIDs, pack.ID)
		catalog = append(catalog, pack)
	}
	sort.SliceStable(catalog, func(i, j int) bool {
		if catalog[i].Source != catalog[j].Source {
			return catalog[i].Source == "system"
		}
		return catalog[i].ID < catalog[j].ID
	})
	return catalog
}

func evaluateFilterPacks(policy Policy, event smtpd.Event) (float64, []string) {
	catalog := FilterPackCatalog(policy.FilterPacks)
	score := 0.0
	var rules []string
	for _, pack := range catalog {
		if !pack.Enabled {
			continue
		}
		for _, rule := range pack.Rules {
			if !rule.Enabled || rule.Score <= 0 {
				continue
			}
			if filterRuleMatches(rule, policy, event) {
				score += rule.Score
				rules = append(rules, "PACK:"+pack.ID+":"+rule.ID)
				if rule.Action == ActionReject {
					score += float64(policy.SpamThreshold + 10)
				}
			}
		}
	}
	return score, rules
}

func filterRuleMatches(rule FilterRuleDefinition, policy Policy, event smtpd.Event) bool {
	switch rule.Type {
	case RuleTypePhrase:
		haystack := strings.ToLower(filterRuleText(rule.Target, event))
		if haystack == "" {
			return false
		}
		for _, pattern := range rule.Patterns {
			if pattern != "" && strings.Contains(haystack, strings.ToLower(pattern)) {
				return true
			}
		}
	case RuleTypeAttachmentExtension:
		patterns := normalizeExts(rule.Patterns)
		for _, attachment := range event.Parsed.Attachments {
			ext := filepath.Ext(strings.ToLower(strings.TrimSpace(attachment.Filename)))
			if ext != "" && contains(patterns, ext) {
				return true
			}
		}
	case RuleTypeBulkRecipient:
		limit := policy.BulkRecipientLimit
		if len(rule.Patterns) > 0 {
			if parsed, err := strconv.Atoi(rule.Patterns[0]); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		return limit > 0 && len(event.Recipients) > limit
	case RuleTypeAuthFailure:
		for _, pattern := range rule.Patterns {
			switch pattern {
			case "dmarc_fail":
				if event.Authentication.DMARC.Result == smtpd.AuthResultFail {
					return true
				}
			case "spf_fail":
				if event.Authentication.SPF.Result == smtpd.AuthResultFail {
					return true
				}
			case "dkim_fail":
				if event.Authentication.DKIM.Result == smtpd.AuthResultFail {
					return true
				}
			case "no_auth_pass":
				if event.Authentication.SPF.Result != smtpd.AuthResultPass && event.Authentication.DKIM.Result != smtpd.AuthResultPass && event.Authentication.DMARC.Result != smtpd.AuthResultPass {
					return true
				}
			}
		}
	}
	return false
}

func filterRuleText(target string, event smtpd.Event) string {
	switch target {
	case RuleTargetSubject:
		return event.Parsed.Subject
	case RuleTargetBody:
		return event.Parsed.TextBody + "\n" + event.Parsed.HTMLBody
	default:
		return event.Parsed.Subject + "\n" + event.Parsed.TextBody + "\n" + event.Parsed.HTMLBody
	}
}

func normalizeFilterPackIDs(values []string, maxItems int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, min(len(values), maxItems))
	for _, value := range values {
		if len(out) >= maxItems {
			break
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if !filterPackIDPattern.MatchString(value) {
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

func normalizeCustomFilterPacks(values []FilterPack) []FilterPack {
	seen := make(map[string]struct{}, len(values))
	out := make([]FilterPack, 0, min(len(values), maxCustomFilterPacks))
	for _, pack := range values {
		if len(out) >= maxCustomFilterPacks {
			break
		}
		pack.ID = strings.ToLower(strings.TrimSpace(pack.ID))
		if !filterPackIDPattern.MatchString(pack.ID) || strings.HasPrefix(pack.ID, "gogomail-core-") {
			continue
		}
		if _, ok := seen[pack.ID]; ok {
			continue
		}
		pack.Version = sanitizePackText(pack.Version, 40)
		if pack.Version == "" {
			pack.Version = "custom"
		}
		pack.Name = sanitizePackText(pack.Name, 120)
		if pack.Name == "" {
			pack.Name = pack.ID
		}
		pack.Description = sanitizePackText(pack.Description, 500)
		pack.Category = sanitizePackText(strings.ToLower(pack.Category), 40)
		if pack.Category == "" {
			pack.Category = "custom"
		}
		pack.Source = "custom"
		pack.Enabled = true
		pack.Rules = normalizeFilterRules(pack.Rules)
		if len(pack.Rules) == 0 {
			continue
		}
		seen[pack.ID] = struct{}{}
		out = append(out, pack)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func normalizeFilterRules(values []FilterRuleDefinition) []FilterRuleDefinition {
	seen := make(map[string]struct{}, len(values))
	out := make([]FilterRuleDefinition, 0, min(len(values), maxRulesPerPack))
	for _, rule := range values {
		if len(out) >= maxRulesPerPack {
			break
		}
		rule.ID = strings.ToLower(strings.TrimSpace(rule.ID))
		if !filterPackIDPattern.MatchString(rule.ID) {
			continue
		}
		if _, ok := seen[rule.ID]; ok {
			continue
		}
		rule.Type = strings.ToLower(strings.TrimSpace(rule.Type))
		if !validRuleType(rule.Type) {
			continue
		}
		rule.Target = strings.ToLower(strings.TrimSpace(rule.Target))
		if rule.Target == "" {
			rule.Target = RuleTargetSubjectBody
		}
		if rule.Type == RuleTypePhrase && !validPhraseTarget(rule.Target) {
			continue
		}
		if rule.Score < 0 {
			rule.Score = 0
		}
		if rule.Score > 20 {
			rule.Score = 20
		}
		if rule.Score == 0 {
			continue
		}
		rule.Patterns = normalizeRulePatterns(rule.Type, rule.Patterns)
		if requiresPatterns(rule.Type) && len(rule.Patterns) == 0 {
			continue
		}
		if rule.Action != "" && rule.Action != ActionQuarantine && rule.Action != ActionReject {
			rule.Action = ""
		}
		rule.Enabled = true
		seen[rule.ID] = struct{}{}
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func normalizeRulePatterns(ruleType string, values []string) []string {
	if ruleType == RuleTypeAttachmentExtension {
		return normalizeExts(values)
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, min(len(values), maxPatternsPerRule))
	for _, value := range values {
		if len(out) >= maxPatternsPerRule {
			break
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || len(value) > maxPatternBytes || strings.ContainsAny(value, "\r\n") || hasControl(value) {
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

func validRuleType(value string) bool {
	switch value {
	case RuleTypePhrase, RuleTypeAttachmentExtension, RuleTypeBulkRecipient, RuleTypeAuthFailure:
		return true
	default:
		return false
	}
}

func validPhraseTarget(value string) bool {
	switch value {
	case RuleTargetSubject, RuleTargetBody, RuleTargetSubjectBody:
		return true
	default:
		return false
	}
}

func requiresPatterns(ruleType string) bool {
	return ruleType != RuleTypeBulkRecipient
}

func sanitizePackText(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") || hasControl(value) {
		return ""
	}
	if len(value) > maxBytes {
		value = value[:maxBytes]
	}
	return value
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 && r != '\t' {
			return true
		}
	}
	return false
}
