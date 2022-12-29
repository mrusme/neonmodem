package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/config"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var sugar *zap.SugaredLogger

func init() {
	cobra.OnInitialize(load)
	rootCmd.
		PersistentFlags().
		Bool(
			"debug",
			false,
			"Debug output",
		)
	viper.BindPFlag(
		"debug",
		rootCmd.PersistentFlags().Lookup("debug"),
	)
}

func load() {
	var logger *zap.Logger

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

var rootCmd = &cobra.Command{
	Use:        "gobbs",
	SuggestFor: []string{"bbs", "discourse", "lemmy"},
	Short:      "Gobbs, the bulletin board system TUI",
	Long: "Gobbs is a bulletin board system (BBS) text user interface written " +
		"in Go, supporting Discourse and Lemmy.\n" +
		"More info available on https://xn--gckvb8fzb.com/projects/gobbs",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		sugar.Errorln(err)
	}
}
