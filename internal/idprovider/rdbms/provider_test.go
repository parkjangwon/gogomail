package rdbms

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/idprovider"
)

func TestNewProvider(t *testing.T) {
	cfg := &Config{
		ConnectionString: "postgres://localhost/test",
		MaxPoolSize:      5,
		UserQuery:        "SELECT id, name FROM users",
		GroupQuery:       "SELECT id, name FROM groups",
		FieldMap: map[string]string{
			"id":           "id",
			"username":     "name",
			"display_name": "name",
		},
	}

	p := New(cfg)
	if p == nil {
		t.Fatal("expected provider, got nil")
	}
	if p.config != cfg {
		t.Fatal("expected config to be set")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name: "empty connection string",
			cfg: &Config{
				ConnectionString: "",
			},
			wantErr: true,
		},
		{
			name: "missing user query",
			cfg: &Config{
				ConnectionString: "postgres://localhost/test",
				GroupQuery:       "SELECT * FROM groups",
			},
			wantErr: true,
		},
		{
			name: "missing group query",
			cfg: &Config{
				ConnectionString: "postgres://localhost/test",
				UserQuery:        "SELECT * FROM users",
			},
			wantErr: true,
		},
		{
			name: "missing field map",
			cfg: &Config{
				ConnectionString: "postgres://localhost/test",
				UserQuery:        "SELECT * FROM users",
				GroupQuery:       "SELECT * FROM groups",
				FieldMap:         nil,
			},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: &Config{
				ConnectionString: "postgres://localhost/test",
				UserQuery:        "SELECT id, name FROM users",
				GroupQuery:       "SELECT id, name FROM groups",
				FieldMap: map[string]string{
					"id":           "id",
					"username":     "name",
					"display_name": "name",
				},
				MaxPoolSize: 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFieldMap(t *testing.T) {
	tests := []struct {
		name      string
		fieldMap  map[string]string
		mapType   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "nil field map",
			fieldMap:  nil,
			mapType:   "user",
			wantErr:   true,
			errSubstr: "field_map is required",
		},
		{
			name:      "empty field map",
			fieldMap:  map[string]string{},
			mapType:   "user",
			wantErr:   true,
			errSubstr: "field_map is required",
		},
		{
			name: "user missing id",
			fieldMap: map[string]string{
				"username":     "name",
				"display_name": "display",
			},
			mapType:   "user",
			wantErr:   true,
			errSubstr: "required field \"id\"",
		},
		{
			name: "group missing slug",
			fieldMap: map[string]string{
				"id":   "id",
				"name": "name",
			},
			mapType:   "group",
			wantErr:   true,
			errSubstr: "required field \"slug\"",
		},
		{
			name: "valid user map",
			fieldMap: map[string]string{
				"id":           "user_id",
				"username":     "user_name",
				"display_name": "full_name",
			},
			mapType: "user",
			wantErr: false,
		},
		{
			name: "valid group map",
			fieldMap: map[string]string{
				"id":   "group_id",
				"name": "group_name",
				"slug": "group_slug",
			},
			mapType: "group",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldMap(tt.fieldMap, tt.mapType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateFieldMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errSubstr != "" {
				if !contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
			}
		})
	}
}

func TestCreateUserReadOnly(t *testing.T) {
	p := New(&Config{})

	user := &idprovider.User{
		DomainID: "domain-1",
		Username: "test",
	}

	err := p.CreateUser(context.Background(), user)
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestUpdateUserReadOnly(t *testing.T) {
	p := New(&Config{})

	user := &idprovider.User{
		ID: "user-1",
	}

	err := p.UpdateUser(context.Background(), user)
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestDeleteUserReadOnly(t *testing.T) {
	p := New(&Config{})

	err := p.DeleteUser(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestCreateGroupReadOnly(t *testing.T) {
	p := New(&Config{})

	group := &idprovider.Group{
		DomainID: "domain-1",
		Name:     "test",
	}

	err := p.CreateGroup(context.Background(), group)
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestDeleteGroupReadOnly(t *testing.T) {
	p := New(&Config{})

	err := p.DeleteGroup(context.Background(), "group-1")
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestAddMemberReadOnly(t *testing.T) {
	p := New(&Config{})

	member := &idprovider.Member{
		Kind:     "user",
		MemberID: "user-1",
	}

	err := p.AddMember(context.Background(), "group-1", member)
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestRemoveMemberReadOnly(t *testing.T) {
	p := New(&Config{})

	err := p.RemoveMember(context.Background(), "group-1", "user-1")
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestGetUserWithoutConnection(t *testing.T) {
	p := New(&Config{})

	_, err := p.GetUser(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestGetGroupWithoutConnection(t *testing.T) {
	p := New(&Config{})

	_, err := p.GetGroup(context.Background(), "group-1")
	if err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestListUsersWithoutConnection(t *testing.T) {
	p := New(&Config{})

	_, err := p.ListUsers(context.Background(), nil)
	if err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestListGroupsWithoutConnection(t *testing.T) {
	p := New(&Config{})

	_, err := p.ListGroups(context.Background(), nil)
	if err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestValidateSourceQuery(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid select", "SELECT id, name FROM users", false},
		{"valid select lowercase", "  select id from t where x=$1  ", false},
		{"no select prefix", "DELETE FROM users", true},
		{"union injection", "SELECT * FROM users UNION SELECT * FROM secrets", true},
		{"insert injection", "SELECT id FROM (INSERT INTO log VALUES(1)) t", true},
		{"drop injection", "SELECT id FROM t; DROP TABLE t", true},
		{"semicolon end only", "SELECT id FROM t;", false},
		{"semicolon in middle", "SELECT id FROM t; SELECT 1", true},
		{"too long", string(make([]byte, 4097)), true},
		{"exec injection", "SELECT EXEC('cmd')", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSourceQuery(c.query)
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
