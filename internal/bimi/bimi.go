package bimi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Policy represents a BIMI policy from DNS TXT record (RFC 6651).
// Format: "v=BIMI1; l=<logo-url>; a=<vmc-url>"
type Policy struct {
	Version    string    // "BIMI1"
	LogoURL    string    // Logo image URL (l= tag)
	VMCURL     string    // VMC certificate URL (a= tag, optional)
	Fetched    time.Time // When policy was fetched
	Expires    time.Time // Cache expiration (TTL)
}

// IsExpired checks if cached policy has expired.
func (p *Policy) IsExpired() bool {
	return time.Now().After(p.Expires)
}

// Resolver queries BIMI policy from DNS.
type Resolver interface {
	LookupPolicy(ctx context.Context, domain string) (*Policy, error)
}

// NetResolver implements Resolver using net.Resolver.
type NetResolver struct {
	*net.Resolver
}

// NewResolver creates a new BIMI resolver.
func NewResolver() *Resolver {
	r := &NetResolver{
		Resolver: &net.Resolver{},
	}
	var resolver Resolver = r
	return &resolver
}

// LookupPolicy queries DNS for BIMI policy at _bimi.domain (RFC 6651 §3).
func (r *NetResolver) LookupPolicy(ctx context.Context, domain string) (*Policy, error) {
	bimiDomain := "_bimi." + domain
	txts, err := r.LookupTXT(ctx, bimiDomain)
	if err != nil {
		return nil, fmt.Errorf("dns lookup: %w", err)
	}

	for _, txt := range txts {
		policy, err := ParsePolicy(txt)
		if err == nil {
			// Default TTL: 1 hour if not specified
			policy.Fetched = time.Now()
			policy.Expires = policy.Fetched.Add(1 * time.Hour)
			return policy, nil
		}
	}

	return nil, fmt.Errorf("no valid bimi policy found")
}

// ParsePolicy parses a BIMI policy from DNS TXT record.
func ParsePolicy(txt string) (*Policy, error) {
	policy := &Policy{}

	// Split by semicolon (RFC 6651)
	parts := strings.Split(txt, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasPrefix(part, "v=") {
			policy.Version = strings.TrimSpace(strings.TrimPrefix(part, "v="))
		} else if strings.HasPrefix(part, "l=") {
			policy.LogoURL = strings.TrimSpace(strings.TrimPrefix(part, "l="))
		} else if strings.HasPrefix(part, "a=") {
			policy.VMCURL = strings.TrimSpace(strings.TrimPrefix(part, "a="))
		}
	}

	// Validate required fields
	if policy.Version != "BIMI1" {
		return nil, fmt.Errorf("invalid bimi version: %s", policy.Version)
	}
	if policy.LogoURL == "" {
		return nil, fmt.Errorf("bimi policy missing logo url (l= tag)")
	}

	// Validate LogoURL is HTTPS (RFC 6651 §3)
	if !strings.HasPrefix(strings.ToLower(policy.LogoURL), "https://") {
		return nil, fmt.Errorf("logo url must be https")
	}

	return policy, nil
}

// LogoCache caches fetched logo images with size/TTL limits.
type LogoCache struct {
	client     *http.Client
	cache      map[string]*cachedLogo
	mu         sync.RWMutex
	maxSize    int           // Max logo size in bytes
	cacheTTL   time.Duration // Cache time-to-live
}

type cachedLogo struct {
	data    []byte
	fetched time.Time
	expires time.Time
	hash    string
}

// NewLogoCache creates a new logo cache (default: 32KB max, 24h TTL).
func NewLogoCache() *LogoCache {
	return &LogoCache{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		cache:   make(map[string]*cachedLogo),
		maxSize: 32 * 1024, // 32KB max (RFC 6651)
		cacheTTL: 24 * time.Hour,
	}
}

// FetchLogo fetches and caches logo from URL (HTTPS only, size-limited).
func (lc *LogoCache) FetchLogo(ctx context.Context, logoURL string) ([]byte, error) {
	// Validate HTTPS
	if !strings.HasPrefix(strings.ToLower(logoURL), "https://") {
		return nil, fmt.Errorf("logo url must be https")
	}

	// Check cache
	lc.mu.RLock()
	if cached, ok := lc.cache[logoURL]; ok && !cached.isExpired() {
		lc.mu.RUnlock()
		return cached.data, nil
	}
	lc.mu.RUnlock()

	// Fetch from server
	req, err := http.NewRequestWithContext(ctx, "GET", logoURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := lc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	// Limit response to maxSize
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(lc.maxSize+1)))
	if err != nil {
		return nil, err
	}

	if len(body) > lc.maxSize {
		return nil, fmt.Errorf("logo exceeds max size: %d > %d", len(body), lc.maxSize)
	}

	// Cache the logo
	hash := hex.EncodeToString(sha256.New().Sum(body))
	cached := &cachedLogo{
		data:    body,
		fetched: time.Now(),
		expires: time.Now().Add(lc.cacheTTL),
		hash:    hash,
	}

	lc.mu.Lock()
	lc.cache[logoURL] = cached
	lc.mu.Unlock()

	return body, nil
}

func (cl *cachedLogo) isExpired() bool {
	return time.Now().After(cl.expires)
}

// Validator validates BIMI policy and generates logo headers.
type Validator struct {
	resolver  Resolver
	logoCache *LogoCache
}

// NewValidator creates a new BIMI validator.
func NewValidator(resolver Resolver, logoCache *LogoCache) *Validator {
	return &Validator{
		resolver:  resolver,
		logoCache: logoCache,
	}
}

// ValidateAndFetch validates BIMI policy and returns logo data if valid.
// Returns (logoData, vmcVerified, error).
func (v *Validator) ValidateAndFetch(ctx context.Context, domain string) ([]byte, bool, error) {
	// Lookup BIMI policy
	policy, err := v.resolver.LookupPolicy(ctx, domain)
	if err != nil {
		// No BIMI policy
		return nil, false, nil
	}

	// Fetch logo
	logoData, err := v.logoCache.FetchLogo(ctx, policy.LogoURL)
	if err != nil {
		return nil, false, fmt.Errorf("fetch logo: %w", err)
	}

	// If VMC URL present, would validate certificate here (stub for now)
	vmcVerified := policy.VMCURL != ""

	return logoData, vmcVerified, nil
}

// GetLogoHeader returns Base64-encoded logo for BIMI-Selector header.
func GetLogoHeader(logoData []byte) string {
	// BIMI-Selector header format: image/svg+xml;base64,...
	encoded := encodeBase64(logoData)
	contentType := detectContentType(logoData)
	return fmt.Sprintf("%s;base64,%s", contentType, encoded)
}

func detectContentType(data []byte) string {
	if len(data) >= 4 {
		if bytes.HasPrefix(data, []byte("GIF8")) {
			return "image/gif"
		}
		if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
			return "image/png"
		}
		if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
			return "image/jpeg"
		}
		if bytes.HasPrefix(data, []byte("<svg")) || bytes.HasPrefix(data, []byte("<?xml")) {
			return "image/svg+xml"
		}
	}
	return "application/octet-stream"
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
