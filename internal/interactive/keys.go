package interactive

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the interactive wizard.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	MoveUp   key.Binding
	MoveDown key.Binding

	Enter  key.Binding
	Back   key.Binding
	Cancel key.Binding

	Tab      key.Binding
	Pick     key.Binding
	Squash   key.Binding
	Reword   key.Binding
	Drop     key.Binding
	EditMsg  key.Binding
	LoadMore key.Binding

	Help key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		MoveUp: key.NewBinding(
			key.WithKeys("K", "ctrl+up"),
			key.WithHelp("K", "move up"),
		),
		MoveDown: key.NewBinding(
			key.WithKeys("J", "ctrl+down"),
			key.WithHelp("J", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "back"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "cancel"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "cycle op"),
		),
		Pick: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pick"),
		),
		Squash: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "squash"),
		),
		Reword: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reword"),
		),
		Drop: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "drop"),
		),
		EditMsg: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit msg"),
		),
		LoadMore: key.NewBinding(
			key.WithKeys("l", "m"),
			key.WithHelp("l", "load more"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

// SelectStepHelp returns help text for the select step.
func (k KeyMap) SelectStepHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.LoadMore, k.Cancel}
}

// EditStepHelp returns help text for the edit step.
func (k KeyMap) EditStepHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.MoveUp, k.MoveDown, k.Tab, k.EditMsg, k.Enter, k.Back}
}

// ConfirmStepHelp returns help text for the confirm step.
func (k KeyMap) ConfirmStepHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Back, k.Cancel}
}
