package ui

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows/postshow"
)

func TestAFeedRefreshDoesNotCancelAPostLoad(t *testing.T) {
	m, c := rootModel(t)

	updated, load := m.Update(msgs.OpenPost{Post: post.Post{ID: "p", Subject: "s", SysIDX: 0}})
	m = updated.(Model)

	updated, refresh := m.Update(msgs.RefreshFeed{})
	m, _ = settleModel(t, updated.(Model), refresh)

	m, _ = settleModel(t, m, load)
	if !m.wm.IsOpen(postshow.WIN_ID) {
		t.Fatal("the post window should still be open")
	}
	if c.IsLoading() {
		t.Error("the post finished loading after the feed refresh, but the spinner keeps running")
	}
}
