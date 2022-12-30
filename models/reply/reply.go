package reply

import "github.com/mrusme/gobbs/models/author"

type Reply struct {
	ID string

	Body string

	Author author.Author
}
