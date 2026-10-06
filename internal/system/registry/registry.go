package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/discourse"
	"github.com/mrusme/neonmodem/internal/system/hackernews"
	"github.com/mrusme/neonmodem/internal/system/hyperuplink"
	"github.com/mrusme/neonmodem/internal/system/lemmy"
	"github.com/mrusme/neonmodem/internal/system/lobsters"
)

type Descriptor struct {
	Kind          string
	Name          string
	NeedsURL      bool
	AllowMultiple bool
	New           func(env system.Env) (system.System, error)
}

var descriptors = map[string]Descriptor{
	discourse.Kind: {
		Kind:          discourse.Kind,
		Name:          "Discourse",
		NeedsURL:      true,
		AllowMultiple: true,
		New:           discourse.New,
	},
	lemmy.Kind: {
		Kind:          lemmy.Kind,
		Name:          "Lemmy",
		NeedsURL:      true,
		AllowMultiple: true,
		New:           lemmy.New,
	},
	lobsters.Kind: {
		Kind:          lobsters.Kind,
		Name:          "Lobsters",
		NeedsURL:      true,
		AllowMultiple: true,
		New:           lobsters.New,
	},
	hackernews.Kind: {
		Kind:          hackernews.Kind,
		Name:          "Hacker News",
		NeedsURL:      false,
		AllowMultiple: false,
		New:           hackernews.New,
	},
	hyperuplink.Kind: {
		Kind:          hyperuplink.Kind,
		Name:          "Hyperuplink",
		NeedsURL:      true,
		AllowMultiple: true,
		New:           hyperuplink.New,
	},
}

func Kinds() []string {
	kinds := make([]string, 0, len(descriptors))
	for kind := range descriptors {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func Lookup(kind string) (Descriptor, bool) {
	d, ok := descriptors[strings.ToLower(strings.TrimSpace(kind))]
	return d, ok
}

func New(kind string, env system.Env) (system.System, error) {
	d, ok := Lookup(kind)
	if !ok {
		return nil, fmt.Errorf("unknown system type %q; known types: %s",
			kind, strings.Join(Kinds(), ", "))
	}
	return d.New(env)
}
