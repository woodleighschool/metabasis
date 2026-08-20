package reconcile

import (
	"testing"
	"time"
)

func TestRetryDelayDoublesAndCaps(t *testing.T) {
	t.Parallel()
	initial := 30 * time.Second
	maximum := 2 * time.Minute
	tests := []struct {
		previousFailures int
		want             time.Duration
	}{
		{previousFailures: 0, want: 30 * time.Second},
		{previousFailures: 1, want: time.Minute},
		{previousFailures: 2, want: 2 * time.Minute},
		{previousFailures: 8, want: 2 * time.Minute},
	}
	for _, test := range tests {
		if got := retryDelay(initial, maximum, test.previousFailures); got != test.want {
			t.Errorf("retryDelay(%d) = %v, want %v", test.previousFailures, got, test.want)
		}
	}
}
