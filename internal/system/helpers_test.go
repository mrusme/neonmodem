package system

import (
	"errors"
	"testing"
)

func TestHostTitle(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"https://lemmy.ml", "lemmy.ml"},
		{"http://localhost:8536", "localhost"},
		{"https://forum.example/community/", "forum.example"},
		{"not a url", "not a url"},
		{"", ""},
	} {
		if got := HostTitle(tc.url); got != tc.want {
			t.Errorf("HostTitle(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestDescribeAndCapabilities(t *testing.T) {
	if got := Describe("Lemmy", false); got != "Lemmy (read-only)" {
		t.Errorf("got %q", got)
	}
	if got := Describe("Lemmy", true); got != "Lemmy" {
		t.Errorf("got %q", got)
	}
	if caps := AccountCapabilities(false); !caps.Has(CapRead) || caps.Has(CapCreatePost) {
		t.Errorf("read-only capabilities: %08b", caps)
	}
	if caps := AccountCapabilities(true); !caps.Has(CapRead | CapWrite) {
		t.Errorf("account capabilities: %08b", caps)
	}
}

func TestNoCredentialsNamesTheCommand(t *testing.T) {
	err := NoCredentials("lemmy", "https://lemmy.ml")
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatal("the error must match ErrNoCredentials")
	}
	want := "this system is connected without an account; run neonmodem connect " +
		"--type lemmy --url https://lemmy.ml again with credentials to post"
	if err.Error() != want {
		t.Errorf("got %q", err)
	}
}
