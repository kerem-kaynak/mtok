# mtok

[![CI](https://github.com/kerem-kaynak/mtok/actions/workflows/ci.yml/badge.svg)](https://github.com/kerem-kaynak/mtok/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/kerem-kaynak/mtok.svg)](https://pkg.go.dev/github.com/kerem-kaynak/mtok)

A terminal dashboard for AI token usage and spend, computed **entirely from
local session logs** of [Claude Code](https://code.claude.com) and
[Codex](https://github.com/openai/codex). No network calls, no accounts, no
telemetry — it reads the JSONL transcripts those tools already write to disk
and turns them into charts, drill-downs, and dollar estimates.

Works no matter how you pay: subscription (Pro/Max/Plus), API key, or an
enterprise gateway (Bedrock, Vertex, Azure/Foundry). Model IDs are normalized
across providers (`us.anthropic.claude-sonnet-4-5-20250929-v1:0` and
`claude-sonnet-4-5` are the same model), and costs are **estimated
API-equivalent list prices** — what the same tokens would have cost at
pay-as-you-go rates. For subscription users that's your "value extracted"
number, not a bill.

```
╭──────────────╮╭──────────────╮╭──────────────╮╭──────────────╮
│ TODAY        ││ MONTH TO DATE││ LAST 30 DAYS ││ ALL TIME*    │
│ $40.65       ││ $1,546       ││ $3,397       ││ $3,449       │
│ 27.52M tok   ││ 802.59M tok  ││ 1.72B tok    ││ 1.78B tok    │
╰──────────────╯╰──────────────╯╰──────────────╯╰──────────────╯
 Daily cost — last 30 days          Cache hit — 30 days
 $394│      ██       ▇▇ ██          ███████████████████░░ 92%
     │      ██       ██ ██ ▂▂       Model mix — 30 days
     │▅▅ ▅▅ ██ ▁▁ ██ ██ ██ ██ …     ██████████████████ ████ █ █
 $0.0┴────────────────────────      ■ claude-fable-5 $2,864 (84%)
```

## Features

- **Dashboard** — today / month-to-date / last-30-days / all-time KPI tiles
  with day-over-day deltas, a 30-day daily cost bar chart, cache-hit meter
  (with what the tokens would have cost without caching), tokens-by-type
  split, Claude-vs-Codex source split, and model-mix share bar
- **Cost ⇄ tokens** — press `t` to flip every chart and KPI to raw token
  counts; token-heavy sessions look very different from cost-heavy ones
  because cache reads are billed at a fraction of the input rate
- **Daily & Monthly** drill-downs — every day/month with cost, tokens, cache
  hit rate, and a detail pane (per-month daily chart, top models by cost)
- **Models** — all-time cost, share, token breakdown (input / output / cache
  read / cache write), hit rate, and calls per model
- **Projects** — spend per working directory, session counts, last active
- **Sessions** — the 500 most recent sessions with duration, model(s), and cost
- **Fast** — incremental parse cache keyed by file size+mtime; a warm rescan
  of ~1,100 files (~450 MB of logs) takes under 0.2 s
- **Correct** — see [Accuracy](#accuracy); the calculation path is the point
  of this tool

## Install

```sh
brew install kerem-kaynak/tap/mtok
```

Or with Go:

```sh
go install github.com/kerem-kaynak/mtok/cmd/mtok@latest
```

Or from a checkout:

```sh
git clone https://github.com/kerem-kaynak/mtok && cd mtok
go build -o mtok ./cmd/mtok
```

Requires Go 1.24+. Runs anywhere Claude Code or Codex runs.

## Usage

```sh
mtok                 # interactive TUI
mtok --summary       # plain-text report (for scripts / quick looks)
mtok --no-cache      # reparse everything, ignore the parse cache
mtok --claude-dir ~/.claude --codex-dir ~/.codex   # repeatable overrides
```

### Keys

| Key | Action |
| --- | --- |
| `1`–`6` | jump to Dashboard / Daily / Monthly / Models / Projects / Sessions |
| `tab` / `shift+tab`, `l` / `h` | next / previous view |
| `↑` `↓` | move the table cursor (detail panes follow it) |
| `t` | toggle the chart metric between cost and token count |
| `r` | rescan logs |
| `q` | quit |

## Configuration

Optional, at `~/.config/mtok/config.json`:

```json
{
  "claude_dirs": ["~/.claude"],
  "codex_dirs": ["~/.codex"],
  "pricing": {
    "my-fine-tune": { "input": 4.0, "output": 16.0, "cache_read": 0.4 },
    "claude-opus-5": { "input": 5.0, "output": 25.0 }
  }
}
```

- `pricing` keys are model IDs (normalized the same way logged IDs are —
  region/vendor prefixes, `-latest`, date stamps, and `-v1:0`-style suffixes
  are stripped). Rates are **USD per million tokens**. Overrides win over the
  bundled defaults.
- `cache_read`, `cache_write_5m`, `cache_write_1h` are optional. `cache_read`
  defaults to 0.1× `input`. The write rates default to the standard Anthropic
  multipliers (1.25× / 2× of `input`) on `claude-*` keys and to **0** on
  everything else — set them explicitly if your provider bills cache writes.
- Models in the logs with no rates show as **—** in the Models view and are
  counted in an "unpriced tokens" note rather than silently priced at zero or
  at a guessed rate.
- Bundled defaults cover current Anthropic and OpenAI list prices (Claude 3 →
  Fable 5, GPT-4o → GPT-5.6), including per-family cache-read rates and
  cache-write billing rules.

## Accuracy

mtok exists to get these numbers right, so the method is documented and the
edge cases are handled deliberately. The calculation path was audited against
official Anthropic/OpenAI pricing pages, the `openai/codex` source, and how
[ccusage](https://github.com/ccusage/ccusage) computes the same numbers.

**Claude Code parsing.** One row per assistant message with a `usage` block.
Streaming writes the *same* API response several times with growing
`output_tokens`, resumed sessions copy history into new files, and subagent
transcripts repeat parent turns — so rows are deduplicated globally by
`message.id` + `requestId`, keeping the copy with the **largest** usage
(early copies are partial) via a strict total order, which makes every number
independent of scan order. `<synthetic>` rows (Claude Code error placeholders,
not API calls) are excluded. Cache writes are split into 5-minute and 1-hour
tiers when the log has the breakdown (all-5m otherwise). Fast-mode turns
(`usage.speed: "fast"`) are billed at 2× list rates.

**Codex parsing.** `token_count` events carry cumulative session totals; mtok
takes per-event deltas, clamped at zero per field — which also absorbs the one
case where Codex clobbers its running counters (on context-window-exceeded
errors). Compaction does *not* reset Codex totals, so deltas stay exact across
compactions. Codex's `input_tokens` *includes* cached and cache-write tokens;
mtok decomposes it so each token is priced exactly once, at its own tier. The
active model tracks both `turn_context` and `thread_settings_applied` events.

**Pricing.** List prices per million tokens, cache tiers priced separately:
Anthropic at the standard multipliers (0.1× read, 1.25× 5m write, 2× 1h
write — flat across the full 1M context window since Anthropic removed the
long-context premium in March 2026); OpenAI cached input at the published
per-family rate (0.5× on 4o/o1-era, 0.25× on 4.1/o3-era, 0.1× from GPT-5 on),
with cache writes billed only from the GPT-5.6 family on (1.25× input).
Reasoning/thinking tokens are a subset of output and are **never** added to
totals — they're shown as an informational count. Everything is aggregated in
your local timezone.

**Retention.** Session logs have TTLs — Claude Code deletes transcripts after
`cleanupPeriodDays` (default 30), and other tools delete their own probe
sessions. Once mtok has scanned a file, its rows are carried forward in the
parse cache even after the file is deleted, so numbers never move backwards
between scans. The all-time tile carries an asterisk because logs deleted
*before* mtok first saw them are gone for good. Tip: raise
`"cleanupPeriodDays"` in `~/.claude/settings.json` to keep history.

### Known limits

- Costs are **estimates at current list prices** — not invoices. Batch
  discounts, negotiated/enterprise rates, and historical price changes are not
  modeled; the `costUSD` field some old logs carry is ignored in favor of
  computing from tokens, so all rows are priced consistently.
- OpenAI's >272K long-context surcharge (GPT-5.4+) is not modeled: Codex logs
  yield per-event deltas, not per-request context sizes, so the per-request
  threshold can't be applied faithfully.
- The retired Sonnet 4.5 1M-*beta* long-context premium is not modeled.
- Anything a tool never wrote to disk (or deleted before the first scan)
  can't be counted.

The parse cache lives at `~/.cache/mtok/scan.gob`; deleting it is always safe
(it costs one cold rescan and any deleted-file retention carryover).

## Development

```sh
go test ./...   # includes a full-app render test at 120×40, 80×24, 60×20
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for layout, correctness invariants, and
how to submit pricing updates. Release notes live in
[CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © Kerem Ali Kaynak
