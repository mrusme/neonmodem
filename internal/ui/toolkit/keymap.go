package toolkit

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
)

func (tk *ToolKit) KeymapAdd(id string, help string, keys ...string) {
	binding := key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(strings.Join(keys, "/"), help),
	)

	tk.keybindings[id] = binding
}

func (tk *ToolKit) KeymapGet(id string) key.Binding {
	if k, ok := tk.keybindings[id]; ok {
		return k
	}

	return key.NewBinding()
}

func (tk *ToolKit) SetCloseHelp(help string) {
	tk.closeHelp = help
}

func (tk *ToolKit) KeymapHelpStrings() []string {
	var bindings []string
	for _, binding := range tk.keybindings {
		bindings = append(bindings, binding.Help().Key+" "+binding.Help().Desc)
	}
	sort.Slice(bindings, func(i, j int) bool {
		a, b := strings.ToLower(bindings[i]), strings.ToLower(bindings[j])
		if a != b {
			return a < b
		}
		return bindings[i] > bindings[j]
	})

	closeHelp := tk.closeHelp
	if closeHelp == "" {
		closeHelp = "close"
	}
	bindings = append(bindings, "esc "+closeHelp)

	return bindings
}
