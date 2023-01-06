package lemmy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

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
			ID:   "list:forums",
			Name: "List Forums",
		},
		adapter.Capability{
			ID:   "list:posts",
			Name: "List Posts",
		},
		// TODO
		// adapter.Capability{
		// 	ID:   "create:post",
		// 	Name: "Create Post",
		// },
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

	err = sys.client.ClientLogin(context.Background(), types.Login{
		UsernameOrEmail: credentials["username"],
		Password:        credentials["password"],
	})
	if err != nil {
		return err
	}
	return nil
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	resp, err := sys.client.ListCommunities(context.Background(), types.ListCommunities{
		Type: types.NewOptional(types.ListingTypeSubscribed),
	})
	if err != nil {
		return []forum.Forum{}, err
	}

	var models []forum.Forum
	for _, i := range resp.Communities {
		sys.logger.Debugf("FORUM:")
		b, _ := json.Marshal(i)
		sys.logger.Debug(string(b))
		models = append(models, forum.Forum{
			ID:   strconv.Itoa(i.Community.ID),
			Name: i.Community.Name,

			Info: i.Community.Description.ValueOr(i.Community.Title),

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	resp, err := sys.client.Posts(context.Background(), types.GetPosts{
		Type:  types.NewOptional(types.ListingTypeSubscribed),
		Sort:  types.NewOptional(types.SortTypeNew),
		Limit: types.NewOptional(int64(50)),
	})
	sys.logger.Debug("DEEEEBUUUUUUUGGGGGGGG")
	if err != nil {
		return []post.Post{}, err
	}

	cfg := sys.GetConfig()
	baseURL := cfg["url"].(string)

	var models []post.Post
	for _, i := range resp.Posts {
		b, _ := json.Marshal(i)
		sys.logger.Debug(string(b))
		t := "post"
		body := i.Post.Body.ValueOr("")
		if i.Post.URL.IsValid() {
			t = "url"
			body = i.Post.URL.ValueOr("")
		}

		createdAt := i.Post.Published.Time
		lastCommentedAt := i.Counts.NewestCommentTime.Time

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

				SysIDX: sys.ID,
			},

			URL: fmt.Sprintf("%s/post/%d", baseURL, i.Post.ID),

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

	resp, err := sys.client.Post(context.Background(), types.GetPost{
		ID: pid,
	})
	if err != nil {
		return err
	}

	for _, i := range resp.Comments {
		createdAt := i.Comment.Published.Time

		p.Replies = append(p.Replies, reply.Reply{
			ID: strconv.Itoa(i.Comment.ID),

			InReplyTo: p.ID,

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
	ID, err := strconv.Atoi(r.ID)
	if err != nil {
		return err
	}

	var create types.CreateComment
	if r.InReplyTo != "" {
		// Reply to a reply of a post
		InReplyTo, err := strconv.Atoi(r.InReplyTo)
		if err != nil {
			return err
		}
		create = types.CreateComment{
			PostID:   InReplyTo,
			ParentID: types.NewOptional(ID),
			Content:  r.Body,
		}
	} else {
		// Reply to a post
		create = types.CreateComment{
			PostID:  ID,
			Content: r.Body,
		}
	}

	resp, err := sys.client.CreateComment(context.Background(), create)
	if err != nil {
		return err
	}

	r.ID = strconv.Itoa(resp.CommentView.Comment.ID)
	return nil
}
