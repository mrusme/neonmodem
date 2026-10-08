package browser

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	pkgbrowser "github.com/pkg/browser"
)

var quiet sync.Once

func Open(url string, program string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	if program == "" {
		quiet.Do(func() {
			pkgbrowser.Stdout = nil
			pkgbrowser.Stderr = nil
		})
		return pkgbrowser.OpenURL(url)
	}

	if _, err := os.Stat(program); err != nil {
		return err
	}

	cmd := exec.CommandContext(context.Background(), program, url)
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()
		var exitErr *exec.ExitError
		switch {
		case errors.As(err, &exitErr):
			logger.Warn("the browser ended with an error",
				"program", program, "status", exitErr.String())
		case err != nil:
			logger.Warn("waiting for the browser failed", "program", program, "error", err)
		}
	}()

	return nil
}
