package apimeter

import "strings"

const (
	AuthSourceAnonymous   = "anonymous"
	AuthSourceUnknown     = "unknown"
	AuthSourceBearer      = "bearer"
	AuthSourceAdminToken  = "admin_token"
	AuthSourceQueryUserID = "query_user_id"
)

// Identity carries stable usage dimensions for aggregation and later billing.
type Identity struct {
	TenantID    string
	CompanyID   string
	DomainID    string
	UserID      string
	APIKeyID    string
	PrincipalID string
	AuthSource  string
}

func (id Identity) Normalize() Identity {
	id.TenantID = strings.TrimSpace(id.TenantID)
	id.CompanyID = strings.TrimSpace(id.CompanyID)
	id.DomainID = strings.TrimSpace(id.DomainID)
	id.UserID = strings.TrimSpace(id.UserID)
	id.APIKeyID = strings.TrimSpace(id.APIKeyID)
	id.PrincipalID = strings.TrimSpace(id.PrincipalID)
	id.AuthSource = normalizeAuthSource(id.AuthSource)
	if id.PrincipalID == "" {
		id.PrincipalID = principalID(id)
	}
	return id
}

func principalID(id Identity) string {
	switch {
	case id.UserID != "":
		return id.UserID
	case id.APIKeyID != "":
		return id.APIKeyID
	case id.AuthSource == AuthSourceAdminToken:
		return AuthSourceAdminToken
	case id.AuthSource == AuthSourceAnonymous:
		return AuthSourceAnonymous
	default:
		return ""
	}
}

func normalizeAuthSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case AuthSourceAnonymous:
		return AuthSourceAnonymous
	case AuthSourceBearer:
		return AuthSourceBearer
	case AuthSourceAdminToken:
		return AuthSourceAdminToken
	case AuthSourceQueryUserID:
		return AuthSourceQueryUserID
	default:
		return AuthSourceUnknown
	}
}
