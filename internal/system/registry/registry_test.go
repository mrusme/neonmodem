package registry

import (
	"slices"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/credential"
)

func TestEverySystemDeclaresItsCredentials(t *testing.T) {
	for _, kind := range Kinds() {
		desc, err := Get(kind)
		if err != nil {
			t.Fatalf("Get(%q): %v", kind, err)
		}
		if len(desc.Credentials) == 0 {
			t.Errorf("%s declares no credential keys", kind)
		}
		seen := map[string]bool{}
		for _, key := range desc.Credentials {
			if key == "" || seen[key] {
				t.Errorf("%s declares an empty or repeated key %q", kind, key)
			}
			if strings.HasSuffix(key, credential.Suffix) {
				t.Errorf("%s declares the command form %q instead of the key", kind, key)
			}
			seen[key] = true
		}
	}
}

func TestLemmyDeclaresATokenAndNoPassword(t *testing.T) {
	desc, err := Get("lemmy")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(desc.Credentials, []string{"username", "token"}) {
		t.Errorf("got %v", desc.Credentials)
	}
}

func TestGetNormalizesTheKind(t *testing.T) {
	desc, err := Get("  Lemmy ")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if desc.Kind != "lemmy" {
		t.Errorf("got kind %q", desc.Kind)
	}
}

func TestGetRejectsUnknownKinds(t *testing.T) {
	_, err := Get("usenet")
	if err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
	if !strings.Contains(err.Error(), "known types:") {
		t.Errorf("error doesn't list the known types: %v", err)
	}
}

func TestEverySystemReportsItsURL(t *testing.T) {
	for _, kind := range Kinds() {
		desc, err := Get(kind)
		if err != nil {
			t.Fatal(err)
		}
		sys, err := desc.New(system.Env{Settings: system.Settings{URL: "https://forum.example"}})
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}

		want := "https://forum.example"
		if !desc.NeedsURL {
			want = "https://news.ycombinator.com"
		}
		if got := sys.URL(); got != want {
			t.Errorf("%s: got %q, want %q", kind, got, want)
		}
	}
}
