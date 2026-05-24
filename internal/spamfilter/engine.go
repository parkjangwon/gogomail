package spamfilter

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"unicode"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type Action string

const (
	ActionAccept     Action = "accept"
	ActionQuarantine Action = "quarantine"
	ActionReject     Action = "reject"
	ActionTempfail   Action = "tempfail"
)

type Decision struct {
	Action Action   `json:"action"`
	Score  float64  `json:"score"`
	Reason string   `json:"reason"`
	Rules  []string `json:"rules"`
}

type Engine struct{}

func NewEngine() Engine {
	return Engine{}
}

func (Engine) Evaluate(policy Policy, event smtpd.Event) Decision {
	policy = NormalizePolicy(policy)
	if !policy.Enabled {
		return Decision{Action: ActionAccept, Reason: "spam filter disabled"}
	}

	from := firstNonEmpty(event.Parsed.From.Address, event.EnvelopeFrom)
	if matchesAddressList(from, policy.AllowedSenders) {
		return Decision{Action: ActionAccept, Reason: "sender allowlisted", Rules: []string{"ALLOW_SENDER"}}
	}
	if matchesAddressList(from, policy.BlockedSenders) {
		return decisionForPolicy(policy, float64(policy.SpamThreshold+10), "sender blocklisted", []string{"BLOCK_SENDER"})
	}

	score := 0.0
	var rules []string
	add := func(points float64, rule string) {
		score += points
		rules = append(rules, rule)
	}

	switch event.Authentication.DMARC.Result {
	case smtpd.AuthResultFail:
		add(4, "DMARC_FAIL")
		if policy.StrictAuthEnabled {
			add(2, "STRICT_DMARC_FAIL")
		}
	case smtpd.AuthResultTemporary:
		add(1, "DMARC_TEMPERROR")
	}
	switch event.Authentication.SPF.Result {
	case smtpd.AuthResultFail:
		add(2, "SPF_FAIL")
		if policy.StrictAuthEnabled {
			add(1, "STRICT_SPF_FAIL")
		}
	case smtpd.AuthResultPermanent:
		add(1, "SPF_PERMERROR")
	case smtpd.AuthResultTemporary:
		add(1, "SPF_TEMPERROR")
	case smtpd.AuthResultNone:
		if policy.StrictAuthEnabled {
			add(1, "STRICT_SPF_NONE")
		}
	}
	switch event.Authentication.DKIM.Result {
	case smtpd.AuthResultFail:
		add(2, "DKIM_FAIL")
		if policy.StrictAuthEnabled {
			add(1, "STRICT_DKIM_FAIL")
		}
	case smtpd.AuthResultNone:
		add(1, "DKIM_NONE")
	}
	if policy.StrictAuthEnabled && event.Authentication.SPF.Result != smtpd.AuthResultPass && event.Authentication.DKIM.Result != smtpd.AuthResultPass && event.Authentication.DMARC.Result != smtpd.AuthResultPass {
		add(2, "STRICT_NO_AUTH_PASS")
	}
	if isSuspiciousSubject(event.Parsed.Subject) {
		add(2, "SUSPICIOUS_SUBJECT")
	}
	if isSuspiciousBody(event.Parsed.TextBody) || isSuspiciousBody(event.Parsed.HTMLBody) {
		add(2, "SUSPICIOUS_BODY")
	}
	if event.Size > 0 && policy.MaxAttachmentMB > 0 && event.Parsed.HasAttachment && event.Size > int64(policy.MaxAttachmentMB)*1024*1024 {
		add(3, "ATTACHMENT_SIZE_LIMIT")
	}
	if policy.VirusScanEnabled {
		for _, attachment := range event.Parsed.Attachments {
			filename := strings.ToLower(strings.TrimSpace(attachment.Filename))
			ext := filepath.Ext(filename)
			if ext != "" && contains(policy.BlockedExtensions, ext) {
				add(10, "BLOCKED_ATTACHMENT_EXTENSION")
				break
			}
			if hasDoubleExecutableExtension(filename, policy.BlockedExtensions) {
				add(8, "DOUBLE_EXTENSION_ATTACHMENT")
				break
			}
		}
	}
	if policy.BulkRecipientLimit > 0 && len(event.Recipients) > policy.BulkRecipientLimit {
		add(3, "BULK_RECIPIENT_COUNT")
	}
	if ip := remoteIP(event.RemoteAddr); ip != "" && isLikelyDynamicPTRAbsent(event.Authentication.SPF.Result, ip) {
		add(1, "UNAUTHENTICATED_REMOTE")
	}
	if packScore, packRules := evaluateFilterPacks(policy, event); packScore > 0 {
		score += packScore
		rules = append(rules, packRules...)
	}
	if score <= 0 {
		return Decision{Action: ActionAccept, Score: 0, Reason: "no spam indicators"}
	}
	return decisionForPolicy(policy, score, fmt.Sprintf("spam score %.1f", score), rules)
}

func decisionForPolicy(policy Policy, score float64, reason string, rules []string) Decision {
	if score < float64(policy.SpamThreshold) {
		return Decision{Action: ActionAccept, Score: score, Reason: reason, Rules: rules}
	}
	if policy.QuarantineEnabled {
		return Decision{Action: ActionQuarantine, Score: score, Reason: reason, Rules: rules}
	}
	return Decision{Action: ActionReject, Score: score, Reason: reason, Rules: rules}
}

func matchesAddressList(addr string, patterns []string) bool {
	addr = strings.ToLower(strings.TrimSpace(strings.Trim(addr, "<>")))
	if addr == "" {
		return false
	}
	_, domain, _ := strings.Cut(addr, "@")
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		switch {
		case pattern == addr:
			return true
		case strings.HasPrefix(pattern, "@") && domain == strings.TrimPrefix(pattern, "@"):
			return true
		case strings.HasPrefix(pattern, "*@") && domain == strings.TrimPrefix(pattern, "*@"):
			return true
		}
	}
	return false
}

func isSuspiciousSubject(subject string) bool {
	subject = spamTextFold(subject)
	if subject == "" {
		return false
	}
	phrases := []string{"urgent", "password expired", "verify your account", "wire transfer", "crypto giveaway", "무료", "당첨", "긴급", "비밀번호 만료", "계정 확인"}
	for _, phrase := range phrases {
		if strings.Contains(subject, phrase) {
			return true
		}
	}
	return false
}

func isSuspiciousBody(body string) bool {
	body = spamTextFold(stripHTML(body))
	if body == "" {
		return false
	}
	phrases := []string{
		"verify your account", "password expired", "reset your password",
		"gift card", "crypto giveaway", "wire transfer", "login immediately",
		"계정 확인", "비밀번호 만료", "송금", "상품권", "긴급 로그인",
	}
	for _, phrase := range phrases {
		if strings.Contains(body, phrase) {
			return true
		}
	}
	return false
}

func hasDoubleExecutableExtension(filename string, blocked []string) bool {
	parts := strings.Split(filename, ".")
	if len(parts) < 3 {
		return false
	}
	last := "." + parts[len(parts)-1]
	prev := "." + parts[len(parts)-2]
	return contains(blocked, last) && !contains([]string{".txt", ".pdf", ".doc", ".docx", ".jpg", ".png"}, prev)
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	if net.ParseIP(host) == nil {
		return ""
	}
	return host
}

func isLikelyDynamicPTRAbsent(spf smtpd.AuthResult, ip string) bool {
	return ip != "" && spf != smtpd.AuthResultPass
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func spamTextFold(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if isZeroWidth(r) {
			continue
		}
		if mapped, ok := confusableRune(r); ok {
			r = mapped
		}
		if unicode.IsSpace(r) {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func isZeroWidth(r rune) bool {
	switch r {
	case '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff':
		return true
	default:
		return false
	}
}

func containsObfuscation(value string) bool {
	for _, r := range value {
		if isZeroWidth(r) {
			return true
		}
		if _, ok := confusableRune(r); ok {
			return true
		}
	}
	return false
}

func confusableRune(r rune) (rune, bool) {
	switch r {
	case 'а', 'А', 'α', 'Α':
		return 'a', true
	case 'е', 'Е', 'ε', 'Ε':
		return 'e', true
	case 'о', 'О', 'ο', 'Ο':
		return 'o', true
	case 'р', 'Р', 'ρ', 'Ρ':
		return 'p', true
	case 'с', 'С':
		return 'c', true
	case 'х', 'Х', 'χ', 'Χ':
		return 'x', true
	case 'у', 'У':
		return 'y', true
	case 'і', 'І':
		return 'i', true
	case 'ѕ', 'Ѕ':
		return 's', true
	case 'ӏ':
		return 'l', true
	default:
		return 0, false
	}
}
