package hackernews

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	hn "github.com/hermanschaaf/hackernews"
	"github.com/mrusme/neonmodem/models/author"
	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/system/adapter"
	"go.uber.org/zap"
)

type System struct {
	ID     int
	config map[string]interface{}
	logger *zap.SugaredLogger
	client *hn.Client
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
		// TODO: https://github.com/hermanschaaf/hackernews/issues/1
		// adapter.Capability{
		// 	ID:   "create:post",
		// 	Name: "Create Post",
		// },
		adapter.Capability{
			ID:   "list:replies",
			Name: "List Replies",
		},
		// TODO: https://github.com/hermanschaaf/hackernews/issues/1
		// adapter.Capability{
		// 	ID:   "create:reply",
		// 	Name: "Create Reply",
		// },
	)

	return caps
}

func (sys *System) FilterValue() string {
	return fmt.Sprintf(
		"Hacker News https://news.ycombinator.com",
	)
}

func (sys *System) Title() string {
	return "news.ycombinator.com"
}

func (sys *System) Description() string {
	return fmt.Sprintf(
		"Hacker News",
	)
}

func (sys *System) Load() error {
	var httpClient *http.Client = nil
	var httpTransport *http.Transport = http.DefaultTransport.(*http.Transport).
		Clone()

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

	sys.client = hn.NewClient(hn.WithHTTPClient(httpClient))
	return nil
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	return []forum.Forum{
		{
			ID:     "top",
			Name:   "Top HN Stories",
			Info:   "Top stories on Hacker News",
			SysIDX: sys.ID,
		},
		{
			ID:     "best",
			Name:   "Best HN Stories",
			Info:   "Best stories on Hacker News",
			SysIDX: sys.ID,
		},
		{
			ID:     "new",
			Name:   "New HN Stories",
			Info:   "New stories on Hacker News",
			SysIDX: sys.ID,
		},
		{
			ID:     "ask",
			Name:   "Ask HN",
			Info:   "Ask Hacker News about the world",
			SysIDX: sys.ID,
		},
		{
			ID:     "show",
			Name:   "Show HN",
			Info:   "Show Hacker News something awesome",
			SysIDX: sys.ID,
		},
		{
			ID:     "jobs",
			Name:   "Jobs HN",
			Info:   "... because we can't *all* become Astronauts",
			SysIDX: sys.ID,
		},
	}, nil
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	var stories []int
	var err error

	switch forumID {
	case "top":
		stories, err = sys.client.TopStories(context.Background())
	case "best":
		stories, err = sys.client.BestStories(context.Background())
	case "ask":
		stories, err = sys.client.AskStories(context.Background())
	case "show":
		stories, err = sys.client.ShowStories(context.Background())
	case "jobs":
		stories, err = sys.client.JobStories(context.Background())
	default:
		stories, err = sys.client.NewStories(context.Background())
	}
	if err != nil {
		return []post.Post{}, err
	}

	converter := md.NewConverter("", true, nil)

	var models []post.Post
	for _, story := range stories[0:10] {
		i, err := sys.client.GetItem(context.Background(), story)
		if err != nil {
			sys.logger.Error(err)
			// TODO: Handle error
			continue
		}

		t := "post"
		body := i.Text
		if i.URL != "" {
			t = "url"
			body = i.URL
		} else {
			bodyMd, err := converter.ConvertString(i.Text)
			if err == nil {
				body = bodyMd
			}
		}

		createdAt := time.Unix(int64(i.Time), 0)
		lastCommentedAt := createdAt

		var replies []reply.Reply
		for _, commentID := range i.Kids {
			replies = append(replies, reply.Reply{
				ID: strconv.Itoa(commentID),
			})
		}

		models = append(models, post.Post{
			ID: strconv.Itoa(i.ID),

			Subject: i.Title,
			Body:    body,

			Type: t,

			Pinned: false,
			Closed: i.Deleted,

			CreatedAt:       createdAt,
			LastCommentedAt: lastCommentedAt,

			Author: author.Author{
				ID:   i.By,
				Name: i.By,
			},

			Forum: forum.Forum{
				ID:   "new",
				Name: "New",

				SysIDX: sys.ID,
			},

			// TODO: Implement chunks loading
			TotalReplies:           0,
			CurrentRepliesStartIDX: -1,
			Replies:                replies,

			URL: fmt.Sprintf("https://news.ycombinator.com/item?id=%d", i.ID),

			SysIDX: sys.ID,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	p.Replies = []reply.Reply{}
	return sys.loadReplies(&p.Replies)
}

func (sys *System) loadReplies(replies *[]reply.Reply) error {
	converter := md.NewConverter("", true, nil)
	for r := 0; r < len(*replies); r++ {
		re := &(*replies)[r]

		id, err := strconv.Atoi(re.ID)
		if err != nil {
			sys.logger.Error(err)
			continue
		}

		i, err := sys.client.GetItem(context.Background(), id)
		if err != nil {
			sys.logger.Error(err)
			continue
		}

		if i.Deleted || i.Dead {
			re.Deleted = true
		}

		createdAt := time.Unix(int64(i.Time), 0)

		re.Body = i.Text
		bodyMd, err := converter.ConvertString(i.Text)
		if err == nil {
			re.Body = bodyMd
		}

		re.CreatedAt = createdAt

		re.Author = author.Author{
			ID:   i.By,
			Name: i.By,
		}

		re.SysIDX = sys.ID

		for _, commentID := range i.Kids {
			re.Replies = append(re.Replies, reply.Reply{
				ID: strconv.Itoa(commentID),
			})
		}

		if err := sys.loadReplies(&re.Replies); err != nil {
			sys.logger.Error(err)
		}
	}

	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	return errors.New("Sorry, this feature isn't available yet for Hacker News!")
}

func (sys *System) CreateReply(r *reply.Reply) error {
	return errors.New("Sorry, this feature isn't available yet for Hacker News!")
}
