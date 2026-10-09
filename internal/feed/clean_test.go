package feed

import (
	"context"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

const (
	clearScreen = "\x1b[2J"
	hyperlink   = "\x1b]8;;https://evil.example\a"
	c1          = "\u009b2J"
)

type hostile struct {
	*fake
}

func (h *hostile) LoadPost(_ context.Context, p *post.Post) error {
	p.Subject = "Loaded" + clearScreen
	p.Author.Name = "author" + hyperlink
	p.Replies = []reply.Reply{{
		Author:  author.Author{Name: "first" + c1},
		Replies: []reply.Reply{{Author: author.Author{Name: "nested" + clearScreen}}},
	}}
	return nil
}

func controlled(s string) bool {
	return strings.ContainsAny(s, "\x1b\a\u009b")
}

func TestTheFeedFiltersTextFromTheSystems(t *testing.T) {
	sys := &hostile{fake: &fake{
		forums: []forum.Forum{{ID: "f", Name: "General" + clearScreen, Info: "About" + hyperlink}},
		lists: map[system.Order][]post.Post{system.OrderNew: {{
			ID:      "p",
			Subject: "Hello" + hyperlink,
			Author:  author.Author{Name: "vera" + c1},
			Forum:   forum.Forum{Name: "General" + clearScreen},
		}}},
	}}
	f := New([]system.System{sys}, 0, 0)

	forums, _ := f.ListForums(context.Background(), -1)
	if len(forums) != 1 || controlled(forums[0].Name) || controlled(forums[0].Info) || forums[0].Name != "General" {
		t.Errorf("the forums are %+v", forums)
	}

	result, err := f.List(context.Background(), 0, "", system.OrderNew)
	if err != nil || len(result.Posts) != 1 {
		t.Fatalf("got %+v, %v", result, err)
	}
	p := result.Posts[0]
	if controlled(p.Subject) || controlled(p.Author.Name) || controlled(p.Forum.Name) || p.Author.Name != "vera2J" {
		t.Errorf("the post is %+v", p)
	}

	if err := f.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if controlled(p.Subject) || controlled(p.Author.Name) || controlled(p.Replies[0].Author.Name) ||
		controlled(p.Replies[0].Replies[0].Author.Name) || p.Replies[0].Replies[0].Author.Name != "nested" {
		t.Errorf("the loaded post is %+v", p)
	}
}
