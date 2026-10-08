package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/config"
)

func TestColorsFallBackToTheOtherVariant(t *testing.T) {
	both := config.Adaptive{Light: "#111111", Dark: "#222222"}
	lightOnly := config.Adaptive{Light: "#333333"}
	darkOnly := config.Adaptive{Dark: "#444444"}

	dark, light := builder{dark: true}, builder{dark: false}

	cases := []struct {
		name string
		b    builder
		a    config.Adaptive
		want string
	}{
		{"dark picks dark", dark, both, "#222222"},
		{"light picks light", light, both, "#111111"},
		{"dark falls back to light", dark, lightOnly, "#333333"},
		{"light falls back to dark", light, darkOnly, "#444444"},
	}
	for _, tc := range cases {
		if got := tc.b.color(tc.a); got != lipgloss.Color(tc.want) {
			t.Errorf("%s: got %v, expected %s", tc.name, got, tc.want)
		}
	}

	if got := dark.color(config.Adaptive{}); got != (lipgloss.NoColor{}) {
		t.Errorf("an unset color is NoColor, got %v", got)
	}
}

func TestThemeAppliesTheConfiguredColors(t *testing.T) {
	cfg := config.Defaults("/cache")
	cfg.Theme.Post.Author.Foreground = config.Adaptive{Light: "#123456", Dark: "#654321"}

	if got := New(&cfg.Theme, true).Post.Author.GetForeground(); got != lipgloss.Color("#654321") {
		t.Errorf("the dark theme uses the dark color, got %v", got)
	}
	if got := New(&cfg.Theme, false).Post.Author.GetForeground(); got != lipgloss.Color("#123456") {
		t.Errorf("the light theme uses the light color, got %v", got)
	}

	th := New(&cfg.Theme, true)
	if th.Notice.GetForeground() != th.DialogBox.Titlebar.Focused.GetBackground() {
		t.Error("notices use the dialog title color")
	}
	if th.Alert.GetForeground() != th.ErrorDialogBox.Titlebar.Focused.GetBackground() {
		t.Error("alerts use the error dialog title color")
	}
}
