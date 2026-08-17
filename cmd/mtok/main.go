// mtok — a terminal dashboard for AI token usage and spend, computed entirely
// from local Claude Code and Codex session logs. No network calls.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kerem-kaynak/mtok/internal/pricing"
	"github.com/kerem-kaynak/mtok/internal/tui"
)

const version = "0.3.0"

type multiFlag []string

func (m *multiFlag) String() string     { return fmt.Sprint(*m) }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	var (
		claudeDirs  multiFlag
		codexDirs   multiFlag
		summary     = flag.Bool("summary", false, "print a plain-text report instead of the TUI")
		noCache     = flag.Bool("no-cache", false, "reparse all files, ignoring the parse cache")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Var(&claudeDirs, "claude-dir", "Claude Code data dir (repeatable; default ~/.claude)")
	flag.Var(&codexDirs, "codex-dir", "Codex data dir (repeatable; default ~/.codex)")
	flag.Parse()

	if *showVersion {
		fmt.Println("mtok " + version)
		return
	}

	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "mtok", "config.json")
	cfg, err := pricing.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mtok: bad config %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	table := pricing.Defaults()
	table.Apply(cfg.Pricing)

	if len(claudeDirs) == 0 {
		claudeDirs = cfg.ClaudeDirs
	}
	if len(claudeDirs) == 0 {
		claudeDirs = []string{filepath.Join(home, ".claude")}
	}
	if len(codexDirs) == 0 {
		codexDirs = cfg.CodexDirs
	}
	if len(codexDirs) == 0 {
		codexDirs = []string{filepath.Join(home, ".codex")}
	}

	cacheFile := filepath.Join(home, ".cache", "mtok", "scan.gob")
	if *noCache {
		cacheFile = ""
	}

	opts := tui.Options{
		ClaudeDirs: claudeDirs,
		CodexDirs:  codexDirs,
		CacheFile:  cacheFile,
		Table:      table,
		Version:    version,
	}

	if *summary {
		err = tui.Summary(opts)
	} else {
		err = tui.Run(opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mtok:", err)
		os.Exit(1)
	}
}
