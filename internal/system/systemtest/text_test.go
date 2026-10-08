package systemtest

import (
	"strings"
	"testing"
)

func TestWordsNameNothing(t *testing.T) {
	for _, w := range words {
		for _, banned := range []string{"neon", "modem", "overdrive", "test"} {
			if strings.Contains(w, banned) {
				t.Errorf("%q contains %q", w, banned)
			}
		}
		if len(w) > 8 {
			t.Errorf("%q is longer than 8 letters", w)
		}
		if w != strings.ToLower(w) {
			t.Errorf("%q is not lowercase", w)
		}
	}
}

func TestRandomText(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		text := RandomText(6)
		if n := len(strings.Fields(text)); n != 6 {
			t.Fatalf("%q has %d words", text, n)
		}
		if text[:1] != strings.ToUpper(text[:1]) {
			t.Fatalf("%q does not start with a capital letter", text)
		}
		if title := "Ask HN: " + text; len(title) > 80 {
			t.Fatalf("%q is longer than the Hacker News title limit", title)
		}
		if len(text) < 15 {
			t.Fatalf("%q is shorter than the Discourse title minimum", text)
		}
		if distinct := distinctChars(text); distinct < 10 {
			t.Fatalf("%q has %d distinct characters, Discourse needs 10", text, distinct)
		}
		seen[text] = true
	}
	if len(seen) < 990 {
		t.Errorf("only %d distinct texts in 1000 runs", len(seen))
	}
}

func TestRandomTextRepeatsNoWord(t *testing.T) {
	for range 100 {
		text := strings.ToLower(RandomText(14))
		used := map[string]bool{}
		for w := range strings.FieldsSeq(text) {
			if used[w] {
				t.Fatalf("%q repeats %q", text, w)
			}
			used[w] = true
		}
	}
}
