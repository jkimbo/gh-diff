package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type Item struct {
	ID      string
	Commit  string
	Subject string
	PrLink  string
}

func (i Item) FilterValue() string { return i.Subject }

type itemDelegate struct {
	// keys   *keys.KeyMap
	styles *styles
}

func newItemDelegate(styles *styles) *itemDelegate {
	return &itemDelegate{
		// keys:   keys,
		styles: styles,
	}
}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	title := d.styles.NormalTitle.Render

	if index == m.Index() {
		title = func(s string) string {
			return d.styles.SelectedTitle.Render("> " + s)
		}
	}

	subject := title(i.Subject)

	var itemListStyle strings.Builder
	itemListStyle.WriteString(subject)
	itemListStyle.WriteString("\n")

	pr := d.styles.NormalDesc.PaddingLeft(4).Render
	if i.PrLink != "" {
		itemListStyle.WriteString(pr(fmt.Sprintf(i.PrLink)))
	} else {
		itemListStyle.WriteString(pr("-"))
	}

	fmt.Fprint(w, itemListStyle.String())
}

func (d itemDelegate) ShortHelp() []key.Binding {
	return []key.Binding{}
}

func (d itemDelegate) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}
