package reply

import (
	"time"

	"github.com/mrusme/neonmodem/internal/models/author"
)

type Reply struct {
	ID       string
	PostID   string
	ParentID string

	Body string

	Deleted bool

	CreatedAt time.Time

	Author author.Author

	Replies []Reply

	SysIDX int
}

func Count(replies []Reply) int {
	n := 0
	for i := range replies {
		n += 1 + Count(replies[i].Replies)
	}
	return n
}
