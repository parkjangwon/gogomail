package smtpd

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// RFCCompliance represents compliance status against multiple RFC standards.
type RFCCompliance struct {
	RFC5322Valid bool        // Internet Message Format
	RFC5321Valid bool        // SMTP Protocol
	RFC3461Valid bool        // DSN Options
	RFC6376Valid bool        // DKIM Signatures
	RFC5891Valid bool        // IDN (Internationalized Domain Names)
	Errors       []string    // Detailed compliance violations
}

// RFCCompliant validator checks messages against multiple RFCs.
type RFCCompliant struct {
	// Email address pattern per RFC 5321/5322
	// Simplified but reasonably permissive
	emailPattern *regexp.Regexp
	// Domain name pattern per RFC 5321
	domainPattern *regexp.Regexp
}

// NewRFCCompliant creates a new RFC compliance validator.
func NewRFCCompliant() *RFCCompliant {
	return &RFCCompliant{
		// Basic email pattern: local@domain
		emailPattern: regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_`+"`"+`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`),
		// Domain pattern per RFC 1035
		domainPattern: regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`),
	}
}

// ValidateMessage checks RFC 5322 (Message Format) compliance.
// This is a basic check; full compliance would require proper header parsing.
func (rc *RFCCompliant) ValidateRFC5322(rawMessage string, compliance *RFCCompliance) {
	errors := []string{}

	// Check for required headers (Date, From)
	if !strings.Contains(strings.ToLower(rawMessage), "from:") {
		errors = append(errors, "RFC 5322: Missing required 'From:' header")
	}
	if !strings.Contains(strings.ToLower(rawMessage), "date:") {
		errors = append(errors, "RFC 5322: Missing recommended 'Date:' header")
	}

	// Check for CRLF line endings
	if !strings.Contains(rawMessage, "\r\n") {
		errors = append(errors, "RFC 5322: Message must use CRLF line endings")
	}

	// Check message size (RFC 5321 recommends 78 char line limit)
	lines := strings.Split(rawMessage, "\r\n")
	for i, line := range lines {
		if len(line) > 1000 {
			errors = append(errors, fmt.Sprintf("RFC 5322: Line %d exceeds 1000 character limit: %d chars", i+1, len(line)))
		}
	}

	compliance.RFC5322Valid = len(errors) == 0
	compliance.Errors = append(compliance.Errors, errors...)
}

// ValidateSMTPEnvelope checks RFC 5321 (SMTP Protocol) compliance.
func (rc *RFCCompliant) ValidateSMTPEnvelope(from string, recipients []string, compliance *RFCCompliance) {
	errors := []string{}

	// Validate sender
	if from == "" {
		errors = append(errors, "RFC 5321: MAIL FROM requires valid sender address")
	} else if !rc.emailPattern.MatchString(from) {
		errors = append(errors, fmt.Sprintf("RFC 5321: MAIL FROM invalid format: %s", from))
	}

	// Validate recipients
	if len(recipients) == 0 {
		errors = append(errors, "RFC 5321: RCPT TO requires at least one recipient")
	} else {
		for _, recipient := range recipients {
			if !rc.emailPattern.MatchString(recipient) {
				errors = append(errors, fmt.Sprintf("RFC 5321: RCPT TO invalid format: %s", recipient))
			}
		}
	}

	compliance.RFC5321Valid = len(errors) == 0
	compliance.Errors = append(compliance.Errors, errors...)
}

// ValidateDSNOptions checks RFC 3461 (Delivery Status Notification) compliance.
func (rc *RFCCompliant) ValidateDSNOptions(dsnReturn string, dsnNotify []string, compliance *RFCCompliance) {
	errors := []string{}

	// Validate RETURN option
	if dsnReturn != "" {
		validReturn := map[string]bool{
			"FULL": true,
			"HDRS": true,
		}
		if !validReturn[strings.ToUpper(dsnReturn)] {
			errors = append(errors, fmt.Sprintf("RFC 3461: Invalid RETURN option: %s (must be FULL or HDRS)", dsnReturn))
		}
	}

	// Validate NOTIFY option
	validNotify := map[string]bool{
		"NEVER":    true,
		"SUCCESS":  true,
		"FAILURE":  true,
		"DELAY":    true,
	}
	for _, notify := range dsnNotify {
		if !validNotify[strings.ToUpper(notify)] {
			errors = append(errors, fmt.Sprintf("RFC 3461: Invalid NOTIFY option: %s", notify))
		}
	}

	compliance.RFC3461Valid = len(errors) == 0
	compliance.Errors = append(compliance.Errors, errors...)
}

// ValidateDKIM checks RFC 6376 (DKIM Signatures) basic structure.
func (rc *RFCCompliant) ValidateDKIM(dkimSignature string, compliance *RFCCompliance) {
	errors := []string{}

	if dkimSignature == "" {
		// DKIM is optional
		compliance.RFC6376Valid = true
		return
	}

	// Basic DKIM-Signature structure check
	// Format: DKIM-Signature: v=1; a=rsa-sha256; ...
	sig := strings.ToLower(dkimSignature)

	requiredTags := []string{"v=1", "a=", "b=", "bh=", "c=", "d=", "s="}
	for _, tag := range requiredTags {
		if !strings.Contains(sig, tag) {
			errors = append(errors, fmt.Sprintf("RFC 6376: DKIM-Signature missing %s tag", strings.Split(tag, "=")[0]))
		}
	}

	// Check algorithm is RSA
	if !strings.Contains(sig, "a=rsa-") {
		errors = append(errors, "RFC 6376: DKIM-Signature must use RSA algorithm")
	}

	compliance.RFC6376Valid = len(errors) == 0
	compliance.Errors = append(compliance.Errors, errors...)
}

// ValidateIDN checks RFC 5891 (Internationalized Domain Names) compliance.
// This is a simplified check; full IDN validation requires IDNA processing.
func (rc *RFCCompliant) ValidateIDN(addresses []string, compliance *RFCCompliance) {
	errors := []string{}

	for _, addr := range addresses {
		parts := strings.Split(addr, "@")
		if len(parts) != 2 {
			continue
		}

		domain := parts[1]

		// Check for non-ASCII characters (potential IDN)
		hasNonASCII := false
		for _, r := range domain {
			if r > unicode.MaxASCII {
				hasNonASCII = true
				break
			}
		}

		if hasNonASCII {
			// IDN domains should be in ACE format (xn--...)
			// or will be converted by the transport layer
			// For now, just note this as something to monitor
			if !strings.HasPrefix(domain, "xn--") && !strings.Contains(domain, ".xn--") {
				// This is a pure Unicode IDN - acceptable but requires proper handling
			}
		}

		// Standard ASCII domain validation
		if !rc.domainPattern.MatchString(domain) {
			errors = append(errors, fmt.Sprintf("RFC 5891: Invalid domain format: %s", domain))
		}
	}

	compliance.RFC5891Valid = len(errors) == 0
	compliance.Errors = append(compliance.Errors, errors...)
}

// ValidateMessage performs comprehensive RFC compliance checking.
func (rc *RFCCompliant) ValidateMessage(
	rawMessage string,
	from string,
	recipients []string,
	dsnReturn string,
	dsnNotify []string,
	dkimSignature string,
) RFCCompliance {

	compliance := RFCCompliance{}

	rc.ValidateRFC5322(rawMessage, &compliance)
	rc.ValidateSMTPEnvelope(from, recipients, &compliance)
	rc.ValidateDSNOptions(dsnReturn, dsnNotify, &compliance)
	rc.ValidateDKIM(dkimSignature, &compliance)
	rc.ValidateIDN(recipients, &compliance)

	return compliance
}

// IsCompliant returns true if message passes all RFC checks.
func (c *RFCCompliance) IsCompliant() bool {
	return c.RFC5322Valid && c.RFC5321Valid && c.RFC3461Valid && c.RFC6376Valid && c.RFC5891Valid
}

// GetSummary returns a human-readable compliance summary.
func (c *RFCCompliance) GetSummary() string {
	var sb strings.Builder

	sb.WriteString("RFC Compliance Report:\n")
	sb.WriteString(fmt.Sprintf("  RFC 5322 (Message Format): %v\n", c.RFC5322Valid))
	sb.WriteString(fmt.Sprintf("  RFC 5321 (SMTP Protocol): %v\n", c.RFC5321Valid))
	sb.WriteString(fmt.Sprintf("  RFC 3461 (DSN): %v\n", c.RFC3461Valid))
	sb.WriteString(fmt.Sprintf("  RFC 6376 (DKIM): %v\n", c.RFC6376Valid))
	sb.WriteString(fmt.Sprintf("  RFC 5891 (IDN): %v\n", c.RFC5891Valid))

	if len(c.Errors) > 0 {
		sb.WriteString("\nViolations:\n")
		for _, err := range c.Errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", err))
		}
	}

	return sb.String()
}
