package pushnotify

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestDeviceResolverSkipsInvalidTargets(t *testing.T) {
	t.Parallel()

	repository := &fakeDeviceRepository{
		devices: []maildb.PushDevice{
			{ID: "", Platform: "fcm", Token: "token-1"},
			{ID: "device-2", Platform: "pager", Token: "token-2"},
			{ID: "device-3", Platform: "apns", Token: " "},
			{ID: "device-4\nbad", Platform: "fcm", Token: "token-4"},
			{ID: "device-5", Platform: "webpush", Token: "token-5\r\nbad"},
			{ID: "device-6", Platform: "fcm", Token: strings.Repeat("t", maxPushTargetTokenBytes+1)},
			{ID: strings.Repeat("d", maxPushTargetIDBytes+1), Platform: "fcm", Token: "token-7"},
			{ID: "device-4", Platform: "webpush", Token: "token-4"},
		},
	}
	resolver := NewDeviceResolver(repository, 20)

	targets, err := resolver.ResolvePushTargets(context.Background(), Event{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ResolvePushTargets returned error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want only one valid target", targets)
	}
	if targets[0].DeviceID != "device-4" || targets[0].Platform != "webpush" || targets[0].Token != "token-4" {
		t.Fatalf("target = %+v", targets[0])
	}
}

func TestDeviceResolverBoundsOptionalTargetMetadata(t *testing.T) {
	t.Parallel()

	repository := &fakeDeviceRepository{
		devices: []maildb.PushDevice{{
			ID:          "device-1",
			Platform:    "fcm",
			Token:       "token-1",
			TokenSuffix: strings.Repeat("s", maxPushTargetLabelBytes) + "\nextra",
			Label:       strings.Repeat("\u20ac", maxPushTargetLabelBytes),
		}},
	}
	resolver := NewDeviceResolver(repository, 20)

	targets, err := resolver.ResolvePushTargets(context.Background(), Event{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ResolvePushTargets returned error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %+v", targets)
	}
	target := targets[0]
	if strings.ContainsAny(target.TokenSuffix+target.Label, "\r\n") {
		t.Fatalf("target contains line break: %+v", target)
	}
	if len(target.TokenSuffix) > maxPushTargetLabelBytes || len(target.Label) > maxPushTargetLabelBytes {
		t.Fatalf("target suffix/label lengths = %d/%d", len(target.TokenSuffix), len(target.Label))
	}
	if !utf8.ValidString(target.Label) {
		t.Fatalf("label is not valid UTF-8: %q", target.Label)
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
