package discourse

import (
	"net/http/httptest"
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestEveryCallEndsWithItsContext(t *testing.T) {
	srv := &httptest.Server{URL: systemtest.Blocking(t)}
	sys := testSystem(t, srv, map[string]string{
		system.CredentialUsername: "vera",
		system.CredentialKey:      "key",
		system.CredentialClientID: "client",
	})

	systemtest.Deadlines(t, sys, "9", "5")
}
