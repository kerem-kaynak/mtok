package aggregate

import (
	"testing"
	"time"

	"github.com/kerem-kaynak/mtok/internal/pricing"
	"github.com/kerem-kaynak/mtok/internal/usage"
)

func mkRows() []usage.Row {
	t0 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.Local)
	return []usage.Row{
		{Time: t0, Source: usage.SourceClaude, Model: "claude-fable-5", Project: "/p1", Session: "s1",
			Input: 100, CacheRead: 900, CacheW5m: 50, Output: 20},
		{Time: t0.Add(time.Hour), Source: usage.SourceClaude, Model: "claude-fable-5", Project: "/p1", Session: "s1",
			Input: 200, CacheRead: 800, Output: 40},
		{Time: t0.AddDate(0, 0, 1), Source: usage.SourceCodex, Model: "gpt-5.5", Project: "/p2", Session: "s2",
			Input: 500, CacheRead: 500, Output: 100, Reasoning: 30},
		{Time: t0.AddDate(0, 0, 1).Add(time.Minute), Source: usage.SourceCodex, Model: "mystery-model", Project: "/p2", Session: "s2",
			Input: 1000, Output: 100},
	}
}

func TestByAndTotals(t *testing.T) {
	tbl := pricing.Defaults()
	rows := mkRows()

	days := By(rows, tbl, DayKey)
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2", len(days))
	}
	models := By(rows, tbl, ModelKey)
	if len(models) != 3 {
		t.Fatalf("got %d models, want 3", len(models))
	}

	all := Totals(rows, tbl)
	if all.Calls != 4 || all.Input != 1800 || all.CacheRead != 2200 || all.Output != 260 || all.Reasoning != 30 {
		t.Errorf("totals wrong: %+v", all)
	}
	if all.UnpricedTokens != 1100 {
		t.Errorf("UnpricedTokens = %d, want 1100 (mystery-model)", all.UnpricedTokens)
	}
	if all.Cost <= 0 {
		t.Error("priced rows should contribute cost")
	}
	// Cache reads are billed at a discount, so pricing the same tokens
	// without cache tiers must cost strictly more here.
	if all.CostNoCache <= all.Cost {
		t.Errorf("CostNoCache %v should exceed Cost %v with cache reads present", all.CostNoCache, all.Cost)
	}
}

func TestCacheHitRate(t *testing.T) {
	b := Bucket{Input: 100, CacheRead: 900, CacheWrite: 0}
	if got := b.CacheHitRate(); got != 0.9 {
		t.Errorf("CacheHitRate = %v, want 0.9", got)
	}
	var zero Bucket
	if zero.CacheHitRate() != 0 {
		t.Error("empty bucket hit rate should be 0")
	}
}

func TestFilter(t *testing.T) {
	rows := mkRows()
	from := time.Date(2026, 8, 2, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 8, 3, 0, 0, 0, 0, time.Local)
	got := Filter(rows, from, to)
	if len(got) != 2 {
		t.Fatalf("Filter returned %d rows, want 2", len(got))
	}
	for _, r := range got {
		if r.Time.Before(from) || !r.Time.Before(to) {
			t.Errorf("row outside window: %v", r.Time)
		}
	}
	if len(Filter(rows, to, to.AddDate(0, 0, 5))) != 0 {
		t.Error("empty window should return no rows")
	}
}
