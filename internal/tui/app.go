// Package tui is the mtok terminal UI.
package tui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kerem-kaynak/mtok/internal/logs"
	"github.com/kerem-kaynak/mtok/internal/pricing"
)

// Options wires the app to its data sources.
type Options struct {
	ClaudeDirs []string
	CodexDirs  []string
	CacheFile  string
	Table      pricing.Table
	Version    string
}

type view interface {
	setSize(w, h int)
	setData(d *data)
	update(msg tea.Msg) tea.Cmd
	render() string
}

type scanDoneMsg struct {
	res *logs.Result
	err error
}

type tickMsg time.Time

var tabNames = []string{"Dashboard", "Daily", "Monthly", "Models", "Projects", "Sessions"}

type app struct {
	opts    Options
	spin    spinner.Model
	loading bool
	err     error
	d       *data
	tab     int
	views   []view
	w, h    int

	progDone, progTotal *atomic.Int64
}

// New creates the root Bubble Tea model.
func New(opts Options) tea.Model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(sAccent))
	return &app{
		opts:      opts,
		spin:      sp,
		loading:   true,
		views:     []view{&dashboardView{}, &dailyView{}, &monthlyView{}, &modelsView{}, &projectsView{}, &sessionsView{}},
		progDone:  &atomic.Int64{},
		progTotal: &atomic.Int64{},
	}
}

// Run starts the TUI.
func Run(opts Options) error {
	_, err := tea.NewProgram(New(opts), tea.WithAltScreen()).Run()
	return err
}

func (a *app) scanCmd() tea.Cmd {
	done, total := a.progDone, a.progTotal
	o := logs.Options{
		ClaudeDirs: a.opts.ClaudeDirs,
		CodexDirs:  a.opts.CodexDirs,
		CacheFile:  a.opts.CacheFile,
		Progress: func(d, t int64) {
			done.Store(d)
			total.Store(t)
		},
	}
	return func() tea.Msg {
		res, err := logs.Scan(o)
		return scanDoneMsg{res, err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a *app) Init() tea.Cmd {
	return tea.Batch(a.spin.Tick, a.scanCmd(), tick())
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		for _, v := range a.views {
			v.setSize(a.contentSize())
		}
		return a, nil

	case scanDoneMsg:
		a.loading = false
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.err = nil
		a.d = newData(msg.res, a.opts.Table, time.Now())
		for _, v := range a.views {
			v.setData(a.d)
			v.setSize(a.contentSize())
		}
		return a, nil

	case tickMsg:
		if a.loading {
			return a, tick()
		}
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spin, cmd = a.spin.Update(msg)
		if a.loading {
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "r":
			if !a.loading {
				a.loading = true
				a.progDone.Store(0)
				return a, tea.Batch(a.spin.Tick, a.scanCmd(), tick())
			}
			return a, nil
		case "t":
			if a.d != nil {
				a.d.tokens = !a.d.tokens
				for _, v := range a.views {
					v.setData(a.d) // re-layout with the new metric
				}
			}
			return a, nil
		case "tab", "right", "l":
			a.tab = (a.tab + 1) % len(tabNames)
			return a, nil
		case "shift+tab", "left", "h":
			a.tab = (a.tab + len(tabNames) - 1) % len(tabNames)
			return a, nil
		case "1", "2", "3", "4", "5", "6":
			a.tab = int(msg.String()[0] - '1')
			return a, nil
		}
	}
	return a, a.views[a.tab].update(msg)
}

func (a *app) contentSize() (int, int) {
	return a.w - 2, a.h - 5 // header, tabs, footer
}

func (a *app) View() string {
	if a.w == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(a.header())
	b.WriteString("\n")
	b.WriteString(a.tabsRow())
	b.WriteString("\n")

	_, ch := a.contentSize()
	var content string
	switch {
	case a.err != nil:
		content = sMuted.Render("error: " + a.err.Error())
	case a.loading && a.d == nil:
		content = fmt.Sprintf("%s scanning session logs… %d/%d files",
			a.spin.View(), a.progDone.Load(), a.progTotal.Load())
	default:
		content = a.views[a.tab].render()
	}
	content = lipgloss.NewStyle().Padding(0, 1).Render(content)
	if n := ch - lipgloss.Height(content); n > 0 {
		content += strings.Repeat("\n", n)
	}
	b.WriteString(content)
	b.WriteString("\n")
	b.WriteString(a.footer())
	// Belt and braces: never let any line exceed the terminal width — a
	// single overflowing line wraps and destroys the whole layout.
	return lipgloss.NewStyle().MaxWidth(a.w).Render(b.String())
}

func (a *app) header() string {
	left := " " + sBrand.Render("mtok") + sMuted.Render(" · local AI usage & spend")
	right := sMuted.Render("est. API list prices ")
	if a.loading && a.d != nil {
		right = a.spin.View() + sMuted.Render(fmt.Sprintf(" rescanning %d/%d ", a.progDone.Load(), a.progTotal.Load()))
	}
	gap := a.w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (a *app) tabsRow() string {
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if i == a.tab {
			tabs = append(tabs, sTabActive.Render(label))
		} else {
			tabs = append(tabs, sTab.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
}

func (a *app) footer() string {
	stats := ""
	if a.d != nil {
		stats = fmt.Sprintf("%d files · %s rows · %s deduped   ",
			a.d.res.Files, comma(int64(len(a.d.res.Rows))), comma(int64(a.d.res.Duplicates)))
	}
	keys := " 1-6 views · tab switch · ↑↓ select · t $/tok · r rescan · q quit"
	gap := a.w - lipgloss.Width(keys) - lipgloss.Width(stats)
	if gap < 1 {
		stats = "" // stats are secondary; keep the key hints readable
		gap = maxInt(a.w-lipgloss.Width(keys), 1)
	}
	return sFooter.Render(keys + strings.Repeat(" ", gap) + stats)
}
