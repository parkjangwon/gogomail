package tlsrpt

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// Policy represents a TLS-RPT policy from DNS TXT record (RFC 8460 §3).
// Format: "v=TLSRPTv1; rua=<mailto:address>; [ruf=<mailto:address>]"
type Policy struct {
	Version string // "TLSRPTv1"
	RUA     string // Aggregate report mailbox (required)
	RUF     string // Failure report mailbox (optional)
}

// ParsePolicy parses a TLS-RPT policy from DNS TXT record.
func ParsePolicy(txt string) (*Policy, error) {
	policy := &Policy{}

	// Split by semicolon (RFC 8460 §3)
	parts := strings.Split(txt, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasPrefix(part, "v=") {
			policy.Version = strings.TrimSpace(strings.TrimPrefix(part, "v="))
		} else if strings.HasPrefix(part, "rua=") {
			policy.RUA = strings.TrimSpace(strings.TrimPrefix(part, "rua="))
		} else if strings.HasPrefix(part, "ruf=") {
			policy.RUF = strings.TrimSpace(strings.TrimPrefix(part, "ruf="))
		}
	}

	// Validate required fields
	if policy.Version != "TLSRPTv1" {
		return nil, fmt.Errorf("invalid tlsrpt version: %s", policy.Version)
	}
	if policy.RUA == "" {
		return nil, fmt.Errorf("tlsrpt policy missing rua")
	}

	// Extract email from "mailto:address"
	if !strings.HasPrefix(policy.RUA, "mailto:") {
		return nil, fmt.Errorf("rua must be mailto: URI")
	}
	policy.RUA = strings.TrimPrefix(policy.RUA, "mailto:")
	if policy.RUA == "" {
		return nil, fmt.Errorf("rua mailto address empty")
	}

	if policy.RUF != "" && strings.HasPrefix(policy.RUF, "mailto:") {
		policy.RUF = strings.TrimPrefix(policy.RUF, "mailto:")
	}

	return policy, nil
}

// DeliveryResult represents a single delivery attempt result (RFC 8460 §4.1).
type DeliveryResult struct {
	ResultType           string `json:"result-type"` // "success", "starttls", "tlsa", "mta-sts", "validation", "other"
	SendingMTA           string `json:"sending-mta"` // Hostname of sending MTA
	ReceivingMTA         string `json:"receiving-mta,omitempty"` // Receiving MTA hostname
	ReceivingMTAHelo     string `json:"receiving-mta-helo,omitempty"` // HELO/EHLO name
	FailureDetails       *FailureDetails `json:"failure-details,omitempty"`
	SuccessDetails       *SuccessDetails `json:"success-details,omitempty"`
	FailureCount         int    `json:"failure-count"` // Number of failures
	SuccessCount         int    `json:"success-count"` // Number of successes
}

// FailureDetails describes a TLS error (RFC 8460 §4.2).
type FailureDetails struct {
	ResultType       string `json:"result-type"` // Error type
	SendingMTAIP     string `json:"sending-mta-ip,omitempty"`
	ReceivingMTAIP   string `json:"receiving-mta-ip,omitempty"`
	FailureReasonCode string `json:"failure-reason-code,omitempty"` // starttls-unsupported, certificate-host-mismatch, etc
	FailureReasonText string `json:"failure-reason-text,omitempty"`
	ErrMsgTLS        string `json:"err-msg-tls,omitempty"`
}

// SuccessDetails describes successful TLS (RFC 8460 §4.3).
type SuccessDetails struct {
	TLSVersion      string `json:"tls-version"` // e.g., "TLSv1.3"
	TLSCipherSuite  string `json:"tls-cipher-suite"`
	CertificateInfo []CertificateInfo `json:"certificate-info,omitempty"`
}

// CertificateInfo describes certificate details (RFC 8460 §4.4).
type CertificateInfo struct {
	Depth      int    `json:"depth"` // 0=end-entity, 1=intermediate, 2=root
	Subject    string `json:"subject"`
	Issuer     string `json:"issuer"`
	ValidFrom  int64  `json:"valid-from"` // Unix timestamp
	ValidTo    int64  `json:"valid-to"`   // Unix timestamp
}

// Report represents a TLS-RPT aggregate report (RFC 8460 §4).
type Report struct {
	Organization string           `json:"organization-name"`
	DomainName   string           `json:"domain-name"`
	ReportID     string           `json:"report-id"` // Unique per domain per day
	DateRange    DateRange        `json:"date-range"`
	ContactInfo  string           `json:"contact-info"`
	ReportCount  int              `json:"report-count"`
	Policies     []PolicySection  `json:"policies"`
}

// DateRange represents report time window (RFC 8460 §4).
type DateRange struct {
	StartDatetime string `json:"start-datetime"` // RFC 3339 format
	EndDatetime   string `json:"end-datetime"`   // RFC 3339 format
}

// PolicySection describes results for a single policy domain (RFC 8460 §4).
type PolicySection struct {
	Policy          *PolicyFields   `json:"policy"`
	SummaryResults  SummaryResults  `json:"summary-results"`
	FailureDetails  []FailureResult `json:"failure-details,omitempty"`
}

// PolicyFields describes the policy being reported (RFC 8460 §4).
type PolicyFields struct {
	Domain     string        `json:"domain"`
	MXHosts    []string      `json:"mx-host,omitempty"`
	PolicyType string        `json:"policy-type"` // "tlsa", "mta-sts"
	PolicyName string        `json:"policy-name,omitempty"`
}

// SummaryResults aggregates success/failure counts (RFC 8460 §4).
type SummaryResults struct {
	TotalSuccessfulSessionCount int64 `json:"total-successful-session-count"`
	TotalFailureSessionCount    int64 `json:"total-failure-session-count"`
}

// FailureResult details failures for a policy (RFC 8460 §4.2).
type FailureResult struct {
	ResultType             string        `json:"result-type"`
	SendingMTAIP           string        `json:"sending-mta-ip,omitempty"`
	ReceivingMTAIP         string        `json:"receiving-mta-ip,omitempty"`
	ReceivingMTA           string        `json:"receiving-mta,omitempty"`
	FailureReasonCode      string        `json:"failure-reason-code,omitempty"`
	FailureReasonText      string        `json:"failure-reason-text,omitempty"`
	ErrMsgTLS              string        `json:"err-msg-tls,omitempty"`
	SessionCount           int64         `json:"session-count"`
}

// Collector accumulates TLS delivery results for reporting (RFC 8460).
type Collector struct {
	domain       string
	reportID     string
	startTime    time.Time
	endTime      time.Time
	results      map[string]*DeliveryResult // key: domain
	failureCount int64
	successCount int64
}

// NewCollector creates a new TLS-RPT collector for a domain.
func NewCollector(domain string) *Collector {
	now := time.Now().UTC()
	// Report ID: domain-YYYYMMDD-sequence
	reportID := fmt.Sprintf("%s-%04d%02d%02d-001",
		domain, now.Year(), now.Month(), now.Day())

	// Start of day in UTC
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)

	return &Collector{
		domain:    domain,
		reportID:  reportID,
		startTime: startTime,
		endTime:   endTime,
		results:   make(map[string]*DeliveryResult),
	}
}

// RecordFailure records a TLS delivery failure.
func (c *Collector) RecordFailure(resultType string, sendingMTA string, details *FailureDetails) {
	key := sendingMTA + ":" + resultType
	if result, ok := c.results[key]; ok {
		result.FailureCount++
		c.failureCount++
	} else {
		c.results[key] = &DeliveryResult{
			ResultType:    resultType,
			SendingMTA:    sendingMTA,
			FailureDetails: details,
			FailureCount:  1,
		}
		c.failureCount++
	}
}

// RecordSuccess records a successful TLS delivery.
func (c *Collector) RecordSuccess(resultType string, sendingMTA string, details *SuccessDetails) {
	key := sendingMTA + ":" + resultType
	if result, ok := c.results[key]; ok {
		result.SuccessCount++
		c.successCount++
	} else {
		c.results[key] = &DeliveryResult{
			ResultType:    resultType,
			SendingMTA:    sendingMTA,
			SuccessDetails: details,
			SuccessCount:  1,
		}
		c.successCount++
	}
}

// GenerateReport creates a TLS-RPT aggregate report (RFC 8460 §4).
func (c *Collector) GenerateReport(organization string, contactInfo string) *Report {
	policies := make([]PolicySection, 0)

	// Aggregate results by domain
	domainResults := make(map[string][]DeliveryResult)
	for _, result := range c.results {
		domain := extractDomain(result.ReceivingMTA)
		if domain == "" {
			domain = "unknown"
		}
		domainResults[domain] = append(domainResults[domain], *result)
	}

	// Create policy sections
	for domain, results := range domainResults {
		var successCount, failureCount int64
		var failures []FailureResult

		for _, result := range results {
			successCount += int64(result.SuccessCount)
			failureCount += int64(result.FailureCount)

			if result.FailureDetails != nil {
				failures = append(failures, FailureResult{
					ResultType:        result.ResultType,
					SendingMTAIP:      result.FailureDetails.SendingMTAIP,
					ReceivingMTAIP:    result.FailureDetails.ReceivingMTAIP,
					FailureReasonCode: result.FailureDetails.FailureReasonCode,
					FailureReasonText: result.FailureDetails.FailureReasonText,
					ErrMsgTLS:         result.FailureDetails.ErrMsgTLS,
					SessionCount:      1,
				})
			}
		}

		section := PolicySection{
			Policy: &PolicyFields{
				Domain:     domain,
				PolicyType: "tlsa", // Could also be "mta-sts"
			},
			SummaryResults: SummaryResults{
				TotalSuccessfulSessionCount: successCount,
				TotalFailureSessionCount:    failureCount,
			},
			FailureDetails: failures,
		}
		policies = append(policies, section)
	}

	return &Report{
		Organization: organization,
		DomainName:   c.domain,
		ReportID:     c.reportID,
		DateRange: DateRange{
			StartDatetime: c.startTime.Format(time.RFC3339),
			EndDatetime:   c.endTime.Format(time.RFC3339),
		},
		ContactInfo: contactInfo,
		ReportCount: len(policies),
		Policies:    policies,
	}
}

// ToJSON serializes report to formatted JSON (RFC 8460 compatible).
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// extractDomain extracts domain from FQDN (removes trailing dot if present).
func extractDomain(fqdn string) string {
	if fqdn == "" {
		return ""
	}
	domain := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	if domain == "" {
		return ""
	}
	// Return rightmost two labels (e.g., example.com from mx.example.com)
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// IsValidReportAddress validates a TLS-RPT report address.
func IsValidReportAddress(addr string) bool {
	// Validate as email address
	addr = strings.TrimSpace(addr)
	if addr == "" || !strings.Contains(addr, "@") {
		return false
	}
	// Simple check: contains @ and has local/domain parts
	parts := strings.Split(addr, "@")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// LookupPolicy queries DNS for TLS-RPT policy at _tlsrpt.domain.
func LookupPolicy(ctx context.Context, domain string) (*Policy, error) {
	tlsrptDomain := "_tlsrpt." + domain
	resolver := net.Resolver{}
	txts, err := resolver.LookupTXT(ctx, tlsrptDomain)
	if err != nil {
		return nil, fmt.Errorf("dns lookup: %w", err)
	}

	for _, txt := range txts {
		policy, err := ParsePolicy(txt)
		if err == nil {
			return policy, nil
		}
	}

	return nil, fmt.Errorf("no valid tlsrpt policy found")
}
