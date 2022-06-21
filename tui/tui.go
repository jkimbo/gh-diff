package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultWidth = 20
	listHeight   = 15
)

type Model struct {
	list   list.Model
	keyMap *KeyMap
	choice list.Item
	// styles styles.Styles
	// state  state
}

func NewModel(items []list.Item) Model {
	styles := defaultStyles()
	keys := NewKeyMap()

	l := list.New(items, newItemDelegate(&styles), defaultWidth, listHeight)
	l.Title = "Your queue"
	l.SetShowStatusBar(false)
	l.Styles.PaginationStyle = styles.Pagination
	l.Styles.HelpStyle = styles.Help
	l.SetFilteringEnabled(false)

	return Model{
		keyMap: keys,
		list:   l,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.ForceQuit):
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.CursorUp):
			m.list.CursorUp()

		case key.Matches(msg, m.keyMap.CursorDown):
			m.list.CursorDown()

		case key.Matches(msg, m.keyMap.Enter):
			m.choice = m.list.SelectedItem()
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}

	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return "\n" + m.list.View()
}

// GetChoice returns the chosen diff ID
func (m Model) GetChoice() list.Item {
	return m.choice
}
