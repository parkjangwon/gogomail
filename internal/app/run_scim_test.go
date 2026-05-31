package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/scim"
)

type fakeSCIMUserRepository struct {
	users     map[string]maildb.UserView
	updateErr error
}

func (r *fakeSCIMUserRepository) GetUser(_ context.Context, id string) (maildb.UserView, error) {
	user, ok := r.users[id]
	if !ok {
		return maildb.UserView{}, errors.New("not found")
	}
	return user, nil
}

func (r *fakeSCIMUserRepository) ListUsers(context.Context, maildb.UserListRequest) ([]maildb.UserView, bool, error) {
	users := make([]maildb.UserView, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users, false, nil
}

func (r *fakeSCIMUserRepository) CreateUser(_ context.Context, req maildb.CreateUserRequest) (maildb.UserView, error) {
	user := maildb.UserView{
		ID:          "created-user",
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Status:      "active",
		CreatedAt:   time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	r.users[user.ID] = user
	return user, nil
}

func (r *fakeSCIMUserRepository) UpdateUserStatus(_ context.Context, req maildb.UpdateUserStatusRequest) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	user, ok := r.users[req.ID]
	if !ok {
		return errors.New("not found")
	}
	user.Status = req.Status
	r.users[req.ID] = user
	return nil
}

func newFakeSCIMService(logger *slog.Logger, updateErr error) *maildbSCIMUserService {
	return &maildbSCIMUserService{
		repo: &fakeSCIMUserRepository{
			updateErr: updateErr,
			users: map[string]maildb.UserView{
				"user-1": {
					ID:          "user-1",
					Username:    "user@example.com",
					DisplayName: "User One",
					Status:      "active",
					CreatedAt:   time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		logger: logger,
	}
}

func TestSCIMPatchUserLogsStatusUpdateFailure(t *testing.T) {
	t.Parallel()

	updateErr := errors.New("idp status sync unavailable")
	var logs bytes.Buffer
	service := newFakeSCIMService(slog.New(slog.NewTextHandler(&logs, nil)), updateErr)

	_, err := service.PatchSCIMUser(context.Background(), "user-1", []scim.PatchOperation{{
		Op:    "replace",
		Path:  "active",
		Value: json.RawMessage(`false`),
	}})
	if !errors.Is(err, updateErr) {
		t.Fatalf("PatchSCIMUser err = %v, want update error", err)
	}
	output := logs.String()
	if !strings.Contains(output, "scim user status update failed") ||
		!strings.Contains(output, "patch_active") ||
		!strings.Contains(output, "user-1") ||
		!strings.Contains(output, "suspended") ||
		!strings.Contains(output, "idp status sync unavailable") {
		t.Fatalf("logs = %q, want SCIM update failure context", output)
	}
}

func TestSCIMDeleteUserLogsStatusUpdateFailure(t *testing.T) {
	t.Parallel()

	updateErr := errors.New("idp status sync unavailable")
	var logs bytes.Buffer
	service := newFakeSCIMService(slog.New(slog.NewTextHandler(&logs, nil)), updateErr)

	err := service.DeleteSCIMUser(context.Background(), "user-1")
	if !errors.Is(err, httpapi.ErrSCIMUserNotFound) {
		t.Fatalf("DeleteSCIMUser err = %v, want mapped not found", err)
	}
	output := logs.String()
	if !strings.Contains(output, "scim user status update failed") ||
		!strings.Contains(output, "delete") ||
		!strings.Contains(output, "user-1") ||
		!strings.Contains(output, "suspended") ||
		!strings.Contains(output, "idp status sync unavailable") {
		t.Fatalf("logs = %q, want SCIM delete failure context", output)
	}
}
