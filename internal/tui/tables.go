package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// newTable builds a bubbles table with mtok chrome: bold header over a
// hairline rule, plain cells, a full-row selection bar. Cells must stay
// unstyled (padding only): a foreground on Cell embeds per-cell ANSI resets,
// and since bubbles applies Selected to the joined row string, the first
// reset would cut the selection background off after the first column.
func newTable(cols []table.Column, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(height))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true).Foreground(cInk2).
		BorderStyle(lipgloss.NormalBorder()).BorderForeground(cGrid).BorderBottom(true)
	st.Selected = sSelected
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
