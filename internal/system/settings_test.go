package system

import (
	"maps"
	"testing"

	"github.com/mrusme/neonmodem/internal/system/prompt"
)

func TestSetCredentialStoresEitherTheValueOrTheCommand(t *testing.T) {
	var s Settings

	s.SetCredential(CredentialUsername, prompt.Answer{Value: "vera"})
	s.SetCredential(CredentialPassword, prompt.Answer{Value: "hunter2", Command: "pass show lemmy"})

	want := map[string]string{"username": "vera", "password_cmd": "pass show lemmy"}
	if !maps.Equal(s.Credentials, want) {
		t.Errorf("got %v, want %v", s.Credentials, want)
	}

	s.SetCredential(CredentialPassword, prompt.Answer{Value: "typed"})
	s.SetCredential(CredentialUsername, prompt.Answer{Value: "vera", Command: "pass show user"})

	want = map[string]string{"username_cmd": "pass show user", "password": "typed"}
	if !maps.Equal(s.Credentials, want) {
		t.Errorf("after switching sources got %v, want %v", s.Credentials, want)
	}
}
