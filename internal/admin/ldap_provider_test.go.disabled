package admin

import (
	"context"
	"fmt"
	"testing"
)

type mockLDAPClient struct {
	users map[string]*ldapUser
	fail  bool
}

func newMockLDAPClient() *mockLDAPClient {
	return &mockLDAPClient{
		users: make(map[string]*ldapUser),
		fail:  false,
	}
}

func (m *mockLDAPClient) Bind(username, password string) error {
	if m.fail {
		return fmt.Errorf("bind failed")
	}
	return nil
}

func (m *mockLDAPClient) Search(baseDN, filter string) ([]*ldapUser, error) {
	if m.fail {
		return nil, fmt.Errorf("search failed")
	}
	var results []*ldapUser
	for _, user := range m.users {
		results = append(results, user)
	}
	return results, nil
}

func (m *mockLDAPClient) Close() error {
	return nil
}

func TestLDAPProviderValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    LDAPConfig
		shouldErr bool
	}{
		{
			name: "valid config",
			config: LDAPConfig{
				ServerURL:    "ldap://ldap.example.com",
				BaseDN:       "dc=example,dc=com",
				BindDN:       "cn=admin,dc=example,dc=com",
				BindPassword: "password",
				UserFilter:   "(uid=%s)",
				EmailAttr:    "mail",
				NameAttr:     "displayName",
				UIDAttr:      "uid",
			},
			shouldErr: false,
		},
		{
			name: "missing ServerURL",
			config: LDAPConfig{
				BaseDN:       "dc=example,dc=com",
				UserFilter:   "(uid=%s)",
			},
			shouldErr: true,
		},
		{
			name: "missing BaseDN",
			config: LDAPConfig{
				ServerURL:  "ldap://ldap.example.com",
				UserFilter: "(uid=%s)",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMockLDAPClient()
			provider := NewLDAPProvider(tt.config, client)
			ctx := context.Background()
			err := provider.Validate(ctx)
			if (err != nil) != tt.shouldErr {
				t.Errorf("Validate() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestLDAPProviderAuthenticate(t *testing.T) {
	config := LDAPConfig{
		ServerURL:    "ldap://ldap.example.com",
		BaseDN:       "dc=example,dc=com",
		UserFilter:   "(uid=%s)",
		EmailAttr:    "mail",
		NameAttr:     "displayName",
		UIDAttr:      "uid",
	}
	client := newMockLDAPClient()
	client.users["uid=user1,dc=example,dc=com"] = &ldapUser{
		dn: "uid=user1,dc=example,dc=com",
		attributes: map[string][]string{
			"uid":         {"user1"},
			"mail":        {"user1@example.com"},
			"displayName": {"Test User"},
		},
	}

	provider := NewLDAPProvider(config, client)
	ctx := context.Background()

	tests := []struct {
		name      string
		email     string
		password  string
		shouldErr bool
	}{
		{
			name:      "valid credentials",
			email:     "user1@example.com",
			password:  "password",
			shouldErr: false,
		},
		{
			name:      "missing email",
			email:     "",
			password:  "password",
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

func TestLDAPProviderSync(t *testing.T) {
	config := LDAPConfig{
		ServerURL:  "ldap://ldap.example.com",
		BaseDN:     "dc=example,dc=com",
		UserFilter: "(objectClass=inetOrgPerson)",
		EmailAttr:  "mail",
		NameAttr:   "displayName",
		UIDAttr:    "uid",
	}
	client := newMockLDAPClient()
	client.users["uid=user1,dc=example,dc=com"] = &ldapUser{
		dn: "uid=user1,dc=example,dc=com",
		attributes: map[string][]string{
			"uid":         {"user1"},
			"mail":        {"user1@example.com"},
			"displayName": {"User One"},
		},
	}

	provider := NewLDAPProvider(config, client)
	ctx := context.Background()

	result, err := provider.SyncUsers(ctx, false)
	if err != nil {
		t.Errorf("SyncUsers() error = %v", err)
	}
	if result == nil {
		t.Error("SyncUsers() returned nil result")
	}
}
