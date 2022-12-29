package system

import (
	"errors"

	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/system/adapter"
	"github.com/mrusme/gobbs/system/discourse"
	"github.com/mrusme/gobbs/system/lemmy"
)

type System interface {
	GetConfig() map[string]interface{}
	SetConfig(cfg *map[string]interface{})
	GetCapabilities() []adapter.Capability

	Load() error

	ListPosts() ([]post.Post, error)
}

func New(sysType string, sysConfig *map[string]interface{}) (System, error) {
	var sys System

	switch sysType {
	case "discourse":
		sys = new(discourse.System)
	case "lemmy":
		sys = new(lemmy.System)
	default:
		return nil, errors.New("No such system")
	}

	sys.SetConfig(sysConfig)
	err := sys.Load()
	if err != nil {
		return nil, err
	}

	return sys, nil
}
