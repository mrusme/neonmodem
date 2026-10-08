package cmd

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/logging"
	"github.com/mrusme/neonmodem/internal/openwith"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/credential"
	"github.com/mrusme/neonmodem/internal/system/registry"
	"github.com/mrusme/neonmodem/internal/ui"
	uictx "github.com/mrusme/neonmodem/internal/ui/ctx"
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
			for _, notice := range cfg.Notices() {
				logger.Warn(notice)
			}
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
			return a.runTUI(cmd.Context())
		},
	}
	cmd.SetVersionTemplate("neonmodem {{.Version}}\n")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Debug output")

	cmd.AddCommand(newConnectCmd(a))

	return cmd
}

func commandRunner() credential.Shell {
	return credential.Shell{
		Stdin:  os.Stdin,
		Stderr: os.Stderr,
	}
}

func (a *app) loadSystems(
	ctx context.Context,
	resolver credential.Resolver,
) ([]system.System, []error) {
	var systems []system.System
	var errs []error

	for i, sysCfg := range a.cfg.Systems {
		sys, err := a.loadSystem(ctx, resolver, len(systems), sysCfg)
		if err != nil {
			a.logger.Error("system unavailable", "index", i, "type", sysCfg.Type,
				"url", sysCfg.Settings.URL, "error", err)
			errs = append(errs, fmt.Errorf("%s (%s): %w", sysCfg.Type, sysCfg.Settings.URL, err))
			continue
		}

		a.logger.Debug("loaded system", "index", len(systems), "type", sysCfg.Type)
		systems = append(systems, sys)
	}

	return systems, errs
}

func (a *app) loadSystem(
	ctx context.Context,
	resolver credential.Resolver,
	index int,
	sysCfg config.SystemConfig,
) (system.System, error) {
	desc, err := registry.Get(sysCfg.Type)
	if err != nil {
		return nil, err
	}

	logger := a.logger.With("system", sysCfg.Type)
	credentials, err := resolver.Resolve(ctx, logger, sysCfg.Settings.Credentials, desc.Credentials)
	if err != nil {
		return nil, err
	}

	settings := sysCfg.Settings
	settings.Credentials = credentials

	return desc.New(system.Env{
		Index:       index,
		Settings:    settings,
		Proxy:       a.cfg.Proxy,
		Logger:      logger,
		ReadTimeout: a.cfg.ReadDeadline(),
	})
}

func (a *app) startOrder() (system.Order, string) {
	if strings.TrimSpace(a.cfg.Sort) == "" {
		return system.OrderNew, ""
	}

	order, err := system.ParseOrder(a.cfg.Sort)
	if err != nil {
		a.logger.Warn("ignoring the Sort setting", "error", err)
		return system.OrderNew, fmt.Sprintf(
			"The Sort setting %q isn't a known order; using New", a.cfg.Sort)
	}
	return order, ""
}

func (a *app) openWithCommands() ([]config.OpenWith, []string) {
	commands, notices := a.cfg.ValidOpenWith()
	for _, notice := range notices {
		a.logger.Warn(notice)
	}
	if len(commands) == 0 {
		return nil, notices
	}

	if err := a.cfg.CommandsAllowed(); err != nil {
		a.logger.Warn("open with commands are turned off", "error", err)
		return nil, append(notices, fmt.Sprintf("Open with commands are turned off: %v", err))
	}
	return commands, notices
}

func (a *app) runTUI(ctx context.Context) error {
	systems, errs := a.loadSystems(ctx, credential.Resolver{
		Runner: commandRunner(),
		Trust:  a.cfg.CommandsAllowed,
	})

	c := uictx.New(a.embedFS, a.cfg, a.logger, systems)
	c.StartupErrors = errs

	order, notice := a.startOrder()
	c.SetOrder(order)
	if notice != "" {
		c.StartupNotices = append(c.StartupNotices, notice)
	}
	c.StartupNotices = append(c.StartupNotices, a.cfg.Notices()...)

	commands, notices := a.openWithCommands()
	c.OpenWith = commands
	c.StartupNotices = append(c.StartupNotices, notices...)

	runner := openwith.NewRunner(a.logger)
	defer runner.Stop()
	c.Launcher = runner

	program := tea.NewProgram(ui.NewModel(&c))
	if _, err := program.Run(); err != nil && !errors.Is(err, tea.ErrInterrupted) {
		return err
	}

	return nil
}

func Execute(efs *embed.FS) error {
	a := &app{embedFS: efs}
	return newRootCmd(a).Execute()
}
