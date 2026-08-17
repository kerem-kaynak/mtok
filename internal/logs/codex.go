package logs

import (
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/kerem-kaynak/mtok/internal/usage"
)

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexMeta struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	ModelProvider string `json:"model_provider"`
}

type codexTurnContext struct {
	Model             string `json:"model"`
	CollaborationMode struct {
		Settings struct {
			Model string `json:"model"`
		} `json:"settings"`
	} `json:"collaboration_mode"`
}

type codexTokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

type codexTokenCount struct {
	Type string `json:"type"`
	Info *struct {
		Total *codexTokenUsage `json:"total_token_usage"`
		Last  *codexTokenUsage `json:"last_token_usage"`
	} `json:"info"`
	// thread_settings_applied events share the event_msg envelope and can
	// switch the active model mid-session before the next turn_context.
	ThreadSettings *struct {
		Model string `json:"model"`
	} `json:"thread_settings"`
}

var (
	codexTokenCountKey     = []byte(`"token_count"`)
	codexThreadSettingsKey = []byte(`"thread_settings_applied"`)
)

// parseCodexFile extracts one row per token_count event from a Codex rollout
// (~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl).
//
// token_count events carry a cumulative total_token_usage; rows are the
// per-event delta, clamped at zero per field. Clamping also absorbs the one
// case where Codex clobbers the running total (on ContextWindowExceeded it
// rewrites total_token_usage as a synthetic value with zeroed components):
// the clobber event yields an empty delta and counting resumes cleanly from
// the synthetic baseline. Compaction does NOT reset the total, so deltas stay
// exact across compactions. The active model comes from the most recent
// turn_context or thread_settings_applied event — session_meta doesn't
// record one. Codex input_tokens includes cached and cache-write tokens
// (both are subsets); they are split out here so Row fields never overlap.
func parseCodexFile(path string) ([]usage.Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		rows     []usage.Row
		session  = sessionFromFilename(path)
		provider string
		project  string
		model    string
		prev     codexTokenUsage
	)
	err = forEachLine(f, func(line []byte) {
		var l codexLine
		if json.Unmarshal(line, &l) != nil {
			return
		}
		switch l.Type {
		case "session_meta":
			var m codexMeta
			if json.Unmarshal(l.Payload, &m) == nil {
				provider, project = m.ModelProvider, m.Cwd
				if m.SessionID != "" {
					session = m.SessionID
				} else if m.ID != "" {
					session = m.ID
				}
			}
		case "turn_context":
			var tc codexTurnContext
			if json.Unmarshal(l.Payload, &tc) == nil {
				if tc.Model != "" {
					model = tc.Model
				} else if tc.CollaborationMode.Settings.Model != "" {
					model = tc.CollaborationMode.Settings.Model
				}
			}
		case "event_msg":
			if !bytes.Contains(l.Payload, codexTokenCountKey) && !bytes.Contains(l.Payload, codexThreadSettingsKey) {
				return
			}
			var tcnt codexTokenCount
			if json.Unmarshal(l.Payload, &tcnt) != nil {
				return
			}
			if tcnt.Type == "thread_settings_applied" {
				if ts := tcnt.ThreadSettings; ts != nil && ts.Model != "" {
					model = ts.Model
				}
				return
			}
			if tcnt.Type != "token_count" || tcnt.Info == nil {
				return
			}
			var d codexTokenUsage
			switch {
			case tcnt.Info.Total != nil:
				d = deltaUsage(*tcnt.Info.Total, prev)
				prev = *tcnt.Info.Total
			case tcnt.Info.Last != nil:
				d = *tcnt.Info.Last
			default:
				return
			}
			if d == (codexTokenUsage{}) {
				return
			}
			t, terr := time.Parse(time.RFC3339, l.Timestamp)
			if terr != nil {
				return
			}
			m := model
			if m == "" {
				m = "unknown"
			}
			rows = append(rows, usage.Row{
				Time:      t,
				Source:    usage.SourceCodex,
				Provider:  provider,
				Model:     m,
				Project:   project,
				Session:   session,
				Input:     clamp0(d.InputTokens - d.CachedInputTokens - d.CacheWriteInputTokens),
				CacheRead: d.CachedInputTokens,
				CacheW5m:  d.CacheWriteInputTokens,
				Output:    d.OutputTokens,
				Reasoning: d.ReasoningOutputTokens,
			})
		}
	})
	return rows, err
}

func deltaUsage(cur, prev codexTokenUsage) codexTokenUsage {
	return codexTokenUsage{
		InputTokens:           clamp0(cur.InputTokens - prev.InputTokens),
		CachedInputTokens:     clamp0(cur.CachedInputTokens - prev.CachedInputTokens),
		CacheWriteInputTokens: clamp0(cur.CacheWriteInputTokens - prev.CacheWriteInputTokens),
		OutputTokens:          clamp0(cur.OutputTokens - prev.OutputTokens),
		ReasoningOutputTokens: clamp0(cur.ReasoningOutputTokens - prev.ReasoningOutputTokens),
	}
}

func clamp0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
