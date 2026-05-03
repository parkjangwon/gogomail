package app

import "testing"

func TestKnownModeStringsAreSorted(t *testing.T) {
	modes := KnownModeStrings()
	for i := 1; i < len(modes); i++ {
		if modes[i-1] > modes[i] {
			t.Fatalf("KnownModeStrings not sorted at %d: %q before %q", i, modes[i-1], modes[i])
		}
	}
}
