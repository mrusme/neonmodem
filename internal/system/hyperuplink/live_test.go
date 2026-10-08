package hyperuplink

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

// TestLive exercises the whole system against a running Hyperuplink instance.
// It is skipped unless HUP_TOKEN is set. HUP_URL defaults to the dev API port.
//
//	HUP_TOKEN=hup_... go test ./internal/system/hyperuplink/ -run TestLive -v
func TestLive(t *testing.T) {
	token := os.Getenv("HUP_TOKEN")
	if token == "" {
		t.Skip("set HUP_TOKEN to run the live integration test")
	}
	url := os.Getenv("HUP_URL")
	if url == "" {
		url = "http://localhost:3001"
	}

	ctx := context.Background()

	sys, err := New(system.Env{
		Settings: system.Settings{
			URL:         url,
			Credentials: map[string]string{"username": "sysop", "token": token},
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// --- ListForums ---
	forums, err := sys.ListForums(ctx)
	if err != nil {
		t.Fatalf("ListForums: %v", err)
	}
	if len(forums) == 0 {
		t.Fatal("ListForums returned no forums")
	}
	var targetForumID string
	for _, f := range forums {
		if !strings.Contains(f.Name, "/") {
			t.Errorf("forum name %q is not a Category/Forum compound", f.Name)
		}
		if strings.HasSuffix(f.Name, "/Announcements") {
			targetForumID = f.ID
		}
		t.Logf("forum: %-28s id=%s", f.Name, f.ID)
	}
	if targetForumID == "" {
		targetForumID = forums[0].ID
	}

	// --- ListPosts("") : global feed ---
	feed, err := sys.ListPosts(ctx, "", system.OrderActive)
	if err != nil {
		t.Fatalf("ListPosts(global): %v", err)
	}
	if len(feed) == 0 {
		t.Fatal("global feed returned no posts")
	}
	t.Logf("global feed: %d posts", len(feed))

	// --- ListPosts(forum) : scoped feed ---
	scoped, err := sys.ListPosts(ctx, targetForumID, system.OrderActive)
	if err != nil {
		t.Fatalf("ListPosts(forum): %v", err)
	}
	t.Logf("forum feed: %d posts", len(scoped))

	// --- LoadPost : find a topic with replies ---
	var loaded bool
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
		t.Logf("loaded %q: body=%dch replies=%d url=%s",
			p.Subject, len(p.Body), len(p.Replies), p.URL)
		loaded = true
		break
	}
	if !loaded {
		t.Log("no topic with replies found to LoadPost")
	}

	// --- CreatePost ---
	newPost := feed[0]
	// Unique per run: the board derives the topic slug from the name and
	// rejects duplicates.
	newPost.Subject = systemtest.RandomText(6)
	newPost.Body = systemtest.RandomText(14)
	newPost.Forum.ID = targetForumID
	newPost.ID = ""
	if err := sys.CreatePost(ctx, &newPost); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if newPost.ID == "" {
		t.Fatal("CreatePost returned no ID")
	}
	t.Logf("created topic id=%s", newPost.ID)

	// --- CreateReply to the topic ---
	topLevel := reply.Reply{PostID: newPost.ID}
	topLevel.Body = systemtest.RandomText(10)
	if err := sys.CreateReply(ctx, &topLevel); err != nil {
		t.Fatalf("CreateReply(topic): %v", err)
	}
	if topLevel.ID == "" {
		t.Fatal("CreateReply(topic) returned no ID")
	}
	t.Logf("created reply id=%s", topLevel.ID)

	// --- CreateReply nested under the previous reply ---
	nested := reply.Reply{PostID: newPost.ID, ParentID: topLevel.ID}
	nested.Body = systemtest.RandomText(10)
	if err := sys.CreateReply(ctx, &nested); err != nil {
		t.Fatalf("CreateReply(nested): %v", err)
	}
	t.Logf("created nested reply id=%s", nested.ID)

	// --- Reload and confirm the tree threaded correctly ---
	if err := sys.LoadPost(ctx, &newPost); err != nil {
		t.Fatalf("LoadPost(reload): %v", err)
	}
	if len(newPost.Replies) != 1 {
		t.Fatalf("expected 1 top-level reply, got %d", len(newPost.Replies))
	}
	if len(newPost.Replies[0].Replies) != 1 {
		t.Fatalf("expected 1 nested reply, got %d", len(newPost.Replies[0].Replies))
	}
	t.Logf("reload OK: 1 top-level reply with 1 nested reply")
}
