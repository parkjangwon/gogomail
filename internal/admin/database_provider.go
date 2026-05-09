package admin

import (
	"context"
	"fmt"
	"time"
)

// DatabaseProvider implements IdentityProvider for database-backed authentication
type DatabaseProvider struct {
	userRepo UserRepository
}

// NewDatabaseProvider creates a new database identity provider
func NewDatabaseProvider(userRepo UserRepository) *DatabaseProvider {
	return &DatabaseProvider{
		userRepo: userRepo,
	}
}

// Authenticate authenticates a user against the database
func (dp *DatabaseProvider) Authenticate(ctx context.Context, credentials map[string]string) (*ProviderUser, error) {
	email, ok := credentials["email"]
	if !ok || email == "" {
		return nil, fmt.Errorf("email required")
	}

	password, ok := credentials["password"]
	if !ok || password == "" {
		return nil, fmt.Errorf("password required")
	}

	// Get company ID from context (in real implementation)
	// For testing, we'll search through all users
	// In production, this would be extracted from the request context
	companyID, ok := credentials["company_id"]
	if !ok {
		companyID = "company-1" // Default for testing
	}

	user, err := dp.userRepo.GetUserByEmail(ctx, companyID, email)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Verify password
	if err := VerifyPassword(password, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check user status
	if user.Status != UserStatusActive {
		return nil, fmt.Errorf("user account is not active")
	}

	// Update last login
	dp.userRepo.UpdateLastLogin(ctx, user.ID)

	return &ProviderUser{
		ExternalID: user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Attributes: map[string]string{
			"status": user.Status,
			"company_id": user.CompanyID,
		},
	}, nil
}

// GetUser retrieves a user by ID
func (dp *DatabaseProvider) GetUser(ctx context.Context, userID string) (*ProviderUser, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	user, err := dp.userRepo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &ProviderUser{
		ExternalID: user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Attributes: map[string]string{
			"status": user.Status,
			"company_id": user.CompanyID,
		},
	}, nil
}

// ListUsers lists users from the database
func (dp *DatabaseProvider) ListUsers(ctx context.Context, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error) {
	companyID, ok := filter["company_id"]
	if !ok || companyID == "" {
		return nil, 0, fmt.Errorf("company_id required in filter")
	}

	if limit == 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	dbFilter := &UserFilter{
		CompanyID: companyID,
		Status:    filter["status"],
		Limit:     limit,
		Offset:    offset,
	}

	users, count, err := dp.userRepo.ListUsers(ctx, dbFilter)
	if err != nil {
		return nil, 0, err
	}

	var providerUsers []*ProviderUser
	for _, user := range users {
		providerUsers = append(providerUsers, &ProviderUser{
			ExternalID: user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Attributes: map[string]string{
				"status": user.Status,
				"company_id": user.CompanyID,
			},
		})
	}

	return providerUsers, count, nil
}

// SyncUsers is a no-op for database provider (no external sync needed)
func (dp *DatabaseProvider) SyncUsers(ctx context.Context, incremental bool) (*SyncResult, error) {
	// Database provider doesn't need to sync - users are already in DB
	return &SyncResult{
		Created:   0,
		Updated:   0,
		Deleted:   0,
		Failed:    0,
		Duration:  0,
		LastToken: "",
	}, nil
}

// Validate validates the provider configuration
func (dp *DatabaseProvider) Validate(ctx context.Context) error {
	// Database provider is always valid
	return nil
}

// SetPassword sets a password for a user
func (dp *DatabaseProvider) SetPassword(ctx context.Context, userID, newPassword string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	if newPassword == "" {
		return fmt.Errorf("%w: newPassword", ErrMissingRequiredField)
	}

	user, err := dp.userRepo.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	// Hash the new password
	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = hash
	user.UpdatedAt = time.Now()
	return dp.userRepo.UpdateUser(ctx, user)
}

// ResetPassword initiates a password reset for a user
func (dp *DatabaseProvider) ResetPassword(ctx context.Context, userID string) (resetToken string, err error) {
	if userID == "" {
		return "", fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	// In a real implementation, this would:
	// 1. Generate a reset token
	// 2. Store it temporarily (with expiry)
	// 3. Send email with reset link
	// For now, return a placeholder
	return fmt.Sprintf("reset-token-%s-%d", userID, time.Now().Unix()), nil
}

// GetUserByEmail retrieves a user by email
func (dp *DatabaseProvider) GetUserByEmail(ctx context.Context, companyID, email string) (*ProviderUser, error) {
	if companyID == "" {
		return nil, fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if email == "" {
		return nil, fmt.Errorf("%w: email", ErrMissingRequiredField)
	}

	user, err := dp.userRepo.GetUserByEmail(ctx, companyID, email)
	if err != nil {
		return nil, err
	}

	return &ProviderUser{
		ExternalID: user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Attributes: map[string]string{
			"status": user.Status,
			"company_id": user.CompanyID,
		},
	}, nil
}
