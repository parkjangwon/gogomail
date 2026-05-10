package admin

import (
	"context"
	"fmt"
	"time"
)

// LDAPConfig holds LDAP connection configuration
type LDAPConfig struct {
	ServerURL    string `json:"server_url"`
	BaseDN       string `json:"base_dn"`
	BindDN       string `json:"bind_dn"`
	BindPassword string `json:"bind_password"`
	UserFilter   string `json:"user_filter"`
	EmailAttr    string `json:"email_attr"`
	NameAttr     string `json:"name_attr"`
	UIDAttr      string `json:"uid_attr"`
}

// ldapUser represents a user from LDAP
type ldapUser struct {
	dn         string
	attributes map[string][]string
}

// LDAPClient interface for LDAP operations
type LDAPClient interface {
	Bind(username, password string) error
	Search(baseDN, filter string) ([]*ldapUser, error)
	Close() error
}

// LDAPProvider implements IdentityProvider for LDAP backends
type LDAPProvider struct {
	config LDAPConfig
	client LDAPClient
}

// NewLDAPProvider creates a new LDAP provider
func NewLDAPProvider(config LDAPConfig, client LDAPClient) *LDAPProvider {
	return &LDAPProvider{
		config: config,
		client: client,
	}
}

// Authenticate authenticates a user against LDAP
func (lp *LDAPProvider) Authenticate(ctx context.Context, credentials map[string]string) (*ProviderUser, error) {
	email, ok := credentials["email"]
	if !ok || email == "" {
		return nil, fmt.Errorf("email required")
	}

	password, ok := credentials["password"]
	if !ok || password == "" {
		return nil, fmt.Errorf("password required")
	}

	// Try to bind with user's credentials
	// In real LDAP, would construct DN from email
	// For now, use email as username
	if err := lp.client.Bind(email, password); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Search for user details
	filter := fmt.Sprintf("(%s=%s)", lp.config.EmailAttr, email)
	users, err := lp.client.Search(lp.config.BaseDN, filter)
	if err != nil || len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	user := users[0]
	return lp.ldapUserToProviderUser(user), nil
}

// GetUser retrieves a user from LDAP
func (lp *LDAPProvider) GetUser(ctx context.Context, userID string) (*ProviderUser, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	filter := fmt.Sprintf("(%s=%s)", lp.config.UIDAttr, userID)
	users, err := lp.client.Search(lp.config.BaseDN, filter)
	if err != nil || len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return lp.ldapUserToProviderUser(users[0]), nil
}

// ListUsers lists users from LDAP
func (lp *LDAPProvider) ListUsers(ctx context.Context, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error) {
	// Use configured user filter to get all users
	ldapFilter := lp.config.UserFilter
	if ldapFilter == "" {
		ldapFilter = "(objectClass=inetOrgPerson)"
	}

	users, err := lp.client.Search(lp.config.BaseDN, ldapFilter)
	if err != nil {
		return nil, 0, err
	}

	var providerUsers []*ProviderUser
	for _, user := range users {
		providerUsers = append(providerUsers, lp.ldapUserToProviderUser(user))
	}

	// Apply offset and limit
	start := offset
	end := start + limit
	if end > len(providerUsers) {
		end = len(providerUsers)
	}
	if start > len(providerUsers) {
		start = len(providerUsers)
	}

	return providerUsers[start:end], int64(len(providerUsers)), nil
}

// SyncUsers syncs users from LDAP
func (lp *LDAPProvider) SyncUsers(ctx context.Context, incremental bool) (*SyncResult, error) {
	ldapFilter := lp.config.UserFilter
	if ldapFilter == "" {
		ldapFilter = "(objectClass=inetOrgPerson)"
	}

	users, err := lp.client.Search(lp.config.BaseDN, ldapFilter)
	if err != nil {
		return nil, err
	}

	// Count users found
	result := &SyncResult{
		Created:   len(users),
		Updated:   0,
		Deleted:   0,
		Failed:    0,
		Duration:  0,
		LastToken: fmt.Sprintf("sync-%d", time.Now().Unix()),
	}

	return result, nil
}

// Validate validates the LDAP configuration
func (lp *LDAPProvider) Validate(ctx context.Context) error {
	if lp.config.ServerURL == "" {
		return fmt.Errorf("ServerURL required")
	}
	if lp.config.BaseDN == "" {
		return fmt.Errorf("BaseDN required")
	}
	if lp.config.UserFilter == "" && lp.config.EmailAttr == "" {
		return fmt.Errorf("UserFilter or EmailAttr required")
	}
	if lp.config.BindPassword == "" && lp.config.BindDN != "" {
		return fmt.Errorf("BindPassword required when BindDN is set")
	}

	// Try to connect and bind
	if err := lp.client.Bind(lp.config.BindDN, lp.config.BindPassword); err != nil {
		return fmt.Errorf("LDAP connection failed: %w", err)
	}

	return nil
}

// ldapUserToProviderUser converts LDAP user to ProviderUser
func (lp *LDAPProvider) ldapUserToProviderUser(ldapUser *ldapUser) *ProviderUser {
	email := lp.getAttr(ldapUser, lp.config.EmailAttr)
	name := lp.getAttr(ldapUser, lp.config.NameAttr)
	uid := lp.getAttr(ldapUser, lp.config.UIDAttr)

	return &ProviderUser{
		ExternalID: uid,
		Email:      email,
		Name:       name,
		Attributes: map[string]string{
			"dn": ldapUser.dn,
		},
	}
}

// getAttr gets an attribute value from LDAP user
func (lp *LDAPProvider) getAttr(user *ldapUser, attrName string) string {
	if user == nil || user.attributes == nil {
		return ""
	}

	values, ok := user.attributes[attrName]
	if !ok || len(values) == 0 {
		return ""
	}

	return values[0]
}

// ValidateLDAPConfig validates LDAP configuration
func ValidateLDAPConfig(config LDAPConfig) error {
	if config.ServerURL == "" {
		return fmt.Errorf("%w: ServerURL", ErrMissingRequiredField)
	}
	if config.BaseDN == "" {
		return fmt.Errorf("%w: BaseDN", ErrMissingRequiredField)
	}
	if config.UserFilter == "" && config.EmailAttr == "" {
		return fmt.Errorf("either UserFilter or EmailAttr must be set")
	}
	if config.EmailAttr == "" {
		return fmt.Errorf("%w: EmailAttr", ErrMissingRequiredField)
	}
	if config.NameAttr == "" {
		return fmt.Errorf("%w: NameAttr", ErrMissingRequiredField)
	}
	if config.UIDAttr == "" {
		return fmt.Errorf("%w: UIDAttr", ErrMissingRequiredField)
	}

	return nil
}
