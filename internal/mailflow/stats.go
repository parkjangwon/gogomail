package mailflow

import (
	"context"
	"time"
)

type MailFlowStatsProvider interface {
	GetStats(ctx context.Context, req MailFlowStatsRequest) (MailFlowStatsResult, error)
	GetDailyStats(ctx context.Context, req MailFlowStatsRequest) ([]MailFlowDailyStatsResult, error)
}

type MailFlowStatsRequest struct {
	Direction string
	CompanyID string
	DomainID  string
	UserID    string
	Since     time.Time
	Until     time.Time
}

type MailFlowStatsResult struct {
	TotalMessages    int64
	UniqueSenders   int64
	UniqueDomains   int64
	TotalSizeBytes  int64
	AverageSizeBytes float64
	MaxSizeBytes    int64
	Delivered       int64
	Failed          int64
	Bounced         int64
	Filtered        int64
	Rejected        int64
	DeliveryRate    float64
}

type MailFlowDailyStatsResult struct {
	Date             time.Time
	InboundMessages  int64
	OutboundMessages int64
	InboundSize     int64
	OutboundSize    int64
	Delivered       int64
	Failed          int64
	Bounced         int64
}
