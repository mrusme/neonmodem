package post

import (
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
)

func TestScoreString(t *testing.T) {
	for _, tc := range []struct {
		score Score
		want  string
	}{
		{Score{}, ""},
		{Score{Value: 42, Unit: ScorePoints}, "42 points"},
		{Score{Value: 1, Unit: ScorePoints}, "1 point"},
		{Score{Value: 1, Unit: ScoreLikes}, "1 like"},
		{Score{Value: 0, Unit: ScoreLikes}, "0 likes"},
	} {
		if got := tc.score.String(); got != tc.want {
			t.Errorf("%+v: got %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestDescriptionWithoutForum(t *testing.T) {
	p := Post{Author: author.Author{Name: "alice"}, CreatedAt: time.Now().Add(-3 * time.Hour)}
	if got := p.Description(); got != "by alice 3 hours ago" {
		t.Errorf("got %q", got)
	}

	p.Forum = forum.Forum{Name: "Show HN"}
	if got := p.Description(); got != "by alice 3 hours ago in Show HN" {
		t.Errorf("got %q", got)
	}
}

func TestLinkBody(t *testing.T) {
	if kind, body := LinkBody("", "text"); kind != KindText || body != "text" {
		t.Errorf("text post: %v %q", kind, body)
	}
	if kind, body := LinkBody("https://x", ""); kind != KindLink || body != "https://x" {
		t.Errorf("bare link: %v %q", kind, body)
	}
	if kind, body := LinkBody("https://x", "text"); kind != KindLink || body != "https://x\n\ntext" {
		t.Errorf("link with text: %v %q", kind, body)
	}
}

func TestRefreshKeepsTheIdentity(t *testing.T) {
	p := Post{ID: "1", Subject: "old", Forum: forum.Forum{ID: "f"}, SysIDX: 3, ReplyCount: 1}
	p.Refresh(Post{ID: "other", Subject: "new", Body: "b", Kind: KindLink, Link: "https://x",
		Pinned: true, Closed: true, Author: author.Author{Name: "a"}, ReplyCount: 7,
		Score: Score{Value: 2, Unit: ScorePoints}, URL: "https://u", Forum: forum.Forum{ID: "g"}, SysIDX: 9})

	if p.ID != "1" || p.Forum.ID != "f" || p.SysIDX != 3 {
		t.Errorf("the identity changed: %+v", p)
	}
	if p.Subject != "new" || p.Body != "b" || p.Kind != KindLink || p.Link != "https://x" ||
		!p.Pinned || !p.Closed || p.Author.Name != "a" || p.ReplyCount != 7 ||
		p.Score.Value != 2 || p.URL != "https://u" {
		t.Errorf("the content wasn't refreshed: %+v", p)
	}
}
