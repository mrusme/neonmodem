package reply

import (
	"time"

	"github.com/mrusme/gobbs/models/author"
)

type Reply struct {
	ID        string
	InReplyTo string

	Body string

	Deleted bool

	CreatedAt time.Time

	Author author.Author

	Replies []Reply

	SysIDX int
}
