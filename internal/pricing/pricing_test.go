package pricing

import (
	"testing"

	"github.com/kerem-kaynak/mtok/internal/usage"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"claude-fable-5":                                     "claude-fable-5",
		"claude-sonnet-4-5-20250929":                         "claude-sonnet-4-5",
		"anthropic.claude-3-5-sonnet-20241022-v2:0":          "claude-3-5-sonnet",
		"us.anthropic.claude-opus-4-6-v1:0":                  "claude-opus-4-6",
		"publishers/anthropic/models/claude-opus-4@20250514": "claude-opus-4",
		"claude-3-5-sonnet-latest":                           "claude-3-5-sonnet",
		"GPT-5.6-Sol":                                        "gpt-5.6-sol",
		"openai.gpt-5.5":                                     "gpt-5.5",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLookupPrefixFallback(t *testing.T) {
	tbl := Defaults()
	if _, norm, ok := tbl.Lookup("gpt-5.5-codex"); !ok || norm != "gpt-5.5-codex" {
		t.Errorf("gpt-5.5-codex should price via gpt-5.5 prefix, ok=%v norm=%q", ok, norm)
	}
	r1, _, _ := tbl.Lookup("gpt-5.5-codex")
	r2, _, _ := tbl.Lookup("gpt-5.5")
	if r1 != r2 {
		t.Errorf("prefix fallback returned different rates: %+v vs %+v", r1, r2)
	}
	// Boundary check: gpt-5.51 must NOT match gpt-5.5 (or gpt-5).
	if _, _, ok := tbl.Lookup("gpt-5x51"); ok {
		t.Error("gpt-5x51 should be unpriced")
	}
	if _, _, ok := tbl.Lookup("totally-unknown-model"); ok {
		t.Error("unknown model should be unpriced")
	}
	// '.' is not a boundary: an unknown version must stay unpriced rather
	// than silently billing at an older version's rate.
	if _, _, ok := tbl.Lookup("gpt-5.9"); ok {
		t.Error("gpt-5.9 should be unpriced, not fall back to gpt-5")
	}
	// Cheaper variants must have their own entries, not the sibling's rate.
	mini, _, _ := tbl.Lookup("gpt-4o-mini")
	full, _, _ := tbl.Lookup("gpt-4o")
	if mini.Input >= full.Input {
		t.Error("gpt-4o-mini must not price at gpt-4o rates")
	}
}

func TestFastModeDoublesCost(t *testing.T) {
	tbl := Defaults()
	r := usage.Row{Model: "claude-opus-5", Input: 1_000_000, Output: 1_000_000}
	base, _ := tbl.Cost(&r)
	r.Fast = true
	fast, _ := tbl.Cost(&r)
	if diff := fast - 2*base; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("fast cost = %v, want 2x base %v", fast, base)
	}
	nc, _ := tbl.CostNoCache(&r)
	r.Fast = false
	ncBase, _ := tbl.CostNoCache(&r)
	if diff := nc - 2*ncBase; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("fast no-cache cost = %v, want 2x base %v", nc, ncBase)
	}
}

func TestOpenAICacheRates(t *testing.T) {
	tbl := Defaults()
	// gpt-5.6 family bills cache writes at 1.25x input; gpt-5.5 writes free.
	sol, _, _ := tbl.Lookup("gpt-5.6-sol")
	if sol.CacheWrite5m != 6.25 || sol.CacheRead != 0.5 {
		t.Errorf("gpt-5.6-sol cache rates wrong: %+v", sol)
	}
	g55, _, _ := tbl.Lookup("gpt-5.5")
	if g55.CacheWrite5m != 0 {
		t.Errorf("gpt-5.5 must not bill cache writes: %+v", g55)
	}
	// The bare gpt-5.6 alias routes to Sol.
	alias, _, _ := tbl.Lookup("gpt-5.6")
	if alias != sol {
		t.Errorf("gpt-5.6 alias should match gpt-5.6-sol: %+v vs %+v", alias, sol)
	}
	// Older families keep their published cached-input rates (not 0.1x).
	g4o, _, _ := tbl.Lookup("gpt-4o")
	if g4o.CacheRead != 1.25 {
		t.Errorf("gpt-4o cached input should be 0.5x = $1.25: %+v", g4o)
	}
}

func TestCost(t *testing.T) {
	tbl := Defaults()
	r := usage.Row{Model: "claude-fable-5", Input: 1_000_000, CacheRead: 10_000_000, CacheW5m: 1_000_000, CacheW1h: 500_000, Output: 200_000}
	got, ok := tbl.Cost(&r)
	if !ok {
		t.Fatal("expected priced")
	}
	// 1M*$10 + 10M*$1 + 1M*$12.50 + 0.5M*$20 + 0.2M*$50 = 10+10+12.5+10+10 = 52.5
	want := 52.5
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("Cost = %v, want %v", got, want)
	}
}

func TestApplyOverrides(t *testing.T) {
	tbl := Defaults()
	cr := 0.25
	tbl.Apply(map[string]RatesConfig{
		"My-Model-20250101": {Input: 2, Output: 8, CacheRead: &cr},
		"claude-custom":     {Input: 4, Output: 20},
	})
	// Non-claude override: cache writes default to 0 (set explicitly if
	// your provider bills them); read defaults to 0.1x input unless given.
	r, norm, ok := tbl.Lookup("my-model-20250101")
	if !ok || norm != "my-model" {
		t.Fatalf("override not applied: ok=%v norm=%q", ok, norm)
	}
	if r.Input != 2 || r.Output != 8 || r.CacheRead != 0.25 || r.CacheWrite5m != 0 || r.CacheWrite1h != 0 {
		t.Errorf("unexpected rates: %+v", r)
	}
	// claude-* override: writes default to the standard 1.25x / 2x.
	c, _, ok := tbl.Lookup("claude-custom")
	if !ok || c.CacheRead != 0.4 || c.CacheWrite5m != 5 || c.CacheWrite1h != 8 {
		t.Errorf("claude override cache defaults wrong: ok=%v %+v", ok, c)
	}
}
