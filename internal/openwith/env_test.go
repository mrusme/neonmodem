package openwith

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

type fakeSystem struct{}

func (fakeSystem) Kind() string                      { return "lemmy" }
func (fakeSystem) Title() string                     { return "lemmy.example" }
func (fakeSystem) URL() string                       { return "https://lemmy.example" }
func (fakeSystem) Description() string               { return "Lemmy" }
func (fakeSystem) Capabilities() system.Capabilities { return system.CapRead }
func (fakeSystem) Connect(context.Context, prompt.Prompter, string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (fakeSystem) ListForums(context.Context) ([]forum.Forum, error) { return nil, nil }
func (fakeSystem) Orders(string) system.Ordering                     { return system.Only(system.OrderNew) }
func (fakeSystem) ListPosts(context.Context, string, system.Order) ([]post.Post, error) {
	return nil, nil
}
func (fakeSystem) LoadPost(context.Context, *post.Post) error      { return nil }
func (fakeSystem) CreatePost(context.Context, *post.Post) error    { return nil }
func (fakeSystem) CreateReply(context.Context, *reply.Reply) error { return nil }

func TestEnvNamesThePost(t *testing.T) {
	p := post.Post{
		ID:        "123456",
		Subject:   "Linux 7.3 released",
		URL:       "https://lemmy.example/post/123456",
		Link:      "https://kernel.org/",
		Forum:     forum.Forum{ID: "3", Name: "linux"},
		CreatedAt: time.Date(2026, 10, 7, 17, 0, 0, 0, time.FixedZone("JST", 9*3600)),
	}

	want := []string{
		"NM_SYSTEM_TYPE=lemmy",
		"NM_SYSTEM_URL=https://lemmy.example",
		"NM_FORUM_ID=3",
		"NM_FORUM_NAME=linux",
		"NM_POST_ID=123456",
		"NM_POST_URL=https://lemmy.example/post/123456",
		"NM_POST_SUBJECT=Linux 7.3 released",
		"NM_POST_CREATED=2026-10-07T08:00:00Z",
		"NM_POST_LINK=https://kernel.org/",
	}
	if got := Env(fakeSystem{}, p); !slices.Equal(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestEnvKeepsEmptyValuesAndDropsNULs(t *testing.T) {
	got := Env(fakeSystem{}, post.Post{ID: "1", Subject: "a\x00b"})

	for _, want := range []string{"NM_FORUM_ID=", "NM_POST_CREATED=", "NM_POST_LINK=", "NM_POST_SUBJECT=ab"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}
