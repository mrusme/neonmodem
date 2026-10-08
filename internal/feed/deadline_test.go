package feed

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

type slow struct {
	delay time.Duration
}

func (s *slow) wait(ctx context.Context) error {
	if s.delay == 0 {
		<-ctx.Done()
		return ctx.Err()
	}
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slow) Kind() string                      { return "slow" }
func (s *slow) URL() string                       { return "" }
func (s *slow) Title() string                     { return "slow" }
func (s *slow) Description() string               { return "slow" }
func (s *slow) Capabilities() system.Capabilities { return system.CapRead | system.CapWrite }
func (s *slow) Connect(context.Context, prompt.Prompter, string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (s *slow) Orders(string) system.Ordering { return system.Only(system.OrderNew) }
func (s *slow) ListForums(ctx context.Context) ([]forum.Forum, error) {
	if err := s.wait(ctx); err != nil {
		return nil, err
	}
	return []forum.Forum{{Name: "Slow"}}, nil
}
func (s *slow) ListPosts(ctx context.Context, _ string, _ system.Order) ([]post.Post, error) {
	if err := s.wait(ctx); err != nil {
		return nil, err
	}
	return []post.Post{{ID: "1"}}, nil
}
func (s *slow) LoadPost(ctx context.Context, _ *post.Post) error      { return s.wait(ctx) }
func (s *slow) CreatePost(ctx context.Context, _ *post.Post) error    { return s.wait(ctx) }
func (s *slow) CreateReply(ctx context.Context, _ *reply.Reply) error { return s.wait(ctx) }

func timedOut(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, expected %q", err, want)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline error must stay recognizable")
	}
}

func TestReadsEndAtTheReadDeadline(t *testing.T) {
	f := New([]system.System{&slow{}}, 50*time.Millisecond, time.Hour)
	start := time.Now()

	_, err := f.List(context.Background(), 0, "", system.OrderNew)
	timedOut(t, err, "didn't answer within 0.05 seconds")

	err = f.LoadPost(context.Background(), &post.Post{SysIDX: 0})
	timedOut(t, err, "didn't answer within 0.05 seconds")

	if waited := time.Since(start); waited > time.Second {
		t.Errorf("the deadline didn't end the calls, they took %s", waited)
	}
}

func TestListForumsReportsOnlyTheSlowSystem(t *testing.T) {
	f := New([]system.System{&slow{}, &slow{delay: time.Millisecond}}, 50*time.Millisecond, 0)

	forums, errs := f.ListForums(context.Background(), All)
	timedOut(t, errs[0], "didn't answer within 0.05 seconds")
	if errs[1] != nil {
		t.Errorf("the quick system failed: %v", errs[1])
	}
	if len(forums) != 1 || forums[0].Name != "Slow" {
		t.Errorf("the quick system's forums are missing: %+v", forums)
	}
}

func TestWritesEndAtTheWriteDeadlineAndSaySo(t *testing.T) {
	f := New([]system.System{&slow{}}, time.Hour, 50*time.Millisecond)
	want := "didn't answer within 0.05 seconds, the post may have reached the site"

	timedOut(t, f.CreatePost(context.Background(), &post.Post{SysIDX: 0}), want)
	timedOut(t, f.CreateReply(context.Background(), &reply.Reply{SysIDX: 0}), want)
}

func TestACancelledParentKeepsItsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := New([]system.System{&slow{}}, 50*time.Millisecond, 0).LoadPost(ctx, &post.Post{SysIDX: 0})
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "didn't answer") {
		t.Errorf("a cancelled load keeps its own error, got %v", err)
	}
}

func TestAZeroTimeoutMeansNoDeadline(t *testing.T) {
	f := New([]system.System{&slow{delay: 20 * time.Millisecond}}, 0, 0)

	if _, err := f.List(context.Background(), 0, "", system.OrderNew); err != nil {
		t.Errorf("without a timeout the call must succeed, got %v", err)
	}
	if err := f.CreatePost(context.Background(), &post.Post{SysIDX: 0}); err != nil {
		t.Errorf("without a timeout the write must succeed, got %v", err)
	}

	quick := New([]system.System{&slow{delay: 20 * time.Millisecond}}, 5*time.Millisecond, 0)
	if _, err := quick.List(context.Background(), 0, "", system.OrderNew); err == nil {
		t.Error("a shorter timeout ends the same call")
	}
}
