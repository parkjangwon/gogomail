package backpressure

import (
	"context"
	"testing"
	"time"
)

func TestRedisBackpressureNilClientAcceptAllows(t *testing.T) {
	t.Parallel()

	b := NewRedisBackpressure(nil, DefaultStateKey)
	accepted, err := b.Accept(context.Background())
	if err != nil {
		t.Fatalf("Accept error = %v, want nil", err)
	}
	if !accepted {
		t.Fatalf("accepted = false, want true for nil client")
	}
}

func TestRedisBackpressureNilClientStateReturnsNormal(t *testing.T) {
	t.Parallel()

	b := NewRedisBackpressure(nil, DefaultStateKey)
	state, err := b.State(context.Background())
	if err != nil {
		t.Fatalf("State error = %v, want nil", err)
	}
	if state.Level != "normal" {
		t.Fatalf("state.Level = %q, want normal", state.Level)
	}
}

func TestRedisBackpressureNilClientSetStateErrors(t *testing.T) {
	t.Parallel()

	b := NewRedisBackpressure(nil, DefaultStateKey)
	_, err := b.SetState(context.Background(), StateUpdate{Level: "danger"})
	if err == nil {
		t.Fatal("SetState accepted nil client without error")
	}
}

func TestAcceptsState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state string
		want  bool
	}{
		{state: "", want: true},
		{state: "normal", want: true},
		{state: "warning", want: true},
		{state: "danger", want: false},
		{state: "critical", want: false},
	}

	for _, tt := range tests {
		if got := acceptsState(tt.state); got != tt.want {
			t.Fatalf("acceptsState(%q) = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestDecodeBackpressureStateSupportsLegacyStrings(t *testing.T) {
	t.Parallel()

	state, err := decodeState(" danger ")
	if err != nil {
		t.Fatalf("decodeState returned error: %v", err)
	}
	if state.Level != "danger" {
		t.Fatalf("level = %q, want danger", state.Level)
	}
	if state.UpdatedAt.IsZero() {
		t.Fatal("legacy state should receive an updated_at timestamp")
	}
}

func TestDecodeBackpressureStateSupportsStructuredJSON(t *testing.T) {
	t.Parallel()

	state, err := decodeState(`{"level":"critical","reason":"queue lag","updated_at":"2026-05-04T01:02:03Z"}`)
	if err != nil {
		t.Fatalf("decodeState returned error: %v", err)
	}
	if state.Level != "critical" || state.Reason != "queue lag" {
		t.Fatalf("state = %+v", state)
	}
}

func TestNormalizeBackpressureStateUpdateRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	if _, err := normalizeStateUpdate(StateUpdate{Level: "panic"}); err == nil {
		t.Fatal("unsupported level accepted")
	}
	if _, err := normalizeStateUpdate(StateUpdate{Level: "warning", Reason: "bad\nreason"}); err == nil {
		t.Fatal("newline reason accepted")
	}
}

func TestNormalizeBackpressureStateUpdateDefaultsNormal(t *testing.T) {
	t.Parallel()

	until := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	state, err := normalizeStateUpdate(StateUpdate{Until: &until})
	if err != nil {
		t.Fatalf("normalizeStateUpdate returned error: %v", err)
	}
	if state.Level != "normal" || state.Until == nil || !state.UpdatedAt.After(time.Time{}) {
		t.Fatalf("state = %+v", state)
	}
}
