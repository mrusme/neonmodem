package systemtest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

type Options struct {
	ForumID         string
	Write           bool
	ExistingPostID  string
	SkipNestedReply bool
	WriteDelay      time.Duration
	ReloadAttempts  int
	ReloadDelay     time.Duration
}

func Env(name string) string {
	return os.Getenv("NEONMODEM_LIVE_" + name)
}

func Exercise(t *testing.T, sys system.System, opts Options) {
	t.Helper()
	ctx := context.Background()

	if opts.ReloadAttempts == 0 {
		opts.ReloadAttempts = 1
	}

	forums, err := sys.ListForums(ctx)
	if err != nil {
		t.Fatalf("ListForums: %v", err)
	}
	if len(forums) == 0 {
		t.Fatal("ListForums returned no forums")
	}
	for i, f := range forums {
		if i < 5 {
			t.Logf("forum: %-30s id=%s", f.Name, f.ID)
		}
		if f.ID == "" || f.Name == "" {
			t.Errorf("forum %d has an empty id or name: %+v", i, f)
		}
	}

	forumID := opts.ForumID
	if forumID == "" {
		forumID = forums[0].ID
	}

	feed, err := sys.ListPosts(ctx, "", sys.Orders("").Default)
	if err != nil {
		t.Fatalf("ListPosts(all): %v", err)
	}
	if len(feed) == 0 {
		t.Fatal("ListPosts(all) returned no posts")
	}
	t.Logf("feed: %d posts", len(feed))
	for i, p := range feed {
		if p.ID == "" || p.Subject == "" {
			t.Errorf("post %d has an empty id or subject: %+v", i, p)
		}
		if p.CreatedAt.IsZero() {
			t.Errorf("post %q has no creation time", p.Subject)
		}
	}

	for _, order := range sys.Orders("").Supported {
		posts, err := sys.ListPosts(ctx, "", order)
		if err != nil {
			t.Errorf("ListPosts(all, %s): %v", order, err)
			continue
		}
		t.Logf("feed in order %s: %d posts", order, len(posts))
	}

	scoped, err := sys.ListPosts(ctx, forumID, sys.Orders(forumID).Default)
	if err != nil {
		t.Fatalf("ListPosts(%s): %v", forumID, err)
	}
	t.Logf("forum %s: %d posts", forumID, len(scoped))

	loaded := false
	for i := range feed {
		if feed[i].ReplyCount == 0 {
			continue
		}
		p := feed[i]
		if err := sys.LoadPost(ctx, &p); err != nil {
			t.Fatalf("LoadPost(%s): %v", p.ID, err)
		}
		if p.Body == "" {
			t.Errorf("LoadPost %q: empty body", p.Subject)
		}
		if len(p.Replies) == 0 {
			t.Errorf("LoadPost %q: expected replies, got none", p.Subject)
		}
		for _, r := range p.Replies {
			if r.PostID != p.ID {
				t.Errorf("reply %s does not point at post %s: %q", r.ID, p.ID, r.PostID)
			}
			if r.ParentID != "" {
				t.Errorf("top-level reply %s has a parent %q", r.ID, r.ParentID)
			}
		}
		t.Logf("loaded %q: body=%dch top=%d all=%d",
			p.Subject, len(p.Body), len(p.Replies), reply.Count(p.Replies))
		loaded = true
		break
	}
	if !loaded {
		t.Log("no post with replies in the feed to load")
	}

	if !opts.Write {
		return
	}

	newPost := post.Post{
		Subject: RandomText(6),
		Body:    RandomText(14),
		Kind:    post.KindText,
		SysIDX:  feed[0].SysIDX,
	}
	newPost.Forum.ID = forumID

	if opts.ExistingPostID != "" {
		newPost.ID = opts.ExistingPostID
		t.Logf("using existing post id=%s", newPost.ID)
	} else {
		if err := sys.CreatePost(ctx, &newPost); err != nil {
			t.Fatalf("CreatePost: %v", err)
		}
		if newPost.ID == "" {
			t.Fatal("CreatePost returned no id")
		}
		t.Logf("created post id=%s", newPost.ID)
		pause(t, opts.WriteDelay)
	}

	top := reply.Reply{PostID: newPost.ID, Body: RandomText(10)}
	if err := sys.CreateReply(ctx, &top); err != nil {
		t.Fatalf("CreateReply(top): %v", err)
	}
	t.Logf("created reply id=%q", top.ID)

	var nested *reply.Reply
	if !opts.SkipNestedReply && top.ID != "" {
		pause(t, opts.WriteDelay)
		nested = &reply.Reply{PostID: newPost.ID, ParentID: top.ID, Body: RandomText(10)}
		if err := sys.CreateReply(ctx, nested); err != nil {
			t.Fatalf("CreateReply(nested): %v", err)
		}
		t.Logf("created nested reply id=%q", nested.ID)
	}

	for attempt := 1; attempt <= opts.ReloadAttempts; attempt++ {
		reloaded := newPost
		if err := sys.LoadPost(ctx, &reloaded); err != nil {
			t.Fatalf("LoadPost(reload): %v", err)
		}

		found := findReply(reloaded.Replies, top)
		ok := found != nil && found.ParentID == ""
		if ok && nested != nil {
			child := findReply(found.Replies, *nested)
			ok = child != nil && child.ParentID == found.ID
		}
		if ok {
			t.Logf("reload OK after %d attempt(s)", attempt)
			return
		}

		if attempt < opts.ReloadAttempts {
			time.Sleep(opts.ReloadDelay)
			continue
		}
		t.Fatalf("reload: the new reply is not in the tree (%d top-level replies)",
			len(reloaded.Replies))
	}
}

func pause(t *testing.T, d time.Duration) {
	t.Helper()
	if d <= 0 {
		return
	}
	t.Logf("waiting %s before the next write", d)
	time.Sleep(d)
}

func findReply(replies []reply.Reply, want reply.Reply) *reply.Reply {
	for i := range replies {
		if want.ID != "" && replies[i].ID == want.ID {
			return &replies[i]
		}
		if want.ID == "" && strings.Contains(replies[i].Body, want.Body) {
			return &replies[i]
		}
	}
	return nil
}

type Commands map[string]string

func (c Commands) Run(_ context.Context, command string) (string, error) {
	value, ok := c[command]
	if !ok {
		return "", fmt.Errorf("ended with exit status 127: %s: command not found", command)
	}
	return value, nil
}
