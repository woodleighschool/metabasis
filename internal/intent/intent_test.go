package intent

import (
	"testing"
	"time"
)

func TestPhaseAtUsesExactHalfOpenBoundaries(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	accepted := Intent{StartsAt: start, EndsAt: end}

	tests := []struct {
		name string
		now  time.Time
		want Phase
	}{
		{name: "before start", now: start.Add(-time.Nanosecond), want: PhasePending},
		{name: "at start", now: start, want: PhaseActive},
		{name: "before end", now: end.Add(-time.Nanosecond), want: PhaseActive},
		{name: "at end", now: end, want: PhaseEnded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := accepted.PhaseAt(test.now); got != test.want {
				t.Errorf("PhaseAt() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCancelledIntentHasNoTransition(t *testing.T) {
	t.Parallel()
	now := time.Now()
	accepted := Intent{StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), Cancelled: true}
	if got := accepted.PhaseAt(now); got != PhaseCancelled {
		t.Fatalf("PhaseAt() = %q, want %q", got, PhaseCancelled)
	}
	if got := accepted.NextTransitionAt(now); got != nil {
		t.Fatalf("NextTransitionAt() = %v, want nil", got)
	}
}
