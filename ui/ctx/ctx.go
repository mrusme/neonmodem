package ctx

import (
	"embed"

	"github.com/mrusme/neonmodem/config"
	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/system"
	"github.com/mrusme/neonmodem/ui/theme"
	"go.uber.org/zap"
)

type Ctx struct {
	Screen  [2]int
	Content [2]int
	Config  *config.Config
	EmbedFS *embed.FS
	Systems []*system.System
	Loading bool
	Logger  *zap.SugaredLogger
	Theme   *theme.Theme

	currentSystem int
	currentForum  forum.Forum
}

func New(
	efs *embed.FS,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) Ctx {
	return Ctx{
		Screen:  [2]int{0, 0},
		Content: [2]int{0, 0},
		Config:  cfg,
		EmbedFS: efs,
		Loading: false,
		Logger:  logger,
		Theme:   theme.New(cfg),

		currentSystem: -1,
		currentForum:  forum.Forum{},
	}
}

func (c *Ctx) AddSystem(sys *system.System) error {
	c.Systems = append(c.Systems, sys)
	return nil
}

func (c *Ctx) NumSystems() int {
	return len(c.Systems)
}

func (c *Ctx) SetCurrentSystem(idx int) {
	c.currentSystem = idx
}

func (c *Ctx) GetCurrentSystem() int {
	return c.currentSystem
}

func (c *Ctx) SetCurrentForum(f forum.Forum) {
	c.currentForum = f
}

func (c *Ctx) GetCurrentForum() forum.Forum {
	return c.currentForum
}
