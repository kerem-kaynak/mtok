package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kerem-kaynak/mtok/internal/logs"
	"github.com/kerem-kaynak/mtok/internal/pricing"
	"github.com/kerem-kaynak/mtok/internal/usage"
)

// fixtureResult builds a small multi-day, multi-model, multi-source result so
// every view has something to draw (charts, share bars, tables, detail panes).
func fixtureResult() *logs.Result {
	base := time.Now().AddDate(0, 0, -40)
	var rows []usage.Row
	models := []struct {
		src, model, proj, sess string
	}{
		{usage.SourceClaude, "claude-fable-5", "/Users/x/proj-a", "sa"},
		{usage.SourceClaude, "claude-opus-5", "/Users/x/proj-a", "sb"},
		{usage.SourceCodex, "gpt-5.6-sol", "/Users/x/proj-b", "sc"},
		{usage.SourceCodex, "gpt-5.5", "/Users/x/proj-c", "sd"},
		{usage.SourceClaude, "unknown-model-xyz", "/Users/x/proj-c", "se"},
	}
	for day := 0; day < 41; day++ {
		m := models[day%len(models)]
		rows = append(rows, usage.Row{
			Time: base.AddDate(0, 0, day).Add(3 * time.Hour), Source: m.src,
			Model: m.model, Project: m.proj, Session: m.sess,
			Input: int64(1000 * (day + 1)), CacheRead: int64(50000 * day),
			CacheW5m: 2000, Output: int64(300 * (day + 1)), Reasoning: 40,
		})
	}
	return &logs.Result{Rows: rows, Files: 5, FromCache: 2, Duplicates: 3}
}

func drainCmd(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	// Only follow synchronous messages we care about; skip ticks/batches.
	if cmd == nil {
		return m
	}
	return m
}

func TestAppRendersAllTabsAtAllSizes(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 120, Height: 40},
		{Width: 80, Height: 24},
		{Width: 60, Height: 20},
	}
	for _, sz := range sizes {
		m := New(Options{Table: pricing.Defaults(), Version: "test"})
		var cmd tea.Cmd
		var mm tea.Model = m
		mm, cmd = mm.Update(sz)
		mm = drainCmd(t, mm, cmd)

		// Loading state should render without data.
		if v := mm.View(); v == "" {
			t.Fatalf("%dx%d: loading view empty", sz.Width, sz.Height)
		}

		mm, _ = mm.Update(scanDoneMsg{res: fixtureResult()})
		// Both metric modes must render cleanly on every tab.
		mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
		mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
		for tab := 0; tab < len(tabNames); tab++ {
			key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + tab)}}
			mm, _ = mm.Update(key)
			v := mm.View()
			if v == "" {
				t.Fatalf("%dx%d tab %s: empty view", sz.Width, sz.Height, tabNames[tab])
			}
			if strings.Contains(v, "PANIC") {
				t.Fatalf("%dx%d tab %s: rendered panic", sz.Width, sz.Height, tabNames[tab])
			}
			// No line may exceed the terminal width, or it wraps and
			// destroys the layout.
			for ln, line := range strings.Split(v, "\n") {
				if w := lipgloss.Width(line); w > sz.Width {
					t.Errorf("%dx%d tab %s line %d: width %d > %d: %q",
						sz.Width, sz.Height, tabNames[tab], ln, w, sz.Width, line)
				}
			}
			// Cursor movement and the metric toggle must not panic either.
			for _, k := range []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyDown}, {Type: tea.KeyUp},
				{Type: tea.KeyRunes, Runes: []rune{'t'}}} {
				mm, _ = mm.Update(k)
			}
			mm.View()
			mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
		}
	}
}

func TestAppRendersEmptyResult(t *testing.T) {
	var mm tea.Model = New(Options{Table: pricing.Defaults(), Version: "test"})
	mm, _ = mm.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mm, _ = mm.Update(scanDoneMsg{res: &logs.Result{}})
	for tab := 0; tab < len(tabNames); tab++ {
		mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + tab)}})
		if mm.View() == "" {
			t.Fatalf("tab %s: empty view with no data", tabNames[tab])
		}
	}
}

func TestChartsPrimitives(t *testing.T) {
	if s := sparkline([]float64{0, 1, 2, 3, 4}, 5); len([]rune(s)) != 5 {
		t.Errorf("sparkline width: %q", s)
	}
	if barChart(nil, nil, 40, 8, moneyAxis) == "" {
		t.Error("empty barChart should render a placeholder, not empty string")
	}
	b := barChart([]float64{1, 5, 3}, []string{"a", "b", "c"}, 40, 8, moneyAxis)
	if b == "" || !strings.Contains(b, "┴") {
		t.Errorf("barChart missing baseline: %q", b)
	}
	segs := []segment{{"a", 5, slotColor(0)}, {"b", 3, slotColor(1)}, {"c", 0, slotColor(2)}}
	if shareBar(segs, 20) == "" {
		t.Error("shareBar empty")
	}
	if meter(0.42, 20) == "" {
		t.Error("meter empty")
	}
}
