package idprovider

import "context"

// IdentityProvider defines the contract for user and group management.
// Implementations can be database-only, LDAP, Azure AD, or external RDBMS.
type IdentityProvider interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	GetGroup(ctx context.Context, groupID string) (*Group, error)
	ListUsers(ctx context.Context, filter *UserFilter) ([]*User, error)
	ListGroups(ctx context.Context, filter *GroupFilter) ([]*Group, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID string) error
	CreateGroup(ctx context.Context, group *Group) error
	DeleteGroup(ctx context.Context, groupID string) error
	AddMember(ctx context.Context, groupID string, member *Member) error
	RemoveMember(ctx context.Context, groupID, memberID string) error
}

type UserFilter struct {
	OrgID       *string
	SearchQuery *string
	Limit       int
	Offset      int
}

type GroupFilter struct {
	OrgID       *string
	SearchQuery *string
	Limit       int
	Offset      int
}
