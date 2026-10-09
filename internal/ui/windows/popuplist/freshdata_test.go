package popuplist

import (
	"errors"
	"log/slog"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func testModel(t *testing.T) *Model {
	t.Helper()

	cfg := config.Defaults("/cache")
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), nil)
	m := NewModel(&c)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	return m
}

// TestLateItemsArePutIntoTheList covers a selector that was opened before its
// contents were known, which is how the forum selector stays responsive.
func TestLateItemsArePutIntoTheList(t *testing.T) {
	m := testModel(t)

	m.Update(msgs.OpenPicker{
		Kind:  msgs.PickForum,
		Items: []list.Item{forum.Forum{Name: "All"}},
	})
	if len(m.list.Items()) != 1 {
		t.Fatalf("expected the selector to open with 1 item, got %d", len(m.list.Items()))
	}
	if !m.loading {
		t.Error("a forum picker starts in the loading state")
	}

	_, cmd := m.Update(msgs.PickerItems{
		Kind: msgs.PickForum,
		Items: []list.Item{
			forum.Forum{Name: "All"},
			forum.Forum{ID: "f1", Name: "Cloud Providers/AWS"},
		},
	})
	if len(m.list.Items()) != 2 {
		t.Fatalf("expected 2 items after the fetch, got %d", len(m.list.Items()))
	}
	if m.loading {
		t.Error("the spinner should stop once the items are in")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, isNotice := msg.(msgs.Notice); isNotice {
				t.Error("no errors occurred, so no notice should be sent")
			}
		}
	}
}

func TestItemsForAnotherPickerAreIgnored(t *testing.T) {
	m := testModel(t)

	m.Update(msgs.OpenPicker{Kind: msgs.PickSystem, Items: []list.Item{forum.Forum{Name: "All"}}})
	m.Update(msgs.PickerItems{Kind: msgs.PickForum, Items: []list.Item{
		forum.Forum{Name: "All"}, forum.Forum{Name: "x"},
	}})
	if len(m.list.Items()) != 1 {
		t.Fatalf("forum items must not land in a system picker: %d items", len(m.list.Items()))
	}
}

// TestFetchErrorsAreSurfaced makes sure a system that failed to answer is
// reported rather than silently yielding a short list.
func TestFetchErrorsAreSurfaced(t *testing.T) {
	m := testModel(t)
	m.Update(msgs.OpenPicker{Kind: msgs.PickForum, Items: []list.Item{forum.Forum{Name: "All"}}})

	_, cmd := m.Update(msgs.PickerItems{
		Kind:   msgs.PickForum,
		Items:  []list.Item{forum.Forum{Name: "All"}},
		Errors: []error{errors.New("system unreachable")},
	})
	if cmd == nil {
		t.Fatal("expected the failure to be surfaced")
	}

	found := false
	collect(cmd, func(msg tea.Msg) {
		if n, ok := msg.(msgs.Notice); ok && n.IsError {
			found = true
		}
	})
	if !found {
		t.Error("expected an error notice")
	}
}

func TestEnterPicksTheSelectedItem(t *testing.T) {
	m := testModel(t)
	m.Update(msgs.OpenPicker{Kind: msgs.PickForum, Items: []list.Item{
		forum.Forum{Name: "All"}, forum.Forum{ID: "f1", Name: "AWS"},
	}})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should report the selection")
	}
	picked, ok := cmd().(msgs.Picked)
	if !ok {
		t.Fatalf("expected a Picked message, got %T", cmd())
	}
	if f, ok := picked.Item.(forum.Forum); !ok || f.ID != "f1" {
		t.Errorf("unexpected pick: %#v", picked.Item)
	}
}

func TestQDoesNotQuit(t *testing.T) {
	m := testModel(t)
	m.Update(msgs.OpenPicker{Kind: msgs.PickSystem, Items: []list.Item{forum.Forum{Name: "All"}}})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	collect(cmd, func(msg tea.Msg) {
		if _, quit := msg.(tea.QuitMsg); quit {
			t.Fatal("pressing q inside the popup list quit the program")
		}
	})
}

func collect(cmd tea.Cmd, visit func(tea.Msg)) {
	if cmd == nil {
		return
	}
	msg := cmd()
	switch batch := msg.(type) {
	case tea.BatchMsg:
		for _, c := range batch {
			collect(c, visit)
		}
	default:
		if msg != nil {
			visit(msg)
		}
	}
}

func TestPickerStartsOnTheSelectedItemAndFitsItsDescriptions(t *testing.T) {
	m := testModel(t)
	m.Update(msgs.OpenPicker{Kind: msgs.PickOrder, Selected: 2, Items: []list.Item{
		forum.Forum{Name: "a", Info: "first"},
		forum.Forum{Name: "b", Info: "second\nwith a note"},
		forum.Forum{Name: "c", Info: "third"},
	}})

	if m.list.Index() != 2 {
		t.Errorf("cursor on %d, want 2", m.list.Index())
	}
	if h := m.delegate().Height(); h != 3 {
		t.Errorf("a two-line description needs three lines, got %d", h)
	}

	m.Update(msgs.OpenPicker{Kind: msgs.PickOrder, Items: []list.Item{forum.Forum{Name: "New", Info: "Newest posts first"}}})
	if h := m.delegate().Height(); h != 2 {
		t.Errorf("entries without notes keep two lines, got %d", h)
	}
}

func TestOnlyTheOrderPickerGrowsWithItsDescriptions(t *testing.T) {
	long := forum.Forum{Name: "programming", Info: "Welcome!\n\n## Rules\n- one\n- two"}

	for _, kind := range []msgs.PickerKind{msgs.PickSystem, msgs.PickForum, msgs.PickOpenWith} {
		m := testModel(t)
		m.Update(msgs.OpenPicker{Kind: kind, Items: []list.Item{forum.Forum{Name: "All"}, long}})
		if h := m.delegate().Height(); h != 2 {
			t.Errorf("picker kind %d opens with entries of %d lines", kind, h)
		}
		m.Update(msgs.PickerItems{Kind: kind, Items: []list.Item{forum.Forum{Name: "All"}, long}})
		if h := m.delegate().Height(); h != 2 {
			t.Errorf("picker kind %d gets entries of %d lines from its items", kind, h)
		}
	}

	m := testModel(t)
	m.Update(msgs.OpenPicker{Kind: msgs.PickOrder, Items: []list.Item{long}})
	if h := m.delegate().Height(); h != 6 {
		t.Errorf("the order picker gives a five-line description %d lines", h)
	}
}
