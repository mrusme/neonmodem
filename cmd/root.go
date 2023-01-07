package cmd

import (
	"embed"
	"net/url"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/config"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var EMBEDFS *embed.FS
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

func loadLogger(filename string, debug bool) (*zap.Logger, error) {
	if runtime.GOOS == "windows" {
		zap.RegisterSink("winfile", func(u *url.URL) (zap.Sink, error) {
			return os.OpenFile(u.Path[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		})
	}

	var cfg zap.Config
	if debug {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	if runtime.GOOS == "windows" {
		cfg.OutputPaths = []string{
			"stdout",
			"winfile:///" + filename,
		}
	} else {
		cfg.OutputPaths = []string{
			filename,
		}
	}

	return cfg.Build()
}

func load() {
	var err error
	var logger *zap.Logger

	CFG, err = config.Load()
	if err != nil {
		panic(err)
	}

	logger, err = loadLogger(CFG.Log, CFG.Debug)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	LOG = logger.Sugar()
}

func loadSystems(c *ctx.Ctx) []error {
	var errs []error

	for _, sysCfg := range c.Config.Systems {
		c.Logger.Debugf("loading system of type %s ...", sysCfg.Type)
		sys, err := system.New(sysCfg.Type, &sysCfg.Config, LOG)
		if err != nil {
			c.Logger.Errorf("error loading system: %s", err)
			errs = append(errs, err)
		} else {
			c.Logger.Debugf("loaded %s system", sysCfg.Type)
		}

		c.AddSystem(&sys)
		c.Logger.Debugf("setting system ID to %d", c.NumSystems()-1)
		sys.SetID(c.NumSystems() - 1)
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
		var err error

		c := ctx.New(EMBEDFS, &CFG, LOG)
		_ = loadSystems(&c) // TODO: Handle errs

		tui := tea.NewProgram(ui.NewModel(&c), tea.WithAltScreen())
		err = tui.Start()
		if err != nil {
			panic(err)
		}
	},
}

func Execute(efs *embed.FS) {
	EMBEDFS = efs
	if err := rootCmd.Execute(); err != nil {
		// LOG.Errorln(err)
	}
}
