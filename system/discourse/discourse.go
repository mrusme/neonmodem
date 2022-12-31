package discourse

import (
	"context"
	"net/http"
	"strconv"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/araddon/dateparse"
	"github.com/mrusme/gobbs/models/author"
	"github.com/mrusme/gobbs/models/forum"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/system/adapter"
	"go.uber.org/zap"
)

type System struct {
	config map[string]interface{}
	logger *zap.SugaredLogger
	client *Client
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

func (sys *System) GetCapabilities() []adapter.Capability {
	var caps []adapter.Capability

	caps = append(caps, adapter.Capability{
		ID:   "posts",
		Name: "Posts",
	})
	caps = append(caps, adapter.Capability{
		ID:   "groups",
		Name: "Groups",
	})
	caps = append(caps, adapter.Capability{
		ID:   "search",
		Name: "Search",
	})

	return caps
}

func (sys *System) Load() error {
	url := sys.config["url"]
	if url == nil {
		return nil
	}

	credentials := make(map[string]string)
	for k, v := range (sys.config["credentials"]).(map[string]interface{}) {
		credentials[k] = v.(string)
	}

	sys.client = NewClient(&ClientConfig{
		Endpoint:    url.(string),
		Credentials: credentials,
		HTTPClient:  http.DefaultClient,
		Logger:      sys.logger,
	})

	return nil
}

func (sys *System) ListPosts(sysIdx int) ([]post.Post, error) {
	cats, err := sys.client.Categories.List(context.Background())
	if err != nil {
		return []post.Post{}, err
	}

	items, err := sys.client.Topics.ListLatest(context.Background())
	if err != nil {
		return []post.Post{}, err
	}

	var models []post.Post
	for _, i := range (*items).TopicList.Topics {
		var userName string = ""
		for _, u := range (*items).Users {
			if u.ID == i.Posters[0].UserID {
				userName = u.Name
				break
			}
		}

		createdAt, err := dateparse.ParseAny(i.CreatedAt)
		if err != nil {
			createdAt = time.Now() // TODO: Errrr
		}
		lastCommentedAt, err := dateparse.ParseAny(i.LastPostedAt)
		if err != nil {
			lastCommentedAt = time.Now() // TODO: Errrrr
		}

		var forumName string = ""
		for _, cat := range cats.CategoryList.Categories {
			if cat.ID == i.CategoryID {
				forumName = cat.Name
				break
			}

			for _, subcat := range cat.SubcategoryList {
				if subcat.ID == i.CategoryID {
					forumName = subcat.Name
					break
				}
			}
		}

		models = append(models, post.Post{
			ID: strconv.Itoa(i.ID),

			Subject: i.Title,

			Type: "post",

			Pinned: i.Pinned,
			Closed: i.Closed,

			CreatedAt:       createdAt,
			LastCommentedAt: lastCommentedAt,

			Author: author.Author{
				ID:   strconv.Itoa(i.Posters[0].UserID),
				Name: userName,
			},

			Forum: forum.Forum{
				ID:   strconv.Itoa(i.CategoryID),
				Name: forumName,
			},

			SysIDX: sysIdx,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	item, err := sys.client.Topics.Show(context.Background(), p.ID)
	if err != nil {
		return err
	}

	converter := md.NewConverter("", true, nil)

	for idx, i := range item.PostStream.Posts {
		cookedMd, err := converter.ConvertString(i.Cooked)
		if err != nil {
			cookedMd = i.Cooked
		}

		if idx == 0 {
			p.Body = cookedMd
			continue
		}

		createdAt, err := dateparse.ParseAny(i.CreatedAt)
		if err != nil {
			createdAt = time.Now() // TODO: Errrrrr
		}
		p.Replies = append(p.Replies, reply.Reply{
			ID: strconv.Itoa(i.ID),

			Body: cookedMd,

			CreatedAt: createdAt,

			Author: author.Author{
				ID:   strconv.Itoa(i.UserID),
				Name: i.Name,
			},
		})
	}

	return nil
}
