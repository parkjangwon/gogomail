package rdbms

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ValidateConfig validates the RDBMS configuration for completeness and correctness.
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if strings.TrimSpace(cfg.ConnectionString) == "" {
		return fmt.Errorf("connection_string is required")
	}
	if strings.TrimSpace(cfg.UserQuery) == "" {
		return fmt.Errorf("user_query is required")
	}
	if strings.TrimSpace(cfg.GroupQuery) == "" {
		return fmt.Errorf("group_query is required")
	}
	if cfg.FieldMap == nil || len(cfg.FieldMap) == 0 {
		return fmt.Errorf("field_map is required")
	}
	if cfg.MaxPoolSize <= 0 {
		cfg.MaxPoolSize = 5
	}
	return nil
}

// TestConnection tests the connection to the external RDBMS.
func TestConnection(connString string) error {
	if strings.TrimSpace(connString) == "" {
		return fmt.Errorf("connection string is required")
	}

	db, err := sql.Open("postgres", connString)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// ValidateUserQuery validates the user query by executing it with a LIMIT 1 test.
func (p *Provider) ValidateUserQuery(ctx context.Context) error {
	if p.config == nil {
		return fmt.Errorf("provider not configured")
	}
	if p.db == nil {
		return fmt.Errorf("provider not connected")
	}
	if p.config.UserQuery == "" {
		return fmt.Errorf("user_query is required")
	}

	// Test query with LIMIT 1
	query := p.config.UserQuery
	if !strings.Contains(strings.ToUpper(query), "LIMIT") {
		query = fmt.Sprintf("%s LIMIT 1", query)
	}

	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("user_query validation failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	if len(cols) == 0 {
		return fmt.Errorf("user_query must return at least one column")
	}

	return nil
}

// ValidateGroupQuery validates the group query by executing it with a LIMIT 1 test.
func (p *Provider) ValidateGroupQuery(ctx context.Context) error {
	if p.config == nil {
		return fmt.Errorf("provider not configured")
	}
	if p.db == nil {
		return fmt.Errorf("provider not connected")
	}
	if p.config.GroupQuery == "" {
		return fmt.Errorf("group_query is required")
	}

	// Test query with LIMIT 1
	query := p.config.GroupQuery
	if !strings.Contains(strings.ToUpper(query), "LIMIT") {
		query = fmt.Sprintf("%s LIMIT 1", query)
	}

	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("group_query validation failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	if len(cols) == 0 {
		return fmt.Errorf("group_query must return at least one column")
	}

	return nil
}

// ValidateFieldMap validates that all required fields are mapped.
func ValidateFieldMap(fieldMap map[string]string, mapType string) error {
	if fieldMap == nil || len(fieldMap) == 0 {
		return fmt.Errorf("field_map is required")
	}

	// Validate that at least the required fields are present
	switch mapType {
	case "user":
		requiredFields := []string{"id", "username", "display_name"}
		for _, field := range requiredFields {
			if _, ok := fieldMap[field]; !ok {
				return fmt.Errorf("required field %q not in field_map", field)
			}
		}
	case "group":
		requiredFields := []string{"id", "name", "slug"}
		for _, field := range requiredFields {
			if _, ok := fieldMap[field]; !ok {
				return fmt.Errorf("required field %q not in field_map", field)
			}
		}
	default:
		return fmt.Errorf("invalid mapType: %s", mapType)
	}

	return nil
}
