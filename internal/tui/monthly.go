package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// monthlyView: table of months; detail pane shows the selected month's daily
// cost chart and model breakdown.
type monthlyView struct {
	d    *data
	t    table.Model
	keys []string
	w, h int
}

func (v *monthlyView) setSize(w, h int) { v.w, v.h = w, h; v.layout() }
func (v *monthlyView) setData(d *data)  { v.d = d; v.layout() }

func (v *monthlyView) detailWidth() int {
	if v.w >= 100 {
		return v.w - 52
	}
	return 0
}

func (v *monthlyView) layout() {
	if v.d == nil || v.w == 0 {
		return
	}
	cols := []table.Column{
		{Title: "Month", Width: 9},
		{Title: "Cost", Width: 10},
		{Title: "Tokens", Width: 8},
		{Title: "Hit", Width: 4},
		{Title: "Calls", Width: 7},
	}
	var rows []table.Row
	v.keys = v.keys[:0]
	for _, g := range v.d.months {
		name := g.Key
		if v.d.partialMonth(g.Key) {
			name += "*"
		}
		rows = append(rows, table.Row{name, money(g.Cost), tok(g.TotalTokens()), pct(g.CacheHitRate()), comma(g.Calls)})
		v.keys = append(v.keys, g.Key)
	}
	cur := v.t.Cursor()
	v.t = newTable(cols, tableHeight(v.h))
	v.t.SetRows(rows)
	if cur > 0 && cur < len(rows) {
		v.t.SetCursor(cur)
	}
}

func (v *monthlyView) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.t, cmd = v.t.Update(msg)
	return cmd
}

func (v *monthlyView) render() string {
	if v.d == nil || len(v.keys) == 0 {
		return sMuted.Render("no usage found")
	}
	tbl := v.t.View()
	if note := v.d.coverageNote(); note != "" {
		tbl += "\n" + sMuted.Render(note)
	}
	dw := v.detailWidth()
	if dw == 0 {
		return tbl
	}
	month := v.keys[minInt(v.t.Cursor(), len(v.keys)-1)]
	vals, labels := v.d.daysOfMonth(month)
	chart := barChart(vals, labels, dw-6, 6, v.d.axisFmt())
	from, _ := time.ParseInLocation("2006-01", month, time.Local)
	rows := v.d.rowsBetween(from, from.AddDate(0, 1, 0))
	detail := panel(month+" · daily "+v.d.metricName(), chart, dw) + "\n" +
		panel(month+" · breakdown", breakdown(rows, v.d, dw-4), dw)
	return lipgloss.JoinHorizontal(lipgloss.Top, tbl, "  ", detail)
}
