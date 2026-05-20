package ldap

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogomail/gogomail/internal/idprovider"
)

const (
	// CapabilityStatusUnavailable marks LDAP sync as unavailable until a live
	// external LDAP backend is configured and wired.
	CapabilityStatusUnavailable = "unavailable"
)

var (
	ErrProviderNotConfigured = errors.New("ldap provider is not configured")
	ErrReadUnavailable       = errors.New("ldap provider read operations are unavailable")
	ErrReadOnly              = errors.New("ldap provider is read-only")
)

// Config represents LDAP server configuration.
type Config struct {
	Host      string
	Port      int
	BaseDN    string
	BindDN    string
	BindPass  string
	UsersDN   string
	GroupsDN  string
	UserAttr  string
	GroupAttr string
	UseSSL    bool
	StartTLS  bool
}

// Provider implements idprovider.IdentityProvider using LDAP directory.
type Provider struct {
	config *Config
}

// New creates a new LDAP identity provider.
func New(cfg *Config) *Provider {
	return &Provider{
		config: cfg,
	}
}

// GetUser retrieves a user by ID from LDAP.
func (p *Provider) GetUser(ctx context.Context, userID string) (*idprovider.User, error) {
	if userID == "" {
		return nil, fmt.Errorf("invalid user id")
	}
	if p.config == nil {
		return nil, ErrProviderNotConfigured
	}
	return nil, ErrReadUnavailable
}

// GetGroup retrieves a group by ID from LDAP.
func (p *Provider) GetGroup(ctx context.Context, groupID string) (*idprovider.Group, error) {
	if groupID == "" {
		return nil, fmt.Errorf("invalid group id")
	}
	if p.config == nil {
		return nil, ErrProviderNotConfigured
	}
	return nil, ErrReadUnavailable
}

// ListUsers lists users from LDAP matching the filter.
func (p *Provider) ListUsers(ctx context.Context, filter *idprovider.UserFilter) ([]*idprovider.User, error) {
	if p.config == nil {
		return nil, ErrProviderNotConfigured
	}
	return nil, ErrReadUnavailable
}

// ListGroups lists groups from LDAP matching the filter.
func (p *Provider) ListGroups(ctx context.Context, filter *idprovider.GroupFilter) ([]*idprovider.Group, error) {
	if p.config == nil {
		return nil, ErrProviderNotConfigured
	}
	return nil, ErrReadUnavailable
}

// CreateUser creates a new user (LDAP is read-only, returns error).
func (p *Provider) CreateUser(ctx context.Context, user *idprovider.User) error {
	if user == nil || user.DomainID == "" {
		return fmt.Errorf("invalid user: missing required fields")
	}
	return ErrReadOnly
}

// UpdateUser updates an existing user (LDAP is read-only, returns error).
func (p *Provider) UpdateUser(ctx context.Context, user *idprovider.User) error {
	if user == nil || user.ID == "" {
		return fmt.Errorf("invalid user: missing id")
	}
	return ErrReadOnly
}

// DeleteUser deletes a user (LDAP is read-only, returns error).
func (p *Provider) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("invalid user id")
	}
	return ErrReadOnly
}

// CreateGroup creates a new group (LDAP is read-only, returns error).
func (p *Provider) CreateGroup(ctx context.Context, group *idprovider.Group) error {
	if group == nil || group.DomainID == "" {
		return fmt.Errorf("invalid group: missing required fields")
	}
	return ErrReadOnly
}

// DeleteGroup deletes a group (LDAP is read-only, returns error).
func (p *Provider) DeleteGroup(ctx context.Context, groupID string) error {
	if groupID == "" {
		return fmt.Errorf("invalid group id")
	}
	return ErrReadOnly
}

// AddMember adds a member to a group (LDAP is read-only, returns error).
func (p *Provider) AddMember(ctx context.Context, groupID string, member *idprovider.Member) error {
	if groupID == "" || member == nil {
		return fmt.Errorf("invalid group id or member")
	}
	return ErrReadOnly
}

// RemoveMember removes a member from a group (LDAP is read-only, returns error).
func (p *Provider) RemoveMember(ctx context.Context, groupID, memberID string) error {
	if groupID == "" || memberID == "" {
		return fmt.Errorf("invalid group id or member id")
	}
	return ErrReadOnly
}
