package config

import (
	"testing"
	"time"
)

func TestValidateRejectsUnknownSearchIndexBackend(t *testing.T) {
	cfg := Load()
	cfg.SearchIndexBackend = "elastic"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unknown search index backend")
	}
}

func TestValidateAcceptsOpenSearchBackendWithEndpointAndIndex(t *testing.T) {
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

	cfg := Load()
	if !cfg.SearchIndexOpenSearchBootstrap {
		t.Fatal("SearchIndexOpenSearchBootstrap = false, want true")
	}
}

func TestLoadSearchIndexOpenSearchTimeoutSetting(t *testing.T) {
	t.Setenv("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT", "3s")

	cfg := Load()
	if cfg.SearchIndexOpenSearchTimeout != 3*time.Second {
		t.Fatalf("SearchIndexOpenSearchTimeout = %s, want 3s", cfg.SearchIndexOpenSearchTimeout)
	}
}
