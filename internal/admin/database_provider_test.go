package admin

import (
	"context"
	"testing"
)

func TestDatabaseProviderAuthenticate(t *testing.T) {
	repo := newMockUserRepository()
	provider := NewDatabaseProvider(repo)
	ctx := context.Background()

	// Create test user with password
	testUser := &User{
		CompanyID: "company-1",
		Email:     "user@example.com",
		Name:      "Test User",
		Status:    "active",
	}
	hash, _ := HashPassword("password123")
	testUser.PasswordHash = hash
	repo.CreateUser(ctx, testUser)

	tests := []struct {
		name      string
		email     string
		password  string
		shouldErr bool
	}{
		{
			name:      "valid credentials",
			email:     "user@example.com",
			password:  "password123",
			shouldErr: false,
		},
		{
			name:      "wrong password",
			email:     "user@example.com",
			password:  "wrongpass",
			shouldErr: true,
		},
		{
			name:      "user not found",
			email:     "notfound@example.com",
			password:  "password123",
			shouldErr: true,
		},
		{
			name:      "missing email",
			email:     "",
			password:  "password123",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := map[string]string{
				"email":      tt.email,
				"password":   tt.password,
				"company_id": "company-1",
			}
			user, err := provider.Authenticate(ctx, creds)
			if (err != nil) != tt.shouldErr {
				t.Errorf("Authenticate() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && user.Email != tt.email {
				t.Errorf("Authenticate() returned wrong user")
			}
		})
	}
}

func TestDatabaseProviderGetUser(t *testing.T) {
	repo := newMockUserRepository()
	provider := NewDatabaseProvider(repo)
	ctx := context.Background()

	// Create test user
	testUser := &User{
		CompanyID: "company-1",
		Email:     "user@example.com",
		Name:      "Test User",
		Status:    "active",
	}
	repo.CreateUser(ctx, testUser)

	tests := []struct {
		name      string
		userID    string
		shouldErr bool
	}{
		{
			name:      "get existing user",
			userID:    testUser.ID,
			shouldErr: false,
		},
		{
			name:      "user not found",
			userID:    "nonexistent",
			shouldErr: true,
		},
		{
			name:      "empty userID",
			userID:    "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := provider.GetUser(ctx, tt.userID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetUser() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && user == nil {
				t.Error("GetUser() returned nil user")
			}
		})
	}
}

func TestDatabaseProviderListUsers(t *testing.T) {
	repo := newMockUserRepository()
	provider := NewDatabaseProvider(repo)
	ctx := context.Background()

	// Create test users
	users := []*User{
		{CompanyID: "company-1", Email: "user1@example.com", Name: "User 1", Status: "active"},
		{CompanyID: "company-1", Email: "user2@example.com", Name: "User 2", Status: "active"},
		{CompanyID: "company-2", Email: "user3@example.com", Name: "User 3", Status: "active"},
	}
	for _, user := range users {
		repo.CreateUser(ctx, user)
	}

	tests := []struct {
		name      string
		filter    map[string]string
		limit     int
		offset    int
		shouldErr bool
	}{
		{
			name:      "list all users",
			filter:    map[string]string{"company_id": "company-1"},
			limit:     10,
			offset:    0,
			shouldErr: false,
		},
		{
			name:      "missing company_id filter",
			filter:    map[string]string{},
			limit:     10,
			offset:    0,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, count, err := provider.ListUsers(ctx, tt.filter, tt.limit, tt.offset)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ListUsers() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && count < 0 {
				t.Errorf("ListUsers() returned negative count")
			}
			if err == nil && len(users) == 0 && count > 0 {
				t.Error("ListUsers() count mismatch")
			}
		})
	}
}

func TestDatabaseProviderSync(t *testing.T) {
	repo := newMockUserRepository()
	provider := NewDatabaseProvider(repo)
	ctx := context.Background()

	// SyncUsers is a no-op for database provider
	result, err := provider.SyncUsers(ctx, false)
	if err != nil {
		t.Errorf("SyncUsers() error = %v", err)
	}
	if result == nil {
		t.Error("SyncUsers() returned nil result")
	}
}

func TestDatabaseProviderValidate(t *testing.T) {
	repo := newMockUserRepository()
	provider := NewDatabaseProvider(repo)
	ctx := context.Background()

	// Validate should always succeed for database provider
	err := provider.Validate(ctx)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestDatabaseProviderSetPassword(t *testing.T) {
	repo := newMockUserRepository()
	provider := NewDatabaseProvider(repo)
	ctx := context.Background()

	// Create test user
	testUser := &User{
		CompanyID: "company-1",
		Email:     "user@example.com",
		Name:      "Test User",
		Status:    "active",
	}
	repo.CreateUser(ctx, testUser)

	tests := []struct {
		name        string
		userID      string
		newPassword string
		shouldErr   bool
	}{
		{
			name:        "set password for existing user",
			userID:      testUser.ID,
			newPassword: "newpass123",
			shouldErr:   false,
		},
		{
			name:        "user not found",
			userID:      "nonexistent",
			newPassword: "newpass123",
			shouldErr:   true,
		},
		{
			name:        "empty password",
			userID:      testUser.ID,
			newPassword: "",
			shouldErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.SetPassword(ctx, tt.userID, tt.newPassword)
			if (err != nil) != tt.shouldErr {
				t.Errorf("SetPassword() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
