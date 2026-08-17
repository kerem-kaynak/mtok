# Contributing to mtok

Thanks for helping out. mtok is deliberately small: a scanner, two parsers, a
pricing table, an aggregator, and a Bubble Tea TUI. Please keep it that way —
no network calls, no telemetry, no daemons.

## Development

```sh
go build ./cmd/mtok     # build
go test ./...           # run all tests
go vet ./...
gofmt -l .              # CI fails on unformatted files
```

Requires Go 1.24+. There are no build tags, code generation steps, or external
services; `go test ./...` runs everything, including a full-app render test
that asserts no line overflows at 120×40, 80×24, and 60×20.

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/mtok` | flag parsing, config loading, entry point |
| `internal/logs` | file discovery, the Claude/Codex parsers, parse cache, global dedup |
| `internal/usage` | the normalized `Row` type shared by all sources |
| `internal/pricing` | model-id normalization, the bundled rate table, cost math, overrides |
| `internal/aggregate` | time/key bucketing on top of rows |
| `internal/tui` | all rendering; views implement a small `view` interface |

## Correctness rules

These invariants hold everywhere; PRs that break them will be asked to change:

- **Token fields never overlap.** `Input`, `CacheRead`, `CacheW5m`, `CacheW1h`,
  and `Output` are disjoint; `Reasoning` is an informational subset of `Output`
  and is never added to totals.
- **Numbers never move backwards between scans.** Dedup is order-independent
  (a strict total order picks the surviving duplicate) and rows from deleted
  files are retained via the cache. If your change can make a rescan show a
  smaller number than the previous scan, it's a bug.
- **No silent pricing.** A model without a table entry is "unpriced" and
  reported as such — never priced at zero or at a guessed rate. This is also
  why the prefix fallback in `Lookup` refuses to cross a `.` version boundary.

## Updating prices

The table in `internal/pricing/pricing.go` holds **list prices in USD per
million tokens**. When updating or adding a model:

1. Cite the official source (Anthropic or OpenAI pricing page / announcement)
   in the PR description, with the date you checked.
2. Update the "checked YYYY-MM-DD" comment on `Defaults()`.
3. Cache rates are part of the entry: Anthropic models use the standard
   multipliers (0.1× read, 1.25× 5m write, 2× 1h write); OpenAI cached-input
   discounts vary by family and cache writes are billed only from the GPT-5.6
   family on.
4. Add or adjust a test in `pricing_test.go` if the change is behavioral
   (new family, new billing rule) rather than a plain rate update.

## Adding a log source

A new source needs: a parser in `internal/logs` producing `usage.Row`s (set
`DedupKey` if the source can repeat usage lines), discovery wiring in
`scan.go`, and fixture-based tests like `logs_test.go`. Parsers must tolerate
torn/foreign lines silently — session logs are written concurrently by other
programs.

## Pull requests

- Keep PRs focused; separate refactors from behavior changes.
- Match the existing code style (plain Go, comments explain *why*, table
  tests).
- `gofmt`, `go vet`, and `go test ./...` must pass — CI runs them on Linux
  and macOS.
- Breaking the parse-cache format is fine when a row field changes — bump
  `cacheVersion` in `scan.go` and note it in the changelog (it costs users one
  cold rescan and resets deleted-file retention).

## Reporting bugs

Numbers you believe are wrong are the most valuable reports. Please include:
`mtok --version`, the summary line (`scanned N files … duplicates removed`),
and if possible a redacted JSONL snippet that reproduces the miscount.
