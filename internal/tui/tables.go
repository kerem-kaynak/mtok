package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// newTable builds a bubbles table with mtok chrome: bold header over a
// hairline rule, quiet cells, an accent selection bar. Cells are plain text —
// identity color lives in charts and detail panes, not table rows.
func newTable(cols []table.Column, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(height))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true).Foreground(cInk2).
		BorderStyle(lipgloss.NormalBorder()).BorderForeground(cGrid).BorderBottom(true)
	st.Cell = st.Cell.Foreground(cInk2)
	st.Selected = sSelected
	t.SetStyles(st)
	return t
}

// stretch widens the last column so the rendered table (each cell pads 2)
// spans exactly w terminal columns — the selection bar then covers the full
// row instead of stopping at the last column's edge.
func stretch(cols []table.Column, w int) []table.Column {
	used := 0
	for _, c := range cols {
		used += c.Width + 2
	}
	if extra := w - used; extra > 0 {
		cols[len(cols)-1].Width += extra
	}
	return cols
}

func tableHeight(h int) int {
	th := h - 3 // header + rule + breathing room
	if th < 3 {
		th = 3
	}
	return th
}
