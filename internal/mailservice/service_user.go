package mailservice

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
)

func (s *Service) MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error) {
	repo, ok := s.repository.(DeliveryStatusRepository)
	if !ok {
		return maildb.MessageDeliveryStatusView{}, fmt.Errorf("delivery status repository is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return maildb.MessageDeliveryStatusView{}, err
	}
	return repo.MessageDeliveryStatus(ctx, userID, messageID)
}

func (s *Service) GetWebmailPreferences(ctx context.Context, userID string) (json.RawMessage, error) {
	repo, ok := s.repository.(PreferencesRepository)
	if !ok {
		return json.RawMessage("{}"), nil
	}
	userID = strings.TrimSpace(userID)
	return repo.GetWebmailPreferences(ctx, userID)
}

func (s *Service) SetWebmailPreferences(ctx context.Context, userID string, prefs json.RawMessage) error {
	repo, ok := s.repository.(PreferencesRepository)
	if !ok {
		return fmt.Errorf("preferences repository is required")
	}
	userID = strings.TrimSpace(userID)
	return repo.SetWebmailPreferences(ctx, userID, prefs)
}

func (s *Service) GetUserProfile(ctx context.Context, userID string) (maildb.UserProfile, error) {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return maildb.UserProfile{}, fmt.Errorf("me repository is required")
	}
	return repo.GetUserProfile(ctx, strings.TrimSpace(userID))
}

func (s *Service) GetUserProfileByEmail(ctx context.Context, email string) (maildb.UserProfile, error) {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return maildb.UserProfile{}, fmt.Errorf("me repository is required")
	}
	return repo.GetUserProfileByEmail(ctx, strings.TrimSpace(email))
}

func (s *Service) ListUserAddresses(ctx context.Context, userID string) ([]maildb.UserAddress, error) {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return nil, fmt.Errorf("me repository is required")
	}
	return repo.ListUserAddresses(ctx, strings.TrimSpace(userID))
}

func (s *Service) UpdateUserDisplayName(ctx context.Context, userID, displayName string) error {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return fmt.Errorf("me repository is required")
	}
	return repo.UpdateUserDisplayName(ctx, strings.TrimSpace(userID), displayName)
}

func (s *Service) UpdateOwnRecoveryEmail(ctx context.Context, userID, recoveryEmail string) error {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return fmt.Errorf("me repository is required")
	}
	return repo.UpdateOwnRecoveryEmail(ctx, strings.TrimSpace(userID), recoveryEmail)
}

func (s *Service) UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string) error {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return fmt.Errorf("me repository is required")
	}
	return repo.UpdateUserAvatarURL(ctx, strings.TrimSpace(userID), avatarURL)
}

func (s *Service) ChangeUserPassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	repo, ok := s.repository.(MeRepository)
	if !ok {
		return fmt.Errorf("me repository is required")
	}
	return repo.ChangeUserPassword(ctx, strings.TrimSpace(userID), currentPassword, newPassword)
}
func (s *Service) ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error) {
	repo, ok := s.repository.(interface {
		ListPushDevices(context.Context, string, int) ([]maildb.PushDevice, error)
	})
	if !ok {
		return nil, fmt.Errorf("push device repository is required")
	}
	userID = strings.TrimSpace(userID)
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListPushDevices(ctx, userID, limit)
}

func (s *Service) UpsertPushDevice(ctx context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.Token = strings.TrimSpace(req.Token)
	req.Label = strings.TrimSpace(req.Label)
	if err := maildb.ValidateUpsertPushDeviceRequest(req); err != nil {
		return maildb.PushDevice{}, err
	}
	repo, ok := s.repository.(interface {
		UpsertPushDevice(context.Context, maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error)
	})
	if !ok {
		return maildb.PushDevice{}, fmt.Errorf("push device repository is required")
	}
	return repo.UpsertPushDevice(ctx, req)
}

func (s *Service) DeletePushDevice(ctx context.Context, userID string, id string) error {
	repo, ok := s.repository.(interface {
		DeletePushDevice(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("push device repository is required")
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if err := validateServiceResourceID("device_id", id); err != nil {
		return fmt.Errorf("delete push device: %w", err)
	}
	return repo.DeletePushDevice(ctx, userID, id)
}

func (s *Service) UpsertWebPushSubscription(ctx context.Context, req maildb.UpsertWebPushSubscriptionRequest) (maildb.WebPushSubscription, error) {
	repo, ok := s.repository.(interface {
		UpsertWebPushSubscription(context.Context, maildb.UpsertWebPushSubscriptionRequest) (maildb.WebPushSubscription, error)
	})
	if !ok {
		return maildb.WebPushSubscription{}, fmt.Errorf("web push subscription repository is required")
	}
	return repo.UpsertWebPushSubscription(ctx, req)
}

func (s *Service) ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]maildb.WebPushSubscription, error) {
	repo, ok := s.repository.(interface {
		ListActiveWebPushSubscriptions(context.Context, string) ([]maildb.WebPushSubscription, error)
	})
	if !ok {
		return nil, fmt.Errorf("web push subscription repository is required")
	}
	userID = strings.TrimSpace(userID)
	return repo.ListActiveWebPushSubscriptions(ctx, userID)
}

func (s *Service) DeleteWebPushSubscription(ctx context.Context, userID, id string) error {
	repo, ok := s.repository.(interface {
		DeleteWebPushSubscription(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("web push subscription repository is required")
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	return repo.DeleteWebPushSubscription(ctx, userID, id)
}

func (s *Service) ListUserMCPAccessKeys(ctx context.Context, userID string) ([]maildb.UserMCPAccessKey, error) {
	repo, ok := s.repository.(interface {
		ListUserMCPAccessKeys(context.Context, string) ([]maildb.UserMCPAccessKey, error)
	})
	if !ok {
		return nil, fmt.Errorf("user mcp access key repository is required")
	}
	return repo.ListUserMCPAccessKeys(ctx, strings.TrimSpace(userID))
}

func (s *Service) CreateUserMCPAccessKey(ctx context.Context, req maildb.CreateUserMCPAccessKeyRequest) (maildb.CreatedUserMCPAccessKey, error) {
	repo, ok := s.repository.(interface {
		CreateUserMCPAccessKey(context.Context, maildb.CreateUserMCPAccessKeyRequest) (maildb.CreatedUserMCPAccessKey, error)
	})
	if !ok {
		return maildb.CreatedUserMCPAccessKey{}, fmt.Errorf("user mcp access key repository is required")
	}
	req.UserID = strings.TrimSpace(req.UserID)
	return repo.CreateUserMCPAccessKey(ctx, req)
}

func (s *Service) RevokeUserMCPAccessKey(ctx context.Context, userID, id string) (maildb.UserMCPAccessKey, error) {
	repo, ok := s.repository.(interface {
		RevokeUserMCPAccessKey(context.Context, string, string) (maildb.UserMCPAccessKey, error)
	})
	if !ok {
		return maildb.UserMCPAccessKey{}, fmt.Errorf("user mcp access key repository is required")
	}
	return repo.RevokeUserMCPAccessKey(ctx, strings.TrimSpace(userID), strings.TrimSpace(id))
}

func (s *Service) GetUserMCPDomainPolicy(ctx context.Context, userID string) (maildb.DomainMCPPolicy, error) {
	repo, ok := s.repository.(interface {
		GetUserMCPDomainPolicy(context.Context, string) (maildb.DomainMCPPolicy, error)
	})
	if !ok {
		return maildb.DomainMCPPolicy{}, fmt.Errorf("user mcp domain policy repository is required")
	}
	return repo.GetUserMCPDomainPolicy(ctx, strings.TrimSpace(userID))
}