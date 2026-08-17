package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kerem-kaynak/mtok/internal/aggregate"
	"github.com/kerem-kaynak/mtok/internal/logs"
	"github.com/kerem-kaynak/mtok/internal/pricing"
	"github.com/kerem-kaynak/mtok/internal/usage"
)

// data holds everything the views render, precomputed once per scan.
type data struct {
	res   *logs.Result
	table pricing.Table
	home  string
	now   time.Time

	// tokens switches the primary chart metric from cost to token count
	// (the 't' key). Tables always show both.
	tokens bool

	days     []*aggregate.Group // newest first
	months   []*aggregate.Group // newest first
	models   []*aggregate.Group // cost desc, all time
	projects []*aggregate.Group // cost desc
	sessions []*aggregate.Group // last active desc
	all      aggregate.Bucket

	// modelSlot freezes each model's categorical slot for the process
	// lifetime (color follows the entity, never its current rank).
	modelSlot map[string]int

	sessProject      map[string]string // session id -> cwd
	sessModels       map[string]string // session id -> "model" or "model +n"
	projSessionCount map[string]int

	// coverage is each source's earliest row on disk. Months that start
	// before a source's coverage are partial: the logs were deleted (Claude
	// Code's cleanupPeriodDays) or the tool wasn't in use yet.
	coverage map[string]time.Time
}

func newData(res *logs.Result, table pricing.Table, now time.Time) *data {
	home, _ := os.UserHomeDir()
	d := &data{res: res, table: table, home: home, now: now}
	rows := res.Rows

	d.days = aggregate.By(rows, table, aggregate.DayKey)
	aggregate.SortByKeyDesc(d.days)
	d.months = aggregate.By(rows, table, aggregate.MonthKey)
	aggregate.SortByKeyDesc(d.months)
	d.models = aggregate.By(rows, table, aggregate.ModelKey)
	aggregate.SortByCostDesc(d.models)
	d.projects = aggregate.By(rows, table, func(r *usage.Row) string { return r.Project })
	aggregate.SortByCostDesc(d.projects)
	d.sessions = aggregate.By(rows, table, func(r *usage.Row) string { return r.Session })
	aggregate.SortByLastDesc(d.sessions)
	d.all = aggregate.Totals(rows, table)

	d.modelSlot = map[string]int{}
	for i, g := range d.models {
		d.modelSlot[g.Key] = i
	}

	d.coverage = map[string]time.Time{}
	for i := range rows { // rows are sorted by time
		if _, seen := d.coverage[rows[i].Source]; !seen {
			d.coverage[rows[i].Source] = rows[i].Time
		}
	}

	d.sessProject = map[string]string{}
	sessModels := map[string]map[string]struct{}{}
	projSess := map[string]map[string]struct{}{}
	for i := range rows {
		r := &rows[i]
		if r.Project != "" {
			d.sessProject[r.Session] = r.Project
		}
		ms := sessModels[r.Session]
		if ms == nil {
			ms = map[string]struct{}{}
			sessModels[r.Session] = ms
		}
		ms[pricing.Normalize(r.Model)] = struct{}{}
		ps := projSess[r.Project]
		if ps == nil {
			ps = map[string]struct{}{}
			projSess[r.Project] = ps
		}
		ps[r.Session] = struct{}{}
	}
	d.sessModels = make(map[string]string, len(sessModels))
	for sess, ms := range sessModels {
		first := ""
		for m := range ms {
			if first == "" || m < first {
				first = m
			}
		}
		if len(ms) > 1 {
			first = fmt.Sprintf("%s +%d", first, len(ms)-1)
		}
		d.sessModels[sess] = first
	}
	d.projSessionCount = make(map[string]int, len(projSess))
	for p, s := range projSess {
		d.projSessionCount[p] = len(s)
	}
	return d
}

func (d *data) modelColor(norm string) lipgloss.AdaptiveColor {
	if slot, ok := d.modelSlot[norm]; ok {
		return slotColor(slot)
	}
	return cOther
}

func sourceColor(source string) lipgloss.AdaptiveColor {
	if source == usage.SourceClaude {
		return slotColor(0)
	}
	return slotColor(1)
}

// rowsBetween returns rows in the local-time window [from, to).
func (d *data) rowsBetween(from, to time.Time) []usage.Row {
	return aggregate.Filter(d.res.Rows, from, to)
}

// partialMonth reports whether a "2006-01" month starts before some
// source's earliest log on disk — its totals are lower bounds, not truth.
func (d *data) partialMonth(month string) bool {
	m0, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return false
	}
	for _, first := range d.coverage {
		if m0.Before(aggregate.StartOfDay(first)) {
			return true
		}
	}
	return false
}

// coverageNote is a one-line "logs on disk begin ..." footnote, or "".
func (d *data) coverageNote() string {
	if len(d.coverage) == 0 {
		return ""
	}
	srcs := make([]string, 0, len(d.coverage))
	for s := range d.coverage {
		srcs = append(srcs, s)
	}
	sort.Strings(srcs)
	parts := make([]string, 0, len(srcs))
	for _, s := range srcs {
		parts = append(parts, s+" "+d.coverage[s].Local().Format("Jan 02 2006"))
	}
	return "logs on disk begin: " + strings.Join(parts, " · ") + " — * months are partial"
}

// metricName, axisFmt, groupMetric and bucketMetric switch every chart
// between cost and token count together, so no view mixes the two.
func (d *data) metricName() string {
	if d.tokens {
		return "tokens"
	}
	return "cost"
}

func (d *data) axisFmt() func(float64) string {
	if d.tokens {
		return tokAxis
	}
	return moneyAxis
}

func (d *data) groupMetric(g *aggregate.Group) float64 {
	if d.tokens {
		return float64(g.TotalTokens())
	}
	return g.Cost
}

func (d *data) bucketMetric(b *aggregate.Bucket) string {
	if d.tokens {
		return tok(b.TotalTokens()) + " tok"
	}
	return money(b.Cost)
}

// dailySeries returns the per-day metric for the n local days ending today,
// including zero days, oldest first, with matching "MM-DD" labels.
func (d *data) dailySeries(n int) (vals []float64, labels []string) {
	byDay := map[string]float64{}
	for _, g := range d.days {
		byDay[g.Key] = d.groupMetric(g)
	}
	start := aggregate.StartOfDay(d.now).AddDate(0, 0, -(n - 1))
	for i := 0; i < n; i++ {
		day := start.AddDate(0, 0, i)
		vals = append(vals, byDay[day.Format("2006-01-02")])
		labels = append(labels, day.Format("01-02"))
	}
	return vals, labels
}

// daysOfMonth returns the per-day metric for one "2006-01" month, oldest first.
func (d *data) daysOfMonth(month string) (vals []float64, labels []string) {
	first, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return nil, nil
	}
	byDay := map[string]float64{}
	for _, g := range d.days {
		byDay[g.Key] = d.groupMetric(g)
	}
	last := first.AddDate(0, 1, 0)
	if last.After(d.now) {
		last = aggregate.StartOfDay(d.now).AddDate(0, 0, 1)
	}
	for day := first; day.Before(last); day = day.AddDate(0, 0, 1) {
		vals = append(vals, byDay[day.Format("2006-01-02")])
		labels = append(labels, day.Format("01-02"))
	}
	return vals, labels
}
