package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	CursorUp   key.Binding
	CursorDown key.Binding
	Enter      key.Binding
	Sync       key.Binding
	Land       key.Binding
	Cancel     key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	var kb []key.Binding

	kb = append(kb, k.Sync, k.Land, k.Cancel, k.ForceQuit)

	return kb
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

func NewKeyMap() *KeyMap {
	return &KeyMap{
		CursorUp: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "move up"),
		),
		CursorDown: key.NewBinding(
			key.WithKeys("ctrl+j"),
			key.WithHelp("ctrl+j", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "sync diff"),
		),
		Sync: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sync diff"),
		),
		Land: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "land diff"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
	}
}
