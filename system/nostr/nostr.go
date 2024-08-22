package nostr

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/system/adapter"
	"github.com/mrusme/neonmodem/system/nostr/nip51"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"go.uber.org/zap"
)

type System struct {
	ID         int
	config     map[string]interface{}
	logger     *zap.SugaredLogger
	client     *nostr.Relay
	clientCtx  context.Context
	clientCcl  context.CancelFunc
	relayURL   string
	privateKey string
	publicKey  string
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
		// TODO
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
		// TODO
		// adapter.Capability{
		// 	ID:   "create:post",
		// 	Name: "Create Post",
		// },
		adapter.Capability{
			ID:   "list:replies",
			Name: "List Replies",
		},
		// TODO
		// adapter.Capability{
		// 	ID:   "create:reply",
		// 	Name: "Create Reply",
		// },
	)

	return caps
}

func (sys *System) FilterValue() string {
	return fmt.Sprintf(
		"Nostr %s",
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
		"Nostr",
	)
}

func (sys *System) Load() error {
	var err error

	url := sys.config["url"]
	if url == nil {
		return nil
	}

	sys.relayURL = url.(string)

	credentials := make(map[string]string)
	for k, v := range (sys.config["credentials"]).(map[string]interface{}) {
		credentials[k] = v.(string)
	}

	_, pk, err := nip19.Decode(credentials["pk"])
	sys.privateKey = pk.(string)
	if sys.publicKey, err = nostr.GetPublicKey(sys.privateKey); err != nil {
		sys.logger.Debugln("Nostr GetPublicKey error")
		return err
	}

	sys.clientCtx, sys.clientCcl = context.WithCancel(context.Background())
	if sys.client, err = nostr.RelayConnect(sys.clientCtx, sys.relayURL); err != nil {
		sys.logger.Debugln("Nostr RelayConnect error")
		return err
	}

	return nil
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	var models []forum.Forum
	var sub *nostr.Subscription
	var err error

	var filters nostr.Filters
	filters = []nostr.Filter{{
		Kinds:   []int{nip51.Deprecated},
		Authors: []string{sys.publicKey},
		Limit:   1,
	}}

	sys.logger.Debugln("Nostr ListForums")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if sub, err = sys.client.Subscribe(ctx, filters); err != nil {
		return models, err
	}

	for ev := range sub.Events {
		sys.logger.Debugf("%v\n", ev)
		le := nip51.ParseListEvent(*ev, sys.privateKey, sys.publicKey)
		if le.Identifier == "communities" {
			for _, co := range le.Communities {
				models = append(models, forum.Forum{
					ID:   co.Identifier,
					Name: "n/" + co.Identifier,

					Info: co.PubKey,

					SysIDX: sys.ID,
				})

			}
		}
	}

	return models, nil
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	var models []post.Post
	// var err error
	// for _, i := range *items {
	// 	createdAt, err := dateparse.ParseAny(i.CreatedAt)
	// 	if err != nil {
	// 		createdAt = time.Now() // TODO: Errrr
	// 	}
	//
	// 	models = append(models, post.Post{
	// 		ID: i.ShortID,
	//
	// 		Subject: i.Title,
	//
	// 		Type: "url",
	//
	// 		Pinned: false,
	// 		Closed: false,
	//
	// 		CreatedAt:       createdAt,
	// 		LastCommentedAt: createdAt, // TODO
	//
	// 		Author: author.Author{
	// 			ID:   i.SubmitterUser.Username,
	// 			Name: i.SubmitterUser.Username,
	// 		},
	//
	// 		Forum: forum.Forum{
	// 			ID:   i.Tags[0],
	// 			Name: i.Tags[0], // TODO: Tag description
	//
	// 			SysIDX: sys.ID,
	// 		},
	//
	// 		// TODO: Implement chunks loading
	// 		TotalReplies:           0,
	// 		CurrentRepliesStartIDX: -1,
	//
	// 		URL: i.ShortIDURL,
	//
	// 		SysIDX: sys.ID,
	// 	})
	// }
	//
	return models, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	// item, err := sys.client.Stories.Show(context.Background(), p.ID)
	// if err != nil {
	// 	return err
	// }
	//
	// converter := md.NewConverter("", true, nil)
	//
	p.Replies = []reply.Reply{}
	// for idx, i := range item.Comments {
	// 	cookedMd, err := converter.ConvertString(i.Comment)
	// 	if err != nil {
	// 		cookedMd = i.CommentPlain
	// 	}
	//
	// 	if idx == 0 {
	// 		p.Body = cookedMd
	// 		continue
	// 	}
	//
	// 	createdAt, err := dateparse.ParseAny(i.CreatedAt)
	// 	if err != nil {
	// 		createdAt = time.Now() // TODO: Errrrrr
	// 	}
	//
	// 	inReplyTo := i.ParentComment
	// 	if inReplyTo == "" {
	// 		inReplyTo = p.ID
	// 	}
	// 	p.Replies = append(p.Replies, reply.Reply{
	// 		ID:        i.ShortID,
	// 		InReplyTo: inReplyTo,
	//
	// 		Body: cookedMd,
	//
	// 		CreatedAt: createdAt,
	//
	// 		Author: author.Author{
	// 			ID:   i.CommentingUser.Username,
	// 			Name: i.CommentingUser.Username,
	// 		},
	//
	// 		SysIDX: sys.ID,
	// 	})
	// }

	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	return nil
}

func (sys *System) CreateReply(r *reply.Reply) error {
	return nil
}
