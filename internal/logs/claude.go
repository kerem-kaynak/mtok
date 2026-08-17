package logs

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"os"
	"path/filepath"
	"time"

	"github.com/kerem-kaynak/mtok/internal/usage"
)

// claudeLine covers only the fields mtok needs from a Claude Code session
// log line (~/.claude/projects/<flattened-cwd>/<session-uuid>.jsonl).
type claudeLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"requestId"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int64  `json:"input_tokens"`
			CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
			OutputTokens             int64  `json:"output_tokens"`
			Speed                    string `json:"speed"` // "fast" bills at 2x
			CacheCreation            *struct {
				Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"`
				Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
			OutputTokensDetails *struct {
				ThinkingTokens int64 `json:"thinking_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	} `json:"message"`
}

var (
	claudeTypeAssistant = []byte(`"type":"assistant"`)
	claudeUsageKey      = []byte(`"usage"`)
)

// parseClaudeFile extracts one row per assistant usage line.
//
// Streaming writes several lines for the same API response (same message.id +
// requestId, identical usage), and resumed sessions copy history into the new
// file — so every row carries a DedupKey and the scanner dedups globally.
// "<synthetic>" models are Claude Code error placeholders, not API calls.
func parseClaudeFile(path string) ([]usage.Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fallbackSession := sessionFromFilename(path)
	var rows []usage.Row
	err = forEachLine(f, func(line []byte) {
		if !bytes.Contains(line, claudeTypeAssistant) || !bytes.Contains(line, claudeUsageKey) {
			return
		}
		var l claudeLine
		if json.Unmarshal(line, &l) != nil {
			return // tolerate torn/foreign lines
		}
		u := l.Message.Usage
		if l.Type != "assistant" || u == nil || l.Message.Model == "" || l.Message.Model == "<synthetic>" {
			return
		}
		t, terr := time.Parse(time.RFC3339, l.Timestamp)
		if terr != nil {
			return
		}
		r := usage.Row{
			Time:      t,
			Source:    usage.SourceClaude,
			Model:     l.Message.Model,
			Project:   l.Cwd,
			Session:   l.SessionID,
			Input:     u.InputTokens,
			CacheRead: u.CacheReadInputTokens,
			Output:    u.OutputTokens,
			Fast:      u.Speed == "fast",
		}
		if d := u.OutputTokensDetails; d != nil {
			r.Reasoning = d.ThinkingTokens
		}
		if r.Session == "" {
			r.Session = fallbackSession
		}
		if cc := u.CacheCreation; cc != nil && (cc.Ephemeral5m > 0 || cc.Ephemeral1h > 0) {
			r.CacheW5m, r.CacheW1h = cc.Ephemeral5m, cc.Ephemeral1h
		} else {
			r.CacheW5m = u.CacheCreationInputTokens // no TTL breakdown: assume 5m
		}
		if l.Message.ID != "" && l.RequestID != "" {
			h := fnv.New64a()
			h.Write([]byte(l.Message.ID))
			h.Write([]byte{'|'})
			h.Write([]byte(l.RequestID))
			r.DedupKey = h.Sum64()
		}
		rows = append(rows, r)
	})
	return rows, err
}

func sessionFromFilename(path string) string {
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}
