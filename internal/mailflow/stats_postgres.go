package mailflow

import (
	"context"

	"github.com/gogomail/gogomail/internal/maildb"
)

type PostgresMailFlowStatsProvider struct {
	repo *maildb.Repository
}

func NewPostgresMailFlowStatsProvider(repo *maildb.Repository) *PostgresMailFlowStatsProvider {
	return &PostgresMailFlowStatsProvider{repo: repo}
}

func (p *PostgresMailFlowStatsProvider) GetStats(ctx context.Context, req MailFlowStatsRequest) (MailFlowStatsResult, error) {
	dbReq := maildb.MailFlowLogStatsRequest{
		Direction: req.Direction,
		CompanyID: req.CompanyID,
		DomainID:  req.DomainID,
		UserID:    req.UserID,
		Since:     req.Since,
		Until:     req.Until,
	}
	stats, err := p.repo.GetMailFlowLogStats(ctx, dbReq)
	if err != nil {
		return MailFlowStatsResult{}, err
	}
	return MailFlowStatsResult{
		TotalMessages:    stats.TotalMessages,
		UniqueSenders:     stats.UniqueSenders,
		UniqueDomains:     stats.UniqueDomains,
		TotalSizeBytes:   stats.TotalSizeBytes,
		AverageSizeBytes: stats.AverageSizeBytes,
		MaxSizeBytes:     stats.MaxSizeBytes,
		Delivered:        stats.Delivered,
		Failed:           stats.Failed,
		Bounced:          stats.Bounced,
		Filtered:         stats.Filtered,
		Rejected:         stats.Rejected,
		DeliveryRate:     stats.DeliveryRate,
	}, nil
}

func (p *PostgresMailFlowStatsProvider) GetDailyStats(ctx context.Context, req MailFlowStatsRequest) ([]MailFlowDailyStatsResult, error) {
	dbReq := maildb.MailFlowLogDailyStatsRequest{
		Direction: req.Direction,
		CompanyID: req.CompanyID,
		DomainID:  req.DomainID,
		UserID:    req.UserID,
		Since:     req.Since,
		Until:     req.Until,
	}
	stats, err := p.repo.GetMailFlowLogDailyStats(ctx, dbReq)
	if err != nil {
		return nil, err
	}
	results := make([]MailFlowDailyStatsResult, 0, len(stats))
	for _, s := range stats {
		results = append(results, MailFlowDailyStatsResult{
			Date:             s.Date,
			InboundMessages:  s.InboundMessages,
			OutboundMessages: s.OutboundMessages,
			InboundSize:     s.InboundSize,
			OutboundSize:    s.OutboundSize,
			Delivered:       s.Delivered,
			Failed:          s.Failed,
			Bounced:         s.Bounced,
		})
	}
	return results, nil
}
