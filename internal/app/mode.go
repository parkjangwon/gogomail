package app

import (
	"fmt"
	"sort"
	"strings"
)

type Mode string

const (
	ModeAllInOne          Mode = "all-in-one"
	ModeEdgeMTA           Mode = "edge-mta"
	ModeInboundMTA        Mode = "inbound-mta"
	ModeOutboundMTA       Mode = "outbound-mta"
	ModeDeliveryWorker    Mode = "delivery-worker"
	ModeAttachmentCleanup Mode = "attachment-cleanup-worker"
	ModeDriveCleanup      Mode = "drive-cleanup-worker"
	ModeDAVSyncRetention  Mode = "dav-sync-retention-worker"
	ModeSearchIndexWorker Mode = "search-index-worker"
	ModeAPIMeteringWorker Mode = "api-metering-worker"
	ModeAPIUsageRetention Mode = "api-usage-retention-worker"
	ModePushWorker        Mode = "push-notification-worker"
	ModeBatchWorker       Mode = "batch-worker"
	ModeOutboxRelay       Mode = "outbox-relay"
	ModeEventWorker       Mode = "event-worker"
	ModeIMAP              Mode = "imap"
	ModePOP3              Mode = "pop3"
	ModeCalDAV            Mode = "caldav"
	ModeCardDAV           Mode = "carddav"
	ModeAuthServer        Mode = "auth-server"
	ModeMailAPI           Mode = "mail-api"
	ModeAdminAPI          Mode = "admin-api"
)

var knownModes = map[Mode]struct{}{
	ModeAllInOne:          {},
	ModeEdgeMTA:           {},
	ModeInboundMTA:        {},
	ModeOutboundMTA:       {},
	ModeDeliveryWorker:    {},
	ModeAttachmentCleanup: {},
	ModeDriveCleanup:      {},
	ModeDAVSyncRetention:  {},
	ModeSearchIndexWorker: {},
	ModeAPIMeteringWorker: {},
	ModeAPIUsageRetention: {},
	ModePushWorker:        {},
	ModeBatchWorker:       {},
	ModeOutboxRelay:       {},
	ModeEventWorker:       {},
	ModeIMAP:              {},
	ModePOP3:              {},
	ModeCalDAV:            {},
	ModeCardDAV:           {},
	ModeAuthServer:        {},
	ModeMailAPI:           {},
	ModeAdminAPI:          {},
}

func ParseMode(raw string) (Mode, error) {
	mode := Mode(strings.TrimSpace(strings.ToLower(raw)))
	if _, ok := knownModes[mode]; ok {
		return mode, nil
	}
	return "", fmt.Errorf("unknown mode %q; valid modes: %s", raw, strings.Join(KnownModeStrings(), ", "))
}

func KnownModeStrings() []string {
	modes := make([]string, 0, len(knownModes))
	for mode := range knownModes {
		modes = append(modes, string(mode))
	}
	sort.Strings(modes)
	return modes
}
