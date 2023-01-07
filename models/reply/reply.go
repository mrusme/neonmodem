package reply

import (
	"time"

	"github.com/mrusme/neonmodem/models/author"
)

type Reply struct {
	ID        string
	InReplyTo string
	Index     int

	Body string

	Deleted bool

	CreatedAt time.Time

	Author author.Author

	Replies []Reply

	SysIDX int
}
