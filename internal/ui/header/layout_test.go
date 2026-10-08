package header

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/system"
)

func TestLayoutShrinksBoxesAndHidesTheBanner(t *testing.T) {
	for _, tc := range []struct {
		width    int
		banner   bool
		selector int
		shown    bool
	}{
		{120, true, 40, true},
		{100, true, 27, true},
		{93, true, 20, true},
		{92, true, 40, false},
		{80, true, 40, false},
		{60, true, 20, false},
		{120, false, 40, false},
	} {
		selector, shown := layout(tc.width, tc.banner)
		if selector != tc.selector || shown != tc.shown {
			t.Errorf("width %d, banner %v: got %d and %v, want %d and %v",
				tc.width, tc.banner, selector, shown, tc.selector, tc.shown)
		}
	}
}

func TestHeaderFitsTheTerminal(t *testing.T) {
	c := testCtx()
	c.SetOrder(system.OrderTopMonth)
	m := NewModel(c)

	for _, width := range []int{60, 80, 100, 120} {
		c.Screen = [2]int{width, 40}
		view := m.View()

		lines := strings.Split(view, "\n")
		if len(lines) != Height {
			t.Errorf("width %d: %d lines, want %d", width, len(lines), Height)
		}
		for i, line := range lines {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line %d is %d columns wide", width, i, w)
			}
		}
		if !strings.Contains(view, "Sort:") || !strings.Contains(view, "Top: past month") {
			t.Errorf("width %d: the order box is missing:\n%s", width, view)
		}
		if hasBanner := strings.Contains(view, "|___|"); hasBanner != (width >= 93) {
			t.Errorf("width %d: banner shown %v", width, hasBanner)
		}
	}
}
