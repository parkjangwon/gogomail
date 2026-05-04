package delivery

import "testing"

func TestRouteCountersSnapshotSortsByPool(t *testing.T) {
	t.Parallel()

	counters := NewRouteCounters()
	counters.observe("zeta", MetricEvent{Stage: MetricTransportDelivered})
	counters.observe("alpha", MetricEvent{Stage: MetricTransportFailed})
	counters.observe("", MetricEvent{Stage: MetricRetryScheduled})

	got := counters.Snapshot()
	if len(got) != 3 {
		t.Fatalf("len(snapshot) = %d, want 3", len(got))
	}

	pools := []string{got[0].Pool, got[1].Pool, got[2].Pool}
	want := []string{"alpha", "default", "zeta"}
	for i := range want {
		if pools[i] != want[i] {
			t.Fatalf("pools = %#v, want %#v", pools, want)
		}
	}
	if got[0].Failed != 1 || got[1].Retried != 1 || got[2].Delivered != 1 {
		t.Fatalf("snapshot counts = %#v", got)
	}
}
