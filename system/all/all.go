package all

import (
	"errors"
	"fmt"

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
}

func (sys *System) SetLogger(logger *zap.SugaredLogger) {
	sys.logger = logger
}

func (sys *System) GetCapabilities() adapter.Capabilities {
	var caps []adapter.Capability

	return caps
}

func (sys *System) FilterValue() string {
	return fmt.Sprintf(
		"All",
	)
}

func (sys *System) Title() string {
	return "All"
}

func (sys *System) Description() string {
	return fmt.Sprintf(
		"Aggregate all systems",
	)
}

func (sys *System) Load() error {
	return nil
}

func (sys *System) Connect(sysURL string) error {
	return errors.New("This system can't be connected to")
}

func (sys *System) ListForums() ([]forum.Forum, error) {
	return []forum.Forum{}, nil
}

func (sys *System) ListPosts(forumID string) ([]post.Post, error) {
	return []post.Post{}, nil
}

func (sys *System) LoadPost(p *post.Post) error {
	return nil
}

func (sys *System) CreatePost(p *post.Post) error {
	return nil
}

func (sys *System) CreateReply(r *reply.Reply) error {
	return nil
}
