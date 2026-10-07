package post

import (
	"fmt"
	"strings"
	"time"

	"github.com/mergestat/timediff"
	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/reply"
)

type Kind uint8

const (
	KindText Kind = iota
	KindLink
)

type ReplyPage struct {
	Offset int
	Size   int
	Total  int
}

func (p ReplyPage) HasOlder() bool {
	return p.Offset > 0
}

type ScoreUnit string

const (
	ScorePoints ScoreUnit = "points"
	ScoreLikes  ScoreUnit = "likes"
)

type Score struct {
	Value int
	Unit  ScoreUnit
}

func (s Score) String() string {
	if s.Unit == "" {
		return ""
	}
	if s.Value == 1 || s.Value == -1 {
		return fmt.Sprintf("%d %s", s.Value, strings.TrimSuffix(string(s.Unit), "s"))
	}
	return fmt.Sprintf("%d %s", s.Value, s.Unit)
}

type Post struct {
	ID string

	Subject string
	Body    string
	Kind    Kind

	Pinned bool
	Closed bool

	CreatedAt       time.Time
	LastCommentedAt time.Time

	Author author.Author

	Forum forum.Forum

	ReplyCount int
	Score      Score
	Replies    []reply.Reply
	ReplyPage  ReplyPage

	URL  string
	Link string

	SysIDX int
}

func (post Post) FilterValue() string {
	return post.Subject
}

func (post Post) Title() string {
	return post.Subject
}

func (post Post) Description() string {
	when := "at an unknown time"
	if !post.CreatedAt.IsZero() {
		when = timediff.TimeDiff(post.CreatedAt.Local())
	}

	desc := fmt.Sprintf("by %s %s", post.Author.Name, when)
	if post.Forum.Name != "" {
		desc += " in " + post.Forum.Name
	}
	return desc
}
