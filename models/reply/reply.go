package reply

import (
	"time"

	"github.com/mrusme/gobbs/models/author"
)

type Reply struct {
	ID string

	Body string

	Deleted bool

	CreatedAt time.Time

	Author author.Author

	Replies []Reply
}
