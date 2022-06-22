package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type PrStatus int

const (
	Pending PrStatus = iota
	Passed
	Failed
)

type Item struct {
	ID             string
	Commit         string
	Subject        string
	PrLink         string
	PrReviewStatus PrStatus
	IsSaved        bool
	IsStacked      bool
}

func (i Item) FilterValue() string { return i.Subject }

type itemDelegate struct {
	keys   *KeyMap
	styles *styles
}

func newItemDelegate(keys *KeyMap, styles *styles) *itemDelegate {
	return &itemDelegate{
		keys:   keys,
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

	subject := d.styles.NormalTitle.Copy().PaddingLeft(2).Render("◯ " + i.Subject)

	if index == m.Index() {
		subject = d.styles.SelectedTitle.Copy().PaddingLeft(2).Render("◉ " + i.Subject)
	}

	var itemListStyle strings.Builder
	itemListStyle.WriteString(subject)
	itemListStyle.WriteString("\n")

	// Render description
	var desc strings.Builder

	if i.IsStacked == true {
		desc.WriteString(d.styles.NormalDesc.Copy().PaddingLeft(2).Render("│ "))
	} else {
		desc.WriteString(d.styles.NormalDesc.Copy().PaddingLeft(4).Render(""))
	}

	pr := d.styles.NormalDesc.Render
	if i.PrLink != "" {
		desc.WriteString(pr(fmt.Sprintf(i.PrLink)))
	} else {
		desc.WriteString(pr("-"))
	}

	itemListStyle.WriteString(desc.String())

	fmt.Fprint(w, itemListStyle.String())
}

func (d itemDelegate) ShortHelp() []key.Binding {
	return []key.Binding{d.keys.Sync, d.keys.Land}
}

func (d itemDelegate) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keys.Sync, d.keys.Land},
	}
}
