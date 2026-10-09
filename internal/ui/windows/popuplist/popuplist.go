package popuplist

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/toolkit"
	"github.com/mrusme/neonmodem/internal/ui/windows"
)

const WIN_ID = "popuplist"

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	kind       msgs.PickerKind
	title      string
	list       list.Model
	loading    bool
	itemHeight int
	filter     list.FilterState
}

func NewModel(c *ctx.Ctx) *Model {
	m := &Model{
		ctx:        c,
		tk:         toolkit.New(WIN_ID, c),
		title:      "Select",
		itemHeight: 2,
	}

	m.list = list.New(nil, m.delegate(), 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.DisableQuitKeybindings()

	m.tk.KeymapAdd("enter", "choose", "enter")
	m.tk.KeymapAdd("filter", "filter", "/")

	m.tk.SetViewFunc(m.buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{ID: "enter", Handler: m.handleSelect},
		},
		OnViewResize: m.handleViewResize,
	})

	return m
}

func (m *Model) delegate() list.DefaultDelegate {
	t := m.ctx.Theme
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = t.PopupList.Item.Focused
	d.Styles.DimmedTitle = t.PopupList.Item.Blurred
	d.Styles.SelectedTitle = t.PopupList.Item.Selected
	d.Styles.NormalDesc = t.PopupList.ItemDetail.Focused
	d.Styles.DimmedDesc = t.PopupList.ItemDetail.Blurred
	d.Styles.SelectedDesc = t.PopupList.ItemDetail.Selected
	d.SetHeight(m.itemHeight)
	return d
}

func itemHeight(items []list.Item) int {
	height := 2
	for _, item := range items {
		if d, ok := item.(list.DefaultItem); ok {
			height = max(height, 2+strings.Count(d.Description(), "\n"))
		}
	}
	return height
}

func (m *Model) setItems(items []list.Item) tea.Cmd {
	m.itemHeight = 2
	if m.kind == msgs.PickOrder {
		m.itemHeight = itemHeight(items)
	}
	m.list.SetDelegate(m.delegate())
	return m.list.SetItems(items)
}

func (m *Model) Update(msg tea.Msg) (windows.Window, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.OpenPicker:
		m.kind = msg.Kind
		if msg.Title != "" {
			m.title = msg.Title
		}
		m.loading = m.kind == msgs.PickForum
		m.list.ResetFilter()
		m.list.ResetSelected()
		cmd := m.setItems(msg.Items)
		m.list.Select(msg.Selected)
		m.syncHelp()
		return m, cmd

	case msgs.PickerItems:
		if msg.Kind != m.kind {
			return m, nil
		}
		m.loading = false
		var cmds []tea.Cmd
		cmds = append(cmds, m.setItems(msg.Items))
		for _, err := range msg.Errors {
			m.ctx.Logger.Error("listing forums failed", "error", err)
			cmds = append(cmds, msgs.Send(msgs.Notice{Text: err.Error(), IsError: true}))
		}
		m.tk.InvalidateCache()
		return m, tea.Batch(cmds...)

	case msgs.ThemeChanged:
		m.list.SetDelegate(m.delegate())
	}

	if handled, cmds := m.tk.HandleMsg(msg); handled {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.syncHelp()
	return m, cmd
}

func (m *Model) FilterState() list.FilterState {
	return m.list.FilterState()
}

func (m *Model) syncHelp() {
	state := m.list.FilterState()
	if state == m.filter {
		return
	}
	m.filter = state

	var enter, esc string
	switch state {
	case list.Filtering:
		enter, esc = "apply filter", "cancel filter"
	case list.FilterApplied:
		enter, esc = "choose", "clear filter"
	case list.Unfiltered:
		enter, esc = "choose", "close"
	}
	m.tk.KeymapAdd("enter", enter, "enter")
	m.tk.SetCloseHelp(esc)
	m.handleViewResize()
	m.tk.InvalidateCache()
}
