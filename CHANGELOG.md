# Changelog

All notable changes to mtok are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org).

## [0.3.1] — 2026-08-17

### Changed
- **Homebrew now installs prebuilt binaries** instead of compiling from
  source — `brew install kerem-kaynak/tap/mtok` no longer pulls in a Go
  toolchain. Releases are built by goreleaser (darwin/linux × amd64/arm64),
  which also publishes the tap package.
- The tap package is now a **cask**; existing formula installs migrate
  automatically via the tap's `tap_migrations.json`.
- `--version` is stamped from the release tag at build time.

## [0.3.0] — 2026-08-17

Pricing-accuracy release: the bundled rate table and both parsers were audited
against official Anthropic/OpenAI pricing pages, the `openai/codex` source, and
ccusage's calculation method.

### Fixed
- **Claude Sonnet 5 priced at $2/$10 per MTok** — Anthropic made the
  introductory price permanent on 2026-08-10; the table had the pre-announcement
  $3/$15.
- **OpenAI cached-input rates are now the published per-family prices** instead
  of a flat 0.1× of input: 0.5× on 4o/o1-era models, 0.25× on 4.1/o3-era, 0.1×
  from GPT-5 on. Previously e.g. `gpt-4o` cache reads were billed at $0.25/MTok
  instead of the correct $1.25/MTok.
- **GPT-5.5 no longer bills cache writes.** OpenAI bills cache writes (at 1.25×
  input, as the total rate for written tokens) only from the GPT-5.6 family on;
  earlier models write for free.
- **Unknown model versions stay unpriced instead of inheriting an older
  version's rate**: `.` is no longer a prefix-fallback boundary, so a future
  `gpt-5.9` shows as unpriced rather than silently billing at `gpt-5` rates.
  Explicit entries were added for cheaper variants (`gpt-4o-mini`,
  `gpt-4.1-mini`/`-nano`, `o3-mini`, …) so they can't inherit their bigger
  sibling's price through the `-` fallback either.
- **Pricing overrides for non-`claude-*` models no longer get a cache-write
  default**: omitted `cache_write_5m`/`cache_write_1h` now default to the
  standard multipliers only on `claude-*` keys and to 0 elsewhere, matching the
  documented behavior.

### Added
- **Claude fast mode**: rows with `usage.speed == "fast"` are billed at 2×
  list rates.
- **Claude thinking tokens**: `output_tokens_details.thinking_tokens` now
  populates the informational reasoning count (already present for Codex);
  totals are unchanged since reasoning is a subset of output.
- **Codex mid-session model switches** via `thread_settings_applied` events are
  now tracked (previously only `turn_context` updated the active model).
- New table entries: the `gpt-5.6` → Sol alias and `daybreak-*` aliases,
  `gpt-5.6-cyber`, `gpt-5.5-cyber`, `gpt-5.4-mini`/`-nano`, `gpt-5.3-codex`,
  `gpt-5.2`(-pro), `gpt-5-pro`, `codex-mini`, `o1`, `o3-pro`/`-mini`,
  `gpt-4o-mini`, `gpt-4.1-mini`/`-nano`.

### Changed
- Parse-cache format bumped (v2) for the new row fields; the first scan after
  upgrading re-parses everything and resets deleted-file retention carryover.
- Module path is now `github.com/kerem-kaynak/mtok`.

## [0.2.0] — 2026-08-15

### Fixed
- **Streaming duplicates were deduplicated to the wrong copy.** Claude Code
  writes the same API response several times while streaming, with growing
  `output_tokens`; keeping the first copy undercounted output tokens by up to
  ~44%. Dedup now keeps the copy with the largest usage via a strict total
  order, making results independent of file/scan order (verified: repeated cold
  scans of a frozen snapshot are byte-identical).

### Added
- **Deleted-file retention**: session logs have TTLs (Claude Code prunes
  transcripts after `cleanupPeriodDays`) and other tools delete their own probe
  sessions. Once mtok has scanned a file, its rows are carried forward in the
  parse cache even after the file disappears, so totals never move backwards
  between scans. Disclosed in the summary line and dashboard footnote;
  `--no-cache` discards retained rows.
- ALL TIME tile footnote explaining that only logs mtok has seen are counted.

## [0.1.0] — 2026-08-14

Initial release.

- Parsers for Claude Code (`~/.claude/projects/**`, including subagent
  transcripts) and Codex (`~/.codex/sessions/**`) session logs; global
  deduplication; Codex cumulative-counter deltas with inclusive-input
  decomposition.
- Bundled list-price table with provider-prefix/date normalization, per-model
  overrides via `~/.config/mtok/config.json`, cache-tier pricing (read /
  5-minute write / 1-hour write), and honest "unpriced" handling for unknown
  models.
- Six-view TUI (dashboard, daily, monthly, models, projects, sessions) with
  cost⇄token toggle, cache-savings estimate, partial-month markers, and
  coverage notes; `--summary` plain-text report.
- Incremental parse cache (size+mtime) — warm rescans of ~1,100 files in
  under 0.2s.
