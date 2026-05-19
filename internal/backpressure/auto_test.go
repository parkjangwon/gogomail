package backpressure

import (
	"testing"
	"time"
)

func TestAutoBackpressureManager_LevelDecision(t *testing.T) {
	tests := []struct {
		name     string
		memLevel string
		qLevel   string
		want     string
	}{
		{"both normal", "normal", "normal", "normal"},
		{"mem warning wins", "warning", "normal", "warning"},
		{"queue danger wins", "normal", "danger", "danger"},
		{"critical takes precedence", "danger", "critical", "critical"},
		{"equal levels", "warning", "warning", "warning"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := higherLevel(tc.memLevel, tc.qLevel)
			if got != tc.want {
				t.Errorf("higherLevel(%q, %q) = %q, want %q", tc.memLevel, tc.qLevel, got, tc.want)
			}
		})
	}
}

func TestAutoBackpressureManager_MemoryThreshold(t *testing.T) {
	tests := []struct {
		ratio float64
		want  string
	}{
		{0.50, "normal"},
		{0.70, "warning"},
		{0.75, "warning"},
		{0.85, "danger"},
		{0.90, "danger"},
		{0.95, "critical"},
		{0.99, "critical"},
	}

	warn := 0.70
	danger := 0.85
	critical := 0.95

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := levelFromRatio(tc.ratio, warn, danger, critical)
			if got != tc.want {
				t.Errorf("levelFromRatio(%.2f) = %q, want %q", tc.ratio, got, tc.want)
			}
		})
	}
}

func TestAutoBackpressureManager_QueueDepthThreshold(t *testing.T) {
	tests := []struct {
		depth int64
		want  string
	}{
		{0, "normal"},
		{5000, "normal"},
		{10000, "warning"},
		{25000, "warning"},
		{50000, "danger"},
		{75000, "danger"},
		{100000, "critical"},
		{200000, "critical"},
	}

	warn := int64(10_000)
	danger := int64(50_000)
	critical := int64(100_000)

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := levelFromDepth(tc.depth, warn, danger, critical)
			if got != tc.want {
				t.Errorf("levelFromDepth(%d) = %q, want %q", tc.depth, got, tc.want)
			}
		})
	}
}

func TestAutoBackpressureConfig_SetDefaults(t *testing.T) {
	cfg := AutoBackpressureConfig{}
	cfg.setDefaults()

	if cfg.CheckInterval != 5*time.Second {
		t.Errorf("CheckInterval default = %v, want 5s", cfg.CheckInterval)
	}
	if cfg.WarningThreshold != 0.70 {
		t.Errorf("WarningThreshold default = %v, want 0.70", cfg.WarningThreshold)
	}
	if cfg.DangerThreshold != 0.85 {
		t.Errorf("DangerThreshold default = %v, want 0.85", cfg.DangerThreshold)
	}
	if cfg.CriticalThreshold != 0.95 {
		t.Errorf("CriticalThreshold default = %v, want 0.95", cfg.CriticalThreshold)
	}
	if cfg.QueueWarningDepth != 10_000 {
		t.Errorf("QueueWarningDepth default = %d, want 10000", cfg.QueueWarningDepth)
	}
	if cfg.QueueDangerDepth != 50_000 {
		t.Errorf("QueueDangerDepth default = %d, want 50000", cfg.QueueDangerDepth)
	}
	if cfg.QueueCriticalDepth != 100_000 {
		t.Errorf("QueueCriticalDepth default = %d, want 100000", cfg.QueueCriticalDepth)
	}
	if cfg.InstanceID == "" {
		t.Error("InstanceID default must not be empty")
	}
	if cfg.LeaderLockTTL != 3*cfg.CheckInterval {
		t.Errorf("LeaderLockTTL default = %v, want 3×CheckInterval (%v)", cfg.LeaderLockTTL, 3*cfg.CheckInterval)
	}
}

func TestAutoBackpressureConfig_ExplicitInstanceIDNotOverwritten(t *testing.T) {
	cfg := AutoBackpressureConfig{InstanceID: "my-node-1"}
	cfg.setDefaults()
	if cfg.InstanceID != "my-node-1" {
		t.Errorf("InstanceID = %q, want my-node-1", cfg.InstanceID)
	}
}

func TestLeaderKeyFor(t *testing.T) {
	key := leaderKeyFor("backpressure:smtp:state")
	if key != "backpressure:smtp:state:auto-leader" {
		t.Errorf("leaderKeyFor = %q, want backpressure:smtp:state:auto-leader", key)
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"12345", 12345},
		{"0", 0},
		{"999999", 999999},
		{"123abc", 123},
		{"", 0},
	}
	for _, tc := range tests {
		var got int64
		parseInt64(&got, tc.input)
		if got != tc.want {
			t.Errorf("parseInt64(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
