package openwith

import (
	"strings"
	"time"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
)

func Env(sys system.System, p post.Post) []string {
	created := ""
	if !p.CreatedAt.IsZero() {
		created = p.CreatedAt.UTC().Format(time.RFC3339)
	}

	pairs := [][2]string{
		{"NM_SYSTEM_TYPE", sys.Kind()},
		{"NM_SYSTEM_URL", sys.URL()},
		{"NM_FORUM_ID", p.Forum.ID},
		{"NM_FORUM_NAME", p.Forum.Name},
		{"NM_POST_ID", p.ID},
		{"NM_POST_URL", p.URL},
		{"NM_POST_SUBJECT", p.Subject},
		{"NM_POST_CREATED", created},
		{"NM_POST_LINK", p.Link},
	}

	env := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		env = append(env, pair[0]+"="+strings.ReplaceAll(pair[1], "\x00", ""))
	}
	return env
}
