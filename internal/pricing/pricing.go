// Package pricing maps model IDs from any provider to list-price rates and
// computes estimated API-equivalent cost per usage row.
package pricing

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/kerem-kaynak/mtok/internal/usage"
)

// Rates are USD per million tokens.
type Rates struct {
	Input        float64
	Output       float64
	CacheRead    float64
	CacheWrite5m float64
	CacheWrite1h float64
}

// Table maps a normalized model id to its rates.
type Table map[string]Rates

func anthropic(in, out float64) Rates {
	return Rates{Input: in, Output: out, CacheRead: 0.1 * in, CacheWrite5m: 1.25 * in, CacheWrite1h: 2 * in}
}

// openai builds OpenAI-style rates. The cached-input price is explicit
// because OpenAI's cached discount varies by family (0.5x of input on
// 4o/o1-era models, 0.25x on 4.1/o3-era, 0.1x from GPT-5 on). Cache writes
// are not billed unless the caller sets CacheWrite5m.
func openai(in, cached, out float64) Rates {
	return Rates{Input: in, Output: out, CacheRead: cached}
}

// openaiBilledWrites: the GPT-5.5/5.6 families bill cache writes at 1.25x
// input (earlier OpenAI models don't bill writes at all).
func openaiBilledWrites(in, cached, out float64) Rates {
	r := openai(in, cached, out)
	r.CacheWrite5m = 1.25 * in
	return r
}

// Defaults returns the bundled list-price table (checked 2026-08-17).
// Override any entry via the config file; see README.
func Defaults() Table {
	return Table{
		// Anthropic. No long-context surcharge: Anthropic removed the >200K
		// premium tier in March 2026; 1M-window models bill flat. (The retired
		// Sonnet 4.5 1M *beta* premium is not modeled.)
		"claude-fable-5":  anthropic(10, 50),
		"claude-mythos-5": anthropic(10, 50),
		"claude-opus-5":   anthropic(5, 25),
		"claude-opus-4-8": anthropic(5, 25),
		"claude-opus-4-7": anthropic(5, 25),
		"claude-opus-4-6": anthropic(5, 25),
		"claude-opus-4-5": anthropic(5, 25),
		"claude-opus-4-1": anthropic(15, 75),
		"claude-opus-4":   anthropic(15, 75),
		"claude-3-opus":   anthropic(15, 75),
		// Sonnet 5's $2/$10 introductory price was made permanent 2026-08-10.
		"claude-sonnet-5":   anthropic(2, 10),
		"claude-sonnet-4-6": anthropic(3, 15),
		"claude-sonnet-4-5": anthropic(3, 15),
		"claude-sonnet-4":   anthropic(3, 15),
		"claude-3-7-sonnet": anthropic(3, 15),
		"claude-3-5-sonnet": anthropic(3, 15),
		"claude-haiku-4-5":  anthropic(1, 5),
		"claude-3-5-haiku":  anthropic(0.8, 4),
		"claude-3-haiku":    anthropic(0.25, 1.25),
		// OpenAI. Cache writes are billed (1.25x input, as the total rate for
		// written tokens) only from the GPT-5.6 family on; earlier models
		// write for free. The >272K long-context surcharge on gpt-5.4+ is not
		// modeled: Codex logs yield per-event deltas, not per-request context
		// sizes, so the per-request threshold can't be applied faithfully.
		"gpt-5.6-sol":   openaiBilledWrites(5, 0.5, 30),
		"gpt-5.6":       openaiBilledWrites(5, 0.5, 30), // alias routes to Sol
		"daybreak-blue": openaiBilledWrites(5, 0.5, 30), // alias of gpt-5.6-sol
		"gpt-5.6-terra": openaiBilledWrites(2, 0.2, 12),
		"gpt-5.6-luna":  openaiBilledWrites(0.2, 0.02, 1.2),
		"gpt-5.6-cyber": openaiBilledWrites(12.5, 1.25, 75),
		"daybreak-red":  openaiBilledWrites(12.5, 1.25, 75), // alias of gpt-5.6-cyber
		"gpt-5.5":       openai(5, 0.5, 30),
		"gpt-5.5-cyber": openai(12.5, 1.25, 75),
		"gpt-5.4":       openai(2.5, 0.25, 15),
		"gpt-5.4-mini":  openai(0.75, 0.075, 4.5),
		"gpt-5.4-nano":  openai(0.2, 0.02, 1.25),
		"gpt-5.3-codex": openai(1.75, 0.175, 14),
		"gpt-5.2":       openai(1.75, 0.175, 14),
		"gpt-5.2-pro":   openai(21, 21, 168), // no cached-input discount
		"gpt-5.1":       openai(1.25, 0.125, 10),
		"gpt-5-codex":   openai(1.25, 0.125, 10),
		"gpt-5-pro":     openai(15, 15, 120), // no cached-input discount
		"gpt-5-mini":    openai(0.25, 0.025, 2),
		"gpt-5-nano":    openai(0.05, 0.005, 0.4),
		"gpt-5":         openai(1.25, 0.125, 10),
		"gpt-4.1":       openai(2, 0.5, 8),
		"gpt-4.1-mini":  openai(0.4, 0.1, 1.6),
		"gpt-4.1-nano":  openai(0.1, 0.025, 0.4),
		"gpt-4o":        openai(2.5, 1.25, 10),
		"gpt-4o-mini":   openai(0.15, 0.075, 0.6),
		"codex-mini":    openai(1.5, 0.375, 6),
		"o3":            openai(2, 0.5, 8),
		"o3-pro":        openai(20, 20, 80), // no cached-input discount
		"o3-mini":       openai(1.1, 0.55, 4.4),
		"o4-mini":       openai(1.1, 0.275, 4.4),
		"o1":            openai(15, 7.5, 60),
	}
}

var (
	reBedrockVer = regexp.MustCompile(`-v\d+(:\d+)*$`)
	reDated      = regexp.MustCompile(`-(19|20)\d{6}$`)
)

// Normalize reduces a provider-specific model id to its canonical form:
// Bedrock ("us.anthropic.claude-sonnet-4-5-20250929-v2:0"), Vertex
// ("publishers/anthropic/models/claude-opus-4@20250514"), dated first-party
// ids and "-latest" aliases all collapse to the same key.
func Normalize(id string) string {
	m := strings.ToLower(strings.TrimSpace(id))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	for _, p := range []string{"us.", "eu.", "apac.", "global.", "jp.", "au."} {
		if strings.HasPrefix(m, p) {
			m = m[len(p):]
			break
		}
	}
	for _, p := range []string{"anthropic.", "openai.", "azure."} {
		if strings.HasPrefix(m, p) {
			m = m[len(p):]
			break
		}
	}
	if i := strings.IndexByte(m, '@'); i >= 0 {
		m = m[:i]
	}
	m = reBedrockVer.ReplaceAllString(m, "")
	m = strings.TrimSuffix(m, "-latest")
	m = reDated.ReplaceAllString(m, "")
	return m
}

// Lookup resolves a raw model id to rates. Falls back to the longest table
// key that is a boundary-respecting prefix of the normalized id (so
// "gpt-5.5-codex" prices as "gpt-5.5"). '.' is not a boundary: a version
// bump ("gpt-5.7" vs "gpt-5") is a different price, so it stays unpriced
// rather than silently billing at the old rate. ok=false means unpriced;
// tokens still count and the model shows as such.
func (t Table) Lookup(raw string) (rates Rates, norm string, ok bool) {
	n := Normalize(raw)
	if r, hit := t[n]; hit {
		return r, n, true
	}
	best := ""
	for k := range t {
		if len(k) > len(best) && len(n) > len(k) && strings.HasPrefix(n, k) {
			switch n[len(k)] {
			case '-', ':':
				best = k
			}
		}
	}
	if best != "" {
		return t[best], n, true
	}
	return Rates{}, n, false
}

// Cost returns the estimated API-equivalent cost of a row in USD.
// ok=false means the model has no rates (tokens still count; cost unknown).
// Fast-mode turns bill at 2x list rates across all token kinds.
func (t Table) Cost(r *usage.Row) (float64, bool) {
	rates, _, ok := t.Lookup(r.Model)
	if !ok {
		return 0, false
	}
	const m = 1e6
	c := float64(r.Input)/m*rates.Input +
		float64(r.CacheRead)/m*rates.CacheRead +
		float64(r.CacheW5m)/m*rates.CacheWrite5m +
		float64(r.CacheW1h)/m*rates.CacheWrite1h +
		float64(r.Output)/m*rates.Output
	if r.Fast {
		c *= 2
	}
	return c, true
}

// CostNoCache prices the same row as if caching didn't exist: every prompt
// token at the full input rate. The gap to Cost is what caching saved.
func (t Table) CostNoCache(r *usage.Row) (float64, bool) {
	rates, _, ok := t.Lookup(r.Model)
	if !ok {
		return 0, false
	}
	const m = 1e6
	c := float64(r.Input+r.CacheRead+r.CacheW5m+r.CacheW1h)/m*rates.Input +
		float64(r.Output)/m*rates.Output
	if r.Fast {
		c *= 2
	}
	return c, true
}

// ---- user config ----

// RatesConfig is the JSON shape of one pricing override. Omitted cache rates
// default from Input: read = 0.1x for every model; writes default to the
// standard multipliers (5m = 1.25x, 1h = 2x) for claude-* keys and to 0 for
// everything else — set them explicitly if your provider bills cache writes.
type RatesConfig struct {
	Input        float64  `json:"input"`
	Output       float64  `json:"output"`
	CacheRead    *float64 `json:"cache_read"`
	CacheWrite5m *float64 `json:"cache_write_5m"`
	CacheWrite1h *float64 `json:"cache_write_1h"`
}

// Config is ~/.config/mtok/config.json.
type Config struct {
	ClaudeDirs []string               `json:"claude_dirs"`
	CodexDirs  []string               `json:"codex_dirs"`
	Pricing    map[string]RatesConfig `json:"pricing"`
}

// LoadConfig reads the config file; a missing file returns an empty config.
func LoadConfig(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

// Apply merges pricing overrides into the table (keys are normalized).
func (t Table) Apply(overrides map[string]RatesConfig) {
	for k, rc := range overrides {
		key := Normalize(k)
		r := Rates{Input: rc.Input, Output: rc.Output}
		r.CacheRead = 0.1 * rc.Input
		if rc.CacheRead != nil {
			r.CacheRead = *rc.CacheRead
		}
		if strings.HasPrefix(key, "claude") {
			r.CacheWrite5m = 1.25 * rc.Input
			r.CacheWrite1h = 2 * rc.Input
		}
		if rc.CacheWrite5m != nil {
			r.CacheWrite5m = *rc.CacheWrite5m
		}
		if rc.CacheWrite1h != nil {
			r.CacheWrite1h = *rc.CacheWrite1h
		}
		t[key] = r
	}
}
