package lemmy

import (
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/system/adapter"
)

type System struct {
	config map[string]interface{}
}

func (sys *System) GetConfig() map[string]interface{} {
	return sys.config
}

func (sys *System) SetConfig(cfg *map[string]interface{}) {
	sys.config = *cfg
}

func (sys *System) Load() error {
	return nil
}

func (sys *System) ListPosts() ([]post.Post, error) {
	return []post.Post{}, nil
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
