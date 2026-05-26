package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/scim"
)

type maildbSCIMUserService struct {
	repo            *maildb.Repository
	defaultDomainID string
}

func (s *maildbSCIMUserService) GetSCIMUser(ctx context.Context, id string) (scim.UserResource, error) {
	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	return maildbUserToSCIM(user), nil
}

func (s *maildbSCIMUserService) ListSCIMUsers(ctx context.Context, filter *scim.Filter, _, count int) ([]scim.UserResource, int, error) {
	users, _, err := s.repo.ListUsers(ctx, maildb.UserListRequest{
		DomainID: s.defaultDomainID,
		Limit:    count,
	})
	if err != nil {
		return nil, 0, err
	}
	var resources []scim.UserResource
	for _, u := range users {
		r := maildbUserToSCIM(u)
		if filter == nil || scim.MatchesFilter(r, filter) {
			resources = append(resources, r)
		}
	}
	return resources, len(resources), nil
}

func (s *maildbSCIMUserService) CreateSCIMUser(ctx context.Context, req scim.UserResource) (scim.UserResource, error) {
	domainID := s.defaultDomainID
	if domainID == "" {
		return scim.UserResource{}, fmt.Errorf("SCIM create requires GOGOMAIL_SCIM_DEFAULT_DOMAIN_ID")
	}
	displayName := req.Name.Formatted
	if displayName == "" {
		displayName = req.UserName
	}
	user, err := s.repo.CreateUser(ctx, maildb.CreateUserRequest{
		DomainID:    domainID,
		Username:    req.UserName,
		DisplayName: displayName,
		Address:     req.UserName,
	})
	if err != nil {
		return scim.UserResource{}, err
	}
	return maildbUserToSCIM(user), nil
}

func (s *maildbSCIMUserService) ReplaceSCIMUser(ctx context.Context, id string, req scim.UserResource) (scim.UserResource, error) {
	status := "active"
	if !req.Active {
		status = "suspended"
	}
	if err := s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: status}); err != nil {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	return s.GetSCIMUser(ctx, id)
}

func (s *maildbSCIMUserService) PatchSCIMUser(ctx context.Context, id string, ops []scim.PatchOperation) (scim.UserResource, error) {
	// Verify the user exists first.
	if _, err := s.repo.GetUser(ctx, id); err != nil {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	for _, op := range ops {
		switch strings.ToLower(op.Op) {
		case "replace":
			// Handle path-less replace with a value object.
			if op.Path == "" {
				var attrs map[string]json.RawMessage
				if err := json.Unmarshal(op.Value, &attrs); err != nil {
					continue
				}
				if raw, ok := attrs["active"]; ok {
					var active bool
					if err := json.Unmarshal(raw, &active); err == nil {
						status := "active"
						if !active {
							status = "suspended"
						}
						_ = s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: status})
					}
				}
				continue
			}
			// Handle path-targeted replace.
			switch strings.ToLower(op.Path) {
			case "active":
				var active bool
				if err := json.Unmarshal(op.Value, &active); err != nil {
					continue
				}
				status := "active"
				if !active {
					status = "suspended"
				}
				_ = s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: status})
			}
		}
	}
	return s.GetSCIMUser(ctx, id)
}

func (s *maildbSCIMUserService) DeleteSCIMUser(ctx context.Context, id string) error {
	if err := s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: "suspended"}); err != nil {
		return httpapi.ErrSCIMUserNotFound
	}
	return nil
}

func maildbUserToSCIM(u maildb.UserView) scim.UserResource {
	return scim.UserResource{
		Schemas:  []string{scim.SchemaUser},
		ID:       u.ID,
		UserName: u.Username,
		Name:     scim.Name{Formatted: u.DisplayName},
		Active:   u.Status == "active",
		Meta: &scim.Meta{
			ResourceType: "User",
			Created:      u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			LastModified: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Location:     "/scim/v2/Users/" + u.ID,
		},
	}
}
