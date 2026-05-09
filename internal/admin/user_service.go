package admin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var (
	ErrUserNotFound       = fmt.Errorf("user not found")
	ErrUserAlreadyExists  = fmt.Errorf("user already exists")
	ErrInvalidEmail       = fmt.Errorf("invalid email format")
	ErrInvalidUserStatus  = fmt.Errorf("invalid user status")
)

const (
	UserStatusActive   = "active"
	UserStatusArchived = "archived"
	UserStatusDisabled = "disabled"
)

var validUserStatuses = map[string]bool{
	UserStatusActive:   true,
	UserStatusArchived: true,
	UserStatusDisabled: true,
}

// UserService handles user operations.
type UserService struct {
	userRepo UserRepository
}

// NewUserService creates a new user service.
func NewUserService(userRepo UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// CreateUser creates a new user.
func (s *UserService) CreateUser(ctx context.Context, user *User) error {
	if err := s.validateUser(user); err != nil {
		return err
	}

	if err := s.validateEmail(user.Email); err != nil {
		return err
	}

	// Check if user already exists
	existing, err := s.userRepo.GetUserByEmail(ctx, user.CompanyID, user.Email)
	if err == nil && existing != nil {
		return ErrUserAlreadyExists
	}

	if user.Status == "" {
		user.Status = UserStatusActive
	}

	return s.userRepo.CreateUser(ctx, user)
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	return s.userRepo.GetUser(ctx, userID)
}

// UpdateUser updates a user.
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
	if err := s.validateUser(user); err != nil {
		return err
	}

	if user.ID == "" {
		return fmt.Errorf("%w: id", ErrMissingRequiredField)
	}

	// Get existing user to verify it exists
	existing, err := s.userRepo.GetUser(ctx, user.ID)
	if err != nil {
		return err
	}

	// Don't allow changing email (would require duplicate check)
	// Can be implemented later with proper email verification
	user.Email = existing.Email

	return s.userRepo.UpdateUser(ctx, user)
}

// DeleteUser deletes a user (soft delete via status change).
func (s *UserService) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	user, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = UserStatusArchived
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// ListUsers lists users with filtering.
func (s *UserService) ListUsers(ctx context.Context, filter *UserFilter) ([]*User, int64, error) {
	if filter.CompanyID == "" {
		return nil, 0, fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	return s.userRepo.ListUsers(ctx, filter)
}

// DisableUser disables a user account.
func (s *UserService) DisableUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	user, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = UserStatusDisabled
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// EnableUser enables a user account.
func (s *UserService) EnableUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	user, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = UserStatusActive
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// UpdateLastLogin updates the last login timestamp for a user.
func (s *UserService) UpdateLastLogin(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	return s.userRepo.UpdateLastLogin(ctx, userID)
}

// ValidateUser validates user data.
func (s *UserService) validateUser(user *User) error {
	if user == nil {
		return fmt.Errorf("%w: user", ErrMissingRequiredField)
	}

	if user.CompanyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	if user.Email == "" {
		return fmt.Errorf("%w: email", ErrMissingRequiredField)
	}

	if user.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingRequiredField)
	}

	if user.Status != "" && !validUserStatuses[user.Status] {
		return fmt.Errorf("%w: %s", ErrInvalidUserStatus, user.Status)
	}

	return nil
}

// validateEmail checks if email format is valid.
func (s *UserService) validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("%w: empty", ErrInvalidEmail)
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("%w: %s", ErrInvalidEmail, email)
	}

	if !strings.Contains(parts[1], ".") {
		return fmt.Errorf("%w: %s", ErrInvalidEmail, email)
	}

	return nil
}
