package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/config"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var LOG *zap.SugaredLogger
var CFG config.Config

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
	var err error
	var logger *zap.Logger

	CFG, err = config.Load()
	if err != nil {
		panic(err)
	}

	if CFG.Debug == "true" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()
	LOG = logger.Sugar()
}

func loadSystems(c *ctx.Ctx) []error {
	var errs []error

	for _, sysCfg := range c.Config.Systems {
		sys, err := system.New(sysCfg.Type, &sysCfg.Config, LOG)
		if err != nil {
			c.Logger.Errorf("error loading system: %s", err)
			errs = append(errs, err)
		} else {
			c.Logger.Debugln("loaded system")
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
	Run: func(cmd *cobra.Command, args []string) {
		c := ctx.New(&CFG, LOG)
		_ = loadSystems(&c) // TODO: Handle errs

		posts, err := (*c.Systems[0]).ListPosts()
		fmt.Printf("%v\n", posts)
		fmt.Printf("%s\n", err)
		os.Exit(0)

		tui := tea.NewProgram(ui.NewModel(&c), tea.WithAltScreen())
		err = tui.Start()
		if err != nil {
			panic(err)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// LOG.Errorln(err)
	}
}
