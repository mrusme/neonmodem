package lemmy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mrusme/neonmodem/models/author"
	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/system/adapter"
	"go.elara.ws/go-lemmy"
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
	var httpClient *http.Client = nil
	var httpTransport *http.Transport = http.DefaultTransport.(*http.Transport).
		Clone()
	var err error

	u := sys.config["url"]
	if u == nil {
		return nil
	}

	proxy := sys.config["proxy"].(string)
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			sys.logger.Error(err)
		} else {
			sys.logger.Debugf("setting up http proxy transport: %s\n",
				proxyURL.String())
			httpTransport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	httpClient = &http.Client{
		Transport: httpTransport,
		Timeout:   time.Second * 10,
	}

	sys.client, err = lemmy.NewWithClient(
		u.(string),
		httpClient,
	)
	if err != nil {
		return err
	}

	credentials := make(map[string]string)
	for k, v := range (sys.config["credentials"]).(map[string]interface{}) {
		credentials[k] = v.(string)
	}

	err = sys.client.ClientLogin(context.Background(), lemmy.Login{
		UsernameOrEmail: credentials["username"],
		Password:        credentials["password"],
	})
	if err != nil {
		return err
	}
	return nil
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	resp, err := sys.client.Communities(context.Background(), lemmy.ListCommunities{
		Type: lemmy.NewOptional(lemmy.ListingTypeSubscribed),
	})
	if err != nil {
		return []forum.Forum{}, err
	}

	var models []forum.Forum
	for _, i := range resp.Communities {
		models = append(models, forum.Forum{
			ID:   strconv.FormatInt(i.Community.ID, 10),
			Name: i.Community.Name,

			Info: i.Community.Description.ValueOr(i.Community.Title),

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	resp, err := sys.client.Posts(context.Background(), lemmy.GetPosts{
		Type:  lemmy.NewOptional(lemmy.ListingTypeSubscribed),
		Sort:  lemmy.NewOptional(lemmy.SortTypeNew),
		Limit: lemmy.NewOptional(int64(50)),
	})

	if err != nil {
		return []post.Post{}, err
	}

	cfg := sys.GetConfig()
	baseURL := cfg["url"].(string)

	var models []post.Post
	for _, i := range resp.Posts {
		t := "post"
		body := i.Post.Body.ValueOr("")
		if i.Post.URL.IsValid() {
			t = "url"
			body = i.Post.URL.ValueOr("")
		}

		createdAt := i.Post.Published
		//lastCommentedAt := i.Counts.NewestCommentTime.Time
		lastCommentedAt := i.Counts.Published

		models = append(models, post.Post{
			ID: strconv.FormatInt(i.Post.ID, 10),

			Subject: i.Post.Name,
			Body:    body,

			Type: t,

			Closed: i.Post.Locked,

			CreatedAt:       createdAt,
			LastCommentedAt: lastCommentedAt,

			Author: author.Author{
				ID:   strconv.FormatInt(i.Post.CreatorID, 10),
				Name: i.Creator.Name,
			},

			Forum: forum.Forum{
				ID:   strconv.FormatInt(i.Post.CommunityID, 10),
				Name: i.Community.Name,

				SysIDX: sys.ID,
			},

			// TODO: Implement chunks loading
			TotalReplies:           0,
			CurrentRepliesStartIDX: -1,

			URL: fmt.Sprintf("%s/post/%d", baseURL, i.Post.ID),

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	pid, err := strconv.ParseInt(p.ID, 10, 64)
	if err != nil {
		return err
	}
	// cid, err := strconv.Atoi(p.Forum.ID)
	// if err != nil {
	// 	return err
	// }

	resp, err := sys.client.Comments(context.Background(), lemmy.GetComments{
		PostID: lemmy.NewOptional[int64](pid),
	})
	if err != nil {
		return err
	}

	p.Replies = []reply.Reply{}
	for _, i := range resp.Comments {
		createdAt := i.Comment.Published

		p.Replies = append(p.Replies, reply.Reply{
			ID: strconv.FormatInt(i.Comment.ID, 10),

			InReplyTo: p.ID,

			Body: i.Comment.Content,

			CreatedAt: createdAt,

			Author: author.Author{
				ID:   strconv.FormatInt(i.Comment.CreatorID, 10),
				Name: i.Creator.Name,
			},

			SysIDX: sys.ID,
		})
	}
	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	communityID, err := strconv.ParseInt(p.Forum.ID, 10, 64)
	if err != nil {
		return err
	}

	resp, err := sys.client.CreatePost(context.Background(), lemmy.CreatePost{
		Name:        p.Subject,
		CommunityID: communityID,
		Body:        lemmy.NewOptional(p.Body),
		NSFW:        lemmy.NewOptional(false),
	})
	if err != nil {
		return err
	}

	p.ID = strconv.FormatInt(resp.PostView.Post.ID, 10)
	return nil
}

func (sys *System) CreateReply(r *reply.Reply) error {
	ID, err := strconv.ParseInt(r.ID, 10, 64)
	if err != nil {
		return err
	}

	var create lemmy.CreateComment
	if r.InReplyTo != "" {
		// Reply to a reply of a post
		InReplyTo, err := strconv.ParseInt(r.InReplyTo, 10, 64)
		if err != nil {
			return err
		}
		create = lemmy.CreateComment{
			PostID:   InReplyTo,
			ParentID: lemmy.NewOptional(ID),
			Content:  r.Body,
		}
	} else {
		// Reply to a post
		create = lemmy.CreateComment{
			PostID:  ID,
			Content: r.Body,
		}
	}

	resp, err := sys.client.CreateComment(context.Background(), create)
	if err != nil {
		return err
	}

	r.ID = strconv.FormatInt(resp.CommentView.Comment.ID, 10)
	return nil
}
