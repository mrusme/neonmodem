package lemmy

import (
	"context"
	"testing"
)

func TestForumsShowTheFirstParagraphOfTheirDescription(t *testing.T) {
	_, srv := newFakeLemmy(t, map[string]route{
		forumsPath: {200, `{"communities":[
			{"community":{"id":1,"name":"python","title":"Python",
				"description":"## Welcome\n\nWelcome to the [Python community](https://programming.dev/c/python)\non **programming.dev**!\n\n- Rule one"}},
			{"community":{"id":2,"name":"privacy","title":"Privacy"}},
			{"community":{"id":3,"name":"art","title":"Art","description":"![banner](https://example.com/b.png)"}}]}`},
	})

	sys := plainSystem(t, srv.URL, nil)
	forums, err := sys.ListForums(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"Welcome to the Python community on programming.dev!",
		"Privacy",
		"Art",
	}
	if len(forums) != len(want) {
		t.Fatalf("got %d forums, want %d", len(forums), len(want))
	}
	for i, f := range forums {
		if f.Info != want[i] {
			t.Errorf("%s has the description %q, want %q", f.Name, f.Info, want[i])
		}
	}
}
