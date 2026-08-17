package tui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// modelsView: all-time per-model table.
type modelsView struct {
	d        *data
	t        table.Model
	w, h     int
	unpriced bool
}

func (v *modelsView) setSize(w, h int) { v.w, v.h = w, h; v.layout() }
func (v *modelsView) setData(d *data)  { v.d = d; v.layout() }

func (v *modelsView) layout() {
	if v.d == nil || v.w == 0 {
		return
	}
	// Each cell is padded 2 wide, so the rendered table is Σ(width+2):
	// base 70, +40 with the token-breakdown columns, 55 without Src/Share.
	wide := v.w >= 110
	slim := v.w < 70
	cols := []table.Column{{Title: "Model", Width: 26}}
	if !slim {
		cols = append(cols, table.Column{Title: "Src", Width: 6})
	}
	cols = append(cols, table.Column{Title: "Cost", Width: 10})
	if !slim {
		cols = append(cols, table.Column{Title: "Share", Width: 5})
	}
	if wide {
		cols = append(cols,
			table.Column{Title: "Input", Width: 8},
			table.Column{Title: "Output", Width: 8},
			table.Column{Title: "CacheRd", Width: 8},
			table.Column{Title: "CacheWr", Width: 8},
		)
	}
	cols = append(cols, table.Column{Title: "Hit", Width: 4}, table.Column{Title: "Calls", Width: 7})

	total := v.d.all.Cost
	v.unpriced = false
	var rows []table.Row
	for _, g := range v.d.models {
		src := ""
		for s := range g.BySource {
			if src != "" {
				src = "both"
				break
			}
			src = s
		}
		cost := money(g.Cost)
		share := ""
		if g.UnpricedTokens > 0 && g.Cost == 0 {
			cost = "—"
			v.unpriced = true
		} else if total > 0 {
			share = pct(g.Cost / total)
		}
		row := table.Row{g.Key}
		if !slim {
			row = append(row, src)
		}
		row = append(row, cost)
		if !slim {
			row = append(row, share)
		}
		if wide {
			row = append(row, tok(g.Input), tok(g.Output), tok(g.CacheRead), tok(g.CacheWrite))
		}
		row = append(row, pct(g.CacheHitRate()), comma(g.Calls))
		rows = append(rows, row)
	}
	v.t = newTable(cols, tableHeight(v.h)-1)
	v.t.SetRows(rows)
}

func (v *modelsView) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.t, cmd = v.t.Update(msg)
	return cmd
}

func (v *modelsView) render() string {
	if v.d == nil {
		return sMuted.Render("no usage found")
	}
	out := v.t.View()
	if v.unpriced {
		out += "\n" + sMuted.Render("— = no rates for this model; add it to ~/.config/mtok/config.json to price it")
	}
	return out
}

// projectsView: per-working-directory table.
type projectsView struct {
	d    *data
	t    table.Model
	w, h int
}

func (v *projectsView) setSize(w, h int) { v.w, v.h = w, h; v.layout() }
func (v *projectsView) setData(d *data)  { v.d = d; v.layout() }

func (v *projectsView) layout() {
	if v.d == nil || v.w == 0 {
		return
	}
	// Rendered width is projW+48 with all columns (cells pad 2 each);
	// drop trailing columns before squeezing Project below 14.
	showLast := v.w >= 62
	showSess := v.w >= 48
	fixed := 24 // Cost + Tokens incl. padding
	if showSess {
		fixed += 10
	}
	if showLast {
		fixed += 14
	}
	projW := maxInt(v.w-fixed-2, 14)
	cols := []table.Column{
		{Title: "Project", Width: projW},
		{Title: "Cost", Width: 10},
		{Title: "Tokens", Width: 8},
	}
	if showSess {
		cols = append(cols, table.Column{Title: "Sessions", Width: 8})
	}
	if showLast {
		cols = append(cols, table.Column{Title: "Last active", Width: 12})
	}
	var rows []table.Row
	for _, g := range v.d.projects {
		row := table.Row{
			shortPath(g.Key, v.d.home, projW),
			money(g.Cost),
			tok(g.TotalTokens()),
		}
		if showSess {
			row = append(row, comma(int64(v.d.projSessionCount[g.Key])))
		}
		if showLast {
			row = append(row, g.Last.Local().Format("Jan 02 15:04"))
		}
		rows = append(rows, row)
	}
	v.t = newTable(cols, tableHeight(v.h))
	v.t.SetRows(rows)
}

func (v *projectsView) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.t, cmd = v.t.Update(msg)
	return cmd
}

func (v *projectsView) render() string {
	if v.d == nil {
		return sMuted.Render("no usage found")
	}
	return v.t.View()
}

// sessionsView: recent sessions, newest first.
type sessionsView struct {
	d    *data
	t    table.Model
	w, h int
}

const maxSessionRows = 500

func (v *sessionsView) setSize(w, h int) { v.w, v.h = w, h; v.layout() }
func (v *sessionsView) setData(d *data)  { v.d = d; v.layout() }

func (v *sessionsView) layout() {
	if v.d == nil || v.w == 0 {
		return
	}
	// Without Project the table renders 67 wide (cells pad 2 each); show
	// Project only when it gets at least 12 columns, and shrink Model when
	// even the projectless layout doesn't fit.
	showProj := v.w >= 81
	modelW := 22
	if !showProj && v.w < 67 {
		modelW = maxInt(22-(67-v.w), 10)
	}
	projW := maxInt(v.w-69, 12)
	cols := []table.Column{{Title: "Start", Width: 12}}
	if showProj {
		cols = append(cols, table.Column{Title: "Project", Width: projW})
	}
	cols = append(cols,
		table.Column{Title: "Model", Width: modelW},
		table.Column{Title: "Dur", Width: 6},
		table.Column{Title: "Tokens", Width: 8},
		table.Column{Title: "Cost", Width: 9},
	)
	var rows []table.Row
	for i, g := range v.d.sessions {
		if i == maxSessionRows {
			break
		}
		row := table.Row{g.First.Local().Format("Jan 02 15:04")}
		if showProj {
			row = append(row, shortPath(v.d.sessProject[g.Key], v.d.home, projW))
		}
		row = append(row,
			v.d.sessModels[g.Key],
			duration(g.Last.Sub(g.First)),
			tok(g.TotalTokens()),
			money(g.Cost),
		)
		rows = append(rows, row)
	}
	v.t = newTable(cols, tableHeight(v.h)-1)
	v.t.SetRows(rows)
}

func (v *sessionsView) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.t, cmd = v.t.Update(msg)
	return cmd
}

func (v *sessionsView) render() string {
	if v.d == nil {
		return sMuted.Render("no usage found")
	}
	out := v.t.View()
	if len(v.d.sessions) > maxSessionRows {
		out += "\n" + sMuted.Render("showing the 500 most recent sessions")
	}
	return out
}
