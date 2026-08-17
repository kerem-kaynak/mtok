package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// newTable builds a bubbles table with mtok chrome: bold header over a
// hairline rule, quiet cells, a full-row selection bar. Cells must stay
// unstyled (padding only): a foreground on Cell embeds per-cell ANSI resets,
// and since bubbles applies Selected to the joined row string, the first
// reset would cut the selection background off after the first column. The
// muted cell tone is re-applied per line by muteRows instead.
func newTable(cols []table.Column, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(height))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true).Foreground(cInk2).
		BorderStyle(lipgloss.NormalBorder()).BorderForeground(cGrid).BorderBottom(true)
	st.Selected = sSelected
	t.SetStyles(st)
	return t
}

// muteRows applies the quiet cell tone to a rendered table view: data rows
// come out of bubbles as plain text and get the muted foreground here, while
// lines that already carry ANSI codes (header, rule, selection bar) are left
// alone. See newTable for why Cell can't hold the color itself.
func muteRows(view string) string {
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		if l != "" && !strings.Contains(l, "\x1b") {
			lines[i] = sInk2.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

func tableHeight(h int) int {
	th := h - 3 // header + rule + breathing room
	if th < 3 {
		th = 3
	}
	return th
}
