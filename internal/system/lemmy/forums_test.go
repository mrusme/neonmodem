package lemmy

import (
	"context"
	"testing"
)

func TestForumsShowTheFirstParagraphOfTheirDescription(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		_, srv := newFakeLemmy(t, fl, map[string]route{
			fl.forums: {200, fl.forumList(t,
				community{id: 1, name: "python", title: "Python",
					description: "## Welcome\n\nWelcome to the [Python community](https://programming.dev/c/python)\non **programming.dev**!\n\n- Rule one"},
				community{id: 2, name: "privacy", title: "Privacy"},
				community{id: 3, name: "art", title: "Art", description: "![banner](https://example.com/b.png)"},
			)},
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
	})
}

func TestForumsPreferTheSummary(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		_, srv := newFakeLemmy(t, fl, map[string]route{
			fl.forums: {200, fl.forumList(t,
				community{id: 2, name: "general", title: "General", summary: "General **talk**",
					description: "## Rules\n\nThe general community of this instance.\n\n- Be kind"},
				community{id: 3, name: "testing", title: "Testing",
					description: "## Rules\n\nThe testing community of this instance.\n\n- Be kind"},
				community{id: 4, name: "quiet", title: "Quiet", summary: "Nothing to see"},
			)},
		})

		sys := plainSystem(t, srv.URL, nil)
		forums, err := sys.ListForums(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		want := []string{"General talk", "The testing community of this instance.", "Nothing to see"}
		if fl.name == "v3" {
			want = []string{"The general community of this instance.", "The testing community of this instance.", "Quiet"}
		}
		if len(forums) != len(want) {
			t.Fatalf("got %d forums, want %d", len(forums), len(want))
		}
		for i, f := range forums {
			if f.Info != want[i] {
				t.Errorf("%s has the info %q, want %q", f.Name, f.Info, want[i])
			}
		}
	})
}
