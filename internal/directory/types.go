package directory

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gogomail/gogomail/internal/mail"
)

const (
	PrincipalKindUser         = "user"
	PrincipalKindOrganization = "organization"
	PrincipalKindGroup        = "group"
	PrincipalKindResource     = "resource"

	ResourceTypeRoom      = "room"
	ResourceTypeEquipment = "equipment"
	ResourceTypeVehicle   = "vehicle"
	ResourceTypeOther     = "other"

	MaxPrincipalIDBytes = 200
)

type Principal struct {
	ID             string
	Kind           string
	CompanyID      string
	DomainID       string
	OrganizationID string
	DisplayName    string
	PrimaryEmail   string
	Status         string
	ResourceType   string
}

type ResolvePrincipalRequest struct {
	ID         string
	Kind       string
	ActiveOnly bool
}

type Alias struct {
	ID              string
	CompanyID       string
	DomainID        string
	Address         string
	AddressACE      string
	TargetKind      string
	TargetID        string
	Status          string
	TargetPrincipal Principal
}

type ResolveAliasRequest struct {
	Address    string
	ActiveOnly bool
}

func NormalizePrincipalKind(kind string) (string, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		return PrincipalKindUser, nil
	}
	switch kind {
	case PrincipalKindUser, PrincipalKindOrganization, PrincipalKindGroup, PrincipalKindResource:
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported principal kind %q", kind)
	}
}

func NormalizePrincipalID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("principal id is required")
	}
	if len(id) > MaxPrincipalIDBytes {
		return "", fmt.Errorf("principal id is too long")
	}
	if strings.ContainsAny(id, "\r\n") {
		return "", fmt.Errorf("principal id must not contain line breaks")
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("principal id must not contain control characters")
		}
	}
	return id, nil
}

func NormalizeResolvePrincipalRequest(req ResolvePrincipalRequest) (ResolvePrincipalRequest, error) {
	id, err := NormalizePrincipalID(req.ID)
	if err != nil {
		return ResolvePrincipalRequest{}, err
	}
	kind, err := NormalizePrincipalKind(req.Kind)
	if err != nil {
		return ResolvePrincipalRequest{}, err
	}
	req.ID = id
	req.Kind = kind
	return req, nil
}

func NormalizeResolveAliasRequest(req ResolveAliasRequest) (ResolveAliasRequest, error) {
	address, err := mail.NormalizeAddress(req.Address)
	if err != nil {
		return ResolveAliasRequest{}, err
	}
	if len(address) > 320 {
		return ResolveAliasRequest{}, fmt.Errorf("alias address is too long")
	}
	if strings.ContainsAny(address, "\r\n") {
		return ResolveAliasRequest{}, fmt.Errorf("alias address must not contain line breaks")
	}
	req.Address = address
	return req, nil
}
