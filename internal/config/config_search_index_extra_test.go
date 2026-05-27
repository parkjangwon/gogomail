package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsUnknownSearchIndexBackend(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.SearchIndexBackend = "elastic"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unknown search index backend")
	}
}

func TestValidateAcceptsOpenSearchBackendWithEndpointAndIndex(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.SearchIndexBackend = "opensearch"
	cfg.SearchIndexOpenSearchEndpoint = "https://search.example.com"
	cfg.SearchIndexOpenSearchIndex = "gogomail-messages"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRejectsOpenSearchBackendWithoutEndpointOrIndex(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "endpoint", edit: func(cfg *Config) { cfg.SearchIndexOpenSearchEndpoint = "" }},
		{name: "index", edit: func(cfg *Config) { cfg.SearchIndexOpenSearchIndex = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOGOMAIL_ENV", "development")
			cfg := Load()
			cfg.SearchIndexBackend = "opensearch"
			cfg.SearchIndexOpenSearchEndpoint = "https://search.example.com"
			cfg.SearchIndexOpenSearchIndex = "gogomail-messages"
			tt.edit(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted incomplete opensearch settings")
			}
		})
	}
}

func TestValidateRejectsInvalidOpenSearchEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "unsupported scheme", endpoint: "ftp://search.example.com"},
		{name: "missing host", endpoint: "http:///missing-host"},
		{name: "newline", endpoint: "https://search.example.com\nbad"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOGOMAIL_ENV", "development")
			cfg := Load()
			cfg.SearchIndexBackend = "opensearch"
			cfg.SearchIndexOpenSearchEndpoint = tt.endpoint
			cfg.SearchIndexOpenSearchIndex = "gogomail-messages"

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted invalid opensearch endpoint")
			}
		})
	}
}

func TestValidateRejectsInvalidOpenSearchIndexName(t *testing.T) {
	for _, index := range []string{"../bad", ".hidden", "_system", "bad name", "bad:index"} {
		index := index
		t.Run(index, func(t *testing.T) {
			t.Setenv("GOGOMAIL_ENV", "development")
			cfg := Load()
			cfg.SearchIndexBackend = "opensearch"
			cfg.SearchIndexOpenSearchEndpoint = "https://search.example.com"
			cfg.SearchIndexOpenSearchIndex = index

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted invalid opensearch index name")
			}
		})
	}
}

func TestValidateRejectsUnsafeOpenSearchCredentials(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "username newline", edit: func(cfg *Config) { cfg.SearchIndexOpenSearchUsername = "admin\nbad" }},
		{name: "username oversized", edit: func(cfg *Config) {
			cfg.SearchIndexOpenSearchUsername = strings.Repeat("u", maxOpenSearchCredentialBytes+1)
		}},
		{name: "password newline", edit: func(cfg *Config) { cfg.SearchIndexOpenSearchPassword = "secret\nbad" }},
		{name: "password oversized", edit: func(cfg *Config) {
			cfg.SearchIndexOpenSearchPassword = strings.Repeat("p", maxOpenSearchCredentialBytes+1)
		}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOGOMAIL_ENV", "development")
			cfg := Load()
			cfg.SearchIndexBackend = "opensearch"
			cfg.SearchIndexOpenSearchEndpoint = "https://search.example.com"
			cfg.SearchIndexOpenSearchIndex = "gogomail-messages"
			tt.edit(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted unsafe opensearch credentials")
			}
		})
	}
}

func TestValidateRejectsNonpositiveSearchIndexLimits(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "max body", edit: func(cfg *Config) { cfg.SearchIndexMaxBodyBytes = 0 }},
		{name: "consumer count", edit: func(cfg *Config) { cfg.SearchIndexConsumerCount = 0 }},
		{name: "consumer block", edit: func(cfg *Config) { cfg.SearchIndexConsumerBlock = 0 }},
		{name: "opensearch timeout", edit: func(cfg *Config) { cfg.SearchIndexOpenSearchTimeout = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOGOMAIL_ENV", "development")
			cfg := Load()
			tt.edit(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted nonpositive search index setting")
			}
		})
	}
}

func TestLoadSearchIndexOpenSearchBootstrapSetting(t *testing.T) {
	t.Setenv("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP", "true")

	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	if !cfg.SearchIndexOpenSearchBootstrap {
		t.Fatal("SearchIndexOpenSearchBootstrap = false, want true")
	}
}

func TestLoadSearchIndexOpenSearchTimeoutSetting(t *testing.T) {
	t.Setenv("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT", "3s")

	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	if cfg.SearchIndexOpenSearchTimeout != 3*time.Second {
		t.Fatalf("SearchIndexOpenSearchTimeout = %s, want 3s", cfg.SearchIndexOpenSearchTimeout)
	}
}
