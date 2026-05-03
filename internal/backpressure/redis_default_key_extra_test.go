package backpressure

import "testing"

func TestNewRedisBackpressureUsesDefaultStateKey(t *testing.T) {
	checker := NewRedisBackpressure(nil, " \t ")
	if checker.key != DefaultStateKey {
		t.Fatalf("key = %q, want default state key", checker.key)
	}
}
