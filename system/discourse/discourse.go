package discourse

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/araddon/dateparse"
	"github.com/mrusme/neonmodem/models/author"
	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/system/adapter"
	"github.com/mrusme/neonmodem/system/discourse/api"
	"go.uber.org/zap"
)

type System struct {
	ID        int
	config    map[string]interface{}
	logger    *zap.SugaredLogger
	client    *api.Client
	clientCfg api.ClientConfig
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
		adapter.Capability{
			ID:   "connect:multiple",
			Name: "Connect Multiple",
		},
		adapter.Capability{
			ID:   "list:forums",
			Name: "List Forums",
		},
		adapter.Capability{
			ID:   "list:posts",
			Name: "List Posts",
		},
		adapter.Capability{
			ID:   "create:post",
			Name: "Create Post",
		},
		adapter.Capability{
			ID:   "list:replies",
			Name: "List Replies",
		},
		adapter.Capability{
			ID:   "create:reply",
			Name: "Create Reply",
		},
	)

	return caps
}

func (sys *System) FilterValue() string {
	return fmt.Sprintf(
		"Discourse %s",
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
		"Discourse",
	)
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

	sys.clientCfg = api.NewDefaultClientConfig(
		url.(string),
		sys.config["proxy"].(string),
		credentials,
		sys.logger,
	)
	sys.client = api.NewClient(&sys.clientCfg)

	return nil
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	var models []forum.Forum

	cats, err := sys.client.Categories.List(context.Background())
	if err != nil {
		return []forum.Forum{}, err
	}

	models = sys.recurseForums(&cats.CategoryList.Categories, nil)

	return models, nil
}

func (sys *System) recurseForums(cats *[]api.CategoryModel, parent *forum.Forum) []forum.Forum {
	var models []forum.Forum

	sys.logger.Debugf("recursing categories: %d\n", len(*cats))
	for i := 0; i < len(*cats); i++ {
		sys.logger.Debugf("adding category: %s\n", (*cats)[i].Name)

		var name = (*cats)[i].Name
		if parent != nil {
			name = fmt.Sprintf("%s / %s", parent.Name, name)
		}
		f := forum.Forum{
			ID:   strconv.Itoa((*cats)[i].ID),
			Name: name,

			Info: (*cats)[i].Description,

			SysIDX: sys.ID,
		}
		models = append(models, f)

		models = append(models, sys.recurseForums(&(*cats)[i].SubcategoryList, &f)...)
	}

	return models
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	var catSlug string = ""
	var catID int = -1
	var err error

	cats, err := sys.client.Categories.List(context.Background())
	if err != nil {
		return []post.Post{}, err
	}

	if forumID != "" {
		catID, err = strconv.Atoi(forumID)
		if err != nil {
			return []post.Post{}, err
		}

		for _, cat := range cats.CategoryList.Categories {
			if cat.ID == catID {
				catSlug = cat.Slug
				break
			}
		}
	}

	items, err := sys.client.Topics.ListLatest(context.Background(), catSlug, catID)
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

		cfg := sys.GetConfig()
		baseURL := cfg["url"].(string)

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

				SysIDX: sys.ID,
			},

			TotalReplies:           0,
			CurrentRepliesStartIDX: -1,

			URL: fmt.Sprintf("%s/t/%d", baseURL, i.ID),

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	item, err := sys.client.Topics.Show(context.Background(), p.ID)
	if err != nil {
		return err
	}

	// API seems to return 20 posts by default. If the stream is greater than 20
	// posts, we need to fetch the latest posts on our own, as we'd only get the
	// first 20 posts otherwise.
	p.TotalReplies = len(item.PostStream.Stream)
	if p.TotalReplies > 20 {
		var postIDs []int

		if p.CurrentRepliesStartIDX == -1 ||
			// Explain to me standard GoFmt logic:
			p.CurrentRepliesStartIDX > (p.TotalReplies-20) {
			p.CurrentRepliesStartIDX = (p.TotalReplies - 20)
			// /)_-)
		} else if p.CurrentRepliesStartIDX < -1 {
			p.CurrentRepliesStartIDX = 0
		}

		if p.CurrentRepliesStartIDX > 0 {
			postIDs = append(postIDs,
				item.PostStream.Stream[0])
			p.CurrentRepliesStartIDX++
		}
		postIDs = append(postIDs,
			item.PostStream.Stream[p.CurrentRepliesStartIDX:(p.CurrentRepliesStartIDX+20)]...)

		replies, err := sys.client.Topics.ShowPosts(
			context.Background(),
			p.ID,
			postIDs,
		)
		if err != nil {
			sys.logger.Error(err)
			return err
		}

		item.PostStream.Posts = replies.PostStream.Posts
	}

	converter := md.NewConverter("", true, nil)

	p.Replies = []reply.Reply{}
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
			ID:        strconv.Itoa(i.ID),
			InReplyTo: p.ID,
			Index:     idx,

			Body: cookedMd,

			CreatedAt: createdAt,

			Author: author.Author{
				ID:   strconv.Itoa(i.UserID),
				Name: i.Name,
			},

			SysIDX: sys.ID,
		})
	}

	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	categoryID, err := strconv.Atoi(p.Forum.ID)
	if err != nil {
		return err
	}

	ap := api.CreatePostModel{
		Title:     p.Subject,
		Raw:       p.Body,
		Category:  categoryID,
		CreatedAt: time.Now().Format(time.RFC3339Nano),
	}

	cp, err := sys.client.Posts.Create(context.Background(), &ap)
	if err != nil {
		return err
	}

	p.ID = strconv.Itoa(cp.ID)
	return nil
}

func (sys *System) CreateReply(r *reply.Reply) error {
	var err error

	ID, err := strconv.Atoi(r.ID)
	if err != nil {
		return err
	}

	var ap api.CreatePostModel

	if r.Index == -1 {
		// Looks like we're replying directly to a post
		ap = api.CreatePostModel{
			Raw:       r.Body,
			TopicID:   ID,
			CreatedAt: time.Now().Format(time.RFC3339Nano),
		}
	} else {
		// Apparently it's a reply to a comment in a post
		inReplyTo, err := strconv.Atoi(r.InReplyTo)
		if err != nil {
			return err
		}

		ap = api.CreatePostModel{
			Raw:               r.Body,
			TopicID:           inReplyTo,
			ReplyToPostNumber: r.Index + 1,
			CreatedAt:         time.Now().Format(time.RFC3339Nano),
		}
	}

	cp, err := sys.client.Posts.Create(context.Background(), &ap)
	if err != nil {
		return err
	}

	r.ID = strconv.Itoa(cp.ID)

	return nil
}
