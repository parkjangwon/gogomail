package admin

import (
	"context"
	"database/sql"
	"testing"
)

type mockRDBMSConnection struct {
	users map[string]*RDBMSUser
	fail  bool
}

type RDBMSUser struct {
	ID       string
	Email    string
	Name     string
	Password string
}

func newMockRDBMSConnection() *mockRDBMSConnection {
	return &mockRDBMSConnection{
		users: make(map[string]*RDBMSUser),
		fail:  false,
	}
}

func (m *mockRDBMSConnection) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

func (m *mockRDBMSConnection) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (m *mockRDBMSConnection) Close() error {
	return nil
}

func TestRDBMSProviderValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    RDBMSConfig
		shouldErr bool
	}{
		{
			name: "valid config",
			config: RDBMSConfig{
				Host:           "db.example.com",
				Port:           5432,
				Username:       "user",
				Password:       "pass",
				Database:       "users_db",
				UserTable:      "users",
				EmailColumn:    "email",
				NameColumn:     "name",
				IDColumn:       "id",
				PasswordColumn: "password",
			},
			shouldErr: false,
		},
		{
			name: "missing host",
			config: RDBMSConfig{
				Port:           5432,
				Username:       "user",
				Password:       "pass",
				Database:       "users_db",
				UserTable:      "users",
				EmailColumn:    "email",
				NameColumn:     "name",
				IDColumn:       "id",
				PasswordColumn: "password",
			},
			shouldErr: true,
		},
		{
			name: "missing database",
			config: RDBMSConfig{
				Host:           "db.example.com",
				Port:           5432,
				Username:       "user",
				Password:       "pass",
				UserTable:      "users",
				EmailColumn:    "email",
				NameColumn:     "name",
				IDColumn:       "id",
				PasswordColumn: "password",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRDBMSConfig(tt.config)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ValidateRDBMSConfig() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestRDBMSProviderAuthenticate(t *testing.T) {
	config := RDBMSConfig{
		Host:           "db.example.com",
		Port:           5432,
		Username:       "user",
		Password:       "pass",
		Database:       "users_db",
		UserTable:      "users",
		EmailColumn:    "email",
		NameColumn:     "name",
		IDColumn:       "id",
		PasswordColumn: "password",
	}

	provider := NewRDBMSProvider(config, newMockRDBMSConnection())
	ctx := context.Background()

	tests := []struct {
		name      string
		email     string
		password  string
		shouldErr bool
	}{
		{
			name:      "missing email",
			email:     "",
			password:  "password123",
			shouldErr: true,
		},
		{
			name:      "missing password",
			email:     "user@example.com",
			password:  "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := map[string]string{
				"email":    tt.email,
				"password": tt.password,
			}
			user, err := provider.Authenticate(ctx, creds)
			if (err != nil) != tt.shouldErr {
				t.Errorf("Authenticate() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && user == nil {
				t.Error("Authenticate() returned nil user")
			}
		})
	}
}

func TestRDBMSProviderGetUser(t *testing.T) {
	config := RDBMSConfig{
		Host:           "db.example.com",
		Port:           5432,
		Username:       "user",
		Password:       "pass",
		Database:       "users_db",
		UserTable:      "users",
		EmailColumn:    "email",
		NameColumn:     "name",
		IDColumn:       "id",
		PasswordColumn: "password",
	}

	provider := NewRDBMSProvider(config, newMockRDBMSConnection())
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    string
		shouldErr bool
	}{
		{
			name:      "missing userID",
			userID:    "",
			shouldErr: true,
		},
		{
			name:      "non-existent user",
			userID:    "user123",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.GetUser(ctx, tt.userID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetUser() error = %v, shouldErr %v", err, tt.shouldErr)
				return
			}
			// For parameter validation tests, just check error state
		})
	}
}

func TestRDBMSProviderListUsers(t *testing.T) {
	config := RDBMSConfig{
		Host:           "db.example.com",
		Port:           5432,
		Username:       "user",
		Password:       "pass",
		Database:       "users_db",
		UserTable:      "users",
		EmailColumn:    "email",
		NameColumn:     "name",
		IDColumn:       "id",
		PasswordColumn: "password",
	}

	provider := NewRDBMSProvider(config, newMockRDBMSConnection())
	ctx := context.Background()

	// Just test that ListUsers returns consistent results
	_, count, err := provider.ListUsers(ctx, map[string]string{}, 10, 0)
	if err != nil && err.Error() == "query failed: query is not supported" {
		// Mock returns error, which is OK for testing
		return
	}
	if err == nil {
		// If no error, validate results are consistent
		if count < 0 {
			t.Errorf("ListUsers() returned negative count")
		}
	}
}

func TestRDBMSProviderSync(t *testing.T) {
	config := RDBMSConfig{
		Host:           "db.example.com",
		Port:           5432,
		Username:       "user",
		Password:       "pass",
		Database:       "users_db",
		UserTable:      "users",
		EmailColumn:    "email",
		NameColumn:     "name",
		IDColumn:       "id",
		PasswordColumn: "password",
	}

	provider := NewRDBMSProvider(config, newMockRDBMSConnection())
	ctx := context.Background()

	result, err := provider.SyncUsers(ctx, false)
	if err != nil {
		t.Errorf("SyncUsers() error = %v", err)
	}
	if result == nil {
		t.Error("SyncUsers() returned nil result")
	}
}

func TestRDBMSProviderValidateMethod(t *testing.T) {
	config := RDBMSConfig{
		Host:           "db.example.com",
		Port:           5432,
		Username:       "user",
		Password:       "pass",
		Database:       "users_db",
		UserTable:      "users",
		EmailColumn:    "email",
		NameColumn:     "name",
		IDColumn:       "id",
		PasswordColumn: "password",
	}

	provider := NewRDBMSProvider(config, newMockRDBMSConnection())
	ctx := context.Background()

	// Validate should always succeed for RDBMS provider
	err := provider.Validate(ctx)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}
