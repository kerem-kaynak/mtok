package logs

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kerem-kaynak/mtok/internal/usage"
)

const claudeFixture = `{"type":"user","message":{"role":"user"},"timestamp":"2026-08-01T10:00:00Z"}
{"type":"assistant","timestamp":"2026-08-01T10:00:05Z","sessionId":"s1","cwd":"/tmp/proj","requestId":"req_1","message":{"id":"msg_1","model":"claude-fable-5","usage":{"input_tokens":100,"cache_creation_input_tokens":50,"cache_read_input_tokens":1000,"output_tokens":3,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":20}}}}
{"type":"assistant","timestamp":"2026-08-01T10:00:06Z","sessionId":"s1","cwd":"/tmp/proj","requestId":"req_1","message":{"id":"msg_1","model":"claude-fable-5","usage":{"input_tokens":100,"cache_creation_input_tokens":50,"cache_read_input_tokens":1000,"output_tokens":20,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":20}}}}
{"type":"assistant","timestamp":"2026-08-01T10:01:00Z","sessionId":"s1","cwd":"/tmp/proj","requestId":"req_2","message":{"id":"msg_2","model":"<synthetic>","usage":{"input_tokens":1,"output_tokens":1}}}
{"type":"assistant","timestamp":"2026-08-01T10:02:00Z","sessionId":"s1","cwd":"/tmp/proj","requestId":"req_3","message":{"id":"msg_3","model":"claude-opus-5","usage":{"input_tokens":10,"cache_creation_input_tokens":5,"cache_read_input_tokens":0,"output_tokens":7,"speed":"fast","output_tokens_details":{"thinking_tokens":4}}}}
`

func TestParseClaudeFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sess.jsonl")
	if err := os.WriteFile(p, []byte(claudeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := parseClaudeFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Parser keeps both streaming copies; scan-level dedup collapses them.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (synthetic excluded)", len(rows))
	}
	r := rows[0]
	if r.Input != 100 || r.CacheRead != 1000 || r.CacheW5m != 30 || r.CacheW1h != 20 || r.Output != 3 {
		t.Errorf("unexpected tokens: %+v", r)
	}
	if r.DedupKey == 0 || r.DedupKey != rows[1].DedupKey {
		t.Error("streaming copies must share a dedup key")
	}
	if rows[2].CacheW5m != 5 {
		t.Errorf("no-breakdown cache write should default to 5m tier, got %+v", rows[2])
	}
	if !rows[2].Fast || rows[2].Reasoning != 4 {
		t.Errorf("speed/thinking not parsed: %+v", rows[2])
	}
	if rows[0].Fast || rows[0].Reasoning != 0 {
		t.Errorf("row without speed/details must not be fast: %+v", rows[0])
	}
	// Dedup must keep the final streaming write (largest usage), stamped
	// with the earliest copy's timestamp — regardless of input order.
	for name, in := range map[string][]usage.Row{
		"file order":    {rows[0], rows[1], rows[2]},
		"reverse order": {rows[2], rows[1], rows[0]},
	} {
		deduped, dropped := dedup(slices.Clone(in))
		if len(deduped) != 2 || dropped != 1 {
			t.Fatalf("%s: dedup got %d rows / %d dropped, want 2 / 1", name, len(deduped), dropped)
		}
		for _, dr := range deduped {
			if dr.DedupKey != r.DedupKey {
				continue
			}
			if dr.Output != 20 {
				t.Errorf("%s: kept partial streaming copy (output %d, want 20)", name, dr.Output)
			}
			if !dr.Time.Equal(rows[0].Time) {
				t.Errorf("%s: survivor time %s, want earliest %s", name, dr.Time, rows[0].Time)
			}
		}
	}
}

// Deleting a scanned file must not move numbers backwards: its rows persist
// from the cache (session logs have TTLs; external tools delete probe logs).
func TestScanRetainsDeletedFiles(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "projects", "p1")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(proj, "sess.jsonl")
	if err := os.WriteFile(file, []byte(claudeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "cache.gob")
	opts := Options{ClaudeDirs: []string{root}, CodexDirs: []string{filepath.Join(root, "nope")}, CacheFile: cache}

	first, err := Scan(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) == 0 || first.Retained != 0 {
		t.Fatalf("first scan: %d rows, %d retained", len(first.Rows), first.Retained)
	}

	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 3; i++ { // survives repeated scans via cache carry-over
		res, err := Scan(opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Rows) != len(first.Rows) || res.Retained != 1 {
			t.Fatalf("scan %d after delete: %d rows (want %d), %d retained (want 1)", i, len(res.Rows), len(first.Rows), res.Retained)
		}
		if len(res.Errors) != 0 {
			t.Fatalf("scan %d errors: %v", i, res.Errors)
		}
	}
}

const codexFixture = `{"timestamp":"2026-08-02T09:00:00Z","type":"session_meta","payload":{"id":"c1","session_id":"c1","cwd":"/tmp/cdx","model_provider":"azure","cli_version":"0.146.1"}}
{"timestamp":"2026-08-02T09:00:01Z","type":"turn_context","payload":{"model":"gpt-5.6-sol"}}
{"timestamp":"2026-08-02T09:00:10Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":700,"output_tokens":50,"reasoning_output_tokens":10,"total_tokens":1050}}}}
{"timestamp":"2026-08-02T09:00:20Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":3000,"cached_input_tokens":1500,"cache_write_input_tokens":900,"output_tokens":150,"reasoning_output_tokens":40,"total_tokens":3150}}}}
{"timestamp":"2026-08-02T09:00:30Z","type":"event_msg","payload":{"type":"token_count","info":null}}
{"timestamp":"2026-08-02T09:00:40Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_settings":{"model":"gpt-5.6-terra","service_tier":"default"}}}
{"timestamp":"2026-08-02T09:00:50Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":4000,"cached_input_tokens":2000,"cache_write_input_tokens":1000,"output_tokens":250,"reasoning_output_tokens":60,"total_tokens":4250}}}}
`

func TestParseCodexFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rollout-2026-08-02T09-00-00-c1.jsonl")
	if err := os.WriteFile(p, []byte(codexFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := parseCodexFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	r0 := rows[0]
	// input 1000 includes cached 200 + writes 700 → fresh input 100.
	if r0.Input != 100 || r0.CacheRead != 200 || r0.CacheW5m != 700 || r0.Output != 50 || r0.Reasoning != 10 {
		t.Errorf("row0 tokens wrong: %+v", r0)
	}
	r1 := rows[1]
	// deltas: input 2000 (cached 1300, writes 200) → fresh 500; output 100.
	if r1.Input != 500 || r1.CacheRead != 1300 || r1.CacheW5m != 200 || r1.Output != 100 || r1.Reasoning != 30 {
		t.Errorf("row1 deltas wrong: %+v", r1)
	}
	if r0.Model != "gpt-5.6-sol" || r0.Provider != "azure" || r0.Project != "/tmp/cdx" || r0.Session != "c1" {
		t.Errorf("row0 metadata wrong: %+v", r0)
	}
	if r0.Source != usage.SourceCodex {
		t.Errorf("source = %q", r0.Source)
	}
	// thread_settings_applied switches the active model mid-session.
	r2 := rows[2]
	if r2.Model != "gpt-5.6-terra" {
		t.Errorf("model switch not applied: %+v", r2)
	}
	if r2.Input != 400 || r2.CacheRead != 500 || r2.CacheW5m != 100 || r2.Output != 100 || r2.Reasoning != 20 {
		t.Errorf("row2 deltas wrong: %+v", r2)
	}
}
