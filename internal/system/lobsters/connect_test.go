package lobsters

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestConnectEndsWhenTheSiteDoesntAnswer(t *testing.T) {
	sys, err := New(system.Env{ReadTimeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader("1\ntester\n2\nprint password\n"), &out,
		systemtest.Commands{"print password": "secret"})
	_, err = sys.Connect(context.Background(), term, systemtest.Blocking(t))
	if err == nil || !strings.Contains(err.Error(), "didn't answer within 0.05 seconds") {
		t.Fatalf("got %v, expected the timeout wording\n%s", err, out.String())
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline error must stay recognizable")
	}
}
