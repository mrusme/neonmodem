package reply

import (
	"time"

	"github.com/mrusme/gobbs/models/author"
)

type Reply struct {
	ID string

	Body string

	CreatedAt time.Time

	Author author.Author
}
