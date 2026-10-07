package cmd

import (
	"context"
	"fmt"
	"io"
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
		Long:  "Add a new connection to a BBS, or replace an existing one.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.connect(cmd.Context(), prompt.Stdio(commandRunner()), cmd.OutOrStdout(),
				sysType, sysURL)
		},
	}

	cmd.Flags().StringVar(&sysType, "type", "",
		"Type of system to connect to ("+strings.Join(registry.Kinds(), ", ")+")")
	cmd.Flags().StringVar(&sysURL, "url", "",
		"URL of system (e.g. https://www.keebtalk.com)")
	cmd.MarkFlagRequired("type")

	return cmd
}

func (a *app) connect(
	ctx context.Context,
	p prompt.Prompter,
	out io.Writer,
	sysType string,
	sysURL string,
) error {
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

	replace := existingConnection(a.cfg.Systems, desc, sysURL)
	if replace >= 0 {
		name := sysURL
		if !desc.AllowMultiple || name == "" {
			name = desc.Name
		}
		p.Notice(name + " is already connected.")
		choice, err := p.Choose("What do you want to do?",
			[]string{"Keep the existing connection", "Replace it"})
		if err != nil {
			return err
		}
		if choice == 0 {
			fmt.Fprintln(out, "Nothing changed.")
			return nil
		}
	}

	index := len(a.cfg.Systems)
	if replace >= 0 {
		index = replace
	}
	sys, err := desc.New(system.Env{
		Index:  index,
		Proxy:  a.cfg.Proxy,
		Logger: a.logger.With("system", kind),
	})
	if err != nil {
		return err
	}

	settings, err := sys.Connect(ctx, p, sysURL)
	if err != nil {
		a.logger.Error("connect failed", "type", kind, "error", err)
		return err
	}

	entry := config.SystemConfig{Type: kind, Settings: settings}
	if replace >= 0 {
		a.cfg.Systems[replace] = entry
	} else {
		a.cfg.Systems = append(a.cfg.Systems, entry)
	}
	if err := a.cfg.Save(); err != nil {
		return fmt.Errorf("saving the configuration: %w", err)
	}

	if replace >= 0 {
		fmt.Fprintf(out, "Successfully replaced the connection! Configuration saved to %s\n",
			a.cfg.Path())
	} else {
		fmt.Fprintf(out, "Successfully added new connection! Configuration saved to %s\n",
			a.cfg.Path())
	}
	return nil
}

func existingConnection(systems []config.SystemConfig, desc registry.Descriptor, sysURL string) int {
	for i, existing := range systems {
		if strings.ToLower(strings.TrimSpace(existing.Type)) != desc.Kind {
			continue
		}
		if !desc.AllowMultiple {
			return i
		}
		if sysURL != "" && strings.EqualFold(
			strings.TrimRight(existing.Settings.URL, "/"), sysURL) {
			return i
		}
	}
	return -1
}
