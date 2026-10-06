package ctx

import (
	"embed"
	"log/slog"
	"sync/atomic"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/theme"
)

type Ctx struct {
	Screen  [2]int
	Content [2]int
	Config  *config.Config
	EmbedFS *embed.FS
	Systems []system.System

	StartupErrors  []error
	StartupNotices []string

	Loading  bool
	Progress string
	Logger   *slog.Logger
	Theme    *theme.Theme

	DarkBackground bool

	currentSystem int
	currentForum  forum.Forum
	currentOrder  system.Order

	loadGen *atomic.Int64
}

func New(
	efs *embed.FS,
	cfg *config.Config,
	logger *slog.Logger,
	systems []system.System,
) Ctx {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return Ctx{
		Screen:  [2]int{0, 0},
		Content: [2]int{0, 0},
		Config:  cfg,
		EmbedFS: efs,
		Systems: systems,
		Logger:  logger,
		Theme:   theme.New(&cfg.Theme, true),

		DarkBackground: true,

		currentSystem: -1,
		currentOrder:  system.OrderNew,

		loadGen: new(atomic.Int64),
	}
}

func (c *Ctx) SetDarkBackground(dark bool) bool {
	if c.DarkBackground == dark {
		return false
	}
	c.DarkBackground = dark
	c.Theme = theme.New(&c.Config.Theme, dark)
	return true
}

func (c *Ctx) NextLoadGen() int64 {
	return c.loadGen.Add(1)
}

func (c *Ctx) IsCurrentLoadGen(gen int64) bool {
	return c.loadGen.Load() == gen
}

func (c *Ctx) CancelLoad() {
	c.loadGen.Add(1)
}

func (c *Ctx) AddSystem(sys system.System) {
	c.Systems = append(c.Systems, sys)
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

func (c *Ctx) SetOrder(order system.Order) {
	c.currentOrder = order
}

func (c *Ctx) GetOrder() system.Order {
	return c.currentOrder
}
