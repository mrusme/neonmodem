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

func Tree(flat []Reply) []Reply {
	index := make(map[string]int, len(flat))
	for i := range flat {
		index[flat[i].ID] = i
	}

	children := make(map[int][]int)
	var roots []int
	for i := range flat {
		parent := flat[i].ParentID
		if idx, ok := index[parent]; ok && parent != "" && idx != i {
			children[idx] = append(children[idx], i)
			continue
		}
		flat[i].ParentID = ""
		roots = append(roots, i)
	}

	var build func(indexes []int) []Reply
	build = func(indexes []int) []Reply {
		out := make([]Reply, 0, len(indexes))
		for _, i := range indexes {
			r := flat[i]
			r.Replies = build(children[i])
			out = append(out, r)
		}
		return out
	}

	return build(roots)
}
