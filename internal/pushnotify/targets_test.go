package pushnotify

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestDeviceResolverMapsActiveDevicesToTargets(t *testing.T) {
	t.Parallel()

	repository := &fakeDeviceRepository{
		devices: []maildb.PushDevice{{
			ID:       "device-1",
			Platform: " FCM ",
			Token:    "very-long-token-1",
			Label:    " phone ",
		}},
	}
	resolver := NewDeviceResolver(repository, 20)

	targets, err := resolver.ResolvePushTargets(context.Background(), Event{UserID: " user-1 "})
	if err != nil {
		t.Fatalf("ResolvePushTargets returned error: %v", err)
	}
	if repository.lastUserID != "user-1" || repository.lastLimit != 20 {
		t.Fatalf("repository call user=%q limit=%d", repository.lastUserID, repository.lastLimit)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %+v", targets)
	}
	if targets[0].Platform != "fcm" || targets[0].Token != "very-long-token-1" || targets[0].TokenSuffix != "-token-1" || targets[0].Label != "phone" {
		t.Fatalf("target = %+v", targets[0])
	}
}

func TestDeviceResolverBoundsTargetLimit(t *testing.T) {
	t.Parallel()

	repository := &fakeDeviceRepository{}
	resolver := NewDeviceResolver(repository, 1000)

	_, err := resolver.ResolvePushTargets(context.Background(), Event{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ResolvePushTargets returned error: %v", err)
	}
	if repository.lastLimit != DefaultTargetLimit {
		t.Fatalf("limit = %d, want %d", repository.lastLimit, DefaultTargetLimit)
	}
}

type fakeDeviceRepository struct {
	devices    []maildb.PushDevice
	lastUserID string
	lastLimit  int
}

func (r *fakeDeviceRepository) ListPushDevices(_ context.Context, userID string, limit int) ([]maildb.PushDevice, error) {
	r.lastUserID = userID
	r.lastLimit = limit
	return r.devices, nil
}
