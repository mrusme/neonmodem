package hackernews

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestConnectEndsWhenTheSiteDoesntAnswer(t *testing.T) {
	sys := &System{logger: slog.New(slog.DiscardHandler), readTimeout: 50 * time.Millisecond}
	base := systemtest.Blocking(t)

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader("1\ntester\n2\nprint password\n"), &out,
		systemtest.Commands{"print password": "secret"})
	_, err := sys.connect(context.Background(), term, base)
	if err == nil || !strings.Contains(err.Error(), "didn't answer within 0.05 seconds") {
		t.Fatalf("got %v, expected the timeout wording\n%s", err, out.String())
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline error must stay recognizable")
	}
}
