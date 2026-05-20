package scim

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	SchemaUser  = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaList  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
)

type Filter struct {
	Attribute string
	Operator  string
	Value     string
}

func ParseFilter(input string) (*Filter, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty filter")
	}

	input = strings.Trim(input, `"`)

	parts := strings.Fields(input)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid filter format")
	}

	value := strings.Join(parts[2:], " ")
	value = strings.Trim(value, `"`)

	return &Filter{
		Attribute: parts[0],
		Operator:  parts[1],
		Value:     value,
	}, nil
}

type Name struct {
	Formatted       string `json:"formatted,omitempty"`
	FamilyName      string `json:"familyName,omitempty"`
	GivenName       string `json:"givenName,omitempty"`
	MiddleName      string `json:"middleName,omitempty"`
	HonorificPrefix string `json:"honorificPrefix,omitempty"`
	HonorificSuffix string `json:"honorificSuffix,omitempty"`
}

type Email struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

type UserResource struct {
	Schemas    []string `json:"schemas"`
	ID         string   `json:"id"`
	ExternalID string   `json:"externalId,omitempty"`
	UserName   string   `json:"userName"`
	Name       Name     `json:"name,omitempty"`
	Emails     []Email  `json:"emails,omitempty"`
	Active     bool     `json:"active"`
	Meta       *Meta    `json:"meta,omitempty"`
}

type Meta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created"`
	LastModified string `json:"lastModified"`
	Version      string `json:"version"`
	Location     string `json:"location"`
}

type ListResponse struct {
	Schemas      []string       `json:"schemas"`
	TotalResults int            `json:"totalResults"`
	Resources    []UserResource `json:"Resources"`
	StartIndex   int            `json:"startIndex,omitempty"`
	ItemsPerPage int            `json:"itemsPerPage,omitempty"`
}

// PatchOperation represents a single RFC 7644 PATCH operation.
type PatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

func NewUserResource(id, userName string) UserResource {
	return UserResource{
		Schemas:  []string{SchemaUser},
		ID:       id,
		UserName: userName,
		Active:   true,
	}
}

func NewListResponse(users []UserResource) ListResponse {
	return ListResponse{
		Schemas:      []string{SchemaList},
		TotalResults: len(users),
		Resources:    users,
		StartIndex:   1,
		ItemsPerPage: len(users),
	}
}

func normalizeAttribute(attr string) string {
	return strings.ToLower(attr)
}

func MatchesFilter(user UserResource, filter *Filter) bool {
	attr := normalizeAttribute(filter.Attribute)
	val := filter.Value

	switch attr {
	case "username":
		return matchString(user.UserName, filter.Operator, val)
	case "externalid":
		return matchString(user.ExternalID, filter.Operator, val)
	case "active":
		b, _ := strconv.ParseBool(val)
		return user.Active == b
	case "name.givenname":
		return matchString(user.Name.GivenName, filter.Operator, val)
	case "name.familyname":
		return matchString(user.Name.FamilyName, filter.Operator, val)
	case "emails.value":
		for _, e := range user.Emails {
			if matchString(e.Value, filter.Operator, val) {
				return true
			}
		}
		return false
	}
	return false
}

func matchString(s, op, val string) bool {
	switch op {
	case "eq":
		return strings.EqualFold(s, val)
	case "sw":
		return strings.HasPrefix(strings.ToLower(s), strings.ToLower(val))
	case "co":
		return strings.Contains(strings.ToLower(s), strings.ToLower(val))
	case "ew":
		return strings.HasSuffix(strings.ToLower(s), strings.ToLower(val))
	}
	return false
}
