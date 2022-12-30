package post

import (
	"github.com/mrusme/gobbs/models/author"
	"github.com/mrusme/gobbs/models/reply"
)

type Post struct {
	ID string

	Subject string
	Body    string
	Type    string // "post", "url"

	Pinned bool
	Closed bool

	Author author.Author

	Replies []reply.Reply
}

func (post Post) FilterValue() string {
	return post.Subject
}

func (post Post) Title() string {
	return post.Subject
}

func (post Post) Description() string {
	return post.ID
}
