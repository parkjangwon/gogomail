package outbound

import "time"

type Farm string

const (
	FarmTransactional Farm = "transactional"
	FarmGeneral       Farm = "general"
	FarmBulk          Farm = "bulk"
	FarmBatch         Farm = "batch"
)

type ClassificationInput struct {
	Transactional  bool
	RecipientCount int
	ScheduledAt    time.Time
}

func Classify(input ClassificationInput) Farm {
	if input.Transactional {
		return FarmTransactional
	}
	if !input.ScheduledAt.IsZero() {
		return FarmBatch
	}
	if input.RecipientCount >= 500 {
		return FarmBulk
	}
	return FarmGeneral
}
