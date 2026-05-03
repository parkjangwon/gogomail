package outbound

import (
	"testing"
	"time"
)

func TestClassifyFarm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   ClassificationInput
		want Farm
	}{
		{name: "transactional", in: ClassificationInput{Transactional: true, RecipientCount: 1}, want: FarmTransactional},
		{name: "scheduled", in: ClassificationInput{RecipientCount: 1, ScheduledAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)}, want: FarmBatch},
		{name: "bulk", in: ClassificationInput{RecipientCount: 500}, want: FarmBulk},
		{name: "general", in: ClassificationInput{RecipientCount: 499}, want: FarmGeneral},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Classify(tt.in); got != tt.want {
				t.Fatalf("Classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeFarm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   Farm
		want Farm
	}{
		{in: " BULK ", want: FarmBulk},
		{in: "transactional", want: FarmTransactional},
		{in: "batch", want: FarmBatch},
		{in: "", want: FarmGeneral},
		{in: "weird", want: FarmGeneral},
	}
	for _, tt := range tests {
		if got := NormalizeFarm(tt.in); got != tt.want {
			t.Fatalf("NormalizeFarm(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
