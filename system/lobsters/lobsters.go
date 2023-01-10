package lobsters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/araddon/dateparse"
	"github.com/mrusme/neonmodem/models/author"
	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/system/adapter"
	"github.com/mrusme/neonmodem/system/lobsters/api"
	"go.uber.org/zap"
)

type System struct {
	ID     int
	config map[string]interface{}
	logger *zap.SugaredLogger
	client *api.Client
}

func (sys *System) GetID() int {
	return sys.ID
}

func (sys *System) SetID(id int) {
	sys.ID = id
}

func (sys *System) GetConfig() map[string]interface{} {
	return sys.config
}

func (sys *System) SetConfig(cfg *map[string]interface{}) {
	sys.config = *cfg
}

func (sys *System) SetLogger(logger *zap.SugaredLogger) {
	sys.logger = logger
}

func (sys *System) GetCapabilities() adapter.Capabilities {
	var caps []adapter.Capability

	caps = append(caps,
		// TODO: Requires accounts
		// adapter.Capability{
		// 	ID:   "connect:multiple",
		// 	Name: "Connect Multiple",
		// },
		adapter.Capability{
			ID:   "list:forums",
			Name: "List Forums",
		},
		adapter.Capability{
			ID:   "list:posts",
			Name: "List Posts",
		},
		// adapter.Capability{
		// 	ID:   "create:post",
		// 	Name: "Create Post",
		// },
		adapter.Capability{
			ID:   "list:replies",
			Name: "List Replies",
		},
		// adapter.Capability{
		// 	ID:   "create:reply",
		// 	Name: "Create Reply",
		// },
	)

	return caps
}

func (sys *System) FilterValue() string {
	return fmt.Sprintf(
		"Lobsters %s",
		sys.config["url"],
	)
}

func (sys *System) Title() string {
	sysUrl := sys.config["url"].(string)
	u, err := url.Parse(sysUrl)
	if err != nil {
		return sysUrl
	}

	return u.Hostname()
}

func (sys *System) Description() string {
	return fmt.Sprintf(
		"Lobsters",
	)
}

func (sys *System) Load() error {
	url := sys.config["url"]
	if url == nil {
		return nil
	}

	credentials := make(map[string]string)

	sys.client = api.NewClient(&api.ClientConfig{
		Endpoint:    url.(string),
		Credentials: credentials,
		HTTPClient:  http.DefaultClient,
		Logger:      sys.logger,
	})

	return nil
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	var models []forum.Forum

	tags, err := sys.client.Tags.List(context.Background())
	if err != nil {
		return []forum.Forum{}, err
	}

	for _, tag := range *tags {
		models = append(models, forum.Forum{
			ID:   tag.Tag,
			Name: tag.Tag,

			Info: tag.Description,

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	var err error

	items, err := sys.client.Stories.List(context.Background(), forumID)
	if err != nil {
		return []post.Post{}, err
	}

	var models []post.Post
	for _, i := range *items {
		createdAt, err := dateparse.ParseAny(i.CreatedAt)
		if err != nil {
			createdAt = time.Now() // TODO: Errrr
		}

		models = append(models, post.Post{
			ID: i.ShortID,

			Subject: i.Title,

			Type: "url",

			Pinned: false,
			Closed: false,

			CreatedAt:       createdAt,
			LastCommentedAt: createdAt, // TODO

			Author: author.Author{
				ID:   i.SubmitterUser.Username,
				Name: i.SubmitterUser.Username,
			},

			Forum: forum.Forum{
				ID:   i.Tags[0],
				Name: i.Tags[0], // TODO: Tag description

				SysIDX: sys.ID,
			},

			// TODO: Implement chunks loading
			TotalReplies:           0,
			CurrentRepliesStartIDX: -1,

			URL: i.ShortIDURL,

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	item, err := sys.client.Stories.Show(context.Background(), p.ID)
	if err != nil {
		return err
	}

	converter := md.NewConverter("", true, nil)

	p.Replies = []reply.Reply{}
	for idx, i := range item.Comments {
		cookedMd, err := converter.ConvertString(i.Comment)
		if err != nil {
			cookedMd = i.CommentPlain
		}

		if idx == 0 {
			p.Body = cookedMd
			continue
		}

		createdAt, err := dateparse.ParseAny(i.CreatedAt)
		if err != nil {
			createdAt = time.Now() // TODO: Errrrrr
		}

		inReplyTo := i.ParentComment
		if inReplyTo == "" {
			inReplyTo = p.ID
		}
		p.Replies = append(p.Replies, reply.Reply{
			ID:        i.ShortID,
			InReplyTo: inReplyTo,

			Body: cookedMd,

			CreatedAt: createdAt,

			Author: author.Author{
				ID:   i.CommentingUser.Username,
				Name: i.CommentingUser.Username,
			},

			SysIDX: sys.ID,
		})
	}

	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	return nil
}

func (sys *System) CreateReply(r *reply.Reply) error {
	return nil
}
