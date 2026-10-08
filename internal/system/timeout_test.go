package system

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBoundWithoutATimeoutReturnsTheContext(t *testing.T) {
	ctx := context.Background()

	bounded, cancel := Bound(ctx, 0)
	defer cancel()
	if bounded != ctx {
		t.Error("a zero timeout must return the context itself")
	}
	if _, ok := bounded.Deadline(); ok {
		t.Error("no deadline expected")
	}

	bounded, cancel = Bound(ctx, time.Minute)
	defer cancel()
	if _, ok := bounded.Deadline(); !ok {
		t.Error("a deadline expected")
	}
}

func TestTimeoutNamesTheSeconds(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{20 * time.Second, "didn't answer within 20 seconds"},
		{time.Second, "didn't answer within 1 second"},
		{50 * time.Millisecond, "didn't answer within 0.05 seconds"},
	}
	for _, tc := range cases {
		err := Timeout(context.Background(), tc.d, fmt.Errorf("request: %w", context.DeadlineExceeded))
		if err == nil || err.Error() != tc.want {
			t.Errorf("got %v, expected %q", err, tc.want)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error("the deadline error must stay recognizable")
		}
		if _, ok := errors.AsType[*TimeoutError](err); !ok {
			t.Error("the error must be a TimeoutError")
		}
	}
}

func TestTimeoutLeavesOtherErrorsAndCancellationsAlone(t *testing.T) {
	if err := Timeout(context.Background(), time.Second, nil); err != nil {
		t.Errorf("nil stays nil, got %v", err)
	}

	other := errors.New("boom")
	if err := Timeout(context.Background(), time.Second, other); !errors.Is(err, other) || err.Error() != "boom" {
		t.Errorf("another error stays as it is, got %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	err := Timeout(cancelled, time.Second, context.DeadlineExceeded)
	if _, wrapped := errors.AsType[*TimeoutError](err); wrapped || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a cancelled parent keeps the error as it is, got %v", err)
	}
}
