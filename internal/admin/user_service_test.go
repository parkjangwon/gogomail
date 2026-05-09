package admin

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockUserRepository implements user operations for testing
type mockUserRepository struct {
	users map[string]*User
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[string]*User),
	}
}

// User represents an admin user or a managed user
type User struct {
	ID             string    `json:"id"`
	CompanyID      string    `json:"company_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	Status         string    `json:"status"` // active, archived, disabled
	PasswordHash   string    `json:"password_hash,omitempty"`
	EmailVerified  bool      `json:"email_verified"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UserFilter holds query parameters for user listing
type UserFilter struct {
	CompanyID string
	Status    string
	Search    string
	Limit     int
	Offset    int
}

// UserRepository defines user data access operations
type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, companyID, email string) (*User, error)
	ListUsers(ctx context.Context, filter *UserFilter) ([]*User, int64, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID string) error
	UpdateLastLogin(ctx context.Context, userID string) error
}

// Mock implementations
func (m *mockUserRepository) CreateUser(ctx context.Context, user *User) error {
	if user.ID == "" {
		user.ID = "user-" + time.Now().Format("20060102150405")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = time.Now()
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepository) GetUser(ctx context.Context, userID string) (*User, error) {
	if user, ok := m.users[userID]; ok {
		return user, nil
	}
	return nil, ErrUserNotFound
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, companyID, email string) (*User, error) {
	for _, user := range m.users {
		if user.CompanyID == companyID && user.Email == email {
			return user, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *mockUserRepository) ListUsers(ctx context.Context, filter *UserFilter) ([]*User, int64, error) {
	var filtered []*User
	for _, user := range m.users {
		if user.CompanyID == filter.CompanyID {
			if filter.Status != "" && user.Status != filter.Status {
				continue
			}
			filtered = append(filtered, user)
		}
	}
	return filtered, int64(len(filtered)), nil
}

func (m *mockUserRepository) UpdateUser(ctx context.Context, user *User) error {
	if _, ok := m.users[user.ID]; !ok {
		return ErrUserNotFound
	}
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepository) DeleteUser(ctx context.Context, userID string) error {
	delete(m.users, userID)
	return nil
}

func (m *mockUserRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	if user, ok := m.users[userID]; ok {
		now := time.Now()
		user.LastLogin = &now
		m.users[userID] = user
		return nil
	}
	return ErrUserNotFound
}

func TestCreateUser(t *testing.T) {
	repo := newMockUserRepository()
	svc := NewUserService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		user      *User
		shouldErr bool
	}{
		{
			name: "valid user creation",
			user: &User{
				CompanyID: "company-1",
				Email:     "user@example.com",
				Name:      "John Doe",
				Status:    "active",
			},
			shouldErr: false,
		},
		{
			name: "missing email",
			user: &User{
				CompanyID: "company-1",
				Name:      "John Doe",
				Status:    "active",
			},
			shouldErr: true,
		},
		{
			name: "missing companyID",
			user: &User{
				Email:  "user@example.com",
				Name:   "John Doe",
				Status: "active",
			},
			shouldErr: true,
		},
		{
			name: "invalid status",
			user: &User{
				CompanyID: "company-1",
				Email:     "user@example.com",
				Name:      "John Doe",
				Status:    "invalid",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateUser(ctx, tt.user)
			if (err != nil) != tt.shouldErr {
				t.Errorf("CreateUser() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	repo := newMockUserRepository()
	ctx := context.Background()

	// Create initial user
	user := &User{
		CompanyID: "company-1",
		Email:     "user@example.com",
		Name:      "John Doe",
		Status:    "active",
	}
	repo.CreateUser(ctx, user)
	userID := user.ID

	tests := []struct {
		name      string
		update    func(*User)
		shouldErr bool
	}{
		{
			name:      "update name",
			update:    func(u *User) { u.Name = "Jane Doe" },
			shouldErr: false,
		},
		{
			name:      "disable user",
			update:    func(u *User) { u.Status = "disabled" },
			shouldErr: false,
		},
		{
			name:      "invalid status",
			update:    func(u *User) { u.Status = "invalid" },
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, _ := repo.GetUser(ctx, userID)
			tt.update(user)
			err := validateUser(user)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateUser() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	repo := newMockUserRepository()
	svc := NewUserService(repo)
	ctx := context.Background()

	// Create test users
	users := []*User{
		{CompanyID: "company-1", Email: "user1@example.com", Name: "User 1", Status: "active"},
		{CompanyID: "company-1", Email: "user2@example.com", Name: "User 2", Status: "disabled"},
		{CompanyID: "company-2", Email: "user3@example.com", Name: "User 3", Status: "active"},
	}
	for _, user := range users {
		svc.CreateUser(ctx, user)
	}

	tests := []struct {
		name      string
		filter    *UserFilter
		shouldErr bool
	}{
		{
			name:      "list all company-1 users",
			filter:    &UserFilter{CompanyID: "company-1"},
			shouldErr: false,
		},
		{
			name:      "list active users only",
			filter:    &UserFilter{CompanyID: "company-1", Status: "active"},
			shouldErr: false,
		},
		{
			name:      "missing companyID",
			filter:    &UserFilter{},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.ListUsers(ctx, tt.filter)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ListUsers() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func validateUser(user *User) error {
	if user == nil {
		return fmt.Errorf("%w: user", ErrMissingRequiredField)
	}
	if user.CompanyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if user.Email == "" {
		return fmt.Errorf("%w: email", ErrMissingRequiredField)
	}
	validStatuses := map[string]bool{
		"active": true,
		"archived": true,
		"disabled": true,
	}
	if !validStatuses[user.Status] {
		return fmt.Errorf("invalid status: %s", user.Status)
	}
	return nil
}
