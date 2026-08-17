// Package aggregate rolls usage rows up into the buckets the TUI displays.
// All date bucketing is in the user's local time zone.
package aggregate

import (
	"sort"
	"time"

	"github.com/kerem-kaynak/mtok/internal/pricing"
	"github.com/kerem-kaynak/mtok/internal/usage"
)

// Bucket accumulates tokens, cost and call counts for one group.
type Bucket struct {
	Cost           float64
	CostNoCache    float64 // same tokens priced without cache discounts
	UnpricedTokens int64   // tokens from models without rates (cost unknown)
	Input          int64
	CacheRead      int64
	CacheWrite     int64
	Output         int64
	Reasoning      int64
	Calls          int64
	First, Last    time.Time
}

func (b *Bucket) add(r *usage.Row, cost, noCache float64, priced bool) {
	if priced {
		b.Cost += cost
		b.CostNoCache += noCache
	} else {
		b.UnpricedTokens += r.TotalTokens()
	}
	b.Input += r.Input
	b.CacheRead += r.CacheRead
	b.CacheWrite += r.CacheW5m + r.CacheW1h
	b.Output += r.Output
	b.Reasoning += r.Reasoning
	b.Calls++
	if b.First.IsZero() || r.Time.Before(b.First) {
		b.First = r.Time
	}
	if r.Time.After(b.Last) {
		b.Last = r.Time
	}
}

// TotalTokens mirrors usage.Row.TotalTokens at bucket level.
func (b *Bucket) TotalTokens() int64 {
	return b.Input + b.CacheRead + b.CacheWrite + b.Output
}

// CacheHitRate is the share of prompt tokens served from cache.
func (b *Bucket) CacheHitRate() float64 {
	prompt := b.Input + b.CacheRead + b.CacheWrite
	if prompt == 0 {
		return 0
	}
	return float64(b.CacheRead) / float64(prompt)
}

// Group is a keyed bucket with per-source split.
type Group struct {
	Key string
	Bucket
	BySource map[string]*Bucket
}

// By groups rows by an arbitrary key. Rows with an empty key fold into
// "(unknown)". Results are unsorted; use the Sort* helpers.
func By(rows []usage.Row, tbl pricing.Table, keyFn func(*usage.Row) string) []*Group {
	idx := map[string]*Group{}
	var out []*Group
	for i := range rows {
		r := &rows[i]
		k := keyFn(r)
		if k == "" {
			k = "(unknown)"
		}
		g := idx[k]
		if g == nil {
			g = &Group{Key: k, BySource: map[string]*Bucket{}}
			idx[k] = g
			out = append(out, g)
		}
		cost, priced := tbl.Cost(r)
		noCache, _ := tbl.CostNoCache(r)
		g.add(r, cost, noCache, priced)
		sb := g.BySource[r.Source]
		if sb == nil {
			sb = &Bucket{}
			g.BySource[r.Source] = sb
		}
		sb.add(r, cost, noCache, priced)
	}
	return out
}

// Filter returns the rows in [from, to). Zero times mean unbounded.
func Filter(rows []usage.Row, from, to time.Time) []usage.Row {
	// rows are sorted by time: binary search the window.
	lo := 0
	if !from.IsZero() {
		lo = sort.Search(len(rows), func(i int) bool { return !rows[i].Time.Before(from) })
	}
	hi := len(rows)
	if !to.IsZero() {
		hi = sort.Search(len(rows), func(i int) bool { return !rows[i].Time.Before(to) })
	}
	return rows[lo:hi]
}

// Day and month key helpers (local time).
func DayKey(r *usage.Row) string   { return r.Time.Local().Format("2006-01-02") }
func MonthKey(r *usage.Row) string { return r.Time.Local().Format("2006-01") }
func ModelKey(r *usage.Row) string { return pricing.Normalize(r.Model) }

// SortByKeyDesc orders groups by key descending (dates: newest first).
func SortByKeyDesc(gs []*Group) {
	sort.Slice(gs, func(i, j int) bool { return gs[i].Key > gs[j].Key })
}

// SortByCostDesc orders groups by cost, then unpriced tokens, descending.
func SortByCostDesc(gs []*Group) {
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].Cost != gs[j].Cost {
			return gs[i].Cost > gs[j].Cost
		}
		return gs[i].UnpricedTokens > gs[j].UnpricedTokens
	})
}

// SortByLastDesc orders groups by most recent activity.
func SortByLastDesc(gs []*Group) {
	sort.Slice(gs, func(i, j int) bool { return gs[i].Last.After(gs[j].Last) })
}

// Totals sums every row into a single bucket.
func Totals(rows []usage.Row, tbl pricing.Table) Bucket {
	var b Bucket
	for i := range rows {
		cost, priced := tbl.Cost(&rows[i])
		noCache, _ := tbl.CostNoCache(&rows[i])
		b.add(&rows[i], cost, noCache, priced)
	}
	return b
}

// Convenient local-time boundaries.
func StartOfDay(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func StartOfMonth(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
