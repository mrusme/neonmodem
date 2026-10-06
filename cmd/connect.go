package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/registry"
	"github.com/spf13/cobra"
)

func validateSysURL(sysURL string) error {
	if sysURL == "" {
		return nil
	}

	parsed, err := url.Parse(sysURL)
	if err != nil {
		return fmt.Errorf("%s doesn't look like a valid URL: %w", sysURL, err)
	}

	if parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf(
			"%s doesn't look like a valid URL, please provide a full address "+
				"including the scheme, e.g. https://example.com",
			sysURL,
		)
	}

	return nil
}

func newConnectCmd(a *app) *cobra.Command {
	var sysType string
	var sysURL string

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to BBS",
		Long:  "Add a new connection to a BBS.",
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := strings.ToLower(strings.TrimSpace(sysType))
			desc, err := registry.Get(kind)
			if err != nil {
				return err
			}

			sysURL = strings.TrimRight(strings.TrimSpace(sysURL), "/")
			if desc.NeedsURL && sysURL == "" {
				return fmt.Errorf("--url is required for %s", desc.Name)
			}
			if err := validateSysURL(sysURL); err != nil {
				return err
			}

			for _, existing := range a.cfg.Systems {
				if existing.Type != kind {
					continue
				}
				if !desc.AllowMultiple {
					return fmt.Errorf("%s is already connected", desc.Name)
				}
				if sysURL != "" && strings.EqualFold(
					strings.TrimRight(existing.Settings.URL, "/"), sysURL) {
					return fmt.Errorf("%s is already connected", sysURL)
				}
			}

			sys, err := desc.New(system.Env{
				Index:  len(a.cfg.Systems),
				Proxy:  a.cfg.Proxy,
				Logger: a.logger.With("system", kind),
			})
			if err != nil {
				return err
			}

			settings, err := sys.Connect(cmd.Context(), prompt.Stdio(commandRunner()), sysURL)
			if err != nil {
				a.logger.Error("connect failed", "type", kind, "error", err)
				return err
			}

			a.cfg.Systems = append(a.cfg.Systems, config.SystemConfig{
				Type:     kind,
				Settings: settings,
			})
			if err := a.cfg.Save(); err != nil {
				return fmt.Errorf("saving the configuration: %w", err)
			}

			fmt.Printf("Successfully added new connection! Configuration saved to %s\n",
				a.cfg.Path())
			return nil
		},
	}

	cmd.Flags().StringVar(&sysType, "type", "",
		"Type of system to connect to ("+strings.Join(registry.Kinds(), ", ")+")")
	cmd.Flags().StringVar(&sysURL, "url", "",
		"URL of system (e.g. https://www.keebtalk.com)")
	cmd.MarkFlagRequired("type")

	return cmd
}
