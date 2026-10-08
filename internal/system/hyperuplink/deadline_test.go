package hyperuplink

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestEveryCallEndsWithItsContext(t *testing.T) {
	sys, err := New(system.Env{Settings: system.Settings{
		URL:         systemtest.Blocking(t),
		Credentials: map[string]string{system.CredentialToken: "hup_test"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	systemtest.Deadlines(t, sys, topicID, forumID)
}
