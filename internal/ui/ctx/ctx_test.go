package ctx

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/config"
)

func TestLoadingTracksEverySource(t *testing.T) {
	cfg := config.Defaults("/cache")
	c := New(nil, &cfg, nil, nil)

	if c.IsLoading() {
		t.Fatal("nothing is loading at the start")
	}

	c.StartLoading(LoadFeed)
	c.StartLoading(LoadPost)
	c.StopLoading(LoadPost)
	if !c.IsLoading() {
		t.Error("the feed is still loading after the post ended")
	}

	c.StopLoading(LoadFeed)
	if c.IsLoading() {
		t.Error("nothing should be loading")
	}

	c.StopLoading(LoadForums)
	c.StartLoading(LoadSubmit)
	c.StartLoading(LoadSubmit)
	c.StopLoading(LoadSubmit)
	if c.IsLoading() {
		t.Error("starting a source twice needs one stop")
	}
}

func TestLoadingIsSharedBetweenCopies(t *testing.T) {
	cfg := config.Defaults("/cache")
	c := New(nil, &cfg, nil, nil)
	copied := c

	copied.StartLoading(LoadFeed)
	if !c.IsLoading() {
		t.Error("a copy of the context must share the loading state")
	}
}

func TestANewLoadCancelsThePreviousOne(t *testing.T) {
	cfg := config.Defaults("/cache")
	c := New(nil, &cfg, nil, nil)

	first, firstGen := c.NextLoad()
	second, secondGen := c.NextLoad()
	if first.Err() == nil {
		t.Error("the second load must cancel the first load's context")
	}
	if second.Err() != nil || !c.IsCurrentLoadGen(secondGen) || c.IsCurrentLoadGen(firstGen) {
		t.Error("the second load is the current one")
	}

	c.CancelLoad()
	if second.Err() == nil || c.IsCurrentLoadGen(secondGen) {
		t.Error("cancelling ends the current load")
	}
}
