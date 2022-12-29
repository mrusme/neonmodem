package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/config"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui"
	"github.com/mrusme/gobbs/ui/ctx"
	"go.uber.org/zap"
)

func main() {
	var logger *zap.Logger
	var sugar *zap.SugaredLogger

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	if cfg.Debug == "true" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()
	sugar = logger.Sugar()

	c := ctx.New(&cfg, sugar)
	_ = loadSystems(&c) // TODO: Handle errs

	tui := tea.NewProgram(ui.NewModel(&c), tea.WithAltScreen())
	err = tui.Start()
	if err != nil {
		panic(err)
	}
}

func loadSystems(c *ctx.Ctx) []error {
	var errs []error

	for _, sysCfg := range c.Config.Systems {
		sys, err := system.New(sysCfg.Type, &sysCfg.Config)
		if err != nil {
			c.Logger.Errorf("error loading system: %s", err)
			errs = append(errs, err)
		}

		c.AddSystem(&sys)
	}

	return errs
}
