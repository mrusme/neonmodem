package toolkit

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

func (tk *ToolKit) KeymapAdd(id string, help string, keys ...string) {
	keysview := ""
	for i, k := range keys {
		if i > 0 {
			keysview += "/"
		}
		keysview += k
	}

	binding := key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(keysview, help),
	)

	tk.keybindings[id] = binding

	return
}

func (tk *ToolKit) KeymapGet(id string) key.Binding {
	if k, ok := tk.keybindings[id]; ok {
		return k
	}

	return key.NewBinding()
}

func (tk *ToolKit) KeymapHelpStrings() []string {
	var bindings []string
	for _, binding := range tk.keybindings {
		var tmp string = ""
		tmp = binding.Help().Key + " " + binding.Help().Desc
		bindings = append(bindings, tmp)
	}
	sort.SliceStable(bindings, func(i, j int) bool {
		return strings.Compare(bindings[i], bindings[j]) == -1
	})

	bindings = append(bindings, "esc close")

	return bindings
}
