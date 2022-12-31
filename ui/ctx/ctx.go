package ctx

import (
	"github.com/mrusme/gobbs/config"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui/theme"
	"go.uber.org/zap"
)

type Ctx struct {
	Screen  [2]int
	Content [2]int
	Config  *config.Config
	Systems []*system.System
	Loading bool
	Logger  *zap.SugaredLogger
	Theme   *theme.Theme
}

func New(
	cfg *config.Config,
	logger *zap.SugaredLogger,
) Ctx {
	return Ctx{
		Screen:  [2]int{0, 0},
		Content: [2]int{0, 0},
		Config:  cfg,
		Loading: false,
		Logger:  logger,
		Theme:   theme.New(cfg),
	}
}

func (c *Ctx) AddSystem(sys *system.System) error {
	c.Systems = append(c.Systems, sys)
	return nil
}
