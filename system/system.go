package system

import (
	"errors"

	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/system/adapter"
	"github.com/mrusme/gobbs/system/discourse"
	"github.com/mrusme/gobbs/system/hackernews"
	"github.com/mrusme/gobbs/system/lemmy"
	"go.uber.org/zap"
)

type System interface {
	GetConfig() map[string]interface{}
	SetConfig(cfg *map[string]interface{})
	SetLogger(logger *zap.SugaredLogger)
	GetCapabilities() []adapter.Capability

	Connect(sysURL string) error
	Load() error

	ListPosts(sysIdx int) ([]post.Post, error)
	LoadPost(p *post.Post) error
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
	case "hackernews":
		sys = new(hackernews.System)
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
