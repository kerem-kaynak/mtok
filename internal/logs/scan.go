package logs

import (
	"encoding/gob"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kerem-kaynak/mtok/internal/usage"
)

// Options configures a scan.
type Options struct {
	ClaudeDirs []string                // roots containing projects/*/*.jsonl (default ~/.claude)
	CodexDirs  []string                // roots containing sessions/**/*.jsonl and archived_sessions/*.jsonl (default ~/.codex)
	CacheFile  string                  // per-file parse cache; empty disables caching
	Progress   func(done, total int64) // optional, called from worker goroutines
}

// Result is the merged, deduplicated output of a scan.
type Result struct {
	Rows       []usage.Row // sorted by time ascending
	Files      int
	FromCache  int
	Retained   int // deleted files whose rows persist from the cache
	Duplicates int // rows removed by global dedup (streaming/resume copies)
	Errors     []string
}

const cacheVersion = 2 // v2: usage.Row gained Fast + Claude Reasoning

type cacheEntry struct {
	Source  string
	Size    int64
	ModTime int64 // ns
	Rows    []usage.Row
}

type cacheFile struct {
	Version int
	Files   map[string]cacheEntry
}

type sourceFile struct {
	path   string
	source string
}

// Scan discovers all session logs, parses changed files concurrently (files
// unchanged by size+mtime come from the cache), refreshes the cache, and
// returns rows deduplicated globally by DedupKey.
func Scan(opts Options) (*Result, error) {
	files, err := discover(opts)
	if err != nil {
		return nil, err
	}

	cached := loadCache(opts.CacheFile)
	res := &Result{Files: len(files)}
	fresh := cacheFile{Version: cacheVersion, Files: make(map[string]cacheEntry, len(files))}

	var (
		mu       sync.Mutex
		done     atomic.Int64
		total    = int64(len(files))
		jobs     = make(chan sourceFile)
		wg       sync.WaitGroup
		nWorkers = min(runtime.NumCPU(), 8)
	)
	for range nWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				entry, fromCache, perr := parseOne(job, cached)
				mu.Lock()
				if perr != nil {
					// A file deleted between discovery and parse is not an
					// error — the retention pass below keeps its last rows.
					if !os.IsNotExist(perr) {
						res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", job.path, perr))
					}
				} else {
					fresh.Files[job.path] = entry
					if fromCache {
						res.FromCache++
					}
				}
				mu.Unlock()
				if opts.Progress != nil {
					opts.Progress(done.Add(1), total)
				}
			}
		}()
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()

	// Retention: session logs have TTLs (Claude Code deletes transcripts
	// after cleanupPeriodDays) and other tools delete their own probe
	// sessions. Once mtok has parsed a file, its rows outlive it — carry
	// cache entries for vanished files forward so totals never move
	// backwards. (--no-cache disables the cache and with it retention.)
	// Codex moves completed rollouts from sessions/ to archived_sessions/;
	// when the archived copy is live, do not also retain its old cache entry.
	liveCodexRollouts := make(map[string]struct{})
	for path, e := range fresh.Files {
		if cacheEntrySource(e) == usage.SourceCodex {
			liveCodexRollouts[codexRolloutKey(path)] = struct{}{}
		}
	}
	for path, e := range cached {
		if _, live := fresh.Files[path]; live {
			continue
		}
		if cacheEntrySource(e) == usage.SourceCodex {
			if _, moved := liveCodexRollouts[codexRolloutKey(path)]; moved {
				continue
			}
		}
		if _, serr := os.Stat(path); serr == nil {
			continue // still on disk; parse failed above and was reported
		}
		fresh.Files[path] = e
		res.Retained++
	}

	saveCache(opts.CacheFile, fresh)

	for _, e := range fresh.Files {
		res.Rows = append(res.Rows, e.Rows...)
	}
	res.Rows, res.Duplicates = dedup(res.Rows)
	sort.Slice(res.Rows, func(i, j int) bool { return res.Rows[i].Time.Before(res.Rows[j].Time) })
	return res, nil
}

func parseOne(job sourceFile, cached map[string]cacheEntry) (cacheEntry, bool, error) {
	st, err := os.Stat(job.path)
	if err != nil {
		return cacheEntry{}, false, err
	}
	if c, ok := cached[job.path]; ok && c.Size == st.Size() && c.ModTime == st.ModTime().UnixNano() {
		return c, true, nil
	}
	var rows []usage.Row
	switch job.source {
	case usage.SourceClaude:
		rows, err = parseClaudeFile(job.path)
	case usage.SourceCodex:
		rows, err = parseCodexFile(job.path)
	}
	if err != nil {
		return cacheEntry{}, false, err
	}
	return cacheEntry{Source: job.source, Size: st.Size(), ModTime: st.ModTime().UnixNano(), Rows: rows}, false, nil
}

// cacheEntrySource keeps caches written before cacheEntry.Source was added
// usable. Parsed rows have always carried their source.
func cacheEntrySource(e cacheEntry) string {
	if e.Source != "" {
		return e.Source
	}
	if len(e.Rows) > 0 {
		return e.Rows[0].Source
	}
	return ""
}

// dedup keeps one row per DedupKey. Claude Code writes the same response
// several times while streaming — early lines carry partial output_tokens,
// the last line the billed total — so the copy with the largest usage is
// authoritative. betterRow is a strict total order, making the result (and
// every downstream number) independent of input order; the survivor keeps
// the group's earliest timestamp so bucketing is stable too.
func dedup(rows []usage.Row) ([]usage.Row, int) {
	idx := make(map[uint64]int, len(rows))
	out := rows[:0]
	dropped := 0
	for i := range rows {
		r := rows[i]
		if r.DedupKey == 0 {
			out = append(out, r)
			continue
		}
		j, ok := idx[r.DedupKey]
		if !ok {
			idx[r.DedupKey] = len(out)
			out = append(out, r)
			continue
		}
		dropped++
		if betterRow(&r, &out[j]) {
			if out[j].Time.Before(r.Time) {
				r.Time = out[j].Time
			}
			out[j] = r
		} else if r.Time.Before(out[j].Time) {
			out[j].Time = r.Time
		}
	}
	return out, dropped
}

// betterRow reports whether a beats b as the surviving duplicate: larger
// usage first (the final streaming write), then fixed field comparisons so
// no tie ever falls back to encounter order.
func betterRow(a, b *usage.Row) bool {
	at := a.Input + a.CacheRead + a.CacheW5m + a.CacheW1h + a.Output
	bt := b.Input + b.CacheRead + b.CacheW5m + b.CacheW1h + b.Output
	if at != bt {
		return at > bt
	}
	if a.Output != b.Output {
		return a.Output > b.Output
	}
	if a.CacheRead != b.CacheRead {
		return a.CacheRead > b.CacheRead
	}
	if a.CacheW5m != b.CacheW5m {
		return a.CacheW5m > b.CacheW5m
	}
	if a.Input != b.Input {
		return a.Input > b.Input
	}
	if a.Reasoning != b.Reasoning {
		return a.Reasoning > b.Reasoning
	}
	if !a.Time.Equal(b.Time) {
		return a.Time.Before(b.Time)
	}
	if a.Session != b.Session {
		return a.Session < b.Session
	}
	if a.Project != b.Project {
		return a.Project < b.Project
	}
	return a.Model < b.Model
}

func discover(opts Options) ([]sourceFile, error) {
	var files []sourceFile
	// Claude Code: projects/<flattened-cwd>/<session>.jsonl plus nested
	// subagent transcripts (<session>/subagents/agent-*.jsonl) — walk it all;
	// global dedup handles any usage repeated across parent and subagent files.
	for _, root := range opts.ClaudeDirs {
		if err := walkJSONL(filepath.Join(expandHome(root), "projects"), usage.SourceClaude, &files); err != nil {
			return nil, err
		}
	}
	// Codex moves completed rollouts from the date-partitioned sessions tree
	// into archived_sessions. Scan both. A rollout can briefly exist in both
	// places while it is being moved, so its UUID-bearing filename is the
	// stable identity; prefer the live sessions copy encountered first.
	var codexFiles []sourceFile
	for _, root := range opts.CodexDirs {
		root = expandHome(root)
		for _, dir := range []string{"sessions", "archived_sessions"} {
			if err := walkJSONL(filepath.Join(root, dir), usage.SourceCodex, &codexFiles); err != nil {
				return nil, err
			}
		}
	}
	seenRollouts := make(map[string]struct{}, len(codexFiles))
	for _, f := range codexFiles {
		key := codexRolloutKey(f.path)
		if _, seen := seenRollouts[key]; seen {
			continue
		}
		seenRollouts[key] = struct{}{}
		files = append(files, f)
	}
	return files, nil
}

func codexRolloutKey(path string) string {
	return filepath.Base(path)
}

func walkJSONL(dir, source string, files *[]sourceFile) error {
	if _, err := os.Stat(dir); err != nil {
		return nil // source not installed on this machine
	}
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			*files = append(*files, sourceFile{path, source})
		}
		return nil
	})
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func loadCache(path string) map[string]cacheEntry {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var c cacheFile
	if gob.NewDecoder(f).Decode(&c) != nil || c.Version != cacheVersion {
		return nil
	}
	return c.Files
}

func saveCache(path string, c cacheFile) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	if gob.NewEncoder(f).Encode(c) != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	f.Close()
	os.Rename(tmp, path)
}
