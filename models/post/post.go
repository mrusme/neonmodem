package post

import (
	"fmt"
	"time"

	"github.com/mrusme/gobbs/models/author"
	"github.com/mrusme/gobbs/models/forum"
	"github.com/mrusme/gobbs/models/reply"
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
		"in %s by %s on %s",
		post.Forum.Name,
		post.Author.Name,
		post.CreatedAt.Format("02 Jan 06 15:04 MST"),
	)
}
