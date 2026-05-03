package outbound

import (
	"strings"
	"time"
)

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

func NormalizeFarm(farm Farm) Farm {
	switch Farm(strings.ToLower(strings.TrimSpace(string(farm)))) {
	case FarmTransactional:
		return FarmTransactional
	case FarmGeneral, "":
		return FarmGeneral
	case FarmBulk:
		return FarmBulk
	case FarmBatch:
		return FarmBatch
	default:
		return FarmGeneral
	}
}
