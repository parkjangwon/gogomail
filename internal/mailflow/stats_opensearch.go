package mailflow

import (
	"context"
	"time"

	"github.com/gogomail/gogomail/internal/searchindex"
)

type OpenSearchMailFlowStatsProvider struct {
	searcher *searchindex.MailFlowStatsSearcher
}

func NewOpenSearchMailFlowStatsProvider(searcher *searchindex.MailFlowStatsSearcher) *OpenSearchMailFlowStatsProvider {
	return &OpenSearchMailFlowStatsProvider{searcher: searcher}
}

func (p *OpenSearchMailFlowStatsProvider) GetStats(ctx context.Context, req MailFlowStatsRequest) (MailFlowStatsResult, error) {
	query := searchindex.MailFlowStatsQuery{
		Direction: req.Direction,
		CompanyID: req.CompanyID,
		DomainID:  req.DomainID,
		UserID:    req.UserID,
		Since:     formatTimeForOpenSearch(req.Since),
		Until:     formatTimeForOpenSearch(req.Until),
	}
	stats, err := p.searcher.GetStats(ctx, query)
	if err != nil {
		return MailFlowStatsResult{}, err
	}
	return MailFlowStatsResult{
		TotalMessages:    stats.TotalMessages,
		UniqueSenders:    stats.UniqueSenders,
		UniqueDomains:    stats.UniqueDomains,
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

func (p *OpenSearchMailFlowStatsProvider) GetDailyStats(ctx context.Context, req MailFlowStatsRequest) ([]MailFlowDailyStatsResult, error) {
	query := searchindex.MailFlowStatsQuery{
		Direction: req.Direction,
		CompanyID: req.CompanyID,
		DomainID:  req.DomainID,
		UserID:    req.UserID,
		Since:     formatTimeForOpenSearch(req.Since),
		Until:     formatTimeForOpenSearch(req.Until),
	}
	stats, err := p.searcher.GetDailyStats(ctx, query)
	if err != nil {
		return nil, err
	}
	results := make([]MailFlowDailyStatsResult, 0, len(stats))
	for _, s := range stats {
		results = append(results, MailFlowDailyStatsResult{
			Date:             s.Date,
			InboundMessages:  s.InboundMessages,
			OutboundMessages: s.OutboundMessages,
			InboundSize:      s.InboundSize,
			OutboundSize:     s.OutboundSize,
			Delivered:        s.Delivered,
			Failed:           s.Failed,
			Bounced:          s.Bounced,
			Filtered:         s.Filtered,
			Rejected:         s.Rejected,
		})
	}
	return results, nil
}

func formatTimeForOpenSearch(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
