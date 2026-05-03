package backpressure

import "testing"

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
