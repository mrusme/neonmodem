package popuplist

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/toolkit"
)

var (
	WIN_ID = "popuplist"
)

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	selectionID string
	list        list.Model
	items       []list.Item

	a *aggregator.Aggregator
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx: c,
		tk: toolkit.New(
			WIN_ID,
			c.Theme,
			c.Logger,
		),
	}

	listDelegate := list.NewDefaultDelegate()
	listDelegate.Styles.NormalTitle = m.ctx.Theme.PopupList.Item.Focused
	listDelegate.Styles.DimmedTitle = m.ctx.Theme.PopupList.Item.Blurred
	listDelegate.Styles.SelectedTitle = m.ctx.Theme.PopupList.Item.Selected
	listDelegate.Styles.NormalDesc = m.ctx.Theme.PopupList.ItemDetail.Focused
	listDelegate.Styles.DimmedDesc = m.ctx.Theme.PopupList.ItemDetail.Blurred
	listDelegate.Styles.SelectedDesc = m.ctx.Theme.PopupList.ItemDetail.Selected

	m.list = list.New(m.items, listDelegate, 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)

	m.tk.KeymapAdd("enter", "choose selection", "enter")

	m.a, _ = aggregator.New(m.ctx)

	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{
				ID:      "enter",
				Handler: handleSelect,
			},
		},
		OnViewResize: handleViewResize,
		OnWinOpenCmd: handleWinOpenCmd,
	})

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	ret, cmds := m.tk.HandleMsg(&m, msg)
	if ret {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
