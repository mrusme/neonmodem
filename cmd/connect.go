package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/credential"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/registry"
	"github.com/mrusme/neonmodem/internal/system/text"
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
	var previous config.SystemConfig
	if replace >= 0 {
		previous = a.cfg.Systems[replace]
		keep, err := askToKeep(p, desc, sysURL)
		if err != nil {
			return err
		}
		if keep {
			fmt.Fprintln(out, "Nothing changed.")
			return nil
		}
	}

	index := len(a.cfg.Systems)
	if replace >= 0 {
		index = replace
	}
	sys, err := desc.New(system.Env{
		Index:       index,
		Proxy:       a.cfg.Proxy,
		Logger:      a.logger.With("system", kind),
		ReadTimeout: a.cfg.ReadDeadline(),
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
	if err := a.save(out, entry, replace); err != nil {
		return err
	}
	if replace >= 0 {
		a.afterReplace(ctx, out, sys, entry, previous)
	}
	return nil
}

func (a *app) afterReplace(
	ctx context.Context,
	out io.Writer,
	sys system.System,
	entry config.SystemConfig,
	previous config.SystemConfig,
) {
	host := system.HostTitle(entry.Settings.URL)
	revoker, ok := sys.(system.Revoker)
	if !ok {
		fmt.Fprintf(out, "The previous credentials stay valid until you revoke them on %s.\n", host)
		return
	}

	if previous.Settings.Credential(system.CredentialToken+credential.Suffix) != "" {
		fmt.Fprintf(out, "The previous API key comes from a command and wasn't revoked. "+
			"It stays valid until you revoke it on %s.\n", host)
		return
	}
	if previous.Settings.Credential(system.CredentialToken) == "" {
		return
	}

	err := revoker.Revoke(ctx, previous.Settings)
	switch {
	case err == nil:
		fmt.Fprintf(out, "Revoked the previous API key on %s.\n", host)
	case httpx.StatusOf(err) == http.StatusUnauthorized:
		fmt.Fprintln(out, "The previous API key was already invalid.")
	default:
		a.logger.Warn("revoking the previous key failed", "type", entry.Type, "error", err)
		fmt.Fprintf(out, "The previous API key couldn't be revoked on %s: %s. "+
			"It stays valid until you revoke it there.\n", host, text.Printable(err.Error()))
	}
}

func askToKeep(p prompt.Prompter, desc registry.Descriptor, sysURL string) (bool, error) {
	name := sysURL
	if !desc.AllowMultiple || name == "" {
		name = desc.Name
	}
	p.Notice(name + " is already connected.")

	choice, err := p.Choose("What do you want to do?",
		[]string{"Keep the existing connection", "Replace it"})
	if err != nil {
		return false, err
	}
	return choice == 0, nil
}

func (a *app) save(out io.Writer, entry config.SystemConfig, replace int) error {
	if replace >= 0 {
		a.cfg.Systems[replace] = entry
	} else {
		a.cfg.Systems = append(a.cfg.Systems, entry)
	}
	if err := a.cfg.Save(); err != nil {
		snippet, snippetErr := config.Snippet(entry)
		if snippetErr != nil {
			return fmt.Errorf("saving the configuration: %w", err)
		}
		fmt.Fprintf(out, "The configuration %s couldn't be written: %v\n\nAdd this to it yourself:\n\n%s",
			a.cfg.Path(), err, snippet)
		return errors.New("the configuration wasn't saved")
	}

	done := "added new connection"
	if replace >= 0 {
		done = "replaced the connection"
	}
	fmt.Fprintf(out, "Successfully %s! Configuration saved to %s\n", done, a.cfg.Path())
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
