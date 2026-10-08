package ctx

import (
	"context"
	"embed"
	"log/slog"
	"sync"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/feed"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/images"
	"github.com/mrusme/neonmodem/internal/ui/theme"
)

type Launcher interface {
	Start(name string, line string, env []string) error
}

type LoadSource string

const (
	LoadFeed   LoadSource = "feed"
	LoadPost   LoadSource = "post"
	LoadSubmit LoadSource = "submit"
	LoadForums LoadSource = "forums"
)

type loadState struct {
	mu     sync.Mutex
	gen    int64
	cancel context.CancelFunc
}

type Ctx struct {
	Screen  [2]int
	Content [2]int
	Config  *config.Config
	EmbedFS *embed.FS
	Systems []system.System

	StartupErrors  []error
	StartupNotices []string

	OpenWith []config.OpenWith
	Launcher Launcher

	Progress string
	Logger   *slog.Logger
	Theme    *theme.Theme
	Images   *images.Renderer
	Feed     *feed.Feed

	DarkBackground bool

	currentSystem int
	currentForum  forum.Forum
	currentOrder  system.Order

	loading map[LoadSource]struct{}
	load    *loadState
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
		Images:  images.New(logger, cfg.Proxy),
		Feed:    feed.New(systems, cfg.ReadDeadline(), cfg.WriteDeadline()),

		DarkBackground: true,

		currentSystem: -1,
		currentOrder:  system.OrderNew,

		loading: map[LoadSource]struct{}{},
		load:    &loadState{},
	}
}

func (c *Ctx) StartLoading(source LoadSource) {
	c.loading[source] = struct{}{}
}

func (c *Ctx) StopLoading(source LoadSource) {
	delete(c.loading, source)
}

func (c *Ctx) IsLoading() bool {
	return len(c.loading) > 0
}

func (c *Ctx) SetDarkBackground(dark bool) bool {
	if c.DarkBackground == dark {
		return false
	}
	c.DarkBackground = dark
	c.Theme = theme.New(&c.Config.Theme, dark)
	return true
}

func (c *Ctx) NextLoad() (context.Context, int64) {
	c.load.mu.Lock()
	defer c.load.mu.Unlock()

	if c.load.cancel != nil {
		c.load.cancel()
	}
	loadCtx, cancel := context.WithCancel(context.Background())
	c.load.cancel = cancel
	c.load.gen++

	return loadCtx, c.load.gen
}

func (c *Ctx) IsCurrentLoadGen(gen int64) bool {
	c.load.mu.Lock()
	defer c.load.mu.Unlock()
	return c.load.gen == gen
}

func (c *Ctx) CancelLoad() {
	c.load.mu.Lock()
	defer c.load.mu.Unlock()

	if c.load.cancel != nil {
		c.load.cancel()
		c.load.cancel = nil
	}
	c.load.gen++
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
