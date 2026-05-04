package config

import "testing"

func TestValidateRejectsUnknownSearchIndexBackend(t *testing.T) {
	cfg := Load()
	cfg.SearchIndexBackend = "opensearch"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unknown search index backend")
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
