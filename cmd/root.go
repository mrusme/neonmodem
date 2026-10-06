package cmd

import (
	"embed"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/logging"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/registry"
	"github.com/mrusme/neonmodem/internal/ui"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/spf13/cobra"
)

type app struct {
	embedFS  *embed.FS
	cfg      *config.Config
	logger   *slog.Logger
	closeLog io.Closer
}

func version() string {
	if config.VERSION != "" {
		return config.VERSION
	}

	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}

	return "unknown"
}

func newRootCmd(a *app) *cobra.Command {
	var debugFlag bool

	cmd := &cobra.Command{
		Use:        "neonmodem",
		SuggestFor: []string{"bbs", "discourse", "lemmy", "lobsters", "hackernews"},
		Short:      "neonmodem, the bulletin board system TUI",
		Long: "neonmodem is a bulletin board system (BBS) text user interface written " +
			"in Go, supporting " + strings.Join(registry.Kinds(), ", ") + ".\n" +
			"More info available on https://xn--gckvb8fzb.com/projects/neonmodem",
		Version:       version(),
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading the configuration: %w", err)
			}
			if debugFlag {
				cfg.Debug = true
			}

			logger, closer, err := logging.Open(cfg.Log, cfg.Debug)
			if err != nil {
				return fmt.Errorf("opening the log file %s: %w", cfg.Log, err)
			}

			a.cfg = cfg
			a.logger = logger
			a.closeLog = closer

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if a.closeLog != nil {
				return a.closeLog.Close()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runTUI()
		},
	}
	cmd.SetVersionTemplate("neonmodem {{.Version}}\n")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Debug output")

	cmd.AddCommand(newConnectCmd(a))

	return cmd
}

func (a *app) loadSystems() ([]system.System, []error) {
	var systems []system.System
	var errs []error

	for i, sysCfg := range a.cfg.Systems {
		sys, err := registry.New(sysCfg.Type, system.Env{
			Index:    len(systems),
			Settings: sysCfg.Settings,
			Proxy:    a.cfg.Proxy,
			Logger:   a.logger.With("system", sysCfg.Type),
		})
		if err != nil {
			a.logger.Error("system unavailable", "index", i, "type", sysCfg.Type, "error", err)
			errs = append(errs, fmt.Errorf("%s (%s): %w", sysCfg.Type, sysCfg.Settings.URL, err))
			continue
		}

		a.logger.Debug("loaded system", "index", len(systems), "type", sysCfg.Type)
		systems = append(systems, sys)
	}

	return systems, errs
}

func (a *app) runTUI() error {
	systems, errs := a.loadSystems()

	c := ctx.New(a.embedFS, a.cfg, a.logger, systems)
	c.StartupErrors = errs

	program := tea.NewProgram(ui.NewModel(&c))
	if _, err := program.Run(); err != nil {
		return err
	}

	return nil
}

func Execute(efs *embed.FS) error {
	a := &app{embedFS: efs}
	return newRootCmd(a).Execute()
}
