package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// newTable builds a bubbles table with mtok chrome: bold header over a
// hairline rule, quiet cells, subtle selection. Cells are plain text —
// identity color lives in charts and detail panes, not table rows.
func newTable(cols []table.Column, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(height))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true).Foreground(cInk2).
		BorderStyle(lipgloss.NormalBorder()).BorderForeground(cGrid).BorderBottom(true)
	st.Cell = st.Cell.Foreground(cInk2)
	st.Selected = lipgloss.NewStyle().Bold(true).Foreground(cInk).Background(cGrid)
	t.SetStyles(st)
	return t
}

func tableHeight(h int) int {
	th := h - 3 // header + rule + breathing room
	if th < 3 {
		th = 3
	}
	return th
}
