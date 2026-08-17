package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kerem-kaynak/mtok/internal/aggregate"
	"github.com/kerem-kaynak/mtok/internal/usage"
)

type dashboardView struct {
	d    *data
	w, h int
}

func (v *dashboardView) setSize(w, h int) { v.w, v.h = w, h }
func (v *dashboardView) setData(d *data)  { v.d = d }
func (v *dashboardView) update(tea.Msg) tea.Cmd {
	return nil
}

func (v *dashboardView) render() string {
	d := v.d
	if d == nil || len(d.res.Rows) == 0 {
		return sMuted.Render("no usage found")
	}
	now := d.now
	today0 := aggregate.StartOfDay(now)
	tomorrow := today0.AddDate(0, 0, 1)
	month0 := aggregate.StartOfMonth(now)

	today := aggregate.Totals(d.rowsBetween(today0, tomorrow), d.table)
	yesterday := aggregate.Totals(d.rowsBetween(today0.AddDate(0, 0, -1), today0), d.table)
	mtd := aggregate.Totals(d.rowsBetween(month0, tomorrow), d.table)
	prev0 := month0.AddDate(0, -1, 0)
	prevMTD := aggregate.Totals(d.rowsBetween(prev0, minTime(prev0.AddDate(0, 0, now.Day()), month0)), d.table)
	last30rows := d.rowsBetween(today0.AddDate(0, 0, -29), tomorrow)
	last30 := aggregate.Totals(last30rows, d.table)

	// Primary metric per the 't' toggle: cost or tokens; the other demotes
	// to the tile's second line. Deltas track the primary metric.
	primary := func(b *aggregate.Bucket) string { return d.bucketMetric(b) }
	secondary := func(b *aggregate.Bucket) string {
		if d.tokens {
			return money(b.Cost)
		}
		return tok(b.TotalTokens()) + " tok"
	}
	mval := func(b *aggregate.Bucket) float64 {
		if d.tokens {
			return float64(b.TotalTokens())
		}
		return b.Cost
	}
	avg := money(last30.Cost/30) + "/day avg"
	if d.tokens {
		avg = tok(last30.TotalTokens()/30) + " tok/day avg"
	}
	tiles := []string{
		tile("TODAY", primary(&today), secondary(&today), delta(mval(&today), mval(&yesterday), "vs yesterday")),
		tile("MONTH TO DATE", primary(&mtd), secondary(&mtd), delta(mval(&mtd), mval(&prevMTD), "vs last MTD")),
		tile("LAST 30 DAYS", primary(&last30), secondary(&last30), avg),
		tile("ALL TIME*", primary(&d.all), secondary(&d.all), fmt.Sprintf("%s calls", comma(d.all.Calls))),
	}
	note := "* session logs have TTLs (Claude Code deletes them after cleanupPeriodDays, default 30d) — logs deleted before mtok first saw them are not counted"
	if d.res.Retained > 0 {
		note = fmt.Sprintf("* session logs have TTLs — %d deleted files retained from mtok's cache; logs deleted before mtok first saw them are not counted", d.res.Retained)
	}
	if v.w < lipgloss.Width(note)+2 {
		note = "* excludes session logs deleted before mtok first saw them (TTLs)"
	}
	kpis := joinTiles(tiles, v.w) + "\n" + sMuted.Render(note)

	// Daily metric, last 30 days — single-series magnitude, accent hue.
	vals, labels := d.dailySeries(30)
	chartW := v.w - 4
	sideW := 0
	sideBySide := v.w >= 96
	if sideBySide {
		sideW = 34
		chartW = v.w - sideW - 6
	}
	chart := panel(fmt.Sprintf("Daily %s — last 30 days · today %s", d.metricName(), primary(&today)),
		barChart(vals, labels, chartW-2, 7, d.axisFmt()), chartW)

	mix := panel("Model mix — 30 days", modelMix(last30rows, d, v.w-6), v.w-2)

	required := []string{kpis}
	var optional []string
	if sideBySide {
		side := lipgloss.JoinVertical(lipgloss.Left,
			panel("Cache hit — 30 days", cacheMeter(&last30, sideW-4), sideW),
			panel("Tokens by type — 30 days", tokenTypes(&last30, sideW-4), sideW),
			panel("By source — 30 days", sourceSplit(last30rows, d, sideW-4), sideW),
		)
		required = append(required, lipgloss.JoinHorizontal(lipgloss.Top, chart, " ", side))
		optional = []string{mix}
	} else {
		required = append(required, chart)
		optional = []string{
			panel("Tokens by type — 30 days", tokenTypes(&last30, v.w-6), v.w-2),
			panel("Cache hit — 30 days", cacheMeter(&last30, v.w-6), v.w-2),
			panel("By source — 30 days", sourceSplit(last30rows, d, v.w-6), v.w-2),
			mix,
		}
	}
	// Fit to the content height: keep required panels, then append optional
	// ones while they fit (small terminals drop the tail, never overflow).
	out := lipgloss.JoinVertical(lipgloss.Left, required...)
	for _, p := range optional {
		next := lipgloss.JoinVertical(lipgloss.Left, out, p)
		if v.h > 0 && lipgloss.Height(next) > v.h {
			break
		}
		out = next
	}
	return out
}

// tokenTypes shows where the tokens themselves go. Type→color is frozen
// (identity), in slots distinct from the source colors used next door.
func tokenTypes(b *aggregate.Bucket, w int) string {
	fresh := b.Input
	segs := []segment{
		{"cache read", float64(b.CacheRead), slotColor(2)},
		{"cache write", float64(b.CacheWrite), slotColor(3)},
		{"fresh in", float64(fresh), slotColor(4)},
		{"output", float64(b.Output), slotColor(5)},
	}
	total := float64(b.TotalTokens())
	var legend []string
	for _, s := range segs {
		share := ""
		if total > 0 {
			share = " (" + pct(s.Value/total) + ")"
		}
		legend = append(legend, swatch(s.Color)+" "+sInk2.Render(s.Label)+" "+
			sInk.Render(tok(int64(s.Value)))+sMuted.Render(share))
	}
	return shareBar(segs, w) + "\n" + strings.Join(legend, "\n")
}

func cacheMeter(b *aggregate.Bucket, w int) string {
	rate := b.CacheHitRate()
	out := meter(rate, maxInt(w-5, 4)) + " " + sInk.Render(pct(rate)) + "\n" +
		sMuted.Render(tok(b.CacheRead)+" cached · "+tok(b.Input)+" fresh · "+tok(b.CacheWrite)+" written")
	if b.CostNoCache > b.Cost && b.Cost > 0 {
		saved := 1 - b.Cost/b.CostNoCache
		out += "\n" + sInk2.Render("≈ "+moneyAxis(b.CostNoCache)+" without caching") +
			sMuted.Render(" · saved "+pct(saved))
	}
	return out
}

func sourceSplit(rows []usage.Row, d *data, w int) string {
	groups := aggregate.By(rows, d.table, func(r *usage.Row) string { return r.Source })
	aggregate.SortByCostDesc(groups)
	var segs []segment
	var legend []string
	for _, g := range groups {
		c := sourceColor(g.Key)
		segs = append(segs, segment{g.Key, d.groupMetric(g), c})
		legend = append(legend, swatch(c)+" "+sInk2.Render(g.Key)+" "+sInk.Render(d.bucketMetric(&g.Bucket)))
	}
	return shareBar(segs, w) + "\n" + strings.Join(legend, sMuted.Render("  ·  "))
}

// modelMix folds models past the slot count into "Other" (series ladder).
func modelMix(rows []usage.Row, d *data, w int) string {
	groups := aggregate.By(rows, d.table, aggregate.ModelKey)
	aggregate.SortByCostDesc(groups)
	total := 0.0
	for _, g := range groups {
		total += d.groupMetric(g)
	}
	var segs []segment
	var legend []string
	other := 0.0
	fmtVal := func(v float64) string {
		if d.tokens {
			return tok(int64(v)) + " tok"
		}
		return money(v)
	}
	for _, g := range groups {
		val := d.groupMetric(g)
		if len(segs) < len(slotColors)-1 && val > 0 {
			c := d.modelColor(g.Key)
			segs = append(segs, segment{g.Key, val, c})
			share := ""
			if total > 0 {
				share = " (" + pct(val/total) + ")"
			}
			legend = append(legend, "  "+swatch(c)+" "+sInk2.Render(g.Key)+" "+sInk.Render(fmtVal(val))+sMuted.Render(share))
		} else {
			other += val
		}
	}
	if other > 0 {
		segs = append(segs, segment{"other", other, cOther})
		legend = append(legend, "  "+swatch(cOther)+" "+sInk2.Render("other")+" "+sInk.Render(fmtVal(other)))
	}
	// Blank line so the legend doesn't sit flush against the bar.
	return shareBar(segs, w) + "\n\n" + strings.Join(legend, "\n")
}

func tile(title, value, sub, sub2 string) string {
	body := sMuted.Render(title) + "\n" + sValueBig.Render(value) + "\n" + sInk2.Render(sub)
	if sub2 != "" {
		body += "\n" + sMuted.Render(sub2)
	}
	return body
}

func joinTiles(tiles []string, w int) string {
	perRow := 4
	if w < 96 {
		perRow = 2
	}
	tileW := (w - perRow*2) / perRow
	var rendered []string
	for _, t := range tiles {
		rendered = append(rendered, sTile.Width(tileW).Render(t))
	}
	var rows []string
	for i := 0; i < len(rendered); i += perRow {
		end := minInt(i+perRow, len(rendered))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rendered[i:end]...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func panel(title, body string, w int) string {
	return sPanel.Width(w - 2).Render(sPanelTitle.Render(title) + "\n" + body)
}

func delta(cur, prev float64, label string) string {
	if prev <= 0 {
		return ""
	}
	ch := (cur - prev) / prev
	arrow := "↑"
	if ch < 0 {
		arrow = "↓"
		ch = -ch
	}
	return fmt.Sprintf("%s %.0f%% %s", arrow, ch*100, label)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
