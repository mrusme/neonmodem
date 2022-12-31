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
		"[%s] %s in %s on %s",
		post.ID,
		post.Author.Name,
		post.Forum.Name,
		post.CreatedAt.Format("Jan 2 2006"),
	)
}
