package hackernews

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hackernews/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestEveryCallEndsWithItsContext(t *testing.T) {
	base := systemtest.Blocking(t)
	client, err := api.NewClientWithBases(httpx.NewHTTPClient(httpx.Options{}), base+"/firebase", base+"/algolia")
	if err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	sys, err := newWithClients(&System{
		idx:      2,
		settings: system.Settings{URL: base, Credentials: account},
		logger:   slog.New(slog.NewTextHandler(&log, nil)),
	}, client, base)
	if err != nil {
		t.Fatal(err)
	}

	systemtest.Deadlines(t, sys, "1", "ask", "ListForums")

	if strings.Contains(log.String(), "falling back") {
		t.Errorf("an ended context must not start the fallback:\n%s", log.String())
	}
}
