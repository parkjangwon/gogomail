package pushnotify

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
)

const DefaultTargetLimit = 200

type DeviceRepository interface {
	ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error)
}

type DeviceResolver struct {
	repository DeviceRepository
	limit      int
}

func NewDeviceResolver(repository DeviceRepository, limit int) *DeviceResolver {
	if limit <= 0 || limit > DefaultTargetLimit {
		limit = DefaultTargetLimit
	}
	return &DeviceResolver{repository: repository, limit: limit}
}

func (r *DeviceResolver) ResolvePushTargets(ctx context.Context, event Event) ([]Target, error) {
	if r == nil || r.repository == nil {
		return nil, fmt.Errorf("push device repository is required")
	}
	userID := strings.TrimSpace(event.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	devices, err := r.repository.ListPushDevices(ctx, userID, r.limit)
	if err != nil {
		return nil, err
	}

	targets := make([]Target, 0, len(devices))
	for _, device := range devices {
		token := strings.TrimSpace(device.Token)
		if token == "" {
			continue
		}
		targets = append(targets, Target{
			DeviceID:    strings.TrimSpace(device.ID),
			Platform:    strings.ToLower(strings.TrimSpace(device.Platform)),
			Token:       token,
			TokenSuffix: pushTokenSuffix(token, device.TokenSuffix),
			Label:       strings.TrimSpace(device.Label),
		})
	}
	return targets, nil
}

func pushTokenSuffix(token string, existing string) string {
	existing = strings.TrimSpace(existing)
	if existing != "" {
		return existing
	}
	const suffixRunes = 8
	runes := []rune(token)
	if len(runes) <= suffixRunes {
		return string(runes)
	}
	return string(runes[len(runes)-suffixRunes:])
}
