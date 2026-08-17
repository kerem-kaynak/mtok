package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kerem-kaynak/mtok/internal/aggregate"
	"github.com/kerem-kaynak/mtok/internal/usage"
)

// dailyView: table of days; the detail pane follows the cursor with the
// selected day's per-model and per-source breakdown.
type dailyView struct {
	d    *data
	t    table.Model
	keys []string
	w, h int
}

func (v *dailyView) setSize(w, h int) { v.w, v.h = w, h; v.layout() }
func (v *dailyView) setData(d *data)  { v.d = d; v.layout() }

func (v *dailyView) detailWidth() int {
	if v.w >= 104 {
		return 42
	}
	return 0
}

func (v *dailyView) layout() {
	if v.d == nil || v.w == 0 {
		return
	}
	tw := v.w - v.detailWidth() - 2
	cols := []table.Column{
		{Title: "Date", Width: 10},
		{Title: "Cost", Width: 9},
		{Title: "Tokens", Width: 8},
	}
	if tw >= 78 {
		cols = append(cols,
			table.Column{Title: "Input", Width: 8},
			table.Column{Title: "Output", Width: 8},
			table.Column{Title: "CacheRd", Width: 8},
		)
	}
	cols = append(cols, table.Column{Title: "Hit", Width: 4}, table.Column{Title: "Calls", Width: 6})

	var rows []table.Row
	v.keys = v.keys[:0]
	for _, g := range v.d.days {
		row := table.Row{g.Key, money(g.Cost), tok(g.TotalTokens())}
		if tw >= 78 {
			row = append(row, tok(g.Input), tok(g.Output), tok(g.CacheRead))
		}
		row = append(row, pct(g.CacheHitRate()), comma(g.Calls))
		rows = append(rows, row)
		v.keys = append(v.keys, g.Key)
	}
	cur := v.t.Cursor()
	v.t = newTable(cols, tableHeight(v.h))
	v.t.SetRows(rows)
	if cur > 0 && cur < len(rows) {
		v.t.SetCursor(cur)
	}
}

func (v *dailyView) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.t, cmd = v.t.Update(msg)
	return cmd
}

func (v *dailyView) render() string {
	if v.d == nil || len(v.keys) == 0 {
		return sMuted.Render("no usage found")
	}
	tbl := v.t.View()
	dw := v.detailWidth()
	if dw == 0 {
		return tbl
	}
	day := v.keys[minInt(v.t.Cursor(), len(v.keys)-1)]
	detail := panel(day, v.dayDetail(day, dw-4), dw)
	return lipgloss.JoinHorizontal(lipgloss.Top, tbl, "  ", detail)
}

// dayDetail renders per-model and per-source breakdowns for one local day.
func (v *dailyView) dayDetail(day string, w int) string {
	from, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return ""
	}
	rows := v.d.rowsBetween(from, from.AddDate(0, 0, 1))
	return breakdown(rows, v.d, w)
}

// breakdown is the shared drill-down body: top models with visible values,
// then the per-source split and cache summary.
func breakdown(rows []usage.Row, d *data, w int) string {
	if len(rows) == 0 {
		return sMuted.Render("no usage")
	}
	models := aggregate.By(rows, d.table, aggregate.ModelKey)
	aggregate.SortByCostDesc(models)
	var lines []string
	for i, g := range models {
		if i == 5 {
			lines = append(lines, sMuted.Render("… and more"))
			break
		}
		cost := money(g.Cost)
		if g.UnpricedTokens > 0 && g.Cost == 0 {
			cost = "unpriced"
		}
		lines = append(lines, swatch(d.modelColor(g.Key))+" "+
			sInk2.Render(padr(g.Key, w-16))+" "+sInk.Render(cost)+sMuted.Render(" · "+tok(g.TotalTokens())))
	}
	tot := aggregate.Totals(rows, d.table)
	lines = append(lines, "")
	lines = append(lines, sourceSplit(rows, d, w))
	lines = append(lines, "")
	lines = append(lines, sMuted.Render("cache hit ")+sInk.Render(pct(tot.CacheHitRate()))+
		sMuted.Render(" · "+comma(tot.Calls)+" calls"))
	if tot.UnpricedTokens > 0 {
		lines = append(lines, sMuted.Render(tok(tot.UnpricedTokens)+" tok from unpriced models"))
	}
	return strings.Join(lines, "\n")
}

func padr(s string, w int) string {
	if w < 1 {
		return s
	}
	if len(s) > w {
		return s[:w-1] + "…"
	}
	return s + strings.Repeat(" ", w-len(s))
}
