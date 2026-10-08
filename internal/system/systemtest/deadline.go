package systemtest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

const deadline = 50 * time.Millisecond

func Deadlines(t *testing.T, sys system.System, postID string, forumID string, skip ...string) {
	t.Helper()

	calls := []struct {
		name string
		call func(ctx context.Context) error
	}{
		{"ListForums", func(ctx context.Context) error {
			_, err := sys.ListForums(ctx)
			return err
		}},
		{"ListPosts", func(ctx context.Context) error {
			_, err := sys.ListPosts(ctx, "", system.OrderNew)
			return err
		}},
		{"LoadPost", func(ctx context.Context) error {
			return sys.LoadPost(ctx, &post.Post{ID: postID, Forum: forum.Forum{ID: forumID}})
		}},
		{"CreatePost", func(ctx context.Context) error {
			return sys.CreatePost(ctx, &post.Post{Subject: "Subject", Body: "Body", Forum: forum.Forum{ID: forumID}})
		}},
		{"CreateReply", func(ctx context.Context) error {
			return sys.CreateReply(ctx, &reply.Reply{PostID: postID, Body: "Body"})
		}},
	}

	for _, c := range calls {
		if slices.Contains(skip, c.name) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		start := time.Now()
		err := c.call(ctx)
		cancel()

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: got %v, expected an error that matches the deadline", c.name, err)
		}
		if waited := time.Since(start); waited > time.Second {
			t.Errorf("%s returned after %s", c.name, waited)
		}
	}
}

func Blocking(t *testing.T) string {
	t.Helper()

	srv := newBlockingServer()
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return srv.URL
}
