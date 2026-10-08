package msgs

import (
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

type ShowPosts struct{}

type FocusView struct{}

type BlurView struct{}

type RefreshFeed struct{}

type OrderChanged struct{}

type FeedResult struct {
	Gen    int64
	System int
	Order  system.Order
	Posts  []post.Post
	Err    error
}

type FeedStatus struct {
	Text string
}

type OpenPost struct {
	Post post.Post
}

type OpenWithMenu struct {
	Post post.Post
}

type ReloadPost struct {
	Delay time.Duration
}

type ComposeAction int

const (
	ComposePost ComposeAction = iota
	ComposeReply
)

type Compose struct {
	Action ComposeAction
	Post   post.Post
	Parent *reply.Reply
	Index  int
}

type PickerKind int

const (
	PickSystem PickerKind = iota
	PickForum
	PickOrder
	PickOpenWith
)

type OpenPicker struct {
	Kind     PickerKind
	Title    string
	Items    []list.Item
	Selected int
}

type PickerItems struct {
	Kind   PickerKind
	Items  []list.Item
	Errors []error
}

type Picked struct {
	Kind PickerKind
	Item list.Item
}

type CloseWindow struct {
	ID string
}

type WindowClosed struct {
	ID string
}

type FocusWindow struct {
	ID string
}

type BlurWindow struct {
	ID string
}

type ShowError struct {
	Messages []string
}

type Notice struct {
	Text    string
	IsError bool
}

type NoticeEntry struct {
	At      time.Time
	Text    string
	IsError bool
}

type NoticesChanged struct {
	Count int
}

type OpenNotices struct{}

type ShowNotices struct {
	Entries []NoticeEntry
}

type ThemeChanged struct{}

func Error(err error) ShowError {
	return ShowError{Messages: []string{err.Error()}}
}

func Message(text string) ShowError {
	return ShowError{Messages: []string{text}}
}

func Send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}
