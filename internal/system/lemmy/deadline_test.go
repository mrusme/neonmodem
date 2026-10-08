package lemmy

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestEveryCallEndsWithItsContext(t *testing.T) {
	sys, err := New(system.Env{Settings: system.Settings{URL: systemtest.Blocking(t), Credentials: token("vera")}})
	if err != nil {
		t.Fatal(err)
	}

	systemtest.Deadlines(t, sys, "1", "2")
}
