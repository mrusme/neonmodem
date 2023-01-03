package toolkit

import "github.com/charmbracelet/bubbles/key"

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
