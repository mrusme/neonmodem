package feed

import (
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system/text"
)

func cleanForum(f *forum.Forum) {
	f.Name = text.Printable(f.Name)
	f.Info = text.Printable(f.Info)
}

func cleanPost(p *post.Post) {
	p.Subject = text.Printable(p.Subject)
	p.Author.Name = text.Printable(p.Author.Name)
	cleanForum(&p.Forum)
	cleanReplies(p.Replies)
}

func cleanReplies(replies []reply.Reply) {
	for i := range replies {
		replies[i].Author.Name = text.Printable(replies[i].Author.Name)
		cleanReplies(replies[i].Replies)
	}
}
