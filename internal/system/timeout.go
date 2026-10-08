package system

import (
	"context"
	"errors"
	"strconv"
	"time"
)

type TimeoutError struct {
	After time.Duration
	Err   error
}

func (e *TimeoutError) Error() string {
	return "didn't answer within " + seconds(e.After)
}

func (e *TimeoutError) Unwrap() error {
	return e.Err
}

func Bound(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func Timeout(parent context.Context, d time.Duration, err error) error {
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || parent.Err() != nil {
		return err
	}
	return &TimeoutError{After: d, Err: err}
}

func seconds(d time.Duration) string {
	if d == time.Second {
		return "1 second"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + " seconds"
	}
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + " seconds"
}
