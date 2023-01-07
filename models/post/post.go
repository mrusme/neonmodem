package post

import (
	"fmt"
	"strings"
	"time"

	"github.com/mergestat/timediff"
	"github.com/mrusme/neonmodem/models/author"
	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/reply"
)

type Post struct {
	ID string

	Subject string
	Body    string
	Type    string // "post", "url"

	Pinned bool
	Closed bool

	CreatedAt       time.Time
	LastCommentedAt time.Time

	Author author.Author

	Forum forum.Forum

	Replies []reply.Reply

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
	return fmt.Sprintf(
		"by %s %s in %s",
		post.Author.Name,
		timediff.TimeDiff(post.CreatedAt.Local()),
		strings.Title(post.Forum.Name),
	)
}
