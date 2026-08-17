package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/kerem-kaynak/mtok/internal/aggregate"
	"github.com/kerem-kaynak/mtok/internal/logs"
)

// Summary scans and prints a plain-text report (for scripts and quick looks;
// no TUI). Costs are estimated API-equivalent list prices.
func Summary(opts Options) error {
	res, err := logs.Scan(logs.Options{
		ClaudeDirs: opts.ClaudeDirs,
		CodexDirs:  opts.CodexDirs,
		CacheFile:  opts.CacheFile,
	})
	if err != nil {
		return err
	}
	d := newData(res, opts.Table, time.Now())

	now := d.now
	today0 := aggregate.StartOfDay(now)
	tomorrow := today0.AddDate(0, 0, 1)
	today := aggregate.Totals(d.rowsBetween(today0, tomorrow), d.table)
	mtd := aggregate.Totals(d.rowsBetween(aggregate.StartOfMonth(now), tomorrow), d.table)
	last30 := aggregate.Totals(d.rowsBetween(today0.AddDate(0, 0, -29), tomorrow), d.table)

	fmt.Printf("mtok — estimated API-equivalent spend from local session logs\n")
	retained := ""
	if res.Retained > 0 {
		retained = fmt.Sprintf(", %d deleted files retained", res.Retained)
	}
	fmt.Printf("scanned %d files (%d cached%s) · %s rows · %s duplicates removed\n\n",
		res.Files, res.FromCache, retained, comma(int64(len(res.Rows))), comma(int64(res.Duplicates)))
	fmt.Printf("%-16s %10s %12s\n", "", "cost", "tokens")
	line := func(name string, b aggregate.Bucket) {
		fmt.Printf("%-16s %10s %12s\n", name, money(b.Cost), tok(b.TotalTokens()))
	}
	line("today", today)
	line("month to date", mtd)
	line("last 30 days", last30)
	line("all time", d.all)

	all := d.all
	tt := float64(all.TotalTokens())
	if tt > 0 {
		p := func(n int64) string { return pct(float64(n) / tt) }
		fmt.Printf("\ntokens by type (all time):\n")
		fmt.Printf("  cache reads %s (%s) · cache writes %s (%s) · fresh input %s (%s) · output %s (%s)\n",
			tok(all.CacheRead), p(all.CacheRead), tok(all.CacheWrite), p(all.CacheWrite),
			tok(all.Input), p(all.Input), tok(all.Output), p(all.Output))
	}
	if all.CostNoCache > all.Cost && all.Cost > 0 {
		fmt.Printf("  without cache pricing this would be %s — caching saved %s\n",
			money(all.CostNoCache), pct(1-all.Cost/all.CostNoCache))
	}

	fmt.Printf("\nby model (all time):\n")
	for _, g := range d.models {
		cost := money(g.Cost)
		if g.UnpricedTokens > 0 && g.Cost == 0 {
			cost = "unpriced"
		}
		fmt.Printf("  %-32s %10s %10s tok  in %s · out %s · cacheRd %s · hit %s\n",
			g.Key, cost, tok(g.TotalTokens()), tok(g.Input), tok(g.Output), tok(g.CacheRead), pct(g.CacheHitRate()))
	}

	fmt.Printf("\nby month:\n")
	months := d.months
	for i := len(months) - 1; i >= 0; i-- {
		g := months[i]
		mark := " "
		if d.partialMonth(g.Key) {
			mark = "*"
		}
		fmt.Printf("  %s%s %10s %10s tok · hit %s\n", g.Key, mark, money(g.Cost), tok(g.TotalTokens()), pct(g.CacheHitRate()))
	}
	if note := d.coverageNote(); note != "" {
		fmt.Printf("  %s\n", note)
	}

	if len(res.Errors) > 0 {
		fmt.Printf("\n%d files had errors:\n  %s\n", len(res.Errors), strings.Join(res.Errors, "\n  "))
	}
	return nil
}
