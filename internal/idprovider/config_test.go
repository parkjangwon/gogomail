package idprovider

import (
	"context"
	"testing"
)

func TestConfigRepositoryInterface(t *testing.T) {
	repo := NewConfigRepository(nil)
	if repo == nil {
		t.Errorf("Expected non-nil repository")
	}
}

func TestConfigInit(t *testing.T) {
	// Test Config struct initialization
	config := &Config{
		DomainID:     "test-domain",
		ProviderType: "database",
		Settings:     make(map[string]interface{}),
	}

	if config.DomainID != "test-domain" {
		t.Errorf("Expected domain_id 'test-domain', got %s", config.DomainID)
	}

	if config.ProviderType != "database" {
		t.Errorf("Expected provider_type 'database', got %s", config.ProviderType)
	}
}

func TestCreateConfigValidation(t *testing.T) {
	repo := NewConfigRepository(nil)

	// Test nil config
	err := repo.CreateConfig(context.Background(), nil)
	if err == nil {
		t.Errorf("Expected error for nil config, got nil")
	}

	// Test missing domain_id
	err = repo.CreateConfig(context.Background(), &Config{
		ProviderType: "database",
	})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}

	// Test missing provider_type
	err = repo.CreateConfig(context.Background(), &Config{
		DomainID: "domain-id",
	})
	if err == nil {
		t.Errorf("Expected error for missing provider_type, got nil")
	}
}

func TestUpdateConfigValidation(t *testing.T) {
	repo := NewConfigRepository(nil)

	// Test nil config
	err := repo.UpdateConfig(context.Background(), nil)
	if err == nil {
		t.Errorf("Expected error for nil config, got nil")
	}

	// Test missing domain_id
	err = repo.UpdateConfig(context.Background(), &Config{
		ProviderType: "database",
	})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}
}

func TestDeleteConfigValidation(t *testing.T) {
	repo := NewConfigRepository(nil)

	// Test empty domain id
	err := repo.DeleteConfig(context.Background(), "")
	if err == nil {
		t.Errorf("Expected error for empty domain_id, got nil")
	}
}
