package credential

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
)

const (
	Suffix  = "_cmd"
	Timeout = 60 * time.Second
)

type Runner interface {
	Run(ctx context.Context, command string) (string, error)
}

type Resolver struct {
	Runner Runner
	Trust  func() error
}

func (r Resolver) Resolve(
	ctx context.Context,
	logger *slog.Logger,
	credentials map[string]string,
	known []string,
) (map[string]string, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	resolved := make(map[string]string, len(known))

	var unknown []string
	for key, value := range credentials {
		base, isCommand := strings.CutSuffix(key, Suffix)
		if !slices.Contains(known, base) {
			unknown = append(unknown, key)
			continue
		}
		if !isCommand {
			resolved[base] = value
		}
	}

	slices.Sort(unknown)
	for _, key := range unknown {
		logger.Warn("ignoring a credential this system doesn't use", "key", key)
	}

	trusted := false
	for _, key := range known {
		commandKey := key + Suffix
		command := strings.TrimSpace(credentials[commandKey])
		if command == "" {
			continue
		}

		if credentials[key] != "" {
			logger.Warn("the command replaces the stored value, which can be removed",
				"key", key, "command", commandKey)
		}

		if !trusted {
			if err := r.trust(); err != nil {
				return nil, fmt.Errorf("%s not run: %w", commandKey, err)
			}
			trusted = true
		}

		logger.Debug("running a credential command", "key", commandKey)
		value, err := r.Runner.Run(ctx, command)
		if err != nil {
			return nil, fmt.Errorf("%s %w", commandKey, err)
		}
		resolved[key] = value
	}

	return resolved, nil
}

func (r Resolver) trust() error {
	if r.Trust == nil {
		return nil
	}
	return r.Trust()
}
