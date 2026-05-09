package mtasts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Policy represents an MTA-STS policy (RFC 8461).
type Policy struct {
	Version string        // "STSv1"
	Mode    string        // "enforce", "testing", or "none"
	MaxAge  int           // Max cache time in seconds
	MXHosts []string      // List of MX patterns (may include wildcards)
	Fetched time.Time     // When this policy was fetched
	Expires time.Time     // When this policy cache expires
}

// IsExpired checks if the cached policy has expired.
func (p *Policy) IsExpired() bool {
	return time.Now().After(p.Expires)
}

// MatchesMX checks if a hostname matches any of the policy's MX patterns.
// Supports wildcard matching: *.example.com matches mx1.example.com but not example.com
func (p *Policy) MatchesMX(hostname string) bool {
	for _, pattern := range p.MXHosts {
		if matchPattern(pattern, hostname) {
			return true
		}
	}
	return false
}

// Client fetches and caches MTA-STS policies.
type Client struct {
	httpClient *http.Client
	cache      map[string]*Policy
	mu         sync.RWMutex
}

// NewClient creates a new MTA-STS client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 2 * time.Second,
				DisableKeepAlives:   true,
			},
		},
		cache: make(map[string]*Policy),
	}
}

// GetPolicy fetches or retrieves from cache the MTA-STS policy for a domain.
func (c *Client) GetPolicy(ctx context.Context, domain string) (*Policy, error) {
	// Check cache first
	c.mu.RLock()
	if cached, ok := c.cache[domain]; ok && !cached.IsExpired() {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	// Fetch fresh policy
	policy, err := c.fetchPolicy(ctx, domain)
	if err != nil {
		return nil, err
	}

	// Cache the policy
	c.mu.Lock()
	c.cache[domain] = policy
	c.mu.Unlock()

	return policy, nil
}

// fetchPolicy fetches the MTA-STS policy from DNS and HTTPS.
func (c *Client) fetchPolicy(ctx context.Context, domain string) (*Policy, error) {
	// Step 1: Check DNS TXT record _mta-sts.domain for version
	version, err := c.checkDNSTXT(ctx, domain)
	if err != nil {
		// No MTA-STS policy
		return &Policy{Mode: "none", Fetched: time.Now()}, nil
	}

	// Step 2: Fetch policy from HTTPS
	policyText, err := c.fetchHTTPS(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("fetch policy: %w", err)
	}

	// Step 3: Parse policy
	policy, err := ParsePolicy(policyText)
	if err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}

	// Validate version matches DNS
	if policy.Version != version {
		return nil, fmt.Errorf("policy version mismatch: DNS=%s, policy=%s", version, policy.Version)
	}

	// Set cache expiration
	policy.Fetched = time.Now()
	policy.Expires = policy.Fetched.Add(time.Duration(policy.MaxAge) * time.Second)

	return policy, nil
}

// checkDNSTXT checks for MTA-STS version in DNS TXT records.
func (c *Client) checkDNSTXT(ctx context.Context, domain string) (string, error) {
	txtDomain := "_mta-sts." + domain
	resolver := net.Resolver{}
	txts, err := resolver.LookupTXT(ctx, txtDomain)
	if err != nil {
		return "", fmt.Errorf("dns lookup: %w", err)
	}

	for _, txt := range txts {
		// Parse "v=STSv1; id=..."
		parts := strings.Split(txt, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "v=") {
				return strings.TrimPrefix(part, "v="), nil
			}
		}
	}

	return "", fmt.Errorf("no version in TXT record")
}

// fetchHTTPS fetches the policy file via HTTPS.
func (c *Client) fetchHTTPS(ctx context.Context, domain string) (string, error) {
	url := fmt.Sprintf("https://mta-sts.%s/.well-known/mta-sts.json", domain)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}

	// Limit response size to 64KB (RFC 8461)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ParsePolicy parses the MTA-STS policy JSON.
func ParsePolicy(policyText string) (*Policy, error) {
	var raw struct {
		Version string   `json:"version"`
		Mode    string   `json:"mode"`
		MaxAge  int      `json:"max_age"`
		MX      []string `json:"mx"`
	}

	if err := json.NewDecoder(bytes.NewReader([]byte(policyText))).Decode(&raw); err != nil {
		return nil, err
	}

	// Validate required fields
	if raw.Version == "" || raw.Mode == "" {
		return nil, fmt.Errorf("missing version or mode")
	}

	if raw.Mode != "enforce" && raw.Mode != "testing" && raw.Mode != "none" {
		return nil, fmt.Errorf("invalid mode: %s", raw.Mode)
	}

	if raw.MaxAge < 0 || raw.MaxAge > 31536000 { // Max 1 year
		return nil, fmt.Errorf("invalid max_age: %d", raw.MaxAge)
	}

	if len(raw.MX) == 0 && raw.Mode != "none" {
		return nil, fmt.Errorf("mode %s requires mx list", raw.Mode)
	}

	return &Policy{
		Version: raw.Version,
		Mode:    raw.Mode,
		MaxAge:  raw.MaxAge,
		MXHosts: raw.MX,
	}, nil
}

// matchPattern matches a hostname against an MTA-STS MX pattern.
// Patterns may include wildcards: *.example.com matches mx1.example.com but not example.com
func matchPattern(pattern, hostname string) bool {
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, hostname)
	}

	// Simple wildcard: *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:] // Remove "*."
		if strings.EqualFold(suffix, hostname) {
			return false // Wildcard doesn't match the base domain
		}
		return strings.HasSuffix(strings.ToLower(hostname), "."+strings.ToLower(suffix))
	}

	return false
}
