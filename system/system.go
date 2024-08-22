package system

import (
	"errors"

	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/system/adapter"
	"github.com/mrusme/neonmodem/system/all"
	"github.com/mrusme/neonmodem/system/discourse"
	"github.com/mrusme/neonmodem/system/hackernews"
	"github.com/mrusme/neonmodem/system/lemmy"
	"github.com/mrusme/neonmodem/system/lobsters"
	"github.com/mrusme/neonmodem/system/nostr"
	"go.uber.org/zap"
)

type System interface {
	SetID(id int)
	GetID() int
	GetConfig() map[string]interface{}
	SetConfig(cfg *map[string]interface{})
	SetLogger(logger *zap.SugaredLogger)
	GetCapabilities() adapter.Capabilities

	FilterValue() string
	Title() string
	Description() string

	Connect(sysURL string) error
	Load() error

	ListForums() ([]forum.Forum, error)
	ListPosts(forumID string) ([]post.Post, error)
	LoadPost(p *post.Post) error
	CreatePost(p *post.Post) error
	CreateReply(r *reply.Reply) error
}

func New(
	sysType string,
	sysConfig *map[string]interface{},
	logger *zap.SugaredLogger,
) (System, error) {
	var sys System

	switch sysType {
	case "discourse":
		sys = new(discourse.System)
	case "lemmy":
		sys = new(lemmy.System)
	case "lobsters":
		sys = new(lobsters.System)
	case "hackernews":
		sys = new(hackernews.System)
	case "nostr":
		sys = new(nostr.System)
	case "all":
		sys = new(all.System)
	default:
		return nil, errors.New("No such system")
	}

	sys.SetConfig(sysConfig)
	sys.SetLogger(logger)
	err := sys.Load()
	if err != nil {
		return nil, err
	}

	return sys, nil
}
