package post

import (
	"fmt"
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
	Replies    []reply.Reply
	ReplyPage  ReplyPage

	URL string

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

	return fmt.Sprintf("by %s %s in %s", post.Author.Name, when, post.Forum.Name)
}
