// Package usage defines the normalized usage event shared by all log sources.
package usage

import "time"

// Source identifies which local tool produced a row.
const (
	SourceClaude = "claude"
	SourceCodex  = "codex"
)

// Row is one billed API turn extracted from a local session log.
//
// Token fields are normalized across sources so that they never overlap:
// Input is uncached, non-cache-write input; CacheRead / CacheW5m / CacheW1h
// are disjoint from it. (Codex logs report input_tokens inclusive of cached
// and cache-write tokens; the parser subtracts them. Claude logs already
// report them separately.) Output includes reasoning/thinking tokens;
// Reasoning is the informational subset (Codex reasoning_output_tokens,
// Claude output_tokens_details.thinking_tokens).
type Row struct {
	Time      time.Time
	Source    string
	Provider  string // e.g. "azure"; empty when the log doesn't record one
	Model     string // raw model id as logged
	Project   string // session working directory
	Session   string
	DedupKey  uint64 // non-zero when the source can emit duplicate usage lines
	Input     int64
	CacheRead int64
	CacheW5m  int64 // 5-minute-TTL cache writes (Claude); all cache writes (Codex)
	CacheW1h  int64 // 1-hour-TTL cache writes (Claude only)
	Output    int64
	Reasoning int64
	Fast      bool // Claude fast mode (usage.speed == "fast"): billed at 2x list rates
}

// TotalTokens is every token the turn consumed or produced.
func (r *Row) TotalTokens() int64 {
	return r.Input + r.CacheRead + r.CacheW5m + r.CacheW1h + r.Output
}

// PromptTokens is everything sent to the model.
func (r *Row) PromptTokens() int64 {
	return r.Input + r.CacheRead + r.CacheW5m + r.CacheW1h
}
