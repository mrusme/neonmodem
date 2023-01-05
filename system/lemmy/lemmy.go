package lemmy

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/araddon/dateparse"
	"github.com/mrusme/gobbs/models/author"
	"github.com/mrusme/gobbs/models/forum"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/system/adapter"
	"go.arsenm.dev/go-lemmy"
	"go.arsenm.dev/go-lemmy/types"
	"go.uber.org/zap"
)

type System struct {
	ID     int
	config map[string]interface{}
	logger *zap.SugaredLogger
	client *lemmy.Client
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
			ID:   "list:posts",
			Name: "List Posts",
		},
		adapter.Capability{
			ID:   "create:post",
			Name: "Create Post",
		},
	)

	return caps
}

func (sys *System) FilterValue() string {
	return fmt.Sprintf(
		"Lemmy %s",
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
		"Lemmy",
	)
}

func (sys *System) Load() error {
	var err error

	url := sys.config["url"]
	if url == nil {
		return nil
	}

	sys.client, err = lemmy.New(url.(string))
	if err != nil {
		return err
	}

	credentials := make(map[string]string)
	for k, v := range (sys.config["credentials"]).(map[string]interface{}) {
		credentials[k] = v.(string)
	}

	err = sys.client.Login(context.Background(), types.Login{
		UsernameOrEmail: credentials["username"],
		Password:        credentials["password"],
	})
	if err != nil {
		return err
	}
	return nil
}

func (sys *System) ListPosts() ([]post.Post, error) {
	resp, err := sys.client.Posts(context.Background(), types.GetPosts{
		Type:  types.NewOptional(types.ListingSubscribed),
		Sort:  types.NewOptional(types.New),
		Limit: types.NewOptional(int64(50)),
	})
	if err != nil {
		return []post.Post{}, err
	}

	var models []post.Post
	for _, i := range resp.Posts {
		t := "post"
		body := i.Post.Body.ValueOr("")
		if i.Post.URL.IsValid() {
			t = "url"
			body = i.Post.URL.ValueOr("")
		}

		createdAt, err := dateparse.ParseAny(i.Post.Published)
		if err != nil {
			createdAt = time.Now() // TODO: Errrr
		}
		lastCommentedAt, err := dateparse.ParseAny(i.Counts.NewestCommentTime)
		if err != nil {
			lastCommentedAt = time.Now() // TODO: Errrrr
		}

		models = append(models, post.Post{
			ID: strconv.Itoa(i.Post.ID),

			Subject: i.Post.Name,
			Body:    body,

			Type: t,

			Pinned: i.Post.Stickied,
			Closed: i.Post.Locked,

			CreatedAt:       createdAt,
			LastCommentedAt: lastCommentedAt,

			Author: author.Author{
				ID:   strconv.Itoa(i.Post.CreatorID),
				Name: i.Creator.Name,
			},

			Forum: forum.Forum{
				ID:   strconv.Itoa(i.Post.CommunityID),
				Name: i.Community.Name,
			},

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	pid, err := strconv.Atoi(p.ID)
	if err != nil {
		return err
	}
	// cid, err := strconv.Atoi(p.Forum.ID)
	// if err != nil {
	// 	return err
	// }

	resp, err := sys.client.Comments(context.Background(), types.GetComments{
		Type: types.NewOptional(types.ListingLocal),
		Sort: types.NewOptional(types.CommentSortHot),
		// CommunityID: types.NewOptional(cid),
		PostID: types.NewOptional(pid),
	})
	if err != nil {
		return err
	}

	for _, i := range resp.Comments {
		createdAt, err := dateparse.ParseAny(i.Comment.Published)
		if err != nil {
			createdAt = time.Now() // TODO: Errrrr
		}

		p.Replies = append(p.Replies, reply.Reply{
			ID: strconv.Itoa(i.Comment.ID),

			Body: i.Comment.Content,

			CreatedAt: createdAt,

			Author: author.Author{
				ID:   strconv.Itoa(i.Comment.CreatorID),
				Name: i.Creator.Name,
			},

			SysIDX: sys.ID,
		})
	}
	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	communityID, err := strconv.Atoi(p.Forum.ID)
	if err != nil {
		return err
	}

	resp, err := sys.client.CreatePost(context.Background(), types.CreatePost{
		Name:        p.Subject,
		CommunityID: communityID,
		Body:        types.NewOptional(p.Body),
		NSFW:        types.NewOptional(false),
	})
	if err != nil {
		return err
	}

	p.ID = strconv.Itoa(resp.PostView.Post.ID)
	return nil
}

func (sys *System) CreateReply(r *reply.Reply) error {
	id, err := strconv.Atoi(r.ID)
	if err != nil {
		return err
	}
	inReplyTo, err := strconv.Atoi(r.InReplyTo)
	if err != nil {
		return err
	}

	resp, err := sys.client.CreateComment(context.Background(), types.CreateComment{
		PostID:   inReplyTo,
		ParentID: types.NewOptional(id),
		Content:  r.Body,
	})
	if err != nil {
		return err
	}

	r.ID = strconv.Itoa(resp.CommentView.Comment.ID)
	return nil
}
